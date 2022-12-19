package compiler

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v3"
	"github.com/alphadose/haxmap"
)

var rootPath string
var fileExt string = "md"
var componentPath string = "components"
var defaultLayoutPath string = "layout"
var publicPath string
var publicUrl string
var constOpts map[string]interface{} = map[string]interface{}{}
var cacheTime int64 = (time.Hour * 2).Milliseconds()

const writeFlushSize = 1000
const compileReadSize = 10
var debugMode = false

type Config struct {
	// Root is the root directory for your html/markdown files to be compiled
	//
	// default: views
	Root string

	// Components is an optional directory that can be used to organize components
	//
	// if a component is not found within the components directory first, it will then be checked for in the root directory you chose for the compiler
	//
	// default: components
	//
	// note: this file is relative to your chosen Root for the compiler
	Components string

	// Layout is the main template that all other files will be placed inside of
	//
	// default: layout
	//
	// pass "!" to disable
	//
	// pass "*" to set to default
	// 
	// note: this file is relative to your chosen Root for the compiler
	Layout string

	// Ext is the file extention for your files to be compiled
	//
	// default: .md
	Ext string

	// Public is an optional path you can use if you have a public directory with client side scripts and stylesheets
	//
	// the compiller will use this directory to auto upgrade to .min files
	//
	// note: the compiler will Not make this directory public, it will simply read from it
	Public string

	// Cache sets the amount of time before the cache will expire
	Cache string

	// ConstOpts is an optional list of constant options you would like to make default
	//
	// note: for an option to be read by the pre-compiler, all keys must start with a "$" in there name
	ConstOpts map[string]interface{}
}


type tagData struct {
	tag []byte
	attr []byte
}

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

var emptyContentTags []tagData = []tagData{
	{[]byte("script"), []byte("src")},
	{[]byte("iframe"), nil},
}

type elmVal struct {
	ind uint
	val []byte
}

type fnData struct {
	tag []byte
	args map[string][]byte
	fnName []byte
	cont []byte
	each eachList
	isComponent bool

	seek int64
}


type pathCacheData struct {
	path string
	tmp string
	cachePath string
	Ready *bool
	Err *error

	lastUsed *int64
}

var pathCache *haxmap.Map[string, pathCacheData] = haxmap.New[string, pathCacheData]()


var cacheTmpPath string

func init(){
	args := goutil.MapArgs()
	if args["debug"] == "true" {
		debugMode = true
	}

	if debugMode {
		dir := "turbx-cache"
		os.RemoveAll(dir)

		err := os.Mkdir(dir, 0755)
		if err != nil {
			panic(err)
		}
		cacheTmpPath = dir
	}else{
		dir, err := os.MkdirTemp("", "turbx-cache." + string(goutil.RandBytes(16, nil)) + ".")
		if err != nil {
			panic(err)
		}
		cacheTmpPath = dir
	}

	if path, err := filepath.Abs("views"); err == nil {
		rootPath = path
	}

	go clearTmpCache()
}


// Close handles stoping the compiler and clearing the cache
var compilingCount uintptr = 0
func Close(){
	// wait for compiler to finish
	time.Sleep(10 * time.Millisecond)
	for compilingCount > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)

	if !debugMode {
		os.RemoveAll(cacheTmpPath)
	}else{
		done := false

		for !done {
			time.Sleep(10 * time.Millisecond)

			done = true
			pathCache.ForEach(func(s string, pcd pathCacheData) bool {
				if !*pcd.Ready && *pcd.Err == nil {
					done = false
					return false
				}
				return true
			})
		}
	}
}

func clearTmpCache(){
	if cacheTmpPath == "" {
		return
	}

	if dir, err := os.ReadDir(cacheTmpPath); err == nil {
		for _, file := range dir {
			if p, err := goutil.JoinPath(cacheTmpPath, file.Name()); err != nil {
				os.RemoveAll(p)
			}
		}
	}
}


// SetConfig can be used to set change the config options provided in the Config struct
//
// this method will also clear the cache
func SetConfig(config Config) error {
	if config.Root != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}

		rootPath = path
	}

	if config.Components != "" {
		componentPath = config.Components
	}

	if config.Layout != "" {
		if config.Layout == "!" {
			defaultLayoutPath = ""
		}else if config.Layout == "*" {
			defaultLayoutPath = "layout"
		}else{
			defaultLayoutPath = config.Layout
		}
	}

	if config.Ext != "" {
		fileExt = string(regex.Compile(`[^\w_-]`).RepStr([]byte(config.Ext), []byte{}))
	}

	if config.Public != "" {
		public := strings.SplitN(config.Public, "@", 2)

		if path, err := filepath.Abs(public[0]); err == nil {
			publicPath = path
		}else{
			publicPath = ""
		}

		if len(public) > 1 {
			if public[1] != "/" {
				publicUrl = public[1]
			}else{
				publicUrl = ""
			}
		}
	}

	if config.ConstOpts != nil {
		if opts, err := goutil.DeepCopyJson(config.ConstOpts); err == nil {
			constOpts = opts
		}else{
			constOpts = nil
		}
	}

	if config.Cache != "" {
		if n, err := time.ParseDuration(config.Cache); err == nil {
			cacheTime = n.Milliseconds()
		}
	}

	go clearTmpCache()
	return nil
}


// preCompile generates a new pre-compiled file for the cache
//
// this compiles markdown and handles other complex methods
//
// this function is useful if you need to update any constand vars, defined with a "$" as the first char in their key name
func PreCompile(path string, opts map[string]interface{}) error {
	if rootPath == "" || cacheTmpPath == "" {
		return errors.New("a root path was never chosen")
	}

	// add constOpts where not already defined
	if constOpts != nil {
		if defOpts, err := goutil.DeepCopyJson(constOpts); err == nil {
			for key, val := range defOpts {
				if _, ok := opts[key]; !ok {
					opts[key] = val
				}
			}
		}
	}
	
	compData := preCompile(path, &opts)

	for !*compData.Ready && *compData.Err == nil {
		time.Sleep(10 * time.Nanosecond)
	}

	if *compData.Err != nil {
		return *compData.Err
	}

	return nil
}

