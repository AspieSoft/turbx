package compiler

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AspieSoft/go-liveread"
	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
	"github.com/alphadose/haxmap"
	"github.com/andybalholm/brotli"
	"github.com/bep/golibsass/libsass"
	"github.com/kib357/less-go"
	"github.com/tdewolff/minify/v2/minify"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type Config struct {
	// Root dir for html files
	// example: "~/MySiteDir/views"
	// default: "./views"
	Root string

	// File Extention to use for html files (without the dot)
	// example: "html"
	// default: "html"
	Ext string

	// A public/static path for js, css, and other static files
	// example: "~/MySiteDir/public"
	// default: "./public"
	Static string

	// The url where public/static files will be served from (js, css, etc)
	// example: "/cdn/assets"
	// default: "/"
	StaticUrl string

	// A dir path to store static html files, for when the precompiler produces static html
	StaticHTML string

	// A dir path to cache dynamic html files, for when the precompiler produces dynamically changing content
	CacheDir string

	// Brotli compression level for precompressed static files (0-11)
	//
	// Gzip will be set to this number capped between 1-9 (if val > 6 {val -= 1})
	PreCompress int

	// Brotli compression level for live compressed files (0-11)
	//
	// Gzip will be set to this number capped between 1-9 (if val > 6 {val -= 1})
	Compress int

	gzipPreCompress int
	gzipCompress    int

	// The maximum size of the compiled output before flushing it to the result writer
	CompileMaxFlush uint

	// Cache Time In Minutes
	CacheTime int

	// Weather or not to include .md files along with the default Ext value.
	//
	// turbx will compile markdown regardless of whether or not it is in a .md file.
	IncludeMD bool

	// A folder level to consider a root domain, to prevent use of components outside a specific root folder
	DomainFolder uint

	// Debug Mode For Developers
	DebugMode bool
}

type cacheObj struct {
	cachePath []string
	static    bool
	accessed  int
}

var compilerConfig Config

var htmlPreCache *haxmap.Map[string, cacheObj] = haxmap.New[string, cacheObj]()
var htmlCacheDel *haxmap.Map[string, int] = haxmap.New[string, int]()

var staticChangeQueue *haxmap.Map[string, int64] = haxmap.New[string, int64]()

var cacheWatcher *goutil.FileWatcher
var staticWatcher *goutil.FileWatcher

func SetConfig(config Config) error {
	if config.Root != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}

		cacheWatcher.WatchDir(path)
		cacheWatcher.CloseWatcher(compilerConfig.Root)
		compilerConfig.Root = path
	}

	rootDir := string(regex.Comp(`\/[\w_\-\.]+\/?$`).RepStr([]byte(compilerConfig.Root), []byte{}))

	staticWatcher.CloseWatcher(compilerConfig.Static)
	if config.Static != "" {
		if path, err := filepath.Abs(config.Static); err == nil {
			compilerConfig.Static = path
		} else if path, err := goutil.FS.JoinPath(rootDir, "public"); err == nil {
			compilerConfig.Static = path
		}
	} else if path, err := goutil.FS.JoinPath(rootDir, "public"); err == nil {
		compilerConfig.Static = path
	}
	staticWatcher.WatchDir(compilerConfig.Static)

	if config.StaticHTML != "" {
		if path, err := filepath.Abs(config.StaticHTML); err == nil {
			compilerConfig.StaticHTML = path
		} else if path, err := goutil.FS.JoinPath(rootDir, "html.static"); err == nil {
			compilerConfig.StaticHTML = path
		}
	} else if path, err := goutil.FS.JoinPath(rootDir, "html.static"); err == nil {
		compilerConfig.StaticHTML = path
	}

	if config.CacheDir != "" {
		if path, err := filepath.Abs(config.CacheDir); err == nil {
			compilerConfig.CacheDir = path
		} else if path, err := goutil.FS.JoinPath(rootDir, "html.cache"); err == nil {
			compilerConfig.CacheDir = path
		}
	} else if path, err := goutil.FS.JoinPath(rootDir, "html.cache"); err == nil {
		compilerConfig.CacheDir = path
	}

	if config.Ext != "" {
		if strings.HasPrefix(config.Ext, ".") {
			config.Ext = config.Ext[1:]
		}
		compilerConfig.Ext = config.Ext
	}

	if config.StaticUrl != "" {
		if strings.HasSuffix(config.StaticUrl, "/") {
			config.StaticUrl = config.StaticUrl[:len(config.StaticUrl)-1]
		}
		compilerConfig.StaticUrl = config.StaticUrl
	}

	if config.PreCompress != 0 {
		if config.PreCompress < 0 {
			config.PreCompress = 0
		} else if config.PreCompress > 11 {
			config.PreCompress = 11
		}
		compilerConfig.PreCompress = config.PreCompress

		c := config.PreCompress
		if c > 6 {
			c--
		}
		if c < 1 {
			c = 1
		} else if c > 9 {
			c = 9
		}
		compilerConfig.gzipPreCompress = c
	}

	if config.Compress != 0 {
		if config.Compress < 0 {
			config.Compress = 0
		} else if config.Compress > 11 {
			config.Compress = 11
		}
		compilerConfig.Compress = config.Compress

		c := config.Compress
		if c > 6 {
			c--
		}
		if c < 1 {
			c = 1
		} else if c > 9 {
			c = 9
		}
		compilerConfig.gzipCompress = c
	}

	if config.CacheTime != 0 {
		if config.CacheTime < 0 {
			config.CacheTime = 0
		}
		compilerConfig.CacheTime = config.CacheTime
	}

	compilerConfig.DebugMode = config.DebugMode

	compilerConfig.CompileMaxFlush = config.CompileMaxFlush

	compilerConfig.DomainFolder = config.DomainFolder

	compilerConfig.IncludeMD = config.IncludeMD

	// ensure directories exist
	InitDefault()

	return nil
}

func InitDefault() {
	// ensure directories exist
	os.MkdirAll(compilerConfig.Root, 0775)
	os.MkdirAll(compilerConfig.Static, 0775)
	os.MkdirAll(compilerConfig.StaticHTML, 0775)
	os.MkdirAll(compilerConfig.CacheDir, 0775)

	// add possible cache files to list
	if files, err := os.ReadDir(compilerConfig.StaticHTML); err == nil {
		for _, file := range files {
			if !file.IsDir() {
				fileName := []byte(file.Name())
				if regex.Comp(`\.(%1)\.html(?:\.br|\.gz|)$`, compilerConfig.Ext).MatchRef(&fileName) {
					fileName = regex.Comp(`\.(%1)\.html(?:\.br|\.gz|)$`, compilerConfig.Ext).RepStrCompRef(&fileName, []byte(".$1"))
					origName := fileName
					fileName = regex.Comp(`\._\.(?!%1$)`, compilerConfig.Ext).RepStrRef(&fileName, []byte{'/'})

					if path, err := goutil.FS.JoinPath(compilerConfig.Root, string(fileName)); err == nil {
						if _, ok := htmlPreCache.Get(path); ok {
							continue
						}

						if staticPath, err := goutil.FS.JoinPath(compilerConfig.StaticHTML, string(origName)); err == nil {
							cachePath := []string{}
							if stat, err := os.Stat(staticPath + ".html.br"); err == nil && !stat.IsDir() {
								cachePath = append(cachePath, staticPath+".html.br")
							}
							if stat, err := os.Stat(staticPath + ".html.gz"); err == nil && !stat.IsDir() {
								cachePath = append(cachePath, staticPath+".html.gz")
							}
							if stat, err := os.Stat(staticPath + ".html"); err == nil && !stat.IsDir() {
								cachePath = append(cachePath, staticPath+".html")
							}

							htmlPreCache.Set(path, cacheObj{
								cachePath: cachePath,
								static:    true,
								accessed:  int(time.Now().UnixMilli() / 60000),
							})
						}
					}
				}
			}
		}
	}

	if files, err := os.ReadDir(compilerConfig.CacheDir); err == nil {
		for _, file := range files {
			if !file.IsDir() {
				fileName := []byte(file.Name())
				if regex.Comp(`\.(%1)\.html\.cache$`, compilerConfig.Ext).MatchRef(&fileName) {
					fileName = regex.Comp(`\.(%1)\.html\.cache$`, compilerConfig.Ext).RepStrCompRef(&fileName, []byte(".$1"))
					origName := fileName
					fileName = regex.Comp(`\._\.(?!%1$)`, compilerConfig.Ext).RepStrRef(&fileName, []byte{'/'})

					if path, err := goutil.FS.JoinPath(compilerConfig.Root, string(fileName)); err == nil {
						if _, ok := htmlPreCache.Get(path); ok {
							continue
						}

						if staticPath, err := goutil.FS.JoinPath(compilerConfig.CacheDir, string(origName)); err == nil {
							cachePath := []string{}
							if stat, err := os.Stat(staticPath + ".html.cache"); err == nil && !stat.IsDir() {
								cachePath = append(cachePath, staticPath+".html.cache")
							}

							if oldCache, ok := htmlPreCache.Get(path); ok {
								for _, file := range oldCache.cachePath {
									if oldCache.static && strings.HasPrefix(file, compilerConfig.StaticHTML) {
										os.Remove(file)
									}
								}
							}

							htmlPreCache.Set(path, cacheObj{
								cachePath: cachePath,
								static:    false,
								accessed:  int(time.Now().UnixMilli() / 60000),
							})
						}
					}
				}
			}
		}
	}

	tryMinifyDir(compilerConfig.Static)
}

type tagData struct {
	tag  []byte
	attr []byte
}

// list of self naturally closing html tags
var singleHtmlTags [][]byte = [][]byte{
	[]byte("br"),
	[]byte("hr"),
	[]byte("wbr"),
	[]byte("meta"),
	[]byte("link"),
	[]byte("param"),
	[]byte("base"),
	[]byte("input"),
	[]byte("img"),
	[]byte("area"),
	[]byte("col"),
	[]byte("command"),
	[]byte("embed"),
	[]byte("keygen"),
	[]byte("source"),
	[]byte("track"),
}

// @tag: tag to detect
// @attr: required attr to consider
var emptyContentTags []tagData = []tagData{
	{[]byte("script"), []byte("src")},
	{[]byte("iframe"), nil},
}

//todo: add more image, video, and audio file types

// regex selectors for image, video, and audio files
var imageRE *regex.Regexp = regex.Comp(`\.(png|jpe?g)$`)
var videoRE *regex.Regexp = regex.Comp(`\.(mp4|mov)$`)
var audioRE *regex.Regexp = regex.Comp(`\.(mp3|wav|ogg)$`)

// regex to determine if a comment should be kept (for copyright or internet explore support)
var keepCommentRE *regex.Regexp = regex.Comp(`(?i)(^\s*(?:\!|\([cr]\))|^\s*\[?[\w_\-\s]+]?\s*>.*<!\s*\[?[\w_\-\s]+\]?\s*$)`)

var runningCompiler bool = true

func init() {
	root, err := filepath.Abs("views")
	if err != nil {
		root = "views"
	}

	static, err := filepath.Abs("public")
	if err != nil {
		static = "public"
	}

	staticHTML, err := filepath.Abs("html.static")
	if err != nil {
		staticHTML = "html.static"
	}

	cacheDir, err := filepath.Abs("html.cache")
	if err != nil {
		cacheDir = "html.cache"
	}

	cacheWatcher = goutil.FS.FileWatcher()
	cacheWatcher.OnFileChange = func(path, op string) {
		if data, ok := htmlPreCache.Get(path); ok {
			htmlPreCache.Del(path)
			for _, file := range data.cachePath {
				if (data.static && strings.HasPrefix(file, compilerConfig.StaticHTML)) || (!data.static && strings.HasPrefix(file, compilerConfig.CacheDir)) {
					os.Remove(file)
				}
			}
		}
	}
	cacheWatcher.OnRemove = func(path, op string) bool {
		if data, ok := htmlPreCache.Get(path); ok {
			htmlPreCache.Del(path)
			for _, file := range data.cachePath {
				if (data.static && strings.HasPrefix(file, compilerConfig.StaticHTML)) || (!data.static && strings.HasPrefix(file, compilerConfig.CacheDir)) {
					os.Remove(file)
				}
			}
		}
		return true
	}

	cacheWatcher.WatchDir(root)

	staticWatcher = goutil.FS.FileWatcher()
	staticWatcher.OnFileChange = func(path, op string) {
		if regex.Comp(`(?<!\.min)\.([jt]s|css|less|s[ac]ss)$`).Match([]byte(path)) || imageRE.Match([]byte(path)) || videoRE.Match([]byte(path)) || audioRE.Match([]byte(path)) {
			staticChangeQueue.Set(path, time.Now().UnixMilli())
		}
	}
	staticWatcher.OnRemove = func(path, op string) (removeWatcher bool) {
		if regex.Comp(`(?<!\.min)\.([jt]s|css|less|s[ac]ss)$`).Match([]byte(path)) {
			staticChangeQueue.Del(path)
			os.Remove(string(regex.Comp(`(?<!\.min)\.([jt]s|css|less|s[ac]ss)$`).RepStrComp([]byte(path), []byte(".min.$1"))))
		} else if imageRE.Match([]byte(path)) || videoRE.Match([]byte(path)) || audioRE.Match([]byte(path)) {
			staticChangeQueue.Del(path)
		}
		return true
	}

	staticWatcher.WatchDir(static)

	compilerConfig = Config{
		Root:            root,
		Ext:             "html",
		Static:          static,
		StaticUrl:       "",
		StaticHTML:      staticHTML,
		CacheDir:        cacheDir,
		PreCompress:     7,
		Compress:        5,
		gzipPreCompress: 6,
		gzipCompress:    5,
		CompileMaxFlush: 100,
		CacheTime:       120, // minutes: 2 hours
		DomainFolder:    0,
		DebugMode:       false,
	}

	// clear cache items as needed
	go func() {
		lastRun := 0

		for {
			time.Sleep(10 * time.Second)

			if !runningCompiler {
				break
			}

			if compilerConfig.CacheTime == 0 {
				continue
			}

			now := int(time.Now().UnixMilli() / 60000)
			if now-lastRun < 10 {
				continue
			}
			lastRun = now

			htmlPreCache.ForEach(func(path string, data cacheObj) bool {
				if now-data.accessed > compilerConfig.CacheTime {
					htmlPreCache.Del(path)
					for _, file := range data.cachePath {
						if (data.static && strings.HasPrefix(file, compilerConfig.StaticHTML)) || (!data.static && strings.HasPrefix(file, compilerConfig.CacheDir)) {
							os.Remove(file)
						}
					}
				}
				return true
			})
		}
	}()

	// handle static change queue
	go func() {
		for {
			time.Sleep(100 * time.Nanosecond)

			if !runningCompiler {
				break
			}

			now := time.Now().UnixMilli()
			staticChangeQueue.ForEach(func(path string, modified int64) bool {
				if now-modified > 1000 {
					staticChangeQueue.Del(path)
					tryMinifyFile(path)
				}
				return true
			})
		}
	}()
}

