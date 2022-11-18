package compiler

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"turbx/funcs"

	"github.com/AspieSoft/go-regex/v3"
	"github.com/AspieSoft/goutil/v3"
)

var rootPath string
var fileExt string = "html"
var publicPath string

var cacheTmpPath string

const writeFlushSize = 1000
var debugMode = false


var preCompFuncs funcs.Pre
var compFuncs funcs.Comp

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

func init(){
	/* if regex.Match([]byte(os.Args[0]), regex.Compile(`^/tmp/go-build[0-9]+/`)) {
		debugMode = true
	} */

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
		dir, err := os.MkdirTemp("", "turbx-cache." + string(randBytes(16, nil)) + ".")
		if err != nil {
			panic(err)
		}
		cacheTmpPath = dir
	}

	SetRoot("views")

	go clearTmpCache()
}

func Close(){
	if !debugMode {
		os.RemoveAll(cacheTmpPath)
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

func SetRoot(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	rootPath = path

	go clearTmpCache()

	return nil
}

func SetPublicPath(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	publicPath = path

	return nil
}

func SetExt(ext string) {
	if ext != "" {
		fileExt = string(regex.RepStr([]byte(ext), regex.Compile(`[^\w_-]`), []byte{}))
	}
}

type Test struct {
	Key string
	Value []byte
	Fn func(t int)
}

func PreCompile(path string, opts map[string]interface{}) (string, error) {
	if rootPath == "" || cacheTmpPath == "" {
		return "", errors.New("a root path was never chosen")
	}

	fullPath, err := goutil.JoinPath(rootPath, path + "." + fileExt)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(fullPath, cacheTmpPath) {
		return "", errors.New("path leaked into tmp cache")
	}

	file, err := os.OpenFile(fullPath, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer func(){
		file.Close()
	}()

	reader := bufio.NewReader(file)

	tmpFile, tmpPath, err := tmpPath(path)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	writer := bufio.NewWriter(tmpFile)

	wSize := uint(0)
	write := func(b []byte){
		writer.Write(b)
		wSize += uint(len(b))
		if wSize >= writeFlushSize {
			writer.Flush()
			wSize = 0
		}
	}

	_ = reader
	_ = writer

	// b, _ := reader.Peek(10)
	// fmt.Println(string(b))

	//todo: compile components and pre funcs
	//todo: convert other funcs to use {{#if}} in place of <_if>
	//todo: compile const vars (leave unhandled vars for main compile method)

	//// may read multiple bytes at a time, and check if they contain '<' in the first check (define read size with a const var)
	//todo: compile markdown while reading file (may ignore above comment for this idea)

	tagInd := [][]byte{}
	_ = tagInd

	fnLevel := []string{}
	ifMode := []uint8{0}

	b, err := reader.Peek(1)
	for err == nil {
		if b[0] == '<' {
			reader.Discard(1)
			b, err = reader.Peek(1)

			// elm := map[string][2][]byte{}
			elm := map[string]elmVal{}
			ind := uint(0)

			selfClose := 0
			mode := uint8(0)
			if b[0] == '_' {
				mode = 1
				reader.Discard(1)
				b, err = reader.Peek(1)
			}else if regex.MatchRef(&b, regex.Compile(`[A-Z]`)) {
				mode = 2
			}else if b[0] == '/' {
				selfClose = 2
				reader.Discard(1)
				b, err = reader.Peek(1)

				if b[0] == '_' {
					mode = 1
					reader.Discard(1)
					b, err = reader.Peek(1)
				}else if regex.MatchRef(&b, regex.Compile(`[A-Z]`)) {
					mode = 2
				}
			}

			// handle elm tag (use all caps to prevent conflicts with attributes)
			tag := []byte{}
			for err == nil && !regex.MatchRef(&b, regex.Compile(`[\s\r\n/>]`)) {
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
					for err == nil && !regex.MatchRef(&b, regex.Compile(`[\s\r\n/>!=&|\(\)]`)) {
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
					for err == nil && b[0] != q && (q != ' ' || !regex.MatchRef(&b, regex.Compile(`[\s\r\n/>!=&|\(\)]`))) {
						if b[0] == '\\' {
							b, err = reader.Peek(2)
							if regex.MatchRef(&b, regex.Compile(`[A-Za-z]`)) {
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
	
					elm[string(bytes.ToLower(key))] = elmVal{ind, val}
					ind++

					if q != ' ' {
						reader.Discard(1)
						b, err = reader.Peek(1)
					}
				}
			}


			if mode == 1 {
				if selfClose == 2 && (len(ifMode) == 0 || ifMode[len(ifMode)-1] == 0) && !bytes.Equal(elm["TAG"].val, []byte("else")) && !bytes.Equal(elm["TAG"].val, []byte("elif")) {
					for len(fnLevel) != 0 && fnLevel[len(fnLevel)-1] != string(elm["TAG"].val) {
						fnLevel = fnLevel[:len(fnLevel)-1]
						ifMode = ifMode[:len(ifMode)-1]
					}
					if len(fnLevel) != 0 {
						fnLevel = fnLevel[:len(fnLevel)-1]
						ifMode = ifMode[:len(ifMode)-1]
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

				//todo: add seperate handler for 'each' and 'for' funcs
				// have 'for' func use ';' seperators (may need to adjust arg compiler to accept multiple reapeats of a key as an indexed version)
				// also ensure keys ending in ':[0-9]+' will allow the index to increase past them

				if bytes.Equal(elm["TAG"].val, []byte("if")) || bytes.Equal(elm["TAG"].val, []byte("else")) || bytes.Equal(elm["TAG"].val, []byte("elif")) {
					elseMode := (bytes.Equal(elm["TAG"].val, []byte("else")) || bytes.Equal(elm["TAG"].val, []byte("elif")))

					if selfClose != 0 && len(ifMode) != 0 && (ifMode[len(ifMode)-1] == 2 || ifMode[len(ifMode)-1] == 3) {

						if elseMode {
							for err == nil {
								if b[0] == '<' {
									b, err = reader.Peek(6)
									if b[1] == '/' && b[2] == '_' && bytes.Equal(b[3:5], []byte("if")) && regex.MatchRef(&[]byte{b[5]}, regex.Compile(`^[\s\r\n/>]$`)) {
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

						if ifMode[len(ifMode)-1] == 3 {
							write(regex.JoinBytes([]byte("{{/if:"), len(fnLevel)-1, []byte("}}")))
							fnLevel = fnLevel[:len(fnLevel)-1]
						}

						// fnLevel = fnLevel[:len(fnLevel)-1]
						ifMode = ifMode[:len(ifMode)-1]
						continue
					}

					// handle if statements
					intArgs := []elmVal{}
					argSize := 0
					for key, arg := range elm {
						if !regex.Match([]byte(key), regex.Compile(`^([A-Z]+)$`)) {
							intArgs = append(intArgs, elmVal{arg.ind, []byte(key)})
							argSize++
							if !regex.Match([]byte(key), regex.Compile(`^([0-9]+)$`)) && arg.val != nil {
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
						if regex.Match([]byte(arg.val), regex.Compile(`^([0-9]+)$`)) {
							args[i] = elm[string(arg.val)].val
							i++
						}else{
							args[i] = arg.val
							i++
							if elm[string(arg.val)].val != nil {
								val := elm[string(arg.val)].val
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

					res, e := callFuncArr("If", &args, nil, &opts, true)
					if e != nil {
						return "", e
					}

					if res == true {
						if elseMode && ifMode[len(ifMode)-1] != 1 {
							write(regex.JoinBytes([]byte("{{#else:"), len(fnLevel)-1, []byte("}}")))
							ifMode[len(ifMode)-1] = 3
						}else{
							ifMode[len(ifMode)-1] = 2
						}
					}else if res == false {
						for err == nil {
							if b[0] == '<' {
								b, err = reader.Peek(7)
								if b[1] == '_' && (bytes.Equal(b[2:6], []byte("else")) || bytes.Equal(b[2:6], []byte("elif"))) && regex.MatchRef(&[]byte{b[6]}, regex.Compile(`^[\s\r\n/>]$`)) {
									b, err = reader.Peek(1)
									fnLevel = append(fnLevel, "if")
									// ifMode = append(ifMode, 1)
									ifMode[len(ifMode)-1] = 1
									break
								}else if b[1] == '/' && b[2] == '_' && bytes.Equal(b[3:5], []byte("if")) && regex.MatchRef(&[]byte{b[5]}, regex.Compile(`^[\s\r\n/>]$`)) {
									b, err = reader.Peek(1)
									break
								}
							}

							skipObjStrComments(reader, &b, &err)
						}

					}else if reflect.TypeOf(res) == goutil.VarType["byteArray"] {
						if elseMode {
							if ifMode[len(ifMode)-1] == 1 {
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
							if ifMode[len(ifMode)-1] == 1 {
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
					args := map[string][]byte{}

					ind := 0
					for key, arg := range elm {
						if arg.val == nil {
							args[strconv.Itoa(ind)] = []byte(key)
							ind++
						}else{
							args[key] = arg.val
						}
					}
					/* sort.Slice(intArgs, func(i, j int) bool {
						return intArgs[i].ind < intArgs[j].ind
					}) */

					res, e := callFunc("Each", &args, nil, &opts, true)
					if e != nil {
						return "", e
					}
					_ = res

					// fmt.Println(res)
				}else{
					//todo: handle normal pre functions
					//// (each will not be a pre func)
				}
			}else if mode == 2 {
				//todo: handle component
				// @args: map[string][]byte
			}else{
				// handle html tags
				if selfClose == 2 {
					write(regex.JoinBytes('<', '/', elm["TAG"].val, '>'))
					reader.Discard(1)
					b, err = reader.Peek(1)
					continue
				}

				// sort html args
				argSort := []string{}
				for key := range elm {
					if !regex.Match([]byte(key), regex.Compile(`^([A-Z]+|[0-9]+)$`)) {
						argSort = append(argSort, key)
					}
				}
				sort.Strings(argSort)

				// convert .min for .js and .css src attrs
				//todo: may auto minify files instead of ignoring (may make optional)
				if _, ok := elm["src"]; ok && publicPath != "" && elm["src"].val != nil && bytes.HasPrefix(elm["src"].val, []byte{'/'}) {
					if regex.Match(elm["src"].val, regex.Compile(`(?<!\.min)\.(js|css)$`)) {
						src := regex.RepStrComplex(elm["src"].val, regex.Compile(`\.(js|css)$`), []byte(".min.$1"))
						if path, e := goutil.JoinPath(publicPath, string(src)); e == nil {
							if _, e := os.Stat(path); e == nil {
								elm["src"] = elmVal{elm["src"].ind, src}
							}
						}
					}
				}

				res := regex.JoinBytes('<', elm["TAG"].val)
				for _, arg := range argSort {
					res = regex.JoinBytes(res, ' ', arg)
					if elm[arg].val != nil {
						val := regex.RepStrComplex(elm[arg].val, regex.Compile(`([\\"])`), []byte(`\$1`))
						res = regex.JoinBytes(res, '=', '"', val, '"')
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

		//temp
		// break
		write(b)
		reader.Discard(1)
		b, err = reader.Peek(1)
	}

	writer.Flush()

	//todo: store tmpPath in a cache for compiler to reference
	// remember to clear the cache and files occasionally and on detected dir changes with watchDir from goutil
	return tmpPath, nil
}


func skipWhitespace(reader *bufio.Reader, b *[]byte, err *error){
	for err != nil && regex.MatchRef(b, regex.Compile(`[\s\r\n]`)) {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
	}
}

func skipObjStrComments(reader *bufio.Reader, b *[]byte, err *error){
	var search []byte
	quote := false

	*b, *err = reader.Peek(4)
	if err != nil {
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

	if search == nil || err != nil {
		reader.Discard(1)
		*b, *err = reader.Peek(1)
		return
	}

	for err == nil {
		if search[0] == '\n' {
			if regex.MatchRef(b, regex.Compile(`^[\r\n]+$`)) {
				*b, *err = reader.Peek(2)
				if regex.MatchRef(b, regex.Compile(`^[\r\n]+$`)) {
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


func callFunc(name string, args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool) (interface{}, error) {
	name = string(regex.RepStr([]byte(name), regex.Compile(`[^\w_]`), []byte{}))

	var m reflect.Value
	if pre {
		m = reflect.ValueOf(&preCompFuncs).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}
	}else{
		m = reflect.ValueOf(&compFuncs).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			// return nil, errors.New("method does not exist in Compiled Functions")
			m = reflect.ValueOf(&preCompFuncs).MethodByName(name)
			if goutil.IsZeroOfUnderlyingType(m) {
				return nil, errors.New("method does not exist in Compiled Functions")
			}
		}
	}

	val := m.Call([]reflect.Value{
		reflect.ValueOf(args),
		reflect.ValueOf(cont),
		reflect.ValueOf(opts),
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

func callFuncArr(name string, args *[][]byte, cont *[]byte, opts *map[string]interface{}, pre bool) (interface{}, error) {
	name = string(regex.RepStr([]byte(name), regex.Compile(`[^\w_]`), []byte{}))

	var m reflect.Value
	if pre {
		m = reflect.ValueOf(&preCompFuncs).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}
	}else{
		m = reflect.ValueOf(&compFuncs).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			// return nil, errors.New("method does not exist in Compiled Functions")
			m = reflect.ValueOf(&preCompFuncs).MethodByName(name)
			if goutil.IsZeroOfUnderlyingType(m) {
				return nil, errors.New("method does not exist in Compiled Functions")
			}
		}
	}

	val := m.Call([]reflect.Value{
		reflect.ValueOf(args),
		reflect.ValueOf(cont),
		reflect.ValueOf(opts),
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
		tmp = []byte(viewPath)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + ".cache." + fileExt)
	}else{
		tmp = randBytes(32, nil)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)
	}

	loops := 0
	for err != nil {
		if _, e := os.Stat(path); e != nil {
			break
		}

		loops++
		if loops > 10000 {
			return nil, "", errors.New("failed to generate a unique tmp cache path within 10000 tries")
		}

		tmp = randBytes(32, nil)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)
	}

	if _, err := os.Stat(path); err == nil {
		return nil, "", errors.New("failed to generate a unique tmp cache path")
	}

	perm := fs.FileMode(1600)
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

func randBytes(size int, exclude ...[]byte) []byte {
	b := make([]byte, size)
	rand.Read(b)
	b = []byte(base64.URLEncoding.EncodeToString(b))

	if len(exclude) >= 2 {
		if exclude[0] == nil || len(exclude[0]) == 0 {
			b = regex.RepStr(b, regex.Compile(`[^\w_-]`), exclude[1])
		}else{
			b = regex.RepStr(b, regex.Compile(`[%1]`, string(exclude[0])), exclude[1])
		}
	}else if len(exclude) >= 1 {
		if exclude[0] == nil || len(exclude[0]) == 0 {
			b = regex.RepStr(b, regex.Compile(`[^\w_-]`), []byte{})
		}else{
			b = regex.RepStr(b, regex.Compile(`[%1]`, string(exclude[0])), []byte{})
		}
	}

	for len(b) < size {
		a := make([]byte, size)
		rand.Read(a)
		a = []byte(base64.URLEncoding.EncodeToString(a))
	
		if len(exclude) >= 2 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.RepStr(a, regex.Compile(`[^\w_-]`), exclude[1])
			}else{
				a = regex.RepStr(a, regex.Compile(`[%1]`, string(exclude[0])), exclude[1])
			}
		}else if len(exclude) >= 1 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.RepStr(a, regex.Compile(`[^\w_-]`), []byte{})
			}else{
				a = regex.RepStr(a, regex.Compile(`[%1]`, string(exclude[0])), []byte{})
			}
		}

		b = append(b, a...)
	}

	return b[:size]
}