func preCompile(path string, opts *map[string]interface{}, componentOf ...string) pathCacheData {
	var cacheReady bool = false
	var cacheError error

	isLayout := false
	if len(componentOf) == 1 && componentOf[0] == "@layout" {
		isLayout = true
		componentOf = []string{}
	}

	// handle modified cache and layout paths
	cachePath := ""
	layoutPath := ""
	layoutCachePath := ""
	if regex.Compile(`@([\w_-]+)(:[\w_-]+|)$`).Match([]byte(path)) {
		path = string(regex.Compile(`@([\w_-]+)(:[\w_-]+|)$`).RepFunc([]byte(path), func(data func(int) []byte) []byte {
			layoutPath = string(data(1))

			if len(data(2)) > 1 {
				layoutCachePath = string(data(2))
			}

			return []byte{}
		}))
	}

	if regex.Compile(`(:[\w_-]+)$`).Match([]byte(path)) {
		path = string(regex.Compile(`(:[\w_-]+)$`).RepFunc([]byte(path), func(data func(int) []byte) []byte {
			cachePath = string(data(1))

			return []byte{}
		}))
	}

	// resolve full file path within root
	var fullPath string
	if len(componentOf) != 0 {
		if p, err := goutil.JoinPath(rootPath, componentPath, path + "." + fileExt); err == nil {
			// prevent path from leaking into tmp cache
			if strings.HasPrefix(p, cacheTmpPath) {
				err := errors.New("path leaked into tmp cache")
				return pathCacheData{Ready: &cacheReady, Err: &err}
			}

			// ensure the file actually exists
			if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
				fullPath = p
			}
		}
	}

	if fullPath == "" {
		var err error
		fullPath, err = goutil.JoinPath(rootPath, path + "." + fileExt)
		if err != nil {
			return pathCacheData{Ready: &cacheReady, Err: &err}
		}
	}

	// prevent path from leaking into tmp cache
	if strings.HasPrefix(fullPath, cacheTmpPath) {
		err := errors.New("path leaked into tmp cache")
		return pathCacheData{Ready: &cacheReady, Err: &err}
	}

	// prevent components from recursively containing themselves
	for _, cPath := range componentOf {
		if fullPath == cPath {
			err := errors.New("recursive component detected")
			return pathCacheData{Ready: &cacheReady, Err: &err}
		}
	}

	// ensure the file actually exists
	if stat, err := os.Stat(fullPath); err != nil || stat.IsDir() {
		err = errors.New("file does not exist or is invalid")
		return pathCacheData{Ready: &cacheReady, Err: &err}
	}

	// create a tmp file for the cache
	tmpFile, tmpPath, err := tmpPath(path)
	if err != nil {
		return pathCacheData{Ready: &cacheReady, Err: &err}
	}


	//todo: make sure and old files are removed from the cache first

	// prepare the cache vars and info
	now := time.Now().UnixMilli()
	cacheRes := pathCacheData{path: fullPath, tmp: tmpPath, cachePath: fullPath + cachePath, lastUsed: &now, Ready: &cacheReady, Err: &cacheError}
	pathCache.Set(fullPath + cachePath, cacheRes)

	// run pre compiler concurrently
	go func(){
		defer tmpFile.Close()

		file, err := os.OpenFile(fullPath, os.O_RDONLY, 0)
		if err != nil {
			cacheError = err
			return
		}
		defer file.Close()


		// get layout data
		var layoutData pathCacheData
		if len(componentOf) == 0 && !isLayout {
			if layoutPath != "" {
				go func(){
					layoutData = preCompile(layoutPath + layoutCachePath, opts, "@layout")
				}()
			}else if defaultLayoutPath != "" {
				go func(){
					layoutData = preCompile(defaultLayoutPath + layoutCachePath, opts, "@layout")
				}()
			}else{
				err := errors.New("layout not found")
				layoutData = pathCacheData{Ready: &cacheReady, Err: &err}
			}
		}


		reader := bufio.NewReader(file)
		writer := bufio.NewWriter(tmpFile)

		fnLevel := []string{}
		ifMode := []uint8{0}
		eachArgs := []KeyVal{}
		tagInd := [][]byte{}
		fnCont := []fnData{}

		wSize := uint(0)
		write := func(b []byte){
			if len(fnCont) != 0 {
				if fnCont[len(fnCont)-1].cont != nil {
					fnCont[len(fnCont)-1].cont = append(fnCont[len(fnCont)-1].cont, b...)
					return
				}else{
					found := false
					for i := len(fnCont)-1; i >= 0; i-- {
						if fnCont[len(fnCont)-1].cont != nil {
							fnCont[len(fnCont)-1].cont = append(fnCont[len(fnCont)-1].cont, b...)
							found = true
							break
						}
					}
					if found {
						return
					}
				}
			}

			writer.Write(b)
			wSize += uint(len(b))
			if wSize >= writeFlushSize {
				writer.Flush()
				wSize = 0
			}
		}


		// handle layout
		var layoutEnd []byte
		if len(componentOf) == 0 && !isLayout && !debugMode {
			for layoutData.Ready == nil || (!*layoutData.Ready && *layoutData.Err == nil) {
				time.Sleep(10 * time.Nanosecond)
			}

			if *layoutData.Err == nil {
				if layout, e := os.ReadFile(layoutData.tmp); e == nil {
					layoutParts := regex.Compile(`(?i){{{?body}}}?`).SplitRef(&layout)
					write(layoutParts[0])
					layoutEnd = bytes.Join(layoutParts[1:], []byte{})
				}
			}
		}

		linePos := 0

		b, err := reader.Peek(1)
		for err == nil {
			if b[0] == '\\' {
				b, err = reader.Peek(2)
				write(escapeChar(b[1]))
				reader.Discard(2)
				b, err = reader.Peek(1)
				linePos++
				continue
			}

			// handle html elements, components, and pre funcs
			if b[0] == '<' {
				reader.Discard(1)
				b, err = reader.Peek(1)

				if err != nil {
					write([]byte{'<'})
					break
				}

				if b[0] == '!' {
					b, err = reader.Peek(3)
					if err == nil && b[1] == '-' && b[2] == '-' {
						reader.Discard(3)
						b, err = reader.Peek(1)
						
						skipWhitespace(reader, &b, &err)

						if err == nil && b[0] == '!' {
							reader.Discard(1)
							comment := []byte("<!--!")

							b, err = reader.Peek(3)
							for err == nil && !(b[0] == '-' && b[1] == '-' && b[2] == '>') {
								comment = append(comment, b[0])
								reader.Discard(1)
								b, err = reader.Peek(3)
							}
							reader.Discard(3)
							b, err = reader.Peek(1)

							write(append(comment, '-', '-', '>'))
						}else if err == nil {
							b, err = reader.Peek(3)
							for err == nil && !(b[0] == '-' && b[1] == '-' && b[2] == '>') {
								reader.Discard(1)
								b, err = reader.Peek(3)
							}
							reader.Discard(3)
							b, err = reader.Peek(1)
						}

						continue
					}

					b, err = reader.Peek(1)
				}else if b[0] == '/' || b[0] == 'B' {
					b, err = reader.Peek(7)
					if err == nil && (bytes.Contains(b, []byte("BODY")) || bytes.Contains(b, []byte("Body"))) {
						i := 0
						if b[0] == '/' {
							i++
						}
						if bytes.Equal(b[i:i+4], []byte("BODY")) || bytes.Equal(b[i:i+4], []byte("Body")) {
							i += 4
							if b[i] == '/' {
								i++
							}
							if b[i] == '>' {
								i++
								write([]byte("{{{BODY}}}"))
								reader.Discard(i)
								b, err = reader.Peek(1)
								continue
							}
						}
					}

					b, err = reader.Peek(1)
				}

				elm := map[string]elmVal{}
				ind := uint(0)

				selfClose := 0
				mode := uint8(0)
				if b[0] == '_' {
					mode = 1
					reader.Discard(1)
					b, err = reader.Peek(1)
				}else if regex.Compile(`[A-Z]`).MatchRef(&b) {
					mode = 2
				}else if b[0] == '/' {
					selfClose = 2
					reader.Discard(1)
					b, err = reader.Peek(1)

					if b[0] == '_' {
						mode = 1
						reader.Discard(1)
						b, err = reader.Peek(1)
					}else if regex.Compile(`[A-Z]`).MatchRef(&b) {
						mode = 2
					}
				}

				// handle elm tag (use all caps to prevent conflicts with attributes)
				tag := []byte{}
				for err == nil && !regex.Compile(`[\s\r\n/>]`).MatchRef(&b) {
					tag = append(tag, b[0])
					reader.Discard(1)
					b, err = reader.Peek(1)
				}
				elm["TAG"] = elmVal{ind, tag}
				ind++

				if selfClose != 2 {
					for err == nil {
						skipWhitespace(reader, &b, &err)

						// handle end of html tag
						if b[0] == '/' {
							b, err = reader.Peek(2)
							if b[1] == '>' {
								reader.Discard(2)
								b, err = reader.Peek(1)
								selfClose = 1
								break
							}
						}else if b[0] == '>' {
							reader.Discard(1)
							b, err = reader.Peek(1)
							tagInd = append(tagInd, elm["TAG"].val)
							break
						}else if b[0] == '!' {
							elm[strconv.Itoa(int(ind))] = elmVal{ind, []byte{'^'}}
							ind++
							reader.Discard(1)
							b, err = reader.Peek(1)
							continue
						}else if b[0] == '&' || b[0] == '|' || b[0] == '(' || b[0] == ')' {
							elm[strconv.Itoa(int(ind))] = elmVal{ind, b}
							ind++
							reader.Discard(1)
							b, err = reader.Peek(1)
							continue
						}

						// get key
						key := []byte{}
						for err == nil && !regex.Compile(`[\s\r\n/>!=&|\(\)]`).MatchRef(&b) {
							key = append(key, b[0])
							reader.Discard(1)
							b, err = reader.Peek(1)
						}

						if len(key) == 0 {
							break
						}

						val := []byte{}
						if b[0] == '!' {
							// handle not operator
							val = []byte{'!'}
							reader.Discard(1)
							b, err = reader.Peek(1)
						}
						if b[0] == '=' {
							reader.Discard(1)
							b, err = reader.Peek(1)
						}else{
							// handle single key without value
							k := string(bytes.ToLower(key))
							if _, ok := elm[k]; !ok {
								elm[k] = elmVal{ind, nil}
								ind++
							}else{
								i := 1
								for {
									iStr := strconv.Itoa(i)
									if _, ok := elm[k+":"+iStr]; !ok {
										elm[k+":"+iStr] = elmVal{ind, nil}
										ind++
										break
									}
									i++
								}
							}
							continue
						}

						// get quote type
						q := byte(' ')
						if b[0] == '"' {
							q = '"'
							reader.Discard(1)
							b, err = reader.Peek(1)
						}else if b[0] == '\'' {
							q = '\''
							reader.Discard(1)
							b, err = reader.Peek(1)
						}else if b[0] == '`' {
							q = '`'
							reader.Discard(1)
							b, err = reader.Peek(1)
						}

						// get value
						for err == nil && b[0] != q && (q != ' ' || !regex.Compile(`[\s\r\n/>!=&|\(\)]`).MatchRef(&b)) {
							if b[0] == '\\' {
								b, err = reader.Peek(2)
								if regex.Compile(`[A-Za-z]`).MatchRef(&b) {
									val = append(val, b[0], b[1])
								}else{
									val = append(val, b[1])
								}
								// val = append(val, b[0], b[1])
		
								reader.Discard(2)
								b, err = reader.Peek(1)
								continue
							}

							val = append(val, b[0])
							reader.Discard(1)
							b, err = reader.Peek(1)
						}

						if q != ' ' {
							reader.Discard(1)
							b, err = reader.Peek(1)
						}

						// convert opts for {{key="val"}} attrs
						if len(key) >= 2 && key[0] == '{' && key[1] == '{' {
							if len(key) > 3 && key[2] == '{' {
								b, err = reader.Peek(3)
								if err == nil && b[0] == '}' && b[1] == '}' && b[2] == '}' {
									reader.Discard(3)
									key = key[3:]
									if len(key) == 0 {
										if regex.Compile(`^["'\']?([\w_-]+).*$`).MatchRef(&val) {
											key = regex.Compile(`^["'\']?([\w_-]+).*$`).RepStrComplexRef(&val, []byte("$1"))
										}
									}

									val = regex.JoinBytes('{', '{', '{', val, '}', '}', '}')
								}else if err == nil && b[0] == '}' && b[1] == '}' {
									reader.Discard(2)
									key = key[3:]
									if len(key) == 0 {
										if regex.Compile(`^["'\']?([\w_-]+).*$`).MatchRef(&val) {
											key = regex.Compile(`^["'\']?([\w_-]+).*$`).RepStrComplexRef(&val, []byte("$1"))
										}
									}

									val = regex.JoinBytes('{', '{', val, '}', '}')
								}
							}else{
								b, err = reader.Peek(2)
								if err == nil && b[0] == '}' && b[1] == '}' {
									reader.Discard(2)
									key = key[2:]
									if len(key) == 0 {
										if regex.Compile(`^["'\']?([\w_-]+).*$`).MatchRef(&val) {
											key = regex.Compile(`^["'\']?([\w_-]+).*$`).RepStrComplexRef(&val, []byte("$1"))
										}
									}

									val = regex.JoinBytes('{', '{', val, '}', '}')
								}
							}

							b, err = reader.Peek(1)
						}

						k := string(bytes.ToLower(key))
						if _, ok := elm[k]; !ok {
							elm[k] = elmVal{ind, val}
							ind++
						}else{
							i := 1
							for {
								iStr := strconv.Itoa(i)
								if _, ok := elm[k+":"+iStr]; !ok {
									elm[k+":"+iStr] = elmVal{ind, val}
									ind++
									break
								}
								i++
							}
						}
					}
				}

				if mode == 1 {
					if selfClose == 2 && len(fnCont) != 0 && !fnCont[len(fnCont)-1].isComponent && bytes.Equal(elm["TAG"].val, fnCont[len(fnCont)-1].tag) {
						fn := fnCont[len(fnCont)-1]

						if bytes.Equal(elm["TAG"].val, []byte("each")) {
							// remove eachArgs from end of list
							rmArgs := map[string][]byte{}
							if fn.each.As != nil {
								rmArgs["as"] = fn.each.As
							}
							if fn.each.Of != nil {
								rmArgs["of"] = fn.each.Of
							}

							for i := len(eachArgs)-1; i >= 0; i-- {
								if bytes.Equal(eachArgs[i].Key, rmArgs["as"]) {
									eachArgs = append(eachArgs[:i], eachArgs[i+1:]...)
									rmArgs["as"] = nil
								}else if bytes.Equal(eachArgs[i].Key, rmArgs["of"]) {
									eachArgs = append(eachArgs[:i], eachArgs[i+1:]...)
									rmArgs["of"] = nil
								}
							}

							if len(*fn.each.List) != 0 {
								// set new each args
								if fn.each.As != nil {
									k := fn.each.As
									eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*fn.each.List)[0].Val})
								}
								if fn.each.Of != nil {
									k := fn.each.Of
									eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*fn.each.List)[0].Key})
								}
	
								*fn.each.List = (*fn.each.List)[1:]

								// return to top of each loop
								file.Seek(fn.seek, io.SeekStart)
								reader.Reset(file)

								b, err = reader.Peek(1)
								continue
							}

							fnCont = fnCont[:len(fnCont)-1]

							reader.Discard(1)
							b, err = reader.Peek(1)
							continue
						}

						fnCont = fnCont[:len(fnCont)-1]

						if res, e := callFunc(string(fn.fnName), &fn.args, &fn.cont, opts, true, &eachArgs); e == nil {
							if res != nil {
								write(goutil.ToByteArray(res))
							}
						}else{
							argStr := []byte{}
							for i := 0; i < len(fn.args); i++ {
								if val, ok := fn.args[strconv.Itoa(i)]; ok {
									argStr = append(argStr, regex.JoinBytes(' ', '"', goutil.EscapeHTMLArgs(val), '"')...)
								}
							}
							for key, val := range fn.args {
								if key == "TAG" {
									continue
								}

								if _, err := strconv.Atoi(key); err != nil {
									argStr = append(argStr, regex.JoinBytes(' ', key, '=', '"', goutil.EscapeHTMLArgs(val), '"')...)
								}
							}

							write(regex.JoinBytes([]byte("{{#"), fn.fnName, ':', len(fnCont), argStr, []byte("}}"), fn.cont, []byte("{{/"), fn.fnName, ':', len(fnCont), []byte("}}")))
						}

						reader.Discard(1)
						b, err = reader.Peek(1)
						continue
					}else if selfClose == 2 && (len(ifMode) == 0 || ifMode[len(ifMode)-1] == 0) && !bytes.Equal(elm["TAG"].val, []byte("else")) && !bytes.Equal(elm["TAG"].val, []byte("elif")) {
						for len(fnLevel) != 0 && fnLevel[len(fnLevel)-1] != string(elm["TAG"].val) {
							fnLevel = fnLevel[:len(fnLevel)-1]
							if len(ifMode) != 0 {
								ifMode = ifMode[:len(ifMode)-1]
							}
						}
						if len(fnLevel) != 0 {
							fnLevel = fnLevel[:len(fnLevel)-1]
							if len(ifMode) != 0 {
								ifMode = ifMode[:len(ifMode)-1]
							}
							write(regex.JoinBytes([]byte("{{/"), elm["TAG"].val, ':', len(fnLevel), []byte("}}")))
						}
						reader.Discard(1)
						b, err = reader.Peek(1)
						continue
					}else if selfClose == 2 {
						reader.Discard(1)
						b, err = reader.Peek(1)
					}

					// handle functions
					if bytes.Equal(elm["TAG"].val, []byte("if")) || bytes.Equal(elm["TAG"].val, []byte("else")) || bytes.Equal(elm["TAG"].val, []byte("elif")) {
						elseMode := (bytes.Equal(elm["TAG"].val, []byte("else")) || bytes.Equal(elm["TAG"].val, []byte("elif")))

						if selfClose != 0 && len(ifMode) != 0 && (ifMode[len(ifMode)-1] == 2 || ifMode[len(ifMode)-1] == 3) {

							if elseMode {
								for err == nil {
									if b[0] == '<' {
										b, err = reader.Peek(6)
										if err == nil && b[1] == '/' && b[2] == '_' && bytes.Equal(b[3:5], []byte("if")) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[5]}) {
											reader.Discard(5)
											b, err = reader.Peek(1)
											for err == nil && b[0] != '>' {
												reader.Discard(1)
												b, err = reader.Peek(1)
											}
											reader.Discard(1)
											b, err = reader.Peek(1)
											break
										}
									}
		
									skipObjStrComments(reader, &b, &err)
								}
							}

							if ifMode[len(ifMode)-1] == 3 && len(fnLevel) != 0 {
								write(regex.JoinBytes([]byte("{{/if:"), len(fnLevel)-1, []byte("}}")))
								fnLevel = fnLevel[:len(fnLevel)-1]
							}

							// fnLevel = fnLevel[:len(fnLevel)-1]
							if len(ifMode) != 0 {
								ifMode = ifMode[:len(ifMode)-1]
							}
							continue
						}

						// handle if statements
						intArgs := []elmVal{}
						argSize := 0
						for key, arg := range elm {
							if !regex.Compile(`^([A-Z]+)$`).Match([]byte(key)) {
								intArgs = append(intArgs, elmVal{arg.ind, []byte(key)})
								argSize++
								if !regex.Compile(`^([0-9]+)$`).Match([]byte(key)) && arg.val != nil {
									argSize += 2
								}
							}
						}
						sort.Slice(intArgs, func(i, j int) bool {
							return intArgs[i].ind < intArgs[j].ind
						})

						args := make([][]byte, argSize)
						i := 0
						for _, arg := range intArgs {
							if regex.Compile(`^([0-9]+)$`).Match([]byte(arg.val)) {
								args[i] = regex.Compile(`:[0-9]+$`).RepStr(elm[string(arg.val)].val, []byte{})
								i++
							}else{
								args[i] = arg.val
								i++
								if elm[string(arg.val)].val != nil {
									val := regex.Compile(`:[0-9]+$`).RepStr(elm[string(arg.val)].val, []byte{})
									if val[0] == '<' || val[0] == '>' {
										args[i] = []byte{val[0]}
										val = val[1:]
										if val[0] == '=' {
											args[i] = append(args[i], val[0])
											val = val[1:]
										}
									}else if val[0] == '!' || val[0] == '=' || val[0] == '~' {
										args[i] = []byte{val[0]}
										val = val[1:]
									}else{
										args[i] = []byte{'='}
									}
									i++

									args[i] = val
									i++
								}
							}
						}

						res, e := callFuncArr("If", &args, nil, opts, true, &eachArgs)
						if e != nil {
							cacheError = e
							return
						}

						if res == true {
							if elseMode && len(ifMode) != 0 && ifMode[len(ifMode)-1] != 1 {
								write(regex.JoinBytes([]byte("{{#else:"), len(fnLevel)-1, []byte("}}")))
								ifMode[len(ifMode)-1] = 3
							}else if len(ifMode) != 0 {
								ifMode[len(ifMode)-1] = 2
							}
						}else if res == false {
							for err == nil {
								if b[0] == '<' {
									b, err = reader.Peek(7)
									if err == nil && b[1] == '_' && (bytes.Equal(b[2:6], []byte("else")) || bytes.Equal(b[2:6], []byte("elif"))) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[6]}) {
										b, err = reader.Peek(1)
										fnLevel = append(fnLevel, "if")
										// ifMode = append(ifMode, 1)
										if len(ifMode) != 0 {
											ifMode[len(ifMode)-1] = 1
										}
										break
									}else if err == nil && b[1] == '/' && b[2] == '_' && bytes.Equal(b[3:5], []byte("if")) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[5]}) {
										b, err = reader.Peek(1)
										break
									}
								}

								skipObjStrComments(reader, &b, &err)
							}
						}else if reflect.TypeOf(res) == goutil.VarType["byteArray"] {
							if elseMode {
								if len(ifMode) != 0 && ifMode[len(ifMode)-1] == 1 {
									ifMode[len(ifMode)-1] = 0
									write(regex.JoinBytes([]byte("{{#if:"), len(fnLevel)-1, ' ', res, []byte("}}")))
								}else{
									write(regex.JoinBytes([]byte("{{#else:"), len(fnLevel)-1, ' ', res, []byte("}}")))
								}
							}else{
								write(regex.JoinBytes([]byte("{{#if:"), len(fnLevel), ' ', res, []byte("}}")))
								fnLevel = append(fnLevel, "if")
								ifMode = append(ifMode, 0)
							}
						}else{
							if elseMode {
								if len(ifMode) != 0 && ifMode[len(ifMode)-1] == 1 {
									ifMode[len(ifMode)-1] = 0
									write(regex.JoinBytes([]byte("{{#if:"), len(fnLevel)-1, ' ', bytes.Join(args, []byte{' '}), []byte("}}")))
								}else{
									write(regex.JoinBytes([]byte("{{#else:"), len(fnLevel)-1, ' ', bytes.Join(args, []byte{' '}), []byte("}}")))
								}
							}else{
								write(regex.JoinBytes([]byte("{{#if:"), len(fnLevel), ' ', bytes.Join(args, []byte{' '}), []byte("}}")))
								fnLevel = append(fnLevel, "if")
								ifMode = append(ifMode, 0)
							}
						}
					}else if bytes.Equal(elm["TAG"].val, []byte("each")) {
						if selfClose == 1 {
							continue
						}

						args := map[string][]byte{}

						for key, arg := range elm {
							if arg.val == nil {
								args[strconv.Itoa(int(arg.ind))] = []byte(key)
							}else{
								args[key] = arg.val
							}
						}

						seekPos, e := getSeekPos(file, reader)
						if err != nil {
							cacheError = e
							return
						}

						res, e := callFunc("Each", &args, nil, opts, true, &eachArgs)
						if e != nil {
							cacheError = e
							return
						}

						rt := reflect.TypeOf(res)
						if rt == goutil.VarType["byteArray"] {
							// return normal func for compiler
							write(regex.JoinBytes([]byte("{{#each:"), len(fnLevel), ' ', res, []byte("}}")))
							fnLevel = append(fnLevel, "each")
						}else if rt == reflect.TypeOf(eachList{}) {
							// get content to run in each loop
							if res.(eachList).As != nil {
								k := res.(eachList).As
								eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*res.(eachList).List)[0].Val})
							}
							if res.(eachList).Of != nil {
								k := res.(eachList).Of
								eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*res.(eachList).List)[0].Key})
							}

							*res.(eachList).List = (*res.(eachList).List)[1:]

							fnCont = append(fnCont, fnData{tag: []byte("each"), seek: seekPos, each: res.(eachList)})
						}else{
							// skip and remove blank const value each loop
							eachLevel := 0

							for err == nil {
								if b[0] == '<' {
									b, err = reader.Peek(8)
									if err == nil && b[1] == '/' && b[2] == '_' && bytes.Equal(b[3:7], []byte("each")) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[7]}) {
										reader.Discard(7)
										b, err = reader.Peek(1)

										for err == nil && b[0] != '>' {
											if b[0] == '\\' {
												reader.Discard(1)
											}

											reader.Discard(1)
											b, err = reader.Peek(1)
											if b[0] == '"' {
												for err == nil && b[0] != '"' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}else if b[0] == '\'' {
												for err == nil && b[0] != '\'' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}else if b[0] == '`' {
												for err == nil && b[0] != '`' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}
										}

										reader.Discard(1)
										b, err = reader.Peek(1)

										eachLevel--
										if eachLevel < 0 {
											break
										}
									}else if err == nil && b[1] == '_' && bytes.Equal(b[2:6], []byte("each")) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[6]}) {
										reader.Discard(6)
										b, err = reader.Peek(1)
										
										for err == nil && b[0] != '>' {
											if b[0] == '\\' {
												reader.Discard(1)
											}

											reader.Discard(1)
											b, err = reader.Peek(1)
											if b[0] == '"' {
												for err == nil && b[0] != '"' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}else if b[0] == '\'' {
												for err == nil && b[0] != '\'' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}else if b[0] == '`' {
												for err == nil && b[0] != '`' {
													if b[0] == '\\' {
														reader.Discard(1)
													}
													reader.Discard(1)
													b, err = reader.Peek(1)
												}
											}
										}
										
										eachLevel++
									}
								}

								reader.Discard(1)
								b, err = reader.Peek(1)
							}

							continue
						}
					}else{
						fnName := append(bytes.ToUpper([]byte{elm["TAG"].val[0]}), elm["TAG"].val[1:]...)

						args := map[string][]byte{}

						for key, arg := range elm {
							if arg.val == nil {
								args[strconv.Itoa(int(arg.ind))] = []byte(key)
							}else{
								args[key] = arg.val
							}
						}

						if selfClose == 1 {
							if res, e := callFunc(string(fnName), &args, nil, opts, true, &eachArgs); e == nil {
								if res != nil {
									write(goutil.ToByteArray(res))
								}
							}else{
								argStr := []byte{}
								for i := 0; i < len(args); i++ {
									if val, ok := args[strconv.Itoa(i)]; ok {
										argStr = append(argStr, regex.JoinBytes(' ', '"', goutil.EscapeHTMLArgs(val), '"')...)
									}
								}
								for key, val := range args {
									if key == "TAG" {
										continue
									}

									if _, err := strconv.Atoi(key); err != nil {
										argStr = append(argStr, regex.JoinBytes(' ', key, '=', '"', goutil.EscapeHTMLArgs(val), '"')...)
									}
								}

								write(regex.JoinBytes([]byte("{{#"), fnName, ':', len(fnCont), argStr, []byte("/}}")))
							}
						}else{
							fnCont = append(fnCont, fnData{tag: elm["TAG"].val, args: args, fnName: fnName, cont: []byte{}})
						}
					}
				}else if mode == 2 {
					if selfClose == 2 && len(fnCont) != 0 && fnCont[len(fnCont)-1].isComponent && bytes.Equal(elm["TAG"].val, fnCont[len(fnCont)-1].tag) {
						fn := fnCont[len(fnCont)-1]
						fnCont = fnCont[:len(fnCont)-1]

						compOpts, e := goutil.DeepCopyJson(*opts)
						if e != nil {
							cacheError = e
							return
						}

						for key, val := range fn.args {
							compOpts[key] = val
						}

						var comp []byte
						compData := preCompile(string(fn.fnName), &compOpts, append(componentOf, fullPath)...)
						for !*compData.Ready && *compData.Err == nil {
							time.Sleep(10 * time.Nanosecond)
						}

						if *compData.Err == nil {
							if c, e := os.ReadFile(compData.tmp); e == nil {
								comp = c
							}

							pathCache.Del(compData.cachePath)
							os.Remove(compData.tmp)
						}

						if comp == nil || *compData.Err != nil {
							if debugMode {
								if *compData.Err != nil {
									fmt.Println("error:", *compData.Err)
									write([]byte("{{#Error: Turbx Component Failed: '"+string(fn.fnName)+"'\n"+(*compData.Err).Error()+"}}"))
								}else{
									write([]byte("{{#Error: Turbx Component Failed: '"+string(fn.fnName)+"}}"))
								}
							}

							reader.Discard(1)
							b, err = reader.Peek(1)
							continue
						}

						write(regex.Compile(`(?i){{({|)body}}(}|)`).RepFunc(comp, func(data func(int) []byte) []byte {
							if len(data(1)) == 0 && len(data(2)) == 0 {
								return goutil.EscapeHTML(fn.cont)
							}

							return fn.cont
						}))

						reader.Discard(1)
						b, err = reader.Peek(1)
						continue
					}

					args := map[string][]byte{}

					ind := 0
					for key, arg := range elm {
						if key == "TAG" {
							args[key] = arg.val
							continue
						}

						if arg.val == nil {
							args[strconv.Itoa(ind)] = []byte(key)
							ind++
						}else{
							if bytes.HasPrefix(arg.val, []byte("{{")) && bytes.HasSuffix(arg.val, []byte("}}")) {
								if val, ok := GetOpt(arg.val, opts, true, &eachArgs); ok {
									if key[0] != '$' {
										args["$"+key] = goutil.ToByteArray(val)
									}else{
										args[key] = goutil.ToByteArray(val)
									}
								}else{
									if key[0] != '$' {
										args["$"+key] = arg.val
									}else{
										args[key] = arg.val
									}
								}
							}else if key[0] != '$' {
								args["$"+key] = arg.val
							}else{
								args[key] = arg.val
							}
						}
					}

					fnName := regex.Compile(`[^\w_\-\.]`).RepStr(elm["TAG"].val, []byte{})
					fnNameList := bytes.Split(fnName, []byte("."))
					ind = 0
					for _, v := range fnNameList {
						if len(v) == 1 {
							fnNameList[ind] = bytes.ToUpper(v)
							ind++
						}else if len(v) != 0 {
							fnNameList[ind] = append(bytes.ToUpper([]byte{v[0]}), v[1:]...)
							ind++
						}
					}
					fnName = bytes.Join(fnNameList[:ind], []byte("/"))

					if selfClose == 1 {
						compOpts, e := goutil.DeepCopyJson(*opts)
						if e != nil {
							cacheError = e
							return
						}

						for key, val := range args {
							compOpts[key] = val
						}

						var comp []byte
						compData := preCompile(string(fnName), &compOpts, append(componentOf, fullPath)...)
						for !*compData.Ready && *compData.Err == nil {
							time.Sleep(10 * time.Nanosecond)
						}

						if *compData.Err == nil {
							if c, e := os.ReadFile(compData.tmp); e == nil {
								comp = c
							}

							pathCache.Del(compData.cachePath)
							os.Remove(compData.tmp)
						}

						if comp == nil || *compData.Err != nil {
							if debugMode {
								if *compData.Err != nil {
									fmt.Println("error:", *compData.Err)
									write([]byte("{{#Error: Turbx Component Failed: '"+string(fnName)+"'\n"+(*compData.Err).Error()+"}}"))
								}else{
									write([]byte("{{#Error: Turbx Component Failed: '"+string(fnName)+"}}"))
								}
							}

							reader.Discard(1)
							b, err = reader.Peek(1)
							continue
						}

						write(regex.Compile(`(?i){{({|)body}}(}|)`).RepStr(comp, []byte{}))
					}else{
						fnCont = append(fnCont, fnData{tag: elm["TAG"].val, fnName: fnName, args: args, isComponent: true, cont: []byte{}})
					}
				}else{
					// handle html tags
					if selfClose == 2 {
						write(regex.JoinBytes('<', '/', elm["TAG"].val, '>'))
						reader.Discard(1)
						b, err = reader.Peek(1)
						continue
					}


					// get arg list
					args := map[string][]byte{}
					ind := 0
					for key, arg := range elm {
						if key == "TAG" {
							continue
						}

						if arg.val == nil {
							if k := []byte(key); !regex.Compile(":[0-9]+$").MatchRef(&k) {
								args[strconv.Itoa(ind)] = k
								ind++
							}
						}else{
							if regex.Compile(":[0-9]+$").Match([]byte(key)) {
								key = string(regex.Compile(":[0-9]+$").RepStr([]byte(key), []byte{}))
								if key != "class" {
									continue
								}
							}

							if bytes.HasPrefix(arg.val, []byte("{{")) && bytes.HasSuffix(arg.val, []byte("}}")) {
								if val, ok := GetOpt(arg.val, opts, true, &eachArgs); ok {
									if _, ok := args[key]; !ok {
										args[key] = goutil.ToByteArray(val)
									}else{
										args[key] = append(append(args[key], ' '), goutil.ToByteArray(val)...)
									}
								}else{
									if _, ok := args[key]; !ok {
										args[key] = arg.val
									}else{
										args[key] = append(append(args[key], ' '), arg.val...)
									}
								}
							}else{
								if _, ok := args[key]; !ok {
									args[key] = arg.val
								}else{
									args[key] = append(append(args[key], ' '), arg.val...)
								}
							}
						}
					}


					// sort html args
					argSort := []string{}
					for key, val := range args {
						if !regex.Compile(`^([A-Z]+|[0-9]+)$`).Match([]byte(key)) {
							argSort = append(argSort, key)
						}else if regex.Compile(`^[0-9]+$`).Match([]byte(key)) && regex.Compile(`^[\w_-]+$`).MatchRef(&val) {
							argSort = append(argSort, string(val))
						}
					}
					sort.Strings(argSort)


					if publicPath != "" {
						link := ""
						if _, ok := args["src"]; ok {
							link = "src"
						}else if _, ok := args["href"]; ok && bytes.Equal(elm["TAG"].val, []byte("link")) {
							link = "href"
						}

						if link != "" && regex.Compile(`^https?:|/`).Match(args[link]) && bytes.HasPrefix(args[link], []byte(publicUrl+"/")) {
							if regex.Compile(`(?<!\.min)\.(\w+)$`).Match(args[link]) {
								minSrc := regex.Compile(`\.(\w+)$`).RepStrComplex(args[link], []byte(".min.$1"))
								if path, e := goutil.JoinPath(publicPath, string(minSrc)); e == nil {
									if _, e := os.Stat(path); e == nil {
										args[link] = minSrc
									}
								}
							}
						}
					}

					res := regex.JoinBytes('<', elm["TAG"].val)
					for _, arg := range argSort {
						if args[arg] != nil {
							if _, err := strconv.Atoi(arg); err == nil {
								val := regex.Compile(`[^\w_-]`).RepStr(args[arg], []byte{})
								res = regex.JoinBytes(res, ' ', val)
							}else{
								val := goutil.EscapeHTMLArgs(args[arg])

								if bytes.HasPrefix(val, []byte("{{{")) && bytes.HasSuffix(val, []byte("}}}")) {
									val = bytes.TrimLeft(val, "{")
									val = bytes.TrimRight(val, "}")
									res = regex.JoinBytes(res, ' ', []byte("{{{"), arg, '=', '"', val, '"', []byte("}}}"))
								}else if bytes.HasPrefix(val, []byte("{{")) && bytes.HasSuffix(val, []byte("}}")) {
									val = bytes.TrimLeft(val, "{")
									val = bytes.TrimRight(val, "}")
									res = regex.JoinBytes(res, ' ', []byte("{{"), arg, '=', '"', val, '"', []byte("}}"))
								}else{
									res = regex.JoinBytes(res, ' ', arg, '=', '"', val, '"')
								}
							}
						}else if _, err := strconv.Atoi(arg); err != nil {
							res = regex.JoinBytes(res, ' ', arg)
						}
					}

					if selfClose == 1 {
						hasEmptyContentTag := false
						for _, cTag := range emptyContentTags {
							if bytes.Equal(cTag.tag, elm["TAG"].val) {
								if cTag.attr != nil {
									if _, ok := elm[string(cTag.attr)]; ok {
										hasEmptyContentTag = true
									}
								}else{
									hasEmptyContentTag = true
								}
								break
							}
						}

						if hasEmptyContentTag {
							res = append(res, []byte("></script>")...)
						}else{
							res = append(res, '/', '>')
						}
					}else{
						if goutil.Contains(singleHtmlTags, elm["TAG"].val) {
							res = append(res, '/', '>')
						}else{
							res = append(res, '>')
						}
					}

					write(res)
				}

				continue
			}

			// handle {{$vars}}
			if b[0] == '{' {
				b, err = reader.Peek(3)
				if err == nil && b[1] == '{' {
					escHTML := true
					if b[2] == '{' {
						escHTML = false
						reader.Discard(3)
					}else{
						reader.Discard(2)
					}

					varName := []byte{}

					b, err = reader.Peek(2)
					for err == nil && !(b[0] == '}' && b[1] == '}') {
						if b[0] == '"' || b[0] == '\'' || b[0] == '`' {
							q := b[0]
							varName = append(varName, q)

							reader.Discard(1)
							b, err = reader.Peek(2)

							for err == nil && b[0] != q {
								if b[0] == '\\' {
									if regex.Compile(`[A-Za-z]`).MatchRef(&b) {
										varName = append(varName, b[0], b[1])
									}else{
										varName = append(varName, b[1])
									}

									reader.Discard(2)
									b, err = reader.Peek(2)
									continue
								}

								varName = append(varName, b[0])

								reader.Discard(1)
								b, err = reader.Peek(2)
							}

							varName = append(varName, q)

							reader.Discard(1)
							b, err = reader.Peek(2)
							continue
						}

						varName = append(varName, b[0])

						reader.Discard(1)
						b, err = reader.Peek(2)
					}

					if b[0] == '}' && b[1] == '}' {
						reader.Discard(2)
						b, err = reader.Peek(1)

						if err == nil && b[0] == '}' {
							reader.Discard(1)
							b, err = reader.Peek(1)
						}else{
							escHTML = true
						}

						if bytes.Equal(bytes.ToLower(varName), []byte("body")) {
							if escHTML {
								write(regex.JoinBytes([]byte("{{"), varName, []byte("}}")))
							}else{
								write(regex.JoinBytes([]byte("{{{"), varName, []byte("}}}")))
							}
						}else if isLayout && bytes.Equal(bytes.ToLower(varName), []byte("head")) {
							if escHTML {
								write(goutil.EscapeHTML(addLayoutHead(opts)))
							}else{
								write(addLayoutHead(opts))
							}
						} else{
							if val, ok := GetOpt(varName, opts, true, &eachArgs); ok {
								if escHTML {
									write(goutil.EscapeHTML(goutil.ToByteArray(val)))
								}else{
									write(goutil.ToByteArray(val))
								}
							}else{
								if escHTML {
									write(regex.JoinBytes([]byte("{{"), varName, []byte("}}")))
								}else{
									write(regex.JoinBytes([]byte("{{{"), varName, []byte("}}}")))
								}
							}
						}
					}

					linePos++
					continue
				}

				b, err = reader.Peek(1)
			}

			// handle strings to prevent accidental markdown and comments
			if b[0] == '"' || b[0] == '\'' || b[0] == '`' {
				q := b[0]
				str := []byte{q}
				reader.Discard(1)
				b, err = reader.Peek(2)
				for err == nil && b[0] != q {
					if b[0] == '\\' {
						if regex.Compile(`[A-Za-z"'\']`).MatchRef(&b) {
							str = append(str, b[0], b[1])
						}else{
							str = append(str, b[1])
						}

						reader.Discard(2)
						b, err = reader.Peek(2)
						continue
					}

					str = append(str, b[0])
					reader.Discard(1)
					b, err = reader.Peek(2)
				}

				str = append(str, q)
				reader.Discard(1)
				b, err = reader.Peek(1)

				write(str)

				linePos++
				continue
			}

			// handle normal comments (//line and /*block*/)
			if b[0] == '/' {
				b, err = reader.Peek(2)
				if b[1] == '/' {
					reader.Discard(2)
					b, err = reader.Peek(1)

					var keepComment []byte
					if b[0] == '!' {
						keepComment = []byte{'/', '/', '!'}
						reader.Discard(1)
						b, err = reader.Peek(1)
					}

					for err == nil && b[0] != '\n' {
						if keepComment != nil {
							keepComment = append(keepComment, b[0])
						}
						reader.Discard(1)
						b, err = reader.Peek(1)
					}

					if keepComment != nil {
						keepComment = append(keepComment, '\n')
						write(keepComment)
					}

					reader.Discard(1)
					b, err = reader.Peek(1)
					continue
				}else if b[1] == '*' {
					reader.Discard(2)
					b, err = reader.Peek(2)

					var keepComment []byte
					if b[0] == '!' {
						keepComment = []byte{'/', '*', '!'}
						reader.Discard(1)
						b, err = reader.Peek(2)
					}

					for err == nil && !(b[0] == '*' && b[1] == '/') {
						if keepComment != nil {
							keepComment = append(keepComment, b[0])
						}
						reader.Discard(1)
						b, err = reader.Peek(2)
					}

					if keepComment != nil {
						keepComment = append(keepComment, '*', '/')
						write(keepComment)
					}
					
					reader.Discard(2)
					b, err = reader.Peek(1)
					continue
				}
			}

			if compileMarkdown(reader, &write, &b, &err, &linePos) {
				linePos++
				continue
			}

			if b[0] == '\n' {
				linePos = 0

				b, err = reader.Peek(2)
				if err == nil {
					write([]byte{'\n'})

					if !debugMode {
						skipWhitespace(reader, &b, &err)
					}else{
						reader.Discard(1)
						b, err = reader.Peek(1)
					}
				}else{
					reader.Discard(1)
					b, err = reader.Peek(1)

					if debugMode && len(componentOf) == 0 {
						write([]byte{'\n'})
					}
				}

				continue
			}else if !regex.Compile(`[\s\r\n]`).MatchRef(&[]byte{b[0]}) {
				linePos++
			}

			write([]byte{b[0]})
			reader.Discard(1)
			b, err = reader.Peek(1)
		}

		if layoutEnd != nil {
			write(layoutEnd)
		}

		writer.Flush()
		cacheReady = true
	}()

	return cacheRes
}