func Close() {
	runningCompiler = false
	cacheWatcher.CloseWatcher("*")
	staticWatcher.CloseWatcher("*")
}

func LogErr(err error) {
	if compilerConfig.DebugMode {
		// fmt.Println(smartErr.New(err).ErrorStack())
		fmt.Println(err)
	}
}

type htmlArgs struct {
	args  map[string][]byte
	ind   []string
	tag   []byte
	close uint8

	passToComp bool
}

type EachArgs struct {
	listMap map[string]interface{}
	listArr []interface{}
	key     []byte
	val     []byte
	ind     uint
	size    uint

	passToComp bool
}

type htmlChanList struct {
	tag  chan handleHtmlData
	comp chan handleHtmlData
	fn   chan handleHtmlData

	running *uint8
}

type handleHtmlData struct {
	html          *[]byte
	options       *map[string]interface{}
	arguments     *htmlArgs
	eachArgs      []EachArgs
	compileError  *error
	componentList [][]byte

	fn      *func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte
	preComp bool

	hasUnhandledVars *bool

	localRoot *string

	stopChan bool
}

// Compile will return html content, (or a static path when possible)
//
// this method will automatically run the PreCompile method as needed
//
// Output Types (note: only []byte or string will have a non empty value. if string == "", you can assume you have an html result)
//
// []byte: raw html (to send to client)
//
// string: path to static html file
//
// uint8: compression type:
//
// - 0: uncompressed raw html
//
// - 1: compressed to brotli
//
// - 2: compressed to gzip
//
// note: putting any extra '.' in a filename (apart from the extention name) may cause conflicts with restoring old cache files
func Compile(path string, opts map[string]interface{}) ([]byte, string, uint8, error) {
	origPath := path

	path, err := goutil.FS.JoinPath(compilerConfig.Root, path+"."+compilerConfig.Ext)
	if err != nil {
		LogErr(err)
		return []byte{}, "", 0, err
	}

	if opts == nil {
		opts = map[string]interface{}{}
	}

	var compressRes []string
	if val, ok := opts["@compress"]; ok && reflect.TypeOf(val) == goutil.VarType["[]string"] {
		compressRes = val.([]string)
	} else if val, ok := opts["@comp"]; ok && reflect.TypeOf(val) == goutil.VarType["[]string"] {
		compressRes = val.([]string)
	} else if val, ok := opts["@compression"]; ok && reflect.TypeOf(val) == goutil.VarType["[]string"] {
		compressRes = val.([]string)
	}

	var compType = uint8(0)
	if goutil.Contains(compressRes, "br") {
		compType = 1
	} else if goutil.Contains(compressRes, "gz") {
		compType = 2
	}

	useCache := true
	if val, ok := opts["@cache"]; ok && reflect.TypeOf(val) == goutil.VarType["bool"] {
		useCache = val.(bool)
	}

	var filePath string

	// get precompiled file from cache
	if useCache {
		if cache, ok := htmlPreCache.Get(path); ok {
			if len(cache.cachePath) == 0 {
				return []byte{}, "", 0, errors.New("cache does not contain any paths for this file")
			}

			if cache.static {
				return getStaticPath(cache, compressRes)
			} else {
				filePath = cache.cachePath[0]
			}
		}
	}

	// precompile file if needed
	if filePath == "" {
		err := PreCompile(origPath, opts)
		if err != nil {
			return []byte{}, "", 0, err
		}

		if cache, ok := htmlPreCache.Get(path); ok {
			if cache.static {
				return getStaticPath(cache, compressRes)
			} else {
				filePath = cache.cachePath[0]
			}
		} else {
			return []byte{}, "", 0, errors.New("failed to precompile file")
		}
	}

	return compile(filePath, &opts, compType)
}