// Compile handles the final output and returns valid html/xhtml that can be passed to the user
//
// this method will automatically call preCompile if needed, and can read the cache file while its being written for an extra performance boost
func Compile(path string, opts map[string]interface{}) ([]byte, error) {
	compilingCount++

	if rootPath == "" || cacheTmpPath == "" {
		compilingCount--
		return []byte{}, errors.New("a root path was never chosen")
	}

	origPath := path

	// handle modified cache and layout paths
	cachePath := ""
	path = string(regex.Compile(`@([\w_-]+)(:[\w_-]+|)$`).RepStr([]byte(path), []byte{}))

	if regex.Compile(`(:[\w_-]+)$`).Match([]byte(path)) {
		path = string(regex.Compile(`(:[\w_-]+)$`).RepFunc([]byte(path), func(data func(int) []byte) []byte {
			cachePath = string(data(1))

			return []byte{}
		}))
	}

	// resolve full file path within root
	fullPath, err := goutil.JoinPath(rootPath, path + "." + fileExt)
	if err != nil {
		compilingCount--
		return []byte{}, err
	}

	// prevent path from leaking into tmp cache
	if strings.HasPrefix(fullPath, cacheTmpPath) {
		compilingCount--
		return []byte{}, errors.New("path leaked into tmp cache")
	}

	// ensure the file actually exists
	if stat, err := os.Stat(fullPath); err != nil || stat.IsDir() {
		compilingCount--
		return []byte{}, errors.New("file does not exist or is invalid")
	}


	// add constOpts where not already defined
	if constOpts != nil {
		if defOpts, err := goutil.DeepCopyJson(constOpts); err == nil {
			for key, val := range defOpts {
				if _, ok := opts[key]; !ok {
					opts[key] = val
				}
			}
		}
	}


	// check cache for pre compiled file
	compData, ok := pathCache.Get(fullPath + cachePath)
	now := time.Now().UnixMilli()
	if !ok || now - *compData.lastUsed > cacheTime {
		compData = preCompile(origPath, &opts)
	}

	// open file reader
	file, err := os.OpenFile(compData.tmp, os.O_RDONLY, 0)
	if err != nil {
		compilingCount--
		return []byte{}, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	ifLevel := [][]byte{}
	eachArgs := []KeyVal{}
	fnCont := []fnData{}

	res := []byte{}
	write := func(b []byte){
		if len(fnCont) != 0 {
			if fnCont[len(fnCont)-1].cont != nil {
				fnCont[len(fnCont)-1].cont = append(fnCont[len(fnCont)-1].cont, b...)
				return
			}else{
				found := false
				for i := len(fnCont)-1; i >= 0; i-- {
					if fnCont[len(fnCont)-1].cont != nil {
						fnCont[len(fnCont)-1].cont = append(fnCont[len(fnCont)-1].cont, b...)
						found = true
						break
					}
				}
				if found {
					return
				}
			}
		}
		res = append(res, b...)
	}

	b, err := reader.Peek(1)

	firstLoop := true
	for !(*compData.Ready || *compData.Err != nil) || err == nil {
		if firstLoop {
			firstLoop = false
		}else{
			time.Sleep(time.Nanosecond * 10)
		}

		offset := compileReadSize

		b, err = reader.Peek(compileReadSize)
		if err != nil && err.Error() == "EOF" {
			for err != nil && err.Error() == "EOF" && !(*compData.Ready || *compData.Err != nil) {
				time.Sleep(time.Nanosecond * 10)
				b, err = reader.Peek(compileReadSize)
			}

			for err != nil && err.Error() == "EOF" && offset > 0 {
				offset--
				b, err = reader.Peek(offset)
			}
		}

		// handle reading bytes
		for err == nil {
			if ind := bytes.IndexRune(b, '{'); ind != -1 {
				write(b[:ind])
				reader.Discard(ind)

				escSize := 0

				recoverPos := int64(-1)

				b, err = reader.Peek(1)
				for err == nil && b[0] == '{' && escSize < 3 {
					escSize++
					reader.Discard(1)
					recoverPos--
					b, err = reader.Peek(1)
				}
				if escSize < 2 {
					write(bytes.Repeat([]byte{'{'}, escSize))
					break
				}

				varData := []byte{}

				b, err = reader.Peek(2)
				recoverPos -= int64(skipWhitespace(reader, &b, &err))

				for err == nil && !(b[0] == '}' && b[1] == '}') {
					if b[0] == '\\' {
						if regex.Compile(`[A-Za-z]`).MatchRef(&b) {
							varData = append(varData, b[0], b[1])
						}else{
							varData = append(varData, b[1])
						}

						reader.Discard(2)
						recoverPos -= 2
						b, err = reader.Peek(2)
						continue
					}

					if b[0] == '"' {
						varData = append(varData, b[0])
						reader.Discard(1)
						recoverPos--
						b, err = reader.Peek(2)

						for err == nil && b[0] != '"' {
							if b[0] == '\\' {
								if regex.Compile(`[A-Za-z\\"]`).MatchRef(&b) {
									varData = append(varData, b[0], b[1])
								}else{
									varData = append(varData, b[1])
								}
		
								reader.Discard(2)
								recoverPos -= 2
								b, err = reader.Peek(2)
								continue
							}

							varData = append(varData, b[0])
							reader.Discard(1)
							recoverPos--
							b, err = reader.Peek(2)
						}

						varData = append(varData, b[0])
						reader.Discard(1)
						recoverPos--
						b, err = reader.Peek(2)
						continue
					}

					varData = append(varData, b[0])

					reader.Discard(1)
					recoverPos--
					b, err = reader.Peek(2)
				}

				if err != nil {
					/* file.Seek(recoverPos, os.SEEK_CUR)
					reader.Reset(file) */
					break
				}

				reader.Discard(2)

				skipWhitespace(reader, &b, &err)

				escHTML := true

				b, err = reader.Peek(1)
				if err == nil && b[0] == '}' {
					reader.Discard(1)
					if escSize >= 3 {
						escHTML = false
					}
				}

				varData = bytes.TrimSpace(varData)

				selfClosing := false
				if varData[len(varData)-1] == '/' {
					selfClosing = true
					varData = varData[:len(varData)-1]
				}

				// handle varData
				if varData[0] == '#' {
					// create special case for handling if statements and each loops
					varData = varData[1:]

					if bytes.HasPrefix(varData, []byte("if:")) || bytes.HasPrefix(varData, []byte("else:")) {
						var ind []byte
						varData = regex.Compile(`^(?:if|else):([0-9]+)\s*`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							ind = data(1)
							return []byte{}
						})

						if len(ifLevel) != 0 && bytes.Equal(ind, ifLevel[len(ifLevel)-1]) {
							// handle closing if statement
							b, err = reader.Peek(2)
							for err == nil {
								if b[0] == '{' && b[1] == '{' {
									b, err = reader.Peek(7 + len(ind))
									if err == nil {
										offset := 2
										if b[2] == '{' {
											offset++
										}

										if bytes.Equal(b[offset:offset+4+len(ind)], append([]byte("/if:"), ind...)) {
											ifLevel = ifLevel[:len(ifLevel)-1]
											skipObjStrComments(reader, &b, &err)
											break
										}
									}
								}

								skipStrComments(reader, &b, &err)
								b, err = reader.Peek(2)
							}
						}else{
							argList := regex.Compile(`([<>]=|[&\|\(\)!=^<>]|"(?:\\[\\"]|.)*?")`).SplitRef(&varData)

							args := [][]byte{}
							for _, v := range argList {
								v = bytes.TrimSpace(v)
								if len(v) == 0 {
									continue
								}
								if regex.Compile(`:[0-9]+$`).MatchRef(&v) {
									v = regex.Compile(`:[0-9]+$`).RepStrRef(&v, []byte{})
								}

								v = regex.Compile(`^(["'\'])(.*?)\1$`).RepFuncRef(&v, func(data func(int) []byte) []byte {
									return regex.Compile(`\\([\\"'\'])`).RepStrComplex(data(2), []byte("$1"))
								})
	
								args = append(args, v)
							}
	
							var res interface{}
							if len(args) == 0 {
								res = true
							}else{
								var e error
								res, e = callFuncArr("If", &args, nil, &opts, false, &eachArgs)
								if e != nil {
									compilingCount--
									return []byte{}, e
								}
							}

							if res == true {
								ifLevel = append(ifLevel, ind)
							}else{
								b, err = reader.Peek(2)
								for err == nil {
									if b[0] == '{' && b[1] == '{' {
										b, err = reader.Peek(9 + len(ind))
										if err == nil {
											offset := 2
											if b[2] == '{' {
												offset++
											}
		
											if bytes.Equal(b[offset:offset+4+len(ind)], append([]byte("/if:"), ind...)) {
												skipObjStrComments(reader, &b, &err)
												break
											}else if bytes.Equal(b[offset:offset+6+len(ind)], append([]byte("#else:"), ind...)) {
												break
											}
										}
									}
	
									skipStrComments(reader, &b, &err)
									b, err = reader.Peek(2)
								}
							}
						}
					}else if bytes.HasPrefix(varData, []byte("each:")) {
						var ind []byte
						varData = regex.Compile(`^each:([0-9]+)\s*`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							ind = data(1)
							return []byte{}
						})

						argList := bytes.Split(varData, []byte{' '})

						args := map[string][]byte{}

						k := ""
						for i, v := range argList {
							if i == 0 {
								args["1"] = v
								continue
							}

							if k == "" {
								k = string(v)
							}else{
								args[k] = v
								k = ""
							}
						}

						seekPos, e := getSeekPos(file, reader)
						if err != nil {
							compilingCount--
							return []byte{}, e
						}

						res, e := callFunc("Each", &args, nil, &opts, false, &eachArgs)
						if e != nil {
							compilingCount--
							return []byte{}, e
						}

						if reflect.TypeOf(res) == reflect.TypeOf(eachList{}) {
							// add each statement to be handled on close
							if res.(eachList).As != nil {
								k := res.(eachList).As
								eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*res.(eachList).List)[0].Val})
							}
							if res.(eachList).Of != nil {
								k := res.(eachList).Of
								eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*res.(eachList).List)[0].Key})
							}

							*res.(eachList).List = (*res.(eachList).List)[1:]

							fnCont = append(fnCont, fnData{tag: append([]byte("each:"), ind...), seek: seekPos, each: res.(eachList)})
						}else{
							// skip each loop to closing tag
							b, err = reader.Peek(2)
							for err == nil {
								if b[0] == '{' && b[1] == '{' {
									b, err = reader.Peek(9 + len(ind))
									if err == nil {
										offset := 2
										if b[2] == '{' {
											offset++
										}

										if bytes.Equal(b[offset:offset+6+len(ind)], append([]byte("/each:"), ind...)) {
											skipObjStrComments(reader, &b, &err)
											break
										}
									}
								}

								skipStrComments(reader, &b, &err)
								b, err = reader.Peek(2)
							}
						}
					}else if !bytes.HasPrefix(varData, []byte("Error:")) {
						var tag []byte
						var ind []byte
						varData = regex.Compile(`^([\w_-]+):([0-9]+)\s*`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							tag = data(1)
							ind = data(2)
							return []byte{}
						})

						args := map[string][]byte{}
						i := 1
						regex.Compile(`([\w_-]+|)=?"((?:\\[\\"]|.)*?)"`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							if len(data(1)) != 0 {
								args[string(data(1))] = regex.Compile(`\\([\\"'\'])`).RepStrComplex(data(2), []byte("$1"))
							}else{
								args[strconv.Itoa(i)] = regex.Compile(`\\([\\"'\'])`).RepStrComplex(data(2), []byte("$1"))
								i++
							}

							return []byte{}
						}, true)

						if selfClosing {
							// run func
							res, e := callFunc(string(tag), &args, nil, &opts, false, &eachArgs)
							if e == nil {
								if res != nil {
									write(goutil.ToByteArray(res))
								}
							}else if debugMode {
								fmt.Println("error:", e)
							}
						}else{
							// let closing tag run func
							fnCont = append(fnCont, fnData{tag: regex.JoinBytes(tag, ':', ind), fnName: tag, args: args, cont: []byte{}})
						}
					}
				}else if varData[0] == '/' {
					varData = varData[1:]
					
					if bytes.HasPrefix(varData, []byte("if:")) {
						var ind []byte
						varData = regex.Compile(`^(?:if|else):([0-9]+)\s*`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							ind = data(1)
							return []byte{}
						})

						if len(ifLevel) != 0 && bytes.Equal(ind, ifLevel[len(ifLevel)-1]) {
							ifLevel = ifLevel[:len(ifLevel)-1]
						}
					}else if len(fnCont) != 0 {
						var tag []byte
						var ind []byte
						varData = regex.Compile(`^([\w_-]+):([0-9]+)\s*`).RepFuncRef(&varData, func(data func(int) []byte) []byte {
							tag = data(1)
							ind = data(2)
							return []byte{}
						})

						if bytes.Equal(regex.JoinBytes(tag, ':', ind), fnCont[len(fnCont)-1].tag) {
							fn := fnCont[len(fnCont)-1]

							if bytes.Equal(tag, []byte("each")) {
								// remove eachArgs from end of list
								rmArgs := map[string][]byte{}
								if fn.each.As != nil {
									rmArgs["as"] = fn.each.As
								}
								if fn.each.Of != nil {
									rmArgs["of"] = fn.each.Of
								}

								for i := len(eachArgs)-1; i >= 0; i-- {
									if bytes.Equal(eachArgs[i].Key, rmArgs["as"]) {
										eachArgs = append(eachArgs[:i], eachArgs[i+1:]...)
										rmArgs["as"] = nil
									}else if bytes.Equal(eachArgs[i].Key, rmArgs["of"]) {
										eachArgs = append(eachArgs[:i], eachArgs[i+1:]...)
										rmArgs["of"] = nil
									}
								}

								if len(*fn.each.List) != 0 {
									// set new each args
									if fn.each.As != nil {
										k := fn.each.As
										eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*fn.each.List)[0].Val})
									}
									if fn.each.Of != nil {
										k := fn.each.Of
										eachArgs = append(eachArgs, KeyVal{Key: k, Val: (*fn.each.List)[0].Key})
									}

									*fn.each.List = (*fn.each.List)[1:]

									// return to top of each loop
									file.Seek(fn.seek, io.SeekStart)
									reader.Reset(file)
								}else{
									fnCont = fnCont[:len(fnCont)-1]
								}
							}else{
								// run other funcs
								fnCont = fnCont[:len(fnCont)-1]

								res, e := callFunc(string(fn.fnName), &fn.args, &fn.cont, &opts, false, &eachArgs)
								if e == nil {
									if res != nil {
										write(goutil.ToByteArray(res))
									}
								}else if debugMode {
									fmt.Println("error:", e)
								}
							}
						}
					}
				}else{
					if val, ok := GetOpt(regex.JoinBytes([]byte("{{"), varData, []byte("}}")), &opts, false, &eachArgs); ok {
						if reflect.TypeOf(val) == reflect.TypeOf(KeyVal{}) {
							v := goutil.EscapeHTMLArgs(goutil.ToByteArray(val.(KeyVal).Val))
							if escHTML {
								v = goutil.EscapeHTMLArgs(v)
							}

							write(regex.JoinBytes(val.(KeyVal).Key, '=', '"', v, '"'))
						}else if val != nil {
							if escHTML {
								write(goutil.EscapeHTML(goutil.ToByteArray(val)))
							}else{
								write(goutil.ToByteArray(val))
							}
						}
					}
				}

				b, err = reader.Peek(compileReadSize)
				continue
			}

			write(b)
			reader.Discard(compileReadSize)
			b, err = reader.Peek(compileReadSize)
		}

		b, err = reader.Peek(1)
	}

	// fmt.Println("debug:", "-----\n"+string(res[len(res)-8:]))

	if *compData.Err != nil {
		compilingCount--
		return res, *compData.Err
	}

	compilingCount--

	// gzip compress result
	return goutil.Compress(res)
}