func compile(path string, options *map[string]interface{}, compType uint8) ([]byte, string, uint8, error) {
	// compile file
	reader, err := liveread.Read[uint8](path)
	if err != nil {
		return []byte{}, "", 0, err
	}

	htmlContTemp := [][]byte{}
	htmlContTempTag := []htmlArgs{}

	// auto compress while writing
	var res bytes.Buffer
	var resSize uint = 0
	var writerRaw *bufio.Writer
	var writerBr *brotli.Writer
	var writerGz *gzip.Writer
	if compType == 1 {
		writerBr = brotli.NewWriterLevel(&res, compilerConfig.Compress)
	} else if compType == 2 {
		writerGz, err = gzip.NewWriterLevel(&res, compilerConfig.gzipCompress)
		if err != nil {
			writerGz = gzip.NewWriter(&res)
		}
	} else {
		writerRaw = bufio.NewWriter(&res)
	}

	write := func(b []byte) {
		if len(htmlContTempTag) != 0 {
			htmlContTemp[len(htmlContTempTag)-1] = append(htmlContTemp[len(htmlContTempTag)-1], b...)
			return
		}

		resSize += uint(len(b))

		if compType == 1 {
			writerBr.Write(b)
			if resSize >= compilerConfig.CompileMaxFlush {
				resSize = 0
				writerBr.Flush()
			}
		} else if compType == 2 {
			writerGz.Write(b)
			if resSize >= compilerConfig.CompileMaxFlush {
				resSize = 0
				writerGz.Flush()
			}
		} else {
			writerRaw.Write(b)
			if resSize >= compilerConfig.CompileMaxFlush {
				resSize = 0
				writerRaw.Flush()
			}
		}
	}

	ifTagLevel := []uint8{}
	eachArgsList := []EachArgs{}

	var buf []byte
	for err == nil {
		buf, err = reader.Peek(2)
		if len(buf) == 0 {
			break
		}

		if buf[0] == '{' && buf[1] == '{' {
			ind := uint(2)
			esc := uint8(2)
			if b, e := reader.Get(ind, 2); e == nil {
				if b[0] == '{' {
					esc--
					ind++
					b, e = reader.Get(ind, 2)
				}

				varData := []byte{}
				for e == nil && !(b[0] == '}' && b[1] == '}') && b[0] != '\r' && b[0] != '\n' {
					if b[0] == '"' || b[0] == '\'' || b[0] == '`' {
						q := b[0]
						varData = append(varData, b[0])
						ind++
						b, e = reader.Get(ind, 2)
						for e == nil && b[0] != q {
							if b[0] == '\\' {
								varData = append(varData, b[0], b[1])
								ind += 2
								b, e = reader.Get(ind, 2)
							} else {
								varData = append(varData, b[0])
								ind++
								b, e = reader.Get(ind, 2)
							}
						}
					}

					varData = append(varData, b[0])
					ind++
					b, e = reader.Get(ind, 2)
				}

				if err == nil && len(b) == 2 && b[0] == '}' && b[1] == '}' {
					ind += 2
					b, e = reader.Get(ind, 1)
					if e == nil && b[0] == '}' {
						ind++
						esc--
					}

					reader.Discard(ind)

					if len(varData) != 0 {
						if varData[0] == '%' {
							varData = varData[1:]
							if len(varData) != 0 {
								args := htmlArgs{
									tag:        []byte{},
									args:       map[string][]byte{},
									ind:        []string{},
									close:      3,
									passToComp: false,
								}

								if varData[0] == '/' {
									varData = varData[1:]
									args.close = 1
								} else if varData[len(varData)-1] == '/' {
									varData = varData[:len(varData)-1]
									args.close = 2
								}

								if len(varData) != 0 {
									tagMode := 0
									ind := 0
									tName := ""
									var tq byte
									for i := 0; i < len(varData); i++ {
										if tagMode == 0 {
											if varData[i] == ' ' || varData[i] == '\r' || varData[i] == '\n' {
												tagMode = 1
											} else {
												args.tag = append(args.tag, varData[i])
											}
										} else {
											if tq == 0 && varData[i] == ' ' || varData[i] == '\r' || varData[i] == '\n' {
												if tagMode == 1 && tName != "" {
													s := strconv.Itoa(ind)
													args.ind = append(args.ind, s)
													args.args[s] = []byte(tName)
													ind++
												} else if tagMode == 2 {
													tagMode = 1
												}
												tName = ""
											} else if tagMode == 1 {
												if varData[i] == '=' && tName != "" {
													tagMode = 2
													args.ind = append(args.ind, tName)
													args.args[tName] = []byte{}
													if i+1 < len(varData) && (varData[i+1] == '"' || varData[i+1] == '\'' || varData[i+1] == '`') {
														tq = varData[i+1]
														i++
													}
												} else {
													tName += string(varData[i])
												}
											} else if tagMode == 2 {
												if varData[i] == tq {
													tq = 0
													continue
												} else if varData[i] == '\\' && i+1 < len(varData) {
													args.args[tName] = append(args.args[tName], varData[i])
													i++
												} else if varData[i] == '"' || varData[i] == '\'' || varData[i] == '`' {
													q := varData[i]
													args.args[tName] = append(args.args[tName], varData[i])
													i++
													for i < len(varData) && varData[i] != q {
														if varData[i] == '\\' && i+1 < len(varData) {
															args.args[tName] = append(args.args[tName], varData[i])
															i++
														}
														args.args[tName] = append(args.args[tName], varData[i])
														i++
													}
												}
												if i < len(varData) {
													args.args[tName] = append(args.args[tName], varData[i])
												}
											}
										}
									}

									// fix args for functions
									for k, v := range args.args {
										if regex.Comp(`^\{\{\{(.*)\}\}\}$`).MatchRef(&v) {
											args.args[k] = append([]byte{2}, regex.Comp(`^\{\{\{(.*)\}\}\}$`).RepStrCompRef(&v, []byte("$1"))...)
										} else if regex.Comp(`^\{\{\{?(.*)\}\}\}?$`).MatchRef(&v) {
											args.args[k] = append([]byte{1}, regex.Comp(`^\{\{\{?(.*)\}\}\}?$`).RepStrCompRef(&v, []byte("$1"))...)
										} else {
											args.args[k] = append([]byte{0}, v...)
										}
									}

									if len(args.tag) != 0 {
										args.tag = bytes.ToLower(args.tag)

										// args.close:
										// 0 = failed to close (<tag)
										// 1 = </tag>
										// 2 = <tag/> (</tag/>)
										// 3 = <tag>

										if bytes.Equal(args.tag, []byte("if")) || bytes.Equal(args.tag, []byte("else")) {
											if args.close == 3 && bytes.Equal(args.tag, []byte("if")) { // open tag
												if _, ok := TagFuncs.If(options, &args, &eachArgsList, false); ok {
													// grab if content and skip else content
													ifTagLevel = append(ifTagLevel, 0)
													removeLineBreak(reader)
												} else {
													// skip if content and move on to next else tag
													ifTagLevel = append(ifTagLevel, 3)
													ib, ie := reader.Peek(2)
													ifLevel := 0
													for ie == nil {
														if ib[0] == '"' || ib[0] == '\'' || ib[0] == '`' {
															q := ib[0]
															reader.Discard(1)
															ib, ie = reader.Peek(2)
															for ie == nil && ib[0] != q {
																if ib[0] == '\\' {
																	reader.Discard(1)
																}
																reader.Discard(1)
																ib, ie = reader.Peek(2)
															}
														} else if ib[0] == '{' && ib[1] == '{' {
															ibTag, ie := reader.Peek(10)
															if ie == nil && ifLevel == 0 && regex.Comp(`^\{\{\{?%/?else[\s/\}:]`).MatchRef(&ibTag) {
																break
															} else if (ie == nil || len(ibTag) > 6) && regex.Comp(`^\{\{\{?%/?if[\s/\}:]`).MatchRef(&ibTag) {
																if ibTag[3] == '/' || ibTag[4] == '/' {
																	ifLevel--
																	if ifLevel < 0 {
																		break
																	}
																} else {
																	ifLevel++
																}
															}
														}

														reader.Discard(1)
														ib, ie = reader.Peek(2)
													}
												}
											} else if args.close == 1 && len(ifTagLevel) != 0 && bytes.Equal(args.tag, []byte("if")) { // close tag
												removeLineBreak(reader)
												ifTagLevel = ifTagLevel[:len(ifTagLevel)-1]
											} else if len(ifTagLevel) != 0 && bytes.Equal(args.tag, []byte("else")) { // else tag
												if ifTagLevel[len(ifTagLevel)-1] == 0 { // true if statement
													// skip content to closing if tag
													ib, ie := reader.Peek(2)
													ifLevel := 0
													for ie == nil {
														if ib[0] == '"' || ib[0] == '\'' || ib[0] == '`' {
															q := ib[0]
															reader.Discard(1)
															ib, ie = reader.Peek(2)
															for ie == nil && ib[0] != q {
																if ib[0] == '\\' {
																	reader.Discard(1)
																}
																reader.Discard(1)
																ib, ie = reader.Peek(2)
															}
														} else if ib[0] == '{' && ib[1] == '{' {
															ibTag, ie := reader.Peek(8)
															if ie == nil && ifLevel == 0 && regex.Comp(`^\{\{\{?%/if[\s/\}:]`).MatchRef(&ibTag) {
																break
															} else if (ie == nil || len(ibTag) > 6) && regex.Comp(`^\{\{\{?%/?if[\s/>:]`).MatchRef(&ibTag) {
																if ibTag[1] == '/' {
																	ifLevel--
																	if ifLevel < 0 {
																		ifLevel = 0
																	}
																} else {
																	ifLevel++
																}
															}
														}

														reader.Discard(1)
														ib, ie = reader.Peek(2)
													}
												} else if ifTagLevel[len(ifTagLevel)-1] == 3 { // false if statement
													if _, ok := TagFuncs.If(options, &args, &eachArgsList, false); ok {
														// grab if content and skip else content
														ifTagLevel[len(ifTagLevel)-1] = 0
														removeLineBreak(reader)
													} else {
														// skip if content and move on to next else tag
														ib, ie := reader.Peek(2)
														ifLevel := 0
														for ie == nil {
															if ib[0] == '"' || ib[0] == '\'' || ib[0] == '`' {
																q := ib[0]
																reader.Discard(1)
																ib, ie = reader.Peek(2)
																for ie == nil && ib[0] != q {
																	if ib[0] == '\\' {
																		reader.Discard(1)
																	}
																	reader.Discard(1)
																	ib, ie = reader.Peek(2)
																}
															} else if ib[0] == '{' && ib[1] == '{' {
																ibTag, ie := reader.Peek(10)
																if ie == nil && ifLevel == 0 && regex.Comp(`^\{\{\{?%/?else[\s/\}:]`).MatchRef(&ibTag) {
																	break
																} else if (ie == nil || len(ibTag) > 6) && regex.Comp(`^\{\{\{?%/?if[\s/\}:]`).MatchRef(&ibTag) {
																	if ibTag[1] == '/' {
																		ifLevel--
																		if ifLevel < 0 {
																			break
																		}
																	} else {
																		ifLevel++
																	}
																}
															}

															reader.Discard(1)
															ib, ie = reader.Peek(2)
														}
													}
												}
											}
										} else if bytes.Equal(args.tag, []byte("each")) {
											if args.close == 3 {
												if args.args["0"] != nil && len(args.args["0"]) != 0 {
													if hasVarOpt(args.args["0"], options, &eachArgsList, 0, false) {
														listArg := GetOpt(args.args["0"], options, &eachArgsList, 0, false, false)
														if t := reflect.TypeOf(listArg); t == goutil.VarType["map[string]interface{}"] || t == goutil.VarType["[]interface{}"] {
															eachArgs := EachArgs{}
															if t == goutil.VarType["map[string]interface{}"] && len(listArg.(map[string]interface{})) != 0 {
																eachArgs.listMap = listArg.(map[string]interface{})
																eachArgs.listArr = []interface{}{}
																for k := range eachArgs.listMap {
																	eachArgs.listArr = append(eachArgs.listArr, k)
																}

																sortStrings(&eachArgs.listArr)
															} else if t == goutil.VarType["[]interface{}"] && len(listArg.([]interface{})) != 0 {
																eachArgs.listArr = listArg.([]interface{})
															} else {
																// skip each content and move on to closing each tag
																ifTagLevel = append(ifTagLevel, 3)
																ib, ie := reader.Peek(2)
																ifLevel := 0
																for ie == nil {
																	if ib[0] == '"' || ib[0] == '\'' || ib[0] == '`' {
																		q := ib[0]
																		reader.Discard(1)
																		ib, ie = reader.Peek(2)
																		for ie == nil && ib[0] != q {
																			if ib[0] == '\\' {
																				reader.Discard(1)
																			}
																			reader.Discard(1)
																			ib, ie = reader.Peek(2)
																		}
																	} else if ib[0] == '{' && ib[1] == '{' {
																		ibTag, ie := reader.Peek(10)
																		if ie == nil && ifLevel == 0 && regex.Comp(`^\{\{\{?%/?each[\s/\}:]`).MatchRef(&ibTag) {
																			break
																		} else if (ie == nil || len(ibTag) > 8) && regex.Comp(`^\{\{\{?%/?each[\s/\}:]`).MatchRef(&ibTag) {
																			if ibTag[1] == '/' {
																				ifLevel--
																				if ifLevel < 0 {
																					break
																				}
																			} else {
																				ifLevel++
																			}
																		}
																	}

																	reader.Discard(1)
																	ib, ie = reader.Peek(2)
																}
																continue
															}

															eachArgs.size = uint(len(eachArgs.listArr))

															if args.args["key"] != nil && len(args.args["key"]) != 0 {
																eachArgs.key = args.args["key"]
															} else if args.args["of"] != nil && len(args.args["of"]) != 0 {
																eachArgs.key = args.args["of"]
															}

															if args.args["value"] != nil && len(args.args["value"]) != 0 {
																eachArgs.val = args.args["value"]
															} else if args.args["as"] != nil && len(args.args["as"]) != 0 {
																eachArgs.val = args.args["as"]
															}

															eachArgsList = append(eachArgsList, eachArgs)
															reader.Save()

															removeLineBreak(reader)
															continue
														}
													} else {
														continue
													}
												}

												// skip each content and move on to closing each tag
												ifTagLevel = append(ifTagLevel, 3)
												ib, ie := reader.Peek(2)
												ifLevel := 0
												for ie == nil {
													if ib[0] == '"' || ib[0] == '\'' || ib[0] == '`' {
														q := ib[0]
														reader.Discard(1)
														ib, ie = reader.Peek(2)
														for ie == nil && ib[0] != q {
															if ib[0] == '\\' {
																reader.Discard(1)
															}
															reader.Discard(1)
															ib, ie = reader.Peek(2)
														}
													} else if ib[0] == '{' && ib[1] == '{' {
														ibTag, ie := reader.Peek(10)
														if ie == nil && ifLevel == 0 && regex.Comp(`^\{\{\{?%/?each[\s/\}:]`).MatchRef(&ibTag) {
															break
														} else if (ie == nil || len(ibTag) > 8) && regex.Comp(`^\{\{\{?%/?each[\s/\}:]`).MatchRef(&ibTag) {
															if ibTag[1] == '/' {
																ifLevel--
																if ifLevel < 0 {
																	break
																}
															} else {
																ifLevel++
															}
														}
													}

													reader.Discard(1)
													ib, ie = reader.Peek(2)
												}
											} else if args.close == 1 {
												if len(eachArgsList) != 0 {
													if eachArgsList[len(eachArgsList)-1].ind < eachArgsList[len(eachArgsList)-1].size-1 {
														if eachArgsList[len(eachArgsList)-1].ind == 0 {
															reader.Restore()
															removeLineBreak(reader)
														} else {
															reader.RestoreReset()
															removeLineBreak(reader)
														}
														eachArgsList[len(eachArgsList)-1].ind++
													} else {
														reader.DelSave()
														removeLineBreak(reader)
														eachArgsList = eachArgsList[:len(eachArgsList)-1]
													}
												}
											}
										} else {
											args.tag[0] = bytes.ToUpper([]byte{args.tag[0]})[0]

											if args.close == 3 {
												htmlContTempTag = append(htmlContTempTag, args)
												htmlContTemp = append(htmlContTemp, []byte{})
											} else if args.close == 1 && len(htmlContTempTag) != 0 {
												for i := len(htmlContTempTag) - 1; i >= 0; i-- {
													sameTag := bytes.Equal(htmlContTempTag[i].tag, args.tag)

													fn, _, fnErr := getCoreTagFunc(htmlContTempTag[i].tag)
													if fnErr != nil {
														if newFn, ok := TagFuncs.list[string(htmlContTempTag[i].tag)]; ok {
															fn = newFn
															fnErr = nil
														}
													}

													if fnErr == nil {
														for k, v := range htmlContTempTag[i].args {
															args.args[k] = v
														}
														args.args["body"] = htmlContTemp[i]

														htmlContTemp = htmlContTemp[:i]
														htmlContTempTag = htmlContTempTag[:i]

														htmlCont := []byte{0}
														var compErr error

														handleHtmlFunc(handleHtmlData{fn: &fn, preComp: false, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr})

														if compErr == nil {
															write(htmlCont[1:])
														} else if compilerConfig.DebugMode {
															LogErr(compErr)
															write(regex.JoinBytes([]byte("<!--{{error: "), compErr, []byte("}}-->")))
														}
													} else {
														if i != 0 && !sameTag && len(htmlContTemp[i]) != 0 {
															if len(htmlContTemp[i]) != 0 && htmlContTemp[i][0] == '\r' {
																htmlContTemp[i] = htmlContTemp[i][1:]
															}
															if len(htmlContTemp[i]) != 0 && htmlContTemp[i][0] == '\n' {
																htmlContTemp[i] = htmlContTemp[i][1:]
															}
															htmlContTemp[i-1] = append(htmlContTemp[i-1], htmlContTemp[i]...)
														}
														htmlContTemp = htmlContTemp[:i]
														htmlContTempTag = htmlContTempTag[:i]
													}

													if sameTag {
														break
													}
												}
											} else if args.close == 2 {
												fn, _, fnErr := getCoreTagFunc(args.tag)
												if fnErr != nil {
													if newFn, ok := TagFuncs.list[string(args.tag)]; ok {
														fn = newFn
														fnErr = nil
													}
												}

												if fnErr == nil {
													htmlCont := []byte{0}
													var compErr error

													handleHtmlFunc(handleHtmlData{fn: &fn, preComp: false, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr})

													if compErr == nil {
														write(htmlCont[1:])
													} else if compilerConfig.DebugMode {
														LogErr(compErr)
														write(regex.JoinBytes([]byte("<!--{{error: "), compErr, []byte("}}-->")))
													}
												}
											}
										}
									}
								}
							}
							continue
						}

						if varData[0] == '#' && (bytes.HasPrefix(varData, []byte("#error:")) || bytes.HasPrefix(varData, []byte("#warning:"))) {
							if compilerConfig.DebugMode {
								write(regex.JoinBytes([]byte("{{"), varData, []byte("}}")))
							}
							continue
						}

						if varData[0] == ':' {
							val := GetOpt(varData[1:], options, &eachArgsList, 4, false, true)
							if !goutil.IsZeroOfUnderlyingType(val) {
								if len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
									val = val.([]byte)[1:]
								}
								write(val.([]byte))
							}
						} else if regex.Comp(`^[\w_-]*=`).MatchRef(&varData) {
							args := bytes.SplitN(varData, []byte{'='}, 2)
							if len(args) == 2 {
								if esc != 0 {
									esc = 3
								} else {
									esc = 1
								}

								if len(args[0]) == 0 {
									if regex.Comp(`^([\w_-]+).*$`).MatchRef(&args[1]) {
										args[0] = regex.Comp(`^([\w_-]+).*$`).RepStrComplexRef(&args[1], []byte("$1"))
										args[0] = append(args[0], '=')
									}
								} else {
									args[0] = append(args[0], '=')
								}

								val := GetOpt(args[1], options, &eachArgsList, esc, false, true)
								if !goutil.IsZeroOfUnderlyingType(val) {
									if len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
										val = val.([]byte)[1:]
									}
									write(regex.JoinBytes(args[0], '"', val.([]byte), '"'))
								}
							}
						} else {
							if esc != 0 {
								esc = 2
							}

							val := GetOpt(varData, options, &eachArgsList, esc, false, true)
							if !goutil.IsZeroOfUnderlyingType(val) {
								if len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
									val = val.([]byte)[1:]
								}
								write(val.([]byte))
							}
						}
					}

					continue
				}
			}
		}

		if !compilerConfig.DebugMode && buf[0] == '{' && buf[1] == '\\' {
			b, e := reader.Peek(4)
			if e == nil && b[2] == '{' && b[3] == '#' {
				ind := uint(4)
				b, e := reader.Get(ind, 1)
				tag := []byte{}
				for e == nil && b[0] != ':' && b[0] != '}' {
					tag = append(tag, b[0])
					ind++
					b, e = reader.Get(ind, 1)
				}

				if e == nil && (bytes.Equal(tag, []byte("error")) || bytes.Equal(tag, []byte("warning"))) {
					b, e := reader.Get(ind, 3)
					for e == nil && !(b[0] == '}' && b[1] == '\\' && b[2] == '}') {
						tag = append(tag, b[0])
						ind++
						b, e = reader.Get(ind, 3)
					}

					if e == nil {
						reader.Discard(ind + 3)
						continue
					}
				}
			}
		}

		if buf[0] == '\\' && (buf[1] == '{' || buf[1] == '}') {
			// remove escape chars from escaped {{MyVar}} tags (example: {\{NotAVar}\}, {\{\{NotAnHTMLVar}\}\})
			reader.Discard(2)
			write([]byte{buf[1]})
			continue
		}

		if buf[0] == '<' && buf[1] == '!' {
			b, e := reader.Peek(4)
			if e == nil && b[2] == '-' && b[3] == '-' {
				reader.Discard(4)
				commentData := []byte{}

				buf, err = reader.Peek(3)
				for err == nil && !(buf[0] == '-' && buf[1] == '-' && buf[2] == '>') {
					commentData = append(commentData, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(3)
				}

				if err == nil {
					reader.Discard(3)
				}

				if compilerConfig.DebugMode || keepCommentRE.MatchRef(&commentData) {
					write(regex.JoinBytes([]byte("<!--"), commentData, []byte("-->")))
				}

				continue
			}
		}

		if compilerConfig.DebugMode {
			if buf[0] == '/' && buf[1] == '/' {
				reader.Discard(2)
				commentData := []byte{}

				buf, err = reader.Peek(1)
				for err == nil && buf[0] != '\n' {
					commentData = append(commentData, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(1)
				}

				if err == nil {
					reader.Discard(1)
				}
				write(regex.JoinBytes('/', '/', commentData, '\n'))

				continue
			} else if buf[0] == '/' && buf[1] == '*' {
				reader.Discard(2)
				commentData := []byte{}

				buf, err = reader.Peek(2)
				for err == nil && !(buf[0] == '*' && buf[1] == '/') {
					commentData = append(commentData, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(2)
				}

				if err == nil {
					reader.Discard(2)
				}
				write(regex.JoinBytes('/', '*', commentData, '*', '/'))

				continue
			}
		}

		write([]byte{buf[0]})
		reader.Discard(1)
	}

	//todo: add public js can css options (may use @js and @css arrays in options)
	// may also handle css maps with '-' seperators

	if compType == 1 {
		writerBr.Flush()
		writerBr.Close()
	} else if compType == 2 {
		writerGz.Flush()
		writerGz.Close()
	} else {
		writerRaw.Flush()
	}

	return res.Bytes(), "", compType, nil
}

func getStaticPath(cache cacheObj, compressRes []string) ([]byte, string, uint8, error) {
	if goutil.Contains(compressRes, "br") {
		for _, p := range cache.cachePath {
			if strings.HasSuffix(p, ".html.br") {
				return []byte{}, p, 1, nil
			}
		}
	}

	if goutil.Contains(compressRes, "gz") {
		for _, p := range cache.cachePath {
			if strings.HasSuffix(p, ".html.gz") {
				return []byte{}, p, 2, nil
			}
		}
	}

	for _, p := range cache.cachePath {
		if strings.HasSuffix(p, ".html") {
			return []byte{}, p, 0, nil
		}
	}

	p := cache.cachePath[0]
	file, err := os.ReadFile(p)
	if err != nil {
		return []byte{}, "", 0, err
	}

	if strings.HasSuffix(p, ".html.br") {
		if goutil.Contains(compressRes, "br") {
			return file, "", 1, nil
		}
		file, err = goutil.BROTLI.UnZip(file)
		if err != nil {
			return []byte{}, "", 0, err
		}
	} else if strings.HasSuffix(p, ".html.gz") {
		if goutil.Contains(compressRes, "gz") {
			return file, "", 2, nil
		}
		file, err = goutil.GZIP.UnZip(file)
		if err != nil {
			return []byte{0}, "", 0, err
		}
	}

	return file, "", 0, nil
}

// HasPreCompile returns true if a file has been PreCompiled and exists in the cache
func HasPreCompile(path string) (bool, error) {
	path, err := goutil.FS.JoinPath(compilerConfig.Root, path+"."+compilerConfig.Ext)
	if err != nil {
		LogErr(err)
		return false, err
	}

	_, ok := htmlPreCache.Get(path)
	return ok, nil
}

// HasStaticCompile returns true if a file has been PreCompiled and is static (and does not need to be compiled)
//
// note: the Compile method will automatically detect this and pull from the cache when available
func HasStaticCompile(path string) (bool, error) {
	path, err := goutil.FS.JoinPath(compilerConfig.Root, path+"."+compilerConfig.Ext)
	if err != nil {
		LogErr(err)
		return false, err
	}

	if cache, ok := htmlPreCache.Get(path); ok {
		if len(cache.cachePath) == 0 {
			return false, errors.New("cache does not contain any paths for this file")
		}
		return cache.static, nil
	}
	return false, nil
}

// PreCompile will generate a new file for the cache (or a static file when possible)
//
// note: putting any extra '.' in a filename (apart from the extention name) may cause conflicts with restoring old cache files
func PreCompile(path string, opts map[string]interface{}) error {
	origPath := path

	path, err := goutil.FS.JoinPath(compilerConfig.Root, path+"."+compilerConfig.Ext)
	if err != nil {
		LogErr(err)
		return err
	}
	origCachePath := path

	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		if compilerConfig.IncludeMD {
			path, err = goutil.FS.JoinPath(compilerConfig.Root, origPath+".md")
			if err != nil {
				err = errors.New(string(regex.Comp(`\.md:`).RepStr([]byte(err.Error()), []byte("."+compilerConfig.Ext))))
				LogErr(err)
				return err
			}

			if stat, err := os.Stat(path); err != nil || stat.IsDir() {
				err = errors.New(string(regex.Comp(`\.md:`).RepStr([]byte(err.Error()), []byte("."+compilerConfig.Ext))))
				LogErr(err)
				return err
			}
		} else {
			LogErr(err)
			return err
		}
	}

	htmlChan := newPreCompileChan()

	html := []byte{0}
	preCompile(path, &opts, &htmlArgs{}, &html, &err, &htmlChan, nil, nil)
	if err != nil || len(html) == 0 || html[0] == 2 {
		if err == nil {
			err = errors.New("failed to precompile: '" + path + "'")
		}
		if compilerConfig.DebugMode && !strings.HasPrefix(err.Error(), "warning:") {
			LogErr(err)
			html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
		} else {
			return err
		}
	}

	resType := html[0]
	html = html[1:]

	// get layout and merge with html
	layoutPath := "layout"
	if lp, ok := opts["@layout"]; ok {
		if str, ok := lp.(string); ok && str != "" {
			layoutPath = str
		}
	}

	localRoot := ""
	if compilerConfig.DomainFolder != 0 {
		for i := int(compilerConfig.DomainFolder); localRoot == "" && i > 0; i-- {
			regex.Comp(`^((?:/[\w_\-\.]+){%1})`, strconv.Itoa(i)).RepFunc([]byte(strings.Replace(path, compilerConfig.Root, "", 1)), func(data func(int) []byte) []byte {
				localRoot = string(data(1))

				// verify root is dir
				if lr, err := goutil.FS.JoinPath(compilerConfig.Root, localRoot); err == nil {
					if stat, err := os.Stat(lr); err == nil && !stat.IsDir() {
						localRoot = ""
					}
				}

				return nil
			}, true)
		}
	}

	if localRoot != "" {
		if path, err := goutil.FS.JoinPath(compilerConfig.Root, localRoot, layoutPath+"."+compilerConfig.Ext); err == nil {
			layoutPath = path
		} else {
			layoutPath = ""
		}
	} else if path, err := goutil.FS.JoinPath(compilerConfig.Root, layoutPath+"."+compilerConfig.Ext); err == nil {
		layoutPath = path
	} else {
		layoutPath = ""
	}

	if layoutPath != "" {
		if localRoot != "" {
			if stat, err := os.Stat(layoutPath); err != nil || stat.IsDir() {
				layoutPath = string(regex.Comp(`^(%1)%2`, compilerConfig.Root, localRoot).RepStrComp([]byte(layoutPath), []byte("$1")))
			}
		}

		if stat, err := os.Stat(layoutPath); err == nil && !stat.IsDir() {
			opts["$body"] = html
			html = []byte{0}
			preCompile(layoutPath, &opts, &htmlArgs{}, &html, &err, nil, nil, nil)
			if err != nil || len(html) == 0 || html[0] == 2 {
				if err == nil {
					err = errors.New("layout - failed to precompile: '" + path + "'")
				}
				if compilerConfig.DebugMode && !strings.HasPrefix(err.Error(), "warning:") {
					LogErr(err)
					html = append(html, regex.JoinBytes([]byte("<!--{{#error: layout - "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
				} else {
					return errors.Join(errors.New("layout - "), err)
				}
			}

			// disable static mode if layout is not static
			if html[0] != 3 {
				resType = 1
			}
			html = html[1:]
		}
	}

	origPath = string(regex.Comp(`[\\\/]+`).RepStr([]byte(origPath), []byte{'.', '_', '.'}))

	if resType == 3 {
		// create static html file
		staticPath, err := goutil.FS.JoinPath(compilerConfig.StaticHTML, origPath+"."+compilerConfig.Ext)
		if err != nil {
			if compilerConfig.DebugMode {
				LogErr(err)
				html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
			}
			return err
		}

		cachePath := []string{}
		if br, err := goutil.BROTLI.Zip(html, compilerConfig.PreCompress); err == nil {
			if err := os.WriteFile(staticPath+".html.br", br, 0775); err == nil {
				cachePath = append(cachePath, staticPath+".html.br")
			}
		}

		if gz, err := goutil.GZIP.Zip(html, compilerConfig.gzipPreCompress); err == nil {
			if err := os.WriteFile(staticPath+".html.gz", gz, 0775); err == nil {
				cachePath = append(cachePath, staticPath+".html.gz")
			}
		}

		if len(cachePath) == 0 {
			if err = os.WriteFile(staticPath+".html", html, 0775); err != nil {
				if compilerConfig.DebugMode {
					LogErr(err)
					html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
				}
				return err
			} else {
				cachePath = append(cachePath, staticPath+".html")
			}
		}

		if len(cachePath) != 0 {
			if oldCache, ok := htmlPreCache.Get(origCachePath); ok {
				for _, file := range oldCache.cachePath {
					if !oldCache.static && strings.HasPrefix(file, compilerConfig.CacheDir) {
						os.Remove(file)
					}
				}
			}

			htmlPreCache.Set(origCachePath, cacheObj{
				cachePath: cachePath,
				static:    true,
				accessed:  int(time.Now().UnixMilli() / 60000),
			})
		}
	} else {
		// cache dynamic html file
		staticPath, err := goutil.FS.JoinPath(compilerConfig.CacheDir, origPath+"."+compilerConfig.Ext)
		if err != nil {
			if compilerConfig.DebugMode {
				LogErr(err)
				html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
			}
			return err
		}

		cachePath := []string{}

		if err = os.WriteFile(staticPath+".html.cache", html, 0775); err != nil {
			if compilerConfig.DebugMode {
				LogErr(err)
				html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
			}
			return err
		} else {
			cachePath = append(cachePath, staticPath+".html.cache")
		}

		if len(cachePath) != 0 {
			if oldCache, ok := htmlPreCache.Get(origCachePath); ok {
				for _, file := range oldCache.cachePath {
					if oldCache.static && strings.HasPrefix(file, compilerConfig.StaticHTML) {
						os.Remove(file)
					}
				}
			}

			htmlPreCache.Set(origCachePath, cacheObj{
				cachePath: cachePath,
				static:    false,
				accessed:  int(time.Now().UnixMilli() / 60000),
			})
		}
	}

	return nil
}

func preCompile(path string, options *map[string]interface{}, arguments *htmlArgs, html *[]byte, compileError *error, htmlChan *htmlChanList, eachArgsList []EachArgs, componentList [][]byte) {
	reader, err := liveread.Read[uint8](path)
	if err != nil {
		*compileError = err
		(*html)[0] = 2
		return
	}

	if componentList == nil {
		componentList = [][]byte{}
	}

	if eachArgsList == nil {
		eachArgsList = []EachArgs{}
	}

	// merge html args with options (and compile options as needed)
	if arguments.args != nil && len(arguments.args) != 0 {
		if opts, err := goutil.JSON.DeepCopy(*options); err == nil {
			for k, v := range arguments.args {
				if !strings.HasPrefix(k, "$") {
					k = "$" + k
				}

				if k == "$body" {
					opts[k] = v
					continue
				}

				if v != nil && len(v) != 0 && v[0] == 0 {
					v = v[1:]

					if len(v) != 0 && bytes.HasPrefix(v, []byte("{{")) && bytes.HasSuffix(v, []byte("}}")) {
						v = v[2 : len(v)-2]

						if len(v) == 0 {
							continue
						}

						esc := uint8(2)

						if len(v) >= 2 && v[0] == '{' && v[1] == '}' {
							esc = 0
							v = v[1 : len(v)-1]
						} else if v[0] == '{' {
							v = v[1:]
						} else if v[len(v)-1] == '}' {
							v = v[:len(v)-1]
						}

						if val := GetOpt(v, options, &eachArgsList, esc, true, false); val != nil {
							opts[k] = val
						}
					} else {
						opts[k] = v
					}

					continue
				}

				if v != nil && len(v) != 0 {
					if v[0] == 0 {
						v = v[1:]

						if len(v) == 0 {
							continue
						}
					}

					opts[k] = v
				}
			}

			options = &opts
		}
	}

	localRoot := ""
	if compilerConfig.DomainFolder != 0 {
		for i := int(compilerConfig.DomainFolder); localRoot == "" && i > 0; i-- {
			regex.Comp(`^((?:/[\w_\-\.]+){%1})`, strconv.Itoa(i)).RepFunc([]byte(strings.Replace(path, compilerConfig.Root, "", 1)), func(data func(int) []byte) []byte {
				localRoot = string(data(1))

				// verify root is dir
				if lr, err := goutil.FS.JoinPath(compilerConfig.Root, localRoot); err == nil {
					if stat, err := os.Stat(lr); err == nil && !stat.IsDir() {
						localRoot = ""
					}
				}

				return nil
			}, true)
		}
	}

	hasUnhandledVars := false

	htmlRes := []byte{}
	htmlTags := []*[]byte{}
	htmlTagsErr := []*error{}

	htmlContTemp := [][]byte{}
	htmlContTempTag := []htmlArgs{}

	endLineBreak := uint(0)
	firstWrite := true
	write := func(b []byte, raw ...bool) {
		if len(b) == 0 {
			return
		}

		if firstWrite {
			b = regex.Comp(`^\s+`).RepStrRef(&b, []byte{})
			if len(b) == 0 {
				return
			}
			firstWrite = false
		}

		if len(raw) == 0 || raw[0] == false {
			b = regex.Comp(`\r+`).RepStrRef(&b, []byte{})
			if len(b) == 0 {
				return
			}

			b = regex.Comp(`(\n{2})\n+`).RepStrCompRef(&b, []byte("$1"))
			if len(b) == 0 {
				return
			}

			if endLineBreak >= 2 {
				b = regex.Comp(`^\n+`).RepStrRef(&b, []byte{})
				if len(b) == 0 {
					return
				}
			} else if endLineBreak == 1 {
				b = regex.Comp(`^\n+`).RepStrRef(&b, []byte{'\n'})
				if len(b) == 0 {
					return
				}
			}

			if endLineBreak != 0 {
				b = regex.Comp(`^[\t ]$`).RepStrRef(&b, []byte{})
				if len(b) == 0 {
					return
				}
			}

			if b[len(b)-1] == '\n' {
				endLineBreak++
				if len(b) > 1 && b[len(b)-2] == '\n' {
					endLineBreak++
				}
			} else {
				endLineBreak = 0
			}
		} else {
			endLineBreak = 0
		}

		if len(htmlContTempTag) != 0 {
			htmlContTemp[len(htmlContTempTag)-1] = append(htmlContTemp[len(htmlContTempTag)-1], b...)
		} else {
			htmlRes = append(htmlRes, b...)
		}
	}

	ifTagLevel := []uint8{}

	firstChar := true
	spaces := uint(0)
	mdStore := map[string]interface{}{}

	tabSize := uint(4)
	if i, ok := (*options)["@tab"]; ok {
		tabSize = goutil.Conv.ToUint(i)
		if tabSize == 0 {
			tabSize = 4
		}
	}

	var buf byte
	for err == nil {
		buf, err = reader.PeekByte(0)
		if buf == 0 {
			break
		}

		if buf == '\n' {
			if !firstChar {
				compileMarkdownNextLine(reader, &write, &firstChar, &spaces, &mdStore)
			}

			write([]byte{'\n'})
			firstChar = true
			spaces = 0

			reader.Discard(1)
			buf, err = reader.PeekByte(0)
			continue
		} else if firstChar && regex.Comp(`^\s`).MatchRef(&[]byte{buf}) {
			if buf == ' ' {
				spaces++
			} else if regex.Comp(`^\t`).MatchRef(&[]byte{buf}) {
				spaces += tabSize
			} else {
				spaces = 0
			}

			reader.Discard(1)
			buf, err = reader.PeekByte(0)
			continue
		}

		if buf == '<' { // handle html tags
			// detect html comments
			comB, comE := reader.Peek(4)
			if comE == nil && comB[1] == '!' && comB[2] == '-' && comB[3] == '-' {
				reader.Discard(4)
				commentData := []byte{}

				comB, comE = reader.Peek(3)
				for comE == nil && !(comB[0] == '-' && comB[1] == '-' && comB[2] == '>') {
					commentData = append(commentData, comB[0])
					reader.Discard(1)
					comB, comE = reader.Peek(3)
				}

				if comE == nil {
					reader.Discard(3)
				}
				if compilerConfig.DebugMode || keepCommentRE.MatchRef(&commentData) {
					write(regex.JoinBytes([]byte("<!--"), commentData, []byte("-->")))
				}

				continue
			}

			args := htmlArgs{
				args: map[string][]byte{},
				ind:  []string{},
			}
			argInd := 0

			ind := uint(1)
			b, e := reader.PeekByte(ind)
			if b == '/' {
				args.close = 1
				ind++

				b, e = reader.PeekByte(ind)
			}

			if regex.Comp(`[\w_]`).MatchRef(&[]byte{b}) {
				args.tag = []byte{b}
				ind++

				// get tag
				for e == nil {
					b, e = reader.PeekByte(ind)
					ind++
					if b == 0 {
						break
					}

					if b == '/' {
						if b2, e2 := reader.PeekByte(ind); e2 == nil && b2 == '>' {
							ind++
							args.close = 2
							break
						}
					} else if b == '>' {
						if args.close == 0 {
							args.close = 3
						}
						break
					} else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
						break
					}

					args.tag = append(args.tag, b)
				}

				if len(args.tag) > 0 {
					// get args
					for e == nil && args.close == 0 {
						b, e = reader.PeekByte(ind)
						ind++
						if b == 0 {
							break
						}

						if b == '/' {
							if b2, e2 := reader.PeekByte(ind); e2 == nil && b2 == '>' {
								ind++
								args.close = 2
								break
							}
						} else if b == '>' {
							if args.close == 0 {
								args.close = 3
							}
							break
						} else if b == '&' || b == '|' || b == '(' || b == ')' || b == '!' {
							i := strconv.Itoa(argInd)
							args.args[i] = []byte{5, b}
							args.ind = append(args.ind, i)
							argInd++
							continue
						} else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
							continue
						}

						var q byte
						if b == '"' || b == '\'' || b == '`' {
							q = b
							b, e = reader.PeekByte(ind)
							ind++
						}

						key := []byte{}
						for e == nil && ((q == 0 && regex.Comp(`[^\s\r\n=/>]`).MatchRef(&[]byte{b})) || (q != 0 && b != q)) {
							if q != 0 && b == '\\' {
								b, e = reader.PeekByte(ind)
								ind++
								if b != q && b != '\\' {
									key = append(key, '\\')
								}
							}

							key = append(key, b)
							b, e = reader.PeekByte(ind)
							ind++
						}

						if b == '>' || b == '/' {
							ind--
						}

						if b != '=' {
							isVar := uint8(0)
							if bytes.HasPrefix(key, []byte("{{")) && bytes.HasSuffix(key, []byte("}}")) {
								key = key[2 : len(key)-2]
								isVar++

								if bytes.HasPrefix(key, []byte("{")) && bytes.HasSuffix(key, []byte("}")) {
									key = key[1 : len(key)-1]
									isVar++
								} else if bytes.HasPrefix(key, []byte("{")) {
									key = key[1:]
								} else if bytes.HasSuffix(key, []byte("}")) {
									key = key[:len(key)-1]
								}
							}

							i := strconv.Itoa(argInd)
							args.args[i] = append([]byte{isVar}, key...)
							args.ind = append(args.ind, i)
							argInd++
							continue
						}

						b, e = reader.PeekByte(ind)
						ind++

						q = 0
						if b == '"' || b == '\'' || b == '`' {
							q = b
							b, e = reader.PeekByte(ind)
							ind++
						}

						val := []byte{}
						for e == nil && ((q == 0 && regex.Comp(`[^\s\r\n=/>]`).MatchRef(&[]byte{b})) || (q != 0 && b != q)) {
							if q != 0 && b == '\\' {
								b, e = reader.PeekByte(ind)
								ind++
								if b != q && b != '\\' {
									val = append(val, '\\')
								}
							}

							val = append(val, b)
							b, e = reader.PeekByte(ind)
							ind++
						}

						if b == '>' || b == '/' {
							ind--
						}

						isVar := uint8(0)
						if len(key) >= 2 && key[0] == '{' && key[1] == '{' {
							key = key[2:]
							isVar++

							if len(key) >= 1 && key[0] == '{' {
								key = key[1:]
								isVar++
							}

							if b2, e2 := reader.Get(ind, 3); e2 == nil && b2[0] == '}' && b2[1] == '}' {
								ind += 2
								if b2[2] == '}' {
									ind++
								} else {
									isVar = 1
								}
							} else if len(val) >= 2 && val[len(val)-2] == '}' && val[len(val)-1] == '}' {
								val = val[:len(val)-2]
								if len(val) >= 1 && val[len(val)-1] == '}' {
									val = val[:len(val)-1]
								} else {
									isVar = 1
								}
							} else if isVar == 2 {
								key = append([]byte("{{{"), key...)
								isVar = 0
							} else {
								key = append([]byte("{{"), key...)
								isVar = 0
							}
						}

						if len(key) != 0 && key[len(key)-1] == '!' {
							key = key[:len(key)-1]
							val = append([]byte{'!'}, val...)
						}
						k := string(regex.Comp(`^([\w_-]+).*$`).RepStrCompRef(&key, []byte("$1")))
						if k == "" {
							k = string(regex.Comp(`^([\w_-]+).*$`).RepStrCompRef(&val, []byte("$1")))
						}

						if args.args[k] != nil {
							i := 1
							for args.args[k+":"+strconv.Itoa(i)] != nil {
								i++
							}
							args.args[k+":"+strconv.Itoa(i)] = append([]byte{isVar}, val...)
							args.ind = append(args.ind, k+":"+strconv.Itoa(i))
						} else {
							args.args[k] = append([]byte{isVar}, val...)
							args.ind = append(args.ind, k)
						}
					}

					// handle html tags
					if e == nil && args.close != 0 {
						reader.Discard(ind)

						// args.close:
						// 0 = failed to close (<tag)
						// 1 = </tag>
						// 2 = <tag/> (</tag/>)
						// 3 = <tag>

						if regex.Comp(`(?i)^_?(el(?:se|if)|if|else_?if)$`).MatchRef(&args.tag) {
							args.tag = bytes.ToLower(args.tag)

							if args.close == 3 && (bytes.Equal(args.tag, []byte("_if")) || bytes.Equal(args.tag, []byte("if"))) { // open tag
								if precompStr, ok := TagFuncs.If(options, &args, &eachArgsList, true); ok {
									if precompStr == nil {
										// grab if content and skip else content
										ifTagLevel = append(ifTagLevel, 0)
										removeLineBreak(reader)
									} else {
										// add string for compiler result and check else content
										write(regex.JoinBytes([]byte("{{%if "), precompStr, []byte("}}")))
										ifTagLevel = append(ifTagLevel, 2)
										hasUnhandledVars = true
									}
								} else {
									// skip if content and move on to next else tag
									ifTagLevel = append(ifTagLevel, 3)
									ib, ie := reader.PeekByte(0)
									ifLevel := 0
									for ie == nil {
										if ib == '"' || ib == '\'' || ib == '`' {
											q := ib
											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
											for ie == nil && ib != q {
												if ib == '\\' {
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
													if ie != nil {
														break
													}
												}
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
											}
										} else if ib == '<' {
											ibTag, ie := reader.Peek(11)
											if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
												break
											} else if (ie == nil || len(ibTag) > 4) && regex.Comp(`^</?_?(if)[\s/>:]`).MatchRef(&ibTag) {
												if ibTag[1] == '/' {
													ifLevel--
													if ifLevel < 0 {
														break
													}
												} else {
													ifLevel++
												}
											}
										}

										reader.Discard(1)
										ib, ie = reader.PeekByte(0)
									}
								}
							} else if args.close == 1 && len(ifTagLevel) != 0 && (bytes.Equal(args.tag, []byte("_if")) || bytes.Equal(args.tag, []byte("if"))) { // close tag
								if ifTagLevel[len(ifTagLevel)-1] == 1 || ifTagLevel[len(ifTagLevel)-1] == 2 {
									write([]byte("{{%/if}}"))
									hasUnhandledVars = true
								} else {
									removeLineBreak(reader)
								}
								ifTagLevel = ifTagLevel[:len(ifTagLevel)-1]
							} else if len(ifTagLevel) != 0 && regex.Comp(`(?i)^_?(el(?:se|if)|else_?if)$`).MatchRef(&args.tag) { // else tag
								if ifTagLevel[len(ifTagLevel)-1] == 0 || ifTagLevel[len(ifTagLevel)-1] == 1 { // true if statement
									// skip content to closing if tag
									ib, ie := reader.PeekByte(0)
									ifLevel := 0
									for ie == nil {
										if ib == '"' || ib == '\'' || ib == '`' {
											q := ib
											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
											for ie == nil && ib != q {
												if ib == '\\' {
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
													if ie != nil {
														break
													}
												}
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
											}
										} else if ib == '<' {
											ibTag, ie := reader.Peek(6)
											if ie == nil && ifLevel == 0 && regex.Comp(`^</_?if[\s/>:]`).MatchRef(&ibTag) {
												break
											} else if (ie == nil || len(ibTag) > 4) && regex.Comp(`^</?_?if[\s/>:]`).MatchRef(&ibTag) {
												if ibTag[1] == '/' {
													ifLevel--
													if ifLevel < 0 {
														ifLevel = 0
													}
												} else {
													ifLevel++
												}
											}
										}

										reader.Discard(1)
										ib, ie = reader.PeekByte(0)
									}
								} else if ifTagLevel[len(ifTagLevel)-1] == 2 { // string if statement
									if precompStr, ok := TagFuncs.If(options, &args, &eachArgsList, true); ok {
										if precompStr == nil {
											// grab content and skip next else content
											ifTagLevel[len(ifTagLevel)-1] = 1
											write([]byte("{{%else}}"))
										} else {
											// add string for compiler result and check else content
											write(regex.JoinBytes([]byte("{{%else "), precompStr, []byte("}}")))
										}
										hasUnhandledVars = true
									} else {
										// skip if content and move on to next else tag
										ib, ie := reader.PeekByte(0)
										ifLevel := 0
										for ie == nil {
											if ib == '"' || ib == '\'' || ib == '`' {
												q := ib
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
												for ie == nil && ib != q {
													if ib == '\\' {
														reader.Discard(1)
														ib, ie = reader.PeekByte(0)
														if ie != nil {
															break
														}
													}
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
												}
											} else if ib == '<' {
												ibTag, ie := reader.Peek(11)
												if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
													break
												} else if (ie == nil || len(ibTag) > 4) && regex.Comp(`^</?_?if[\s/>:]`).MatchRef(&ibTag) {
													if ibTag[1] == '/' {
														ifLevel--
														if ifLevel < 0 {
															break
														}
													} else {
														ifLevel++
													}
												}
											}

											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
										}
									}
								} else if ifTagLevel[len(ifTagLevel)-1] == 3 { // false if statement
									if precompStr, ok := TagFuncs.If(options, &args, &eachArgsList, true); ok {
										if precompStr == nil {
											// grab if content and skip else content
											ifTagLevel[len(ifTagLevel)-1] = 0
											removeLineBreak(reader)
										} else {
											// add string for compiler result and check else content
											ifTagLevel[len(ifTagLevel)-1] = 2
											write(regex.JoinBytes([]byte("{{%if "), precompStr, []byte("}}")))
											hasUnhandledVars = true
										}
									} else {
										// skip if content and move on to next else tag
										ib, ie := reader.PeekByte(0)
										ifLevel := 0
										for ie == nil {
											if ib == '"' || ib == '\'' || ib == '`' {
												q := ib
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
												for ie == nil && ib != q {
													if ib == '\\' {
														reader.Discard(1)
														ib, ie = reader.PeekByte(0)
														if ie != nil {
															break
														}
													}
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
												}
											} else if ib == '<' {
												ibTag, ie := reader.Peek(11)
												if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
													break
												} else if (ie == nil || len(ibTag) > 4) && regex.Comp(`^</?_?if[\s/>:]`).MatchRef(&ibTag) {
													if ibTag[1] == '/' {
														ifLevel--
														if ifLevel < 0 {
															break
														}
													} else {
														ifLevel++
													}
												}
											}

											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
										}
									}
								}
							}
						} else if regex.Comp(`(?i)^_?(each|for|for_?each)$`).MatchRef(&args.tag) {
							args.tag = bytes.ToLower(args.tag)

							if args.close == 3 {
								if args.args["0"] != nil && len(args.args["0"]) != 0 && args.args["0"][0] == 0 {
									if hasVarOpt(args.args["0"][1:], options, &eachArgsList, 0, true) {
										listArg := GetOpt(args.args["0"][1:], options, &eachArgsList, 0, true, false)
										if t := reflect.TypeOf(listArg); t == goutil.VarType["map[string]interface{}"] || t == goutil.VarType["[]interface{}"] {
											eachArgs := EachArgs{}
											if t == goutil.VarType["map[string]interface{}"] && len(listArg.(map[string]interface{})) != 0 {
												eachArgs.listMap = listArg.(map[string]interface{})
												eachArgs.listArr = []interface{}{}
												for k := range eachArgs.listMap {
													eachArgs.listArr = append(eachArgs.listArr, k)
												}

												sortStrings(&eachArgs.listArr)
											} else if t == goutil.VarType["[]interface{}"] && len(listArg.([]interface{})) != 0 {
												eachArgs.listArr = listArg.([]interface{})
											} else {
												// skip each content and move on to closing each tag
												ifTagLevel = append(ifTagLevel, 3)
												ib, ie := reader.PeekByte(0)
												ifLevel := 0
												for ie == nil {
													if ib == '"' || ib == '\'' || ib == '`' {
														q := ib
														reader.Discard(1)
														ib, ie = reader.PeekByte(0)
														for ie == nil && ib != q {
															if ib == '\\' {
																reader.Discard(1)
																ib, ie = reader.PeekByte(0)
																if ie != nil {
																	break
																}
															}
															reader.Discard(1)
															ib, ie = reader.PeekByte(0)
														}
													} else if ib == '<' {
														ibTag, ie := reader.Peek(12)
														if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
															break
														} else if (ie == nil || len(ibTag) > 5) && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
															if ibTag[1] == '/' {
																ifLevel--
																if ifLevel < 0 {
																	break
																}
															} else {
																ifLevel++
															}
														}
													}

													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
												}
												continue
											}

											eachArgs.size = uint(len(eachArgs.listArr))

											if args.args["key"] != nil && len(args.args["key"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.key = args.args["key"][1:]
											} else if args.args["of"] != nil && len(args.args["of"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.key = args.args["of"][1:]
											}

											if args.args["value"] != nil && len(args.args["value"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.val = args.args["value"][1:]
											} else if args.args["as"] != nil && len(args.args["as"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.val = args.args["as"][1:]
											}

											eachArgsList = append(eachArgsList, eachArgs)
											reader.Save()

											removeLineBreak(reader)
											continue
										}
									} else {
										// return new each function to run in compiler
										argStr := args.args["0"][1:]
										eachArgs := EachArgs{passToComp: true}

										if args.args["value"] != nil && len(args.args["value"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["value"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										} else if args.args["val"] != nil && len(args.args["val"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["val"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										} else if args.args["as"] != nil && len(args.args["as"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["as"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										}

										if args.args["key"] != nil && len(args.args["key"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.key = args.args["key"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" of=\""), eachArgs.key, '"')
										} else if args.args["of"] != nil && len(args.args["of"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.key = args.args["of"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" of=\""), eachArgs.key, '"')
										}

										eachArgsList = append(eachArgsList, eachArgs)
										write(regex.JoinBytes([]byte("{{%each"), ' ', argStr, []byte("}}")))
										hasUnhandledVars = true

										continue
									}
								}

								// skip each content and move on to closing each tag
								ifTagLevel = append(ifTagLevel, 3)
								ib, ie := reader.PeekByte(0)
								ifLevel := 0
								for ie == nil {
									if ib == '"' || ib == '\'' || ib == '`' {
										q := ib
										reader.Discard(1)
										ib, ie = reader.PeekByte(0)
										for ie == nil && ib != q {
											if ib == '\\' {
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
												if ie != nil {
													break
												}
											}
											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
										}
									} else if ib == '<' {
										ibTag, ie := reader.Peek(12)
										if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
											break
										} else if (ie == nil || len(ibTag) > 5) && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
											if ibTag[1] == '/' {
												ifLevel--
												if ifLevel < 0 {
													break
												}
											} else {
												ifLevel++
											}
										}
									}

									reader.Discard(1)
									ib, ie = reader.PeekByte(0)
								}
							} else if args.close == 1 {
								if len(eachArgsList) != 0 {
									if eachArgsList[len(eachArgsList)-1].passToComp {
										eachArgsList = eachArgsList[:len(eachArgsList)-1]
										write([]byte("{{%/each}}"))
										hasUnhandledVars = true
									} else if eachArgsList[len(eachArgsList)-1].ind < eachArgsList[len(eachArgsList)-1].size-1 {
										if eachArgsList[len(eachArgsList)-1].ind == 0 {
											reader.Restore()
											removeLineBreak(reader)
										} else {
											reader.RestoreReset()
											removeLineBreak(reader)
										}
										eachArgsList[len(eachArgsList)-1].ind++
									} else {
										reader.DelSave()
										removeLineBreak(reader)
										eachArgsList = eachArgsList[:len(eachArgsList)-1]
									}
								}
							}
						} else if args.tag[0] == '_' && len(args.tag) > 1 {
							args.tag = bytes.ToLower(args.tag)
							args.tag[1] = bytes.ToUpper([]byte{args.tag[1]})[0]

							if args.close == 3 {
								htmlContTempTag = append(htmlContTempTag, args)
								htmlContTemp = append(htmlContTemp, []byte{})
							} else if args.close == 1 && len(htmlContTempTag) != 0 {
								for i := len(htmlContTempTag) - 1; i >= 0; i-- {
									sameTag := bytes.Equal(htmlContTempTag[i].tag, args.tag)

									fn, isSync, fnErr := getCoreTagFunc(htmlContTempTag[i].tag)
									if fnErr != nil {
										if newFn, ok := TagFuncs.list[string(htmlContTempTag[i].tag)]; ok {
											fn = newFn
											fnErr = nil
										}
									}

									if fnErr == nil {
										for k, v := range htmlContTempTag[i].args {
											args.args[k] = v
										}
										args.args["body"] = htmlContTemp[i]

										htmlContTemp = htmlContTemp[:i]
										htmlContTempTag = htmlContTempTag[:i]

										htmlCont := []byte{0}
										var compErr error
										htmlTags = append(htmlTags, &htmlCont)
										htmlTagsErr = append(htmlTagsErr, &compErr)

										if htmlChan != nil && !isSync {
											htmlChan.fn <- handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars}
										} else {
											handleHtmlFunc(handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars})
										}
										write([]byte{0})
									} else {
										if i != 0 && !sameTag && len(htmlContTemp[i]) != 0 {
											if len(htmlContTemp[i]) != 0 && htmlContTemp[i][0] == '\r' {
												htmlContTemp[i] = htmlContTemp[i][1:]
											}
											if len(htmlContTemp[i]) != 0 && htmlContTemp[i][0] == '\n' {
												htmlContTemp[i] = htmlContTemp[i][1:]
											}
											htmlContTemp[i-1] = append(htmlContTemp[i-1], htmlContTemp[i]...)
										}
										htmlContTemp = htmlContTemp[:i]
										htmlContTempTag = htmlContTempTag[:i]
									}

									if sameTag {
										break
									}
								}
							} else if args.close == 2 {
								fn, isSync, fnErr := getCoreTagFunc(args.tag)
								if fnErr != nil {
									if newFn, ok := TagFuncs.list[string(args.tag)]; ok {
										fn = newFn
										fnErr = nil
									}
								}

								if fnErr == nil {
									htmlCont := []byte{0}
									var compErr error
									htmlTags = append(htmlTags, &htmlCont)
									htmlTagsErr = append(htmlTagsErr, &compErr)

									if htmlChan != nil && !isSync {
										htmlChan.fn <- handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars}
									} else {
										handleHtmlFunc(handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars})
									}
									write([]byte{0})
								}
							}
						} else if args.tag[0] == bytes.ToUpper([]byte{args.tag[0]})[0] {
							if args.close == 3 {
								htmlContTempTag = append(htmlContTempTag, args)
								htmlContTemp = append(htmlContTemp, []byte{})
							} else if args.close == 1 && bytes.Equal(args.tag, htmlContTempTag[len(htmlContTemp)-1].tag) {
								for k, v := range htmlContTempTag[len(htmlContTemp)-1].args {
									args.args[k] = v
								}

								// merge html tags to component body
								if len(htmlContTemp[len(htmlContTempTag)-1]) != 0 {
									args.args["body"] = []byte{}
									body := htmlContTemp[len(htmlContTempTag)-1]

									count := bytes.Count(htmlContTemp[len(htmlContTempTag)-1], []byte{0})
									ind := 0
									i := bytes.IndexByte(body, 0)
									for i != -1 {
										args.args["body"] = append(args.args["body"], body[:i]...)
										body = body[i+1:]

										if ind >= count {
											break
										}

										cont := htmlTags[len(htmlTags)-(count-ind)]
										if len(*cont) == 0 {
											i = bytes.IndexByte(body, 0)
											ind++
											continue
										}

										for (*cont)[0] == 0 {
											time.Sleep(1 * time.Nanosecond)
										}

										if (*cont)[0] == 2 {
											*compileError = *htmlTagsErr[len(htmlTagsErr)-(count-ind)]
											(*html)[0] = 2
											return
										}

										args.args["body"] = append(args.args["body"], (*cont)[1:]...)

										i = bytes.IndexByte(body, 0)
										ind++
									}

									args.args["body"] = append(args.args["body"], body...)

									htmlTags = htmlTags[:len(htmlTags)-count]
									htmlTagsErr = htmlTagsErr[:len(htmlTagsErr)-count]
								}

								htmlContTemp = htmlContTemp[:len(htmlContTempTag)-1]
								htmlContTempTag = htmlContTempTag[:len(htmlContTempTag)-1]

								htmlCont := []byte{0}
								var compErr error
								htmlTags = append(htmlTags, &htmlCont)
								htmlTagsErr = append(htmlTagsErr, &compErr)

								if htmlChan != nil {
									htmlChan.comp <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars, localRoot: &localRoot}
								} else {
									handleHtmlComponent(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars, localRoot: &localRoot})
								}
								write([]byte{0})
							} else if args.close == 2 {
								htmlCont := []byte{0}
								var compErr error
								htmlTags = append(htmlTags, &htmlCont)
								htmlTagsErr = append(htmlTagsErr, &compErr)

								if htmlChan != nil {
									htmlChan.comp <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars, localRoot: &localRoot}
								} else {
									handleHtmlComponent(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList, hasUnhandledVars: &hasUnhandledVars, localRoot: &localRoot})
								}
								write([]byte{0})
							}
						} else {
							// handle normal tags
							if (args.close == 3 || args.close == 1) && goutil.Contains(singleHtmlTags, bytes.ToLower(args.tag)) {
								args.close = 2
							}

							htmlCont := []byte{0}
							var compErr error
							if len(htmlContTemp) != 0 {
								handleHtmlTag(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, hasUnhandledVars: &hasUnhandledVars})
								if htmlCont[0] == 2 {
									*compileError = compErr
									(*html)[0] = 2
									return
								}

								write(htmlCont[1:])
							} else {
								htmlTags = append(htmlTags, &htmlCont)
								htmlTagsErr = append(htmlTagsErr, &compErr)

								// pass through channel instead of a goroutine (like a queue)
								if htmlChan != nil {
									htmlChan.tag <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, hasUnhandledVars: &hasUnhandledVars}
								} else {
									handleHtmlTag(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, hasUnhandledVars: &hasUnhandledVars})
								}
								write([]byte{0})
							}
						}

						continue
					}
				}
			}
		} else if buf == '{' { // handle html vars
			ind := uint(3)
			if b, e := reader.Peek(3); e == nil {
				if b[0] == '{' && b[1] == '{' {
					esc := uint8(2)
					if b[2] == '{' {
						esc = 0
					} else {
						ind--
					}

					b, e = reader.Get(ind, 2)
					for e == nil && !(b[0] == '}' && b[1] == '}') && b[0] != '\r' && b[0] != '\n' {
						if b[0] == '"' || b[0] == '\'' || b[0] == '`' {
							q := b[0]
							ind++
							b, e = reader.Get(ind, 2)
							for e == nil && b[0] != q {
								if b[0] == '\\' {
									ind++
								}
								ind++
								b, e = reader.Get(ind, 2)
							}
						}
						ind++
						b, e = reader.Get(ind, 2)
					}

					if e == nil && b[0] == '}' && b[1] == '}' {
						if esc == 0 {
							reader.Discard(3)
						} else {
							reader.Discard(2)
						}

						if esc == 0 {
							b, e = reader.Peek(ind - 3)
							reader.Discard(ind - 1)
						} else {
							b, e = reader.Peek(ind - 2)
							reader.Discard(ind)
						}

						if p, e := reader.PeekByte(0); e == nil {
							if p == '}' {
								reader.Discard(1)
							} else {
								esc = 2
							}
						} else {
							esc = 2
						}

						val := GetOpt(b, options, &eachArgsList, esc, true, true)
						if !goutil.IsZeroOfUnderlyingType(val) {
							if len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
								v := val.([]byte)[1:]
								if !regex.Comp(`(?i)^\{\{\{?body\}\}\}?$`).MatchRef(&v) {
									write(v)
									hasUnhandledVars = true
								} else {
									removeLineBreak(reader)
								}
							} else {
								write(val.([]byte))
							}
						}

						continue
					}
				}
			}
		} else if buf == '/' {
			c, e := reader.PeekByte(1)
			if e == nil && (c == '/' || c == '*') {
				reader.Discard(2)
				commentData := []byte{}

				b, e := reader.Peek(2)
				for e == nil && ((c == '/' && b[0] != '\n') || (c == '*' && !(b[0] == '*' && b[1] == '/'))) {
					commentData = append(commentData, b[0])
					reader.Discard(1)
					b, e = reader.Peek(2)
				}

				if e == nil {
					if c == '*' {
						reader.Discard(2)
					} else if c == '\n' {
						reader.Discard(1)
					}
				}

				if compilerConfig.DebugMode {
					wC := []byte{}
					if c == '*' {
						wC = []byte("*/")
					} else if c == '\n' {
						wC = []byte{'\n'}
					}
					write(regex.JoinBytes('/', c, commentData, wC))
				}

				continue
			}
		}

		//todo: add optional shortcode handler (ie: {{#shortcode@plugin}} {{#priorityShortcode}}) ("@plugin" should be optional)
		// may add in a "@shortcodes" option to options, and pass in a list of functions that return html/markdown
		// may also add a mothod for shortcodes to run other shortcodes (apart from themselves to avoid recursion)
		// may have shortcodes run in elixir or another lightweight programming language (may also add subfolder for shortcodes)

		//todo: consider using 'AspieSoft/go-memshare' module if a funcs.go file is detected in the $PWD directory and link it to the TagFuncs.AddFN method

		// handle markdown
		if compileMarkdown(reader, &write, &firstChar, &spaces, &mdStore) {
			continue
		}

		firstChar = false
		write([]byte{buf})
		reader.Discard(1)
	}

	// stop concurrent channels from running
	if htmlChan != nil {
		htmlChan.tag <- handleHtmlData{stopChan: true}
		htmlChan.comp <- handleHtmlData{stopChan: true}
		htmlChan.fn <- handleHtmlData{stopChan: true}
	}

	// merge html tags when done
	htmlTagsInd := 0
	i := bytes.IndexByte(htmlRes, 0)
	for i != -1 {
		*html = append(*html, htmlRes[:i]...)
		htmlRes = htmlRes[i+1:]

		if htmlTagsInd >= len(htmlTags) {
			break
		}

		htmlCont := htmlTags[htmlTagsInd]
		for (*htmlCont)[0] == 0 {
			if htmlChan != nil && *htmlChan.running == 0 {
				break
			}
			time.Sleep(10 * time.Nanosecond)
		}

		if (*htmlCont)[0] == 2 {
			*compileError = *htmlTagsErr[htmlTagsInd]
			(*html)[0] = 2
			return
		}

		if (*htmlCont)[0] == 4 {
			hasUnhandledVars = true
		}

		*html = append(*html, (*htmlCont)[1:]...)
		htmlTagsInd++

		i = bytes.IndexByte(htmlRes, 0)
	}

	*html = append(*html, htmlRes...)

	*html = regex.Comp(`\s+$`).RepStrRef(html, []byte{'\n'})

	/* if regex.Comp(`(?i)<[^\w<>]*(?:[^<>"'\'\s]*:)?[^\w<>]*(?:\W*s\W*c\W*r\W*i\W*p\W*t|\W*f\W*o\W*r\W*m|\W*s\W*t\W*y\W*l\W*e|\W*s\W*v\W*g|\W*m\W*a\W*r\W*q\W*u\W*e\W*e|(?:\W*l\W*i\W*n\W*k|\W*o\W*b\W*j\W*e\W*c\W*t|\W*e\W*m\W*b\W*e\W*d|\W*a\W*p\W*p\W*l\W*e\W*t|\W*p\W*a\W*r\W*a\W*m|\W*i?\W*f\W*r\W*a\W*m\W*e|\W*b\W*a\W*s\W*e|\W*b\W*o\W*d\W*y|\W*m\W*e\W*t\W*a|\W*i\W*m\W*a?\W*g\W*e?|\W*v\W*i\W*d\W*e\W*o|\W*a\W*u\W*d\W*i\W*o|\W*b\W*i\W*n\W*d\W*i\W*n\W*g\W*s|\W*s\W*e\W*t|\W*i\W*s\W*i\W*n\W*d\W*e\W*x|\W*a\W*n\W*i\W*m\W*a\W*t\W*e)[^>\w])|(?:<\w[\s\S]*[\s\0\/]|["'\'])(?:formaction|style|background|src|lowsrc|ping|on(?:d(?:e(?:vice(?:(?:orienta|mo)tion|proximity|found|light)|livery(?:success|error)|activate)|r(?:ag(?:e(?:n(?:ter|d)|xit)|(?:gestur|leav)e|start|drop|over)?|op)|i(?:s(?:c(?:hargingtimechange|onnect(?:ing|ed))|abled)|aling)|ata(?:setc(?:omplete|hanged)|(?:availabl|chang)e|error)|urationchange|ownloading|blclick)|Moz(?:M(?:agnifyGesture(?:Update|Start)?|ouse(?:PixelScroll|Hittest))|S(?:wipeGesture(?:Update|Start|End)?|crolledAreaChanged)|(?:(?:Press)?TapGestur|BeforeResiz)e|EdgeUI(?:C(?:omplet|ancel)|Start)ed|RotateGesture(?:Update|Start)?|A(?:udioAvailable|fterPaint))|c(?:o(?:m(?:p(?:osition(?:update|start|end)|lete)|mand(?:update)?)|n(?:t(?:rolselect|extmenu)|nect(?:ing|ed))|py)|a(?:(?:llschang|ch)ed|nplay(?:through)?|rdstatechange)|h(?:(?:arging(?:time)?ch)?ange|ecking)|(?:fstate|ell)change|u(?:echange|t)|l(?:ick|ose))|m(?:o(?:z(?:pointerlock(?:change|error)|(?:orientation|time)change|fullscreen(?:change|error)|network(?:down|up)load)|use(?:(?:lea|mo)ve|o(?:ver|ut)|enter|wheel|down|up)|ve(?:start|end)?)|essage|ark)|s(?:t(?:a(?:t(?:uschanged|echange)|lled|rt)|k(?:sessione|comma)nd|op)|e(?:ek(?:complete|ing|ed)|(?:lec(?:tstar)?)?t|n(?:ding|t))|u(?:ccess|spend|bmit)|peech(?:start|end)|ound(?:start|end)|croll|how)|b(?:e(?:for(?:e(?:(?:scriptexecu|activa)te|u(?:nload|pdate)|p(?:aste|rint)|c(?:opy|ut)|editfocus)|deactivate)|gin(?:Event)?)|oun(?:dary|ce)|l(?:ocked|ur)|roadcast|usy)|a(?:n(?:imation(?:iteration|start|end)|tennastatechange)|fter(?:(?:scriptexecu|upda)te|print)|udio(?:process|start|end)|d(?:apteradded|dtrack)|ctivate|lerting|bort)|DOM(?:Node(?:Inserted(?:IntoDocument)?|Removed(?:FromDocument)?)|(?:CharacterData|Subtree)Modified|A(?:ttrModified|ctivate)|Focus(?:Out|In)|MouseScroll)|r(?:e(?:s(?:u(?:m(?:ing|e)|lt)|ize|et)|adystatechange|pea(?:tEven)?t|movetrack|trieving|ceived)|ow(?:s(?:inserted|delete)|e(?:nter|xit))|atechange)|p(?:op(?:up(?:hid(?:den|ing)|show(?:ing|n))|state)|a(?:ge(?:hide|show)|(?:st|us)e|int)|ro(?:pertychange|gress)|lay(?:ing)?)|t(?:ouch(?:(?:lea|mo)ve|en(?:ter|d)|cancel|start)|ime(?:update|out)|ransitionend|ext)|u(?:s(?:erproximity|sdreceived)|p(?:gradeneeded|dateready)|n(?:derflow|load))|f(?:o(?:rm(?:change|input)|cus(?:out|in)?)|i(?:lterchange|nish)|ailed)|l(?:o(?:ad(?:e(?:d(?:meta)?data|nd)|start)?|secapture)|evelchange|y)|g(?:amepad(?:(?:dis)?connected|button(?:down|up)|axismove)|et)|e(?:n(?:d(?:Event|ed)?|abled|ter)|rror(?:update)?|mptied|xit)|i(?:cc(?:cardlockerror|infochange)|n(?:coming|valid|put))|o(?:(?:(?:ff|n)lin|bsolet)e|verflow(?:changed)?|pen)|SVG(?:(?:Unl|L)oad|Resize|Scroll|Abort|Error|Zoom)|h(?:e(?:adphoneschange|l[dp])|ashchange|olding)|v(?:o(?:lum|ic)e|ersion)change|w(?:a(?:it|rn)ing|heel)|key(?:press|down|up)|(?:AppComman|Loa)d|no(?:update|match)|Request|zoom))[\s\0]*=
	`).MatchRef(html) {
	    *compileError = errors.New("warning: xss injection was detected")
	    (*html)[0] = 2
	    return
	  } */

	if !hasUnhandledVars {
		// can be added to static html (and gzipped)
		(*html)[0] = 3
	} else {
		(*html)[0] = 1
	}
}

func handleHtmlTag(htmlData handleHtmlData) {
	//htmlData: html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error

	// auto fix "emptyContentTags" to closing (ie: <script/> <iframe/>)
	closeEnd := false
	for _, tag := range emptyContentTags {
		if bytes.Equal(tag.tag, htmlData.arguments.tag) {
			if tag.attr != nil {
				if _, ok := htmlData.arguments.args[string(tag.attr)]; ok {
					htmlData.arguments.close = 3
					closeEnd = true
				} else {
					(*htmlData.html)[0] = 1
					return
				}
			} else {
				htmlData.arguments.close = 3
				closeEnd = true
			}

			break
		}
	}

	if htmlData.arguments.close == 1 {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes([]byte{'<', '/'}, htmlData.arguments.tag, '>')...)
		(*htmlData.html)[0] = 1
		return
	}

	sort.Strings(htmlData.arguments.ind)

	for _, v := range htmlData.arguments.ind {
		if htmlData.arguments.args[v][0] == 0 {
			htmlData.arguments.args[v] = htmlData.arguments.args[v][1:]
		} else if htmlData.arguments.args[v][0] == 1 {
			esc := uint8(3)
			if _, err := strconv.Atoi(v); err == nil {
				esc = 4
			}

			arg := GetOpt(htmlData.arguments.args[v][1:], htmlData.options, &htmlData.eachArgs, esc, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(htmlData.arguments.args, v)
				continue
			} else {
				if len(arg.([]byte)) != 0 && arg.([]byte)[0] == 0 {
					*htmlData.hasUnhandledVars = true
				}
				htmlData.arguments.args[v] = arg.([]byte)
			}
		} else if htmlData.arguments.args[v][0] == 2 {
			arg := GetOpt(htmlData.arguments.args[v][1:], htmlData.options, &htmlData.eachArgs, 1, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(htmlData.arguments.args, v)
				continue
			} else {
				if len(arg.([]byte)) != 0 && arg.([]byte)[0] == 0 {
					*htmlData.hasUnhandledVars = true
				}
				htmlData.arguments.args[v] = arg.([]byte)
			}
		}

		if regex.Comp(`:([0-9]+)$`).Match([]byte(v)) {
			k := string(regex.Comp(`:([0-9]+)$`).RepStr([]byte(v), []byte{}))
			if htmlData.arguments.args[k] == nil {
				htmlData.arguments.args[k] = []byte{}
			}
			htmlData.arguments.args[k] = append(append(htmlData.arguments.args[k], ' '), htmlData.arguments.args[v]...)
			delete(htmlData.arguments.args, v)
		}
	}

	args := [][]byte{}
	for _, v := range htmlData.arguments.ind {
		if htmlData.arguments.args[v] != nil && len(htmlData.arguments.args[v]) != 0 {
			if _, err := strconv.Atoi(v); err == nil {
				if bytes.HasPrefix(htmlData.arguments.args[v], []byte{0, '{', '{'}) && bytes.HasSuffix(htmlData.arguments.args[v], []byte("}}")) {
					htmlData.arguments.args[v] = htmlData.arguments.args[v][1:]
				} else {
					htmlData.arguments.args[v] = regex.Comp(`({{+|}}+)`).RepFunc(htmlData.arguments.args[v], func(data func(int) []byte) []byte {
						return bytes.Join(bytes.Split(data(1), []byte{}), []byte{'\\'})
					})
				}
				args = append(args, htmlData.arguments.args[v])
			} else {
				if bytes.HasPrefix(htmlData.arguments.args[v], []byte{0, '{', '{'}) && bytes.HasSuffix(htmlData.arguments.args[v], []byte("}}")) {
					htmlData.arguments.args[v] = htmlData.arguments.args[v][1:]

					size := 2
					if htmlData.arguments.args[v][2] == '{' && htmlData.arguments.args[v][len(htmlData.arguments.args[v])-3] == '}' {
						size = 3
					}

					if htmlData.arguments.args[v][size] == '=' {
						args = append(args, regex.JoinBytes(bytes.Repeat([]byte("{"), size), v, htmlData.arguments.args[v][size:len(htmlData.arguments.args[v])-size], bytes.Repeat([]byte("}"), size)))
					}
				} else {
					htmlData.arguments.args[v] = regex.Comp(`({{+|}}+)`).RepFunc(htmlData.arguments.args[v], func(data func(int) []byte) []byte {
						return bytes.Join(bytes.Split(data(1), []byte{}), []byte{'\\'})
					})

					// check local js and css link args for .min files (unless in debug mode)
					// also check for .webp, .webm, and .weba files
					if !compilerConfig.DebugMode && (v == "src" || v == "href" || v == "url") && len(htmlData.arguments.args[v]) != 0 && htmlData.arguments.args[v][0] == '/' {
						link := htmlData.arguments.args[v]
						if regex.Comp(`(\.min|)\.([jt]s|css|less|s[ac]ss)$`).MatchRef(&link) {
							link = regex.Comp(`(\.min|)\.([jt]s|css|less|s[ac]ss)$`).RepFuncRef(&link, func(data func(int) []byte) []byte {
								ext := data(2)
								if bytes.Equal(ext, []byte("ts")) {
									//todo: add support for auto compiling typescript to javascript
									// remove this first if condition when done (and let the file extension be updated to js)
									htmlData.arguments.args["type"] = []byte("text/typescript")
								} else if regex.Comp(`([jt]s)`).MatchRef(&ext) {
									ext = []byte("js")
								} else if regex.Comp(`(css|less|s[ac]ss)`).MatchRef(&ext) {
									ext = []byte("css")
								}

								return regex.JoinBytes([]byte(".min."), ext)
							})
							if linkPath, err := goutil.FS.JoinPath(compilerConfig.Static, string(link)); err == nil {
								if stat, err := os.Stat(linkPath); err == nil && !stat.IsDir() {
									htmlData.arguments.args[v] = link
								}
							}
						} else if imageRE.MatchRef(&link) {
							link = imageRE.RepStrRef(&link, []byte(".webp"))
							if linkPath, err := goutil.FS.JoinPath(compilerConfig.Static, string(link)); err == nil {
								if stat, err := os.Stat(linkPath); err == nil && !stat.IsDir() {
									htmlData.arguments.args[v] = link
								}
							}
						} else if videoRE.MatchRef(&link) {
							link = videoRE.RepStrRef(&link, []byte(".webm"))
							if linkPath, err := goutil.FS.JoinPath(compilerConfig.Static, string(link)); err == nil {
								if stat, err := os.Stat(linkPath); err == nil && !stat.IsDir() {
									htmlData.arguments.args[v] = link
								}
							}
						} else if audioRE.MatchRef(&link) {
							link = audioRE.RepStrRef(&link, []byte(".weba"))
							if linkPath, err := goutil.FS.JoinPath(compilerConfig.Static, string(link)); err == nil {
								if stat, err := os.Stat(linkPath); err == nil && !stat.IsDir() {
									htmlData.arguments.args[v] = link
								}
							}
						}
					}

					args = append(args, regex.JoinBytes(v, []byte{'=', '"'}, goutil.HTML.EscapeArgs(htmlData.arguments.args[v], '"'), '"'))
				}

			}
		}
	}

	sort.Slice(args, func(i, j int) bool {
		a := bytes.Split(args[i], []byte{'='})[0]
		b := bytes.Split(args[j], []byte{'='})[0]

		if a[0] == 0 {
			a = a[1:]
		}
		if b[0] == 0 {
			b = b[1:]
		}

		a = bytes.Trim(a, "{}")
		b = bytes.Trim(b, "{}")

		if a[0] != ':' && b[0] == ':' {
			return true
		}

		return bytes.Compare(a, b) == -1
	})

	if len(args) == 0 {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes('<', htmlData.arguments.tag)...)
	} else {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes('<', htmlData.arguments.tag, ' ', bytes.Join(args, []byte{' '}))...)
	}

	if htmlData.arguments.close == 2 {
		(*htmlData.html) = append((*htmlData.html), '/', '>')
	} else {
		(*htmlData.html) = append((*htmlData.html), '>')
	}

	if closeEnd {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes('<', '/', htmlData.arguments.tag, '>')...)
	}

	(*htmlData.html)[0] = 1
}