// HasPreCompile returns true if a file has been pre compiled in the cache and is not expired
func HasPreCompile(path string) bool {
	if rootPath == "" || cacheTmpPath == "" {
		return false
	}

	// handle modified cache and layout paths
	path = string(regex.Compile(`@([\w_-]+)(:[\w_-]+|)$`).RepStr([]byte(path), []byte{}))

	cachePath := ""
	if regex.Compile(`(:[\w_-]+)$`).Match([]byte(path)) {
		path = string(regex.Compile(`(:[\w_-]+)$`).RepFunc([]byte(path), func(data func(int) []byte) []byte {
			cachePath = string(data(1))
			return []byte{}
		}))
	}

	// resolve full file path within root
	fullPath, err := goutil.JoinPath(rootPath, path + "." + fileExt)
	if err != nil {
		return false
	}

	// prevent path from leaking into tmp cache
	if strings.HasPrefix(fullPath, cacheTmpPath) {
		return false
	}

	cache, ok := pathCache.Get(fullPath + cachePath)
	if !ok {
		return false
	}

	now := time.Now().UnixMilli()
	if now - *cache.lastUsed > cacheTime {
		return false
	}

	return true
}


func skipWhitespace(reader *bufio.Reader, b *[]byte, err *error) int {
	i := 0
	for *err == nil && regex.Compile(`[\s\r\n]`).MatchRef(b) {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		i++
	}

	return i
}

func skipObjStrComments(reader *bufio.Reader, b *[]byte, err *error){
	var search []byte
	quote := false

	*b, *err = reader.Peek(4)
	if *err != nil {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		return
	}

	if (*b)[0] == '{' && (*b)[1] == '{' {
		if (*b)[2] == '{' {
			search = []byte("}}}")
			reader.Discard(3)
			*b, *err = reader.Peek(1)
		}else{
			search = []byte("}}")
			reader.Discard(2)
			*b, *err = reader.Peek(1)
		}
	}else if (*b)[0] == '<' && (*b)[1] == '!' && (*b)[2] == '-' && (*b)[3] == '-' {
		search = []byte("-->")
		reader.Discard(4)
		*b, *err = reader.Peek(1)
	}else if (*b)[0] == '/' {
		if (*b)[1] == '*' {
			search = []byte("*/")
			reader.Discard(2)
			*b, *err = reader.Peek(1)
		}else if (*b)[1] == '/' {
			search = []byte{'\n'}
			reader.Discard(2)
			*b, *err = reader.Peek(1)
		}
	}else if (*b)[0] == '"' {
		search = []byte{'"'}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}else if (*b)[0] == '\'' {
		search = []byte{'\''}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}else if (*b)[0] == '`' {
		search = []byte{'`'}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}

	if search == nil || *err != nil {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		return
	}

	for *err == nil {
		if search[0] == '\n' {
			if regex.Compile(`^[\r\n]+$`).MatchRef(b) {
				*b, *err = reader.Peek(2)
				if regex.Compile(`^[\r\n]+$`).MatchRef(b) {
					reader.Discard(2)
					*b, *err = reader.Peek(1)
				}else{
					reader.Discard(1)
					*b, *err = reader.Peek(1)
				}
				break
			}
		}else if (*b)[0] == search[0] {
			*b, *err = reader.Peek(len(search))
			if bytes.Equal(*b, search) {
				reader.Discard(len(search))
				*b, *err = reader.Peek(1)
				break
			}
		}

		if quote && (*b)[0] == '\\' {
			reader.Discard(2)
			*b, *err = reader.Peek(1)
			continue
		}

		skipObjStrComments(reader, b, err)
	}
}