func handleHtmlFunc(htmlData handleHtmlData) {
	//htmlData: fn *func(/*tag function args*/)[]byte, preComp bool, html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error

	res := (*htmlData.fn)(htmlData.options, htmlData.arguments, &htmlData.eachArgs, htmlData.preComp)
	if res != nil && len(res) != 0 {
		if res[0] == 0 {
			if htmlData.preComp {
				if body, ok := htmlData.arguments.args["body"]; ok {
					*htmlData.html = append(*htmlData.html, regex.JoinBytes([]byte("{{%"), htmlData.arguments.tag[1:], ' ', res[1:], []byte("}}"), body, []byte("{{%/"), htmlData.arguments.tag[1:], []byte("}}"))...)
				} else {
					*htmlData.html = append(*htmlData.html, regex.JoinBytes([]byte("{{%"), htmlData.arguments.tag[1:], ' ', res[1:], []byte("/}}"))...)
				}
			}
		} else if res[0] == 1 {
			*htmlData.compileError = errors.New(string(res[1:]))
			(*htmlData.html)[0] = 2
			return
		} else {
			*htmlData.html = append(*htmlData.html, res...)
		}
	}

	// set first index to 1 to mark as ready
	(*htmlData.html)[0] = 1
}

func handleHtmlComponent(htmlData handleHtmlData) {
	//htmlData: html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error, componentList [][]byte

	// note: components cannot wait in the same channel as their parents without possibly getting stuck (ie: waiting for a parent that is also waiting for itself)

	for _, tag := range htmlData.componentList {
		if bytes.Equal(htmlData.arguments.tag, tag) {
			*htmlData.compileError = errors.New("recursion detected in component:\n  in: '" + string(htmlData.componentList[len(htmlData.componentList)-1]) + "'\n  with: '" + string(htmlData.arguments.tag) + "'\n  contains:\n    '" + string(bytes.Join(htmlData.componentList, []byte("'\n    '"))) + "'\n")
			(*htmlData.html)[0] = 2
			return
		}
	}

	// get component filepath
	path := string(regex.Comp(`\.`).RepStr(regex.Comp(`[^\w_\-\.]`).RepStrRef(&htmlData.arguments.tag, []byte{}), []byte{'/'}))
	oPath := path

	var err error
	if *htmlData.localRoot != "" {
		path, err = goutil.FS.JoinPath(compilerConfig.Root, *htmlData.localRoot, path+"."+compilerConfig.Ext)
	} else {
		path, err = goutil.FS.JoinPath(compilerConfig.Root, path+"."+compilerConfig.Ext)
	}

	if err != nil {
		*htmlData.compileError = err
		(*htmlData.html)[0] = 2
		return
	}

	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		if compilerConfig.IncludeMD {
			if *htmlData.localRoot != "" {
				path, err = goutil.FS.JoinPath(compilerConfig.Root, *htmlData.localRoot, oPath+".md")
			} else {
				path, err = goutil.FS.JoinPath(compilerConfig.Root, oPath+".md")
			}

			if err != nil {
				*htmlData.compileError = errors.New(string(regex.Comp(`\.md:`).RepStr([]byte(err.Error()), []byte("."+compilerConfig.Ext))))
				(*htmlData.html)[0] = 2
				return
			}

			if stat, err := os.Stat(path); err != nil || stat.IsDir() {
				*htmlData.compileError = errors.New("component not found: '" + string(htmlData.arguments.tag) + "'")
				(*htmlData.html)[0] = 2
				return
			}
		} else {
			*htmlData.compileError = errors.New("component not found: '" + string(htmlData.arguments.tag) + "'")
			(*htmlData.html)[0] = 2
			return
		}
	}

	// merge options with html args
	opts, err := goutil.JSON.DeepCopy(*htmlData.options)
	if err != nil {
		opts = map[string]interface{}{}
	}

	htmlData.componentList = append(htmlData.componentList, htmlData.arguments.tag)

	// precompile component
	preCompile(path, &opts, htmlData.arguments, htmlData.html, htmlData.compileError, nil, htmlData.eachArgs, htmlData.componentList)
	if *htmlData.compileError != nil {
		(*htmlData.html)[0] = 2
		return
	}

	// set first index to 1 to mark as ready
	if (*htmlData.html)[0] == 1 {
		(*htmlData.html)[0] = 4
	} else {
		(*htmlData.html)[0] = 1
	}
}