func skipStrComments(reader *bufio.Reader, b *[]byte, err *error){
	var search []byte
	quote := false

	*b, *err = reader.Peek(4)
	if *err != nil {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		return
	}

	if (*b)[0] == '<' && (*b)[1] == '!' && (*b)[2] == '-' && (*b)[3] == '-' {
		search = []byte("-->")
		reader.Discard(4)
		*b, *err = reader.Peek(1)
	}else if (*b)[0] == '/' {
		if (*b)[1] == '*' {
			search = []byte("*/")
			reader.Discard(2)
			*b, *err = reader.Peek(1)
		}else if (*b)[1] == '/' {
			search = []byte{'\n'}
			reader.Discard(2)
			*b, *err = reader.Peek(1)
		}
	}else if (*b)[0] == '"' {
		search = []byte{'"'}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}else if (*b)[0] == '\'' {
		search = []byte{'\''}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}else if (*b)[0] == '`' {
		search = []byte{'`'}
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		quote = true
	}

	if search == nil || *err != nil {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		return
	}

	for *err == nil {
		if search[0] == '\n' {
			if regex.Compile(`^[\r\n]+$`).MatchRef(b) {
				*b, *err = reader.Peek(2)
				if regex.Compile(`^[\r\n]+$`).MatchRef(b) {
					reader.Discard(2)
					*b, *err = reader.Peek(1)
				}else{
					reader.Discard(1)
					*b, *err = reader.Peek(1)
				}
				break
			}
		}else if (*b)[0] == search[0] {
			*b, *err = reader.Peek(len(search))
			if bytes.Equal(*b, search) {
				reader.Discard(len(search))
				*b, *err = reader.Peek(1)
				break
			}
		}

		if quote && (*b)[0] == '\\' {
			reader.Discard(2)
			*b, *err = reader.Peek(1)
			continue
		}

		// skipStrComments(reader, b, err)
		reader.Discard(1)
		*b, *err = reader.Peek(1)
	}
}


func getSeekPos(file *os.File, reader *bufio.Reader) (int64, error) {
	seekPosEnd, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	seekPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// if not having weird error with current referencing as end
	if seekPos < seekPosEnd {
		return seekPos, err
	}

	size, err := reader.Read(make([]byte, seekPosEnd))
	if err != nil {
		return 0, err
	}
	reader.UnreadByte()

	seekPos, err = file.Seek(-int64(size), io.SeekEnd)
	if err != nil {
		return 0, err
	}

	reader.Reset(file)

	return seekPos, nil
}


func callFunc(name string, args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
	name = string(regex.Compile(`[^\w_]`).RepStr([]byte(name), []byte{}))

	m := reflect.ValueOf(&funcs).MethodByName(name)
	if goutil.IsZeroOfUnderlyingType(m) {
		found := false
		for key, fn := range userFuncList {
			if key == name {
				m = reflect.ValueOf(fn)
				if !goutil.IsZeroOfUnderlyingType(m) {
					found = true
				}
			}
		}

		if !found {
			return nil, errors.New("method '"+name+"' does not exist in Compiled Functions")
		}
	}


	val := m.Call([]reflect.Value{
		reflect.ValueOf(args),
		reflect.ValueOf(cont),
		reflect.ValueOf(opts),
		reflect.ValueOf(pre),
		reflect.ValueOf(addVars),
	})

	var data interface{}
	var err error
	if val[0].CanInterface() {
		data = val[0].Interface()
	}
	if val[1].CanInterface() {
		if e := val[1].Interface(); e != nil {
			err = e.(error)
		}
	}

	return data, err
}