func newPreCompileChan() htmlChanList {
	tagChan := make(chan handleHtmlData)
	compChan := make(chan handleHtmlData)
	fnChan := make(chan handleHtmlData)

	running := uint8(3)
	mu := sync.Mutex{}

	go func() {
		for {
			handleHtml := <-tagChan
			if handleHtml.stopChan {
				break
			}

			handleHtmlTag(handleHtml)
		}

		mu.Lock()
		running--
		mu.Unlock()
	}()

	go func() {
		for {
			handleHtml := <-compChan
			if handleHtml.stopChan {
				break
			}

			handleHtmlComponent(handleHtml)
		}

		mu.Lock()
		running--
		mu.Unlock()
	}()

	go func() {
		for {
			handleHtml := <-fnChan
			if handleHtml.stopChan {
				break
			}

			handleHtmlFunc(handleHtml)
		}

		mu.Lock()
		running--
		mu.Unlock()
	}()

	return htmlChanList{tag: tagChan, comp: compChan, fn: fnChan, running: &running}
}

// getCoreTagFunc returns a tag function based on the name
//
// @bool: isSync
func getCoreTagFunc(name []byte) (func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte, bool, error) {
	if name[0] == '_' {
		name = name[1:]
	}
	nameStr := string(regex.Compile(`[^\w_]`).RepStrRef(&name, []byte{}))

	isSync := false

	found := true
	m := reflect.ValueOf(&TagFuncs).MethodByName(nameStr)
	if goutil.IsZeroOfUnderlyingType(m) {
		m = reflect.ValueOf(&TagFuncs).MethodByName(nameStr + "_SYNC")
		if goutil.IsZeroOfUnderlyingType(m) {
			found = false
		} else {
			isSync = true
		}
	}

	if !found {
		return nil, false, errors.New("method '" + nameStr + "' does not exist in Compiled Functions")
	}

	if fn, ok := m.Interface().(func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte); ok {
		return fn, isSync, nil
	}

	return nil, false, errors.New("method '" + nameStr + "' does not return the expected args")
}