func callFuncArr(name string, args *[][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
	name = string(regex.Compile(`[^\w_]`).RepStr([]byte(name), []byte{}))

	m := reflect.ValueOf(&funcs).MethodByName(name)
	if goutil.IsZeroOfUnderlyingType(m) {
		return nil, errors.New("method '"+name+"' does not exist in Compiled Functions")
	}

	val := m.Call([]reflect.Value{
		reflect.ValueOf(args),
		reflect.ValueOf(cont),
		reflect.ValueOf(opts),
		reflect.ValueOf(pre),
		reflect.ValueOf(addVars),
	})

	var data interface{}
	var err error
	if val[0].CanInterface() {
		data = val[0].Interface()
	}
	if val[1].CanInterface() {
		if e := val[1].Interface(); e != nil {
			err = e.(error)
		}
	}

	return data, err
}


var addingTmpPath int = 0
func tmpPath(viewPath string, tries ...int) (*os.File, string, error) {
	for addingTmpPath >= 10 {
		time.Sleep(100 * time.Nanosecond)
	}
	addingTmpPath++
	time.Sleep(10 * time.Nanosecond)
	if addingTmpPath > 10 {
		addingTmpPath--

		if len(tries) != 0 && tries[0] > 10000 {
			return nil, "", errors.New("failed to queue the generation of a unique tmp cache path within 10000 tries")
		}

		time.Sleep(1000 * time.Nanosecond)

		if len(tries) != 0 {
			return tmpPath(viewPath, tries[0] + 1)
		}
		return tmpPath(viewPath, 1)
	}

	now := time.Now().UnixNano()
	t := strconv.Itoa(int(now))
	t = t[len(t)-12:]

	time.Sleep(10 * time.Nanosecond)
	addingTmpPath--

	var tmp []byte
	var path string
	var err error
	if debugMode {
		tmp = regex.Compile(`[^\w_\-\.]+`).RepStr(regex.Compile(`/`).RepStr([]byte(viewPath), []byte{'.'}), []byte{})
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + ".cache." + fileExt)
	}else{
		tmp = goutil.RandBytes(32, nil)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)
	}

	loops := 0
	for err == nil {
		if _, e := os.Stat(path); e != nil {
			break
		}

		loops++
		if loops > 10000 {
			return nil, "", errors.New("failed to generate a unique tmp cache path within 10000 tries")
		}

		tmp = goutil.RandBytes(32, nil)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)
	}

	if _, err := os.Stat(path); err == nil {
		return nil, "", errors.New("failed to generate a unique tmp cache path")
	}

	// perm := fs.FileMode(1600)
	perm := fs.FileMode(0600)
	if debugMode {
		perm = 0755
	}

	// err = os.WriteFile(path, []byte{}, 1600)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, perm)
	if err != nil {
		return nil, path, err
	}

	return file, path, nil
}