var ranTypeScriptNotice bool = false

// tryMinifyFile attempts to minify files
//
// example: .js -> .min.js, .less -> .min.css, .png -> .webp
func tryMinifyFile(path string) {
	if imageRE.Match([]byte(path)) {
		resPath := string(regex.Comp(`\.([\w_-]+)$`).RepStr([]byte(path), []byte(".webp")))
		if err := ffmpeg.Input(path).Output(resPath).OverWriteOutput().Run(); err != nil {
			os.Remove(resPath)
		}
		return
	} else if videoRE.Match([]byte(path)) {
		resPath := string(regex.Comp(`\.([\w_-]+)$`).RepStr([]byte(path), []byte(".webm")))
		if err := ffmpeg.Input(path).Output(resPath).OverWriteOutput().Run(); err != nil {
			os.Remove(resPath)
		}
		return
	} else if audioRE.Match([]byte(path)) {
		resPath := string(regex.Comp(`\.([\w_-]+)$`).RepStr([]byte(path), []byte(".weba")))
		if err := ffmpeg.Input(path).Output(resPath).OverWriteOutput().Run(); err != nil {
			os.Remove(resPath)
		}
		return
	}

	resPath := string(regex.Comp(`(?<!\.min)\.([jt]s|css|less|s[ac]ss)$`).RepFunc([]byte(path), func(data func(int) []byte) []byte {
		ext := data(1)
		if regex.Comp(`^([jt]s)$`).MatchRef(&ext) {
			return []byte(".min.js")
		} else if regex.Comp(`^(css|less|s[ac]ss)$`).MatchRef(&ext) {
			return []byte(".min.css")
		}
		return regex.JoinBytes([]byte(".min"), ext)
	}))
	if code, err := os.ReadFile(path); err == nil {
		if strings.HasSuffix(path, ".js") {
			if res, err := minify.JS(string(code)); err == nil {
				os.WriteFile(resPath, []byte(";"+res+";"), 0775)
			}
		} else if strings.HasSuffix(path, ".css") {
			if res, err := minify.CSS(string(code)); err == nil {
				os.WriteFile(resPath, []byte(res), 0775)
			}
		} else if strings.HasSuffix(path, ".ts") {
			//todo: add support for auto compiling typescript to javascript
			// compile typescript here
			if compilerConfig.DebugMode && !ranTypeScriptNotice {
				ranTypeScriptNotice = true
				LogErr(errors.New("notice: turbx compiler does not currently support auto compiling typescript to javascript in static files. (maybe this feature will be available in a future update)"))
			}
		} else if strings.HasSuffix(path, ".less") {
			if err := less.RenderFile(path, resPath, map[string]interface{}{"compress": true}); err != nil {
				os.Remove(resPath)
			}
		} else if strings.HasSuffix(path, ".sass") || strings.HasSuffix(path, ".scss") {
			// prevent import paths from leaking outside the static root
			code = regex.Comp(`@((?:import|use)\s*)(["'\'])((?:\\[\\"'\']|.)*?)\2;?`).RepFuncRef(&code, func(data func(int) []byte) []byte {
				if path, err := goutil.FS.JoinPath(compilerConfig.Static, string(data(3))); err == nil {
					return regex.JoinBytes('@', data(1), data(2), path, data(2), ';')
				}
				return []byte{}
			})

			if transpiler, err := libsass.New(libsass.Options{OutputStyle: libsass.CompressedStyle, IncludePaths: []string{compilerConfig.Static}, SassSyntax: strings.HasSuffix(path, ".sass")}); err == nil {
				if res, err := transpiler.Execute(string(code)); err == nil {
					os.WriteFile(resPath, []byte(res.CSS), 0775)
				}
			}
		}
	}
}

// tryMinifyDir runs tryMinifyFile recursively on a directory
func tryMinifyDir(dirPath string) {
	if files, err := os.ReadDir(dirPath); err == nil {
		for _, file := range files {
			if path, err := goutil.FS.JoinPath(dirPath, file.Name()); err == nil {
				if file.IsDir() {
					tryMinifyDir(path)
				} else {
					if regex.Comp(`(?<!\.min)\.([jt]s|css|less|s[ac]ss)$`).Match([]byte(path)) || imageRE.Match([]byte(path)) || videoRE.Match([]byte(path)) || audioRE.Match([]byte(path)) {
						tryMinifyFile(path)
					}
				}
			}
		}
	}
}

// removeLineBreak removes one extra line break from the compiler
func removeLineBreak[T interface{ uint8 | uint16 }](reader *liveread.Reader[T]) bool {
	b, e := reader.Peek(2)
	if e == nil {
		if b[0] == '\r' && b[1] == '\n' {
			reader.Discard(2)
			return true
		} else if b[0] == '\n' {
			reader.Discard(1)
			return true
		}
	}
	return false
}

// sortStrings will sort a list of strings
//
// this method will also split numbers and return `10 > 2`, rather than seeing `[1,0] < [2,_]`
func sortStrings[T any](list *[]T) {
	sort.Slice(*list, func(i, j int) bool {
		l1 := regex.Comp(`([0-9]+)`).Split(goutil.Conv.ToBytes((*list)[i]))
		l2 := regex.Comp(`([0-9]+)`).Split(goutil.Conv.ToBytes((*list)[j]))

		for i := len(l1) - 1; i >= 0; i-- {
			if len(l1[i]) == 0 {
				l1 = append(l1[:i], l1[i+1:]...)
			}
		}

		for i := len(l2) - 1; i >= 0; i-- {
			if len(l2[i]) == 0 {
				l2 = append(l2[:i], l2[i+1:]...)
			}
		}

		var smaller uint8 = 2
		l := len(l2)
		if n := len(l1); n <= l {
			if n == l {
				smaller--
			}
			l = n
			smaller--
		}

		for i := 0; i < l; i++ {
			n1 := l1[i][0] >= '0' && l1[i][0] <= '9'
			n2 := l2[i][0] >= '0' && l2[i][0] <= '9'
			if n1 && n2 {
				i1, _ := strconv.Atoi(string(l1[i]))
				i2, _ := strconv.Atoi(string(l2[i]))
				if i1 < i2 {
					return true
				} else if i1 > i2 {
					return false
				}
			} else if n1 {
				return true
			} else if n2 {
				return false
			} else {
				var small uint8 = 2
				ln := len(l2[i])
				if n := len(l1[i]); n <= ln {
					if n == ln {
						small--
					}
					ln = n
					small--
				}

				for j := 0; j < ln; j++ {
					if l1[i][j] < l2[i][j] {
						return true
					} else if l1[i][j] > l2[i][j] {
						return false
					}
				}

				if small == 1 {
					return true
				} else if small == 2 {
					return false
				}
			}
		}

		return smaller == 1
	})
}

func cloneArr[T any](list []T) []T {
	clone := make([]T, len(list))
	for i, v := range list {
		clone[i] = v
	}
	return clone
}
