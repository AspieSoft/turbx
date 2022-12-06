package compiler

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"turbx/funcs"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v3"
	"github.com/alphadose/haxmap"
)

var rootPath string
var fileExt string = "html"
var componentPath string = "components"
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

type fnData struct {
	tag []byte
	args map[string][]byte
	fnName []byte
	cont []byte
	each funcs.EachList
	isComponent bool
}


var pathCache *haxmap.Map[string, string] = haxmap.New[string, string]()


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

func SetComponentPath(path string) error {
	if path == componentPath {
		return errors.New("path is already set")
	}

	componentPath = path

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
		fileExt = string(regex.Compile(`[^\w_-]`).RepStr([]byte(ext), []byte{}))
	}
}

func PreCompile(path string, opts map[string]interface{}, componentOf ...string) (string, error) {
	if rootPath == "" || cacheTmpPath == "" {
		return "", errors.New("a root path was never chosen")
	}

	var fullPath string
	if len(componentOf) != 0 {
		if p, err := goutil.JoinPath(rootPath, componentPath, path + "." + fileExt); err == nil {
			fullPath = p
		}
	}

	if fullPath == "" {
		var err error
		fullPath, err = goutil.JoinPath(rootPath, path + "." + fileExt)
		if err != nil {
			return "", err
		}
	}

	for _, cPath := range componentOf {
		if fullPath == cPath {
			return "", errors.New("Recursive Component Detected!")
		}
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

	fnLevel := []string{}
	ifMode := []uint8{0}
	tagInd := [][]byte{}
	fnCont := []fnData{}

	wSize := uint(0)
	write := func(b []byte){
		if len(fnCont) != 0 {
			fnCont[len(fnCont)-1].cont = append(fnCont[len(fnCont)-1].cont, b...)
			return
		}

		writer.Write(b)
		wSize += uint(len(b))
		if wSize >= writeFlushSize {
			writer.Flush()
			wSize = 0
		}
	}

	//todo: compile markdown while reading file (may ignore above comment for this idea)

	b, err := reader.Peek(1)
	for err == nil {
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

			// elm := map[string][2][]byte{}
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

					// fix opts for {{key="val"}} attrs
					if len(key) > 2 && key[0] == '{' && key[1] == '{' {
						if len(key) > 3 && key[2] == '{' {
							b, err = reader.Peek(3)
							if err == nil && b[0] == '}' && b[1] == '}' && b[2] == '}' {
								reader.Discard(3)
								key = key[3:]
								val = regex.JoinBytes('{', '{', '{', val, '}', '}', '}')
							}else if err == nil && b[0] == '}' && b[1] == '}' {
								reader.Discard(2)
								key = key[3:]
								val = regex.JoinBytes('{', '{', val, '}', '}')
							}
						}else{
							b, err = reader.Peek(2)
							if err == nil && b[0] == '}' && b[1] == '}' {
								reader.Discard(2)
								key = key[2:]
								val = regex.JoinBytes('{', '{', val, '}', '}')
							}
						}

						b, err = reader.Peek(1)
					}

					elm[string(bytes.ToLower(key))] = elmVal{ind, val}
					ind++
				}
			}

			if mode == 1 {
				if selfClose == 2 && len(fnCont) != 0 && !fnCont[len(fnCont)-1].isComponent && bytes.Equal(elm["TAG"].val, fnCont[len(fnCont)-1].tag) {
					fn := fnCont[len(fnCont)-1]
					fnCont = fnCont[:len(fnCont)-1]

					if bytes.Equal(elm["TAG"].val, []byte("each")) {
						eachOpts, e := goutil.DeepCopyJson(opts)
						if e != nil {
							return "", e
						}
						for i := range eachOpts {
							if !strings.HasPrefix(i, "$") {
								delete(eachOpts, i)
							}
						}

						if fn.each.In != nil {
							eachOpts[string(fn.each.In)] = fn.each.List
						}

						for _, list := range fn.each.List {
							b := regex.Compile(`(?s){{({|)\s*((?:"(?:\\[\\"]|[^"])*"|'(?:\\[\\']|[^'])*'|\'(?:\\[\\\']|[^\'])*\'|.)*?)\s*}}(}|)`, string(fn.each.As), string(fn.each.Of), string(fn.each.In)).RepFuncRef(&fn.cont, func(data func(int) []byte) []byte {
								allowHTML := false
								_ = allowHTML
								if len(data(1)) != 0 && len(data(3)) != 0 {
									allowHTML = true
								}

								if fn.each.As != nil {
									eachOpts[string(fn.each.As)] = list.Val
								}else{
									eachOpts[string(fn.each.As)] = nil
								}
								if fn.each.Of != nil {
									eachOpts[string(fn.each.Of)] = list.Key
								}else{
									eachOpts[string(fn.each.Of)] = nil
								}

								if opt, ok := funcs.GetOpt(data(0), &eachOpts); ok {
									b := goutil.ToByteArray(opt)
									if !allowHTML {
										b = goutil.EscapeHTML(b)
									}
									return b
								}

								return data(0)
							})

							write(b)
						}

						reader.Discard(1)
						b, err = reader.Peek(1)
						continue
					}

					res, e := callFunc(string(fn.fnName), &fn.args, &fn.cont, &opts, true)
					if e != nil {
						return "", e
					}

					if res != nil {
						write(goutil.ToByteArray(res))
					}

					reader.Discard(1)
					b, err = reader.Peek(1)
					continue
				}else if selfClose == 2 && (len(ifMode) == 0 || ifMode[len(ifMode)-1] == 0) && !bytes.Equal(elm["TAG"].val, []byte("else")) && !bytes.Equal(elm["TAG"].val, []byte("elif")) {
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
								if err == nil && b[1] == '_' && (bytes.Equal(b[2:6], []byte("else")) || bytes.Equal(b[2:6], []byte("elif"))) && regex.Compile(`^[\s\r\n/>]$`).MatchRef(&[]byte{b[6]}) {
									b, err = reader.Peek(1)
									fnLevel = append(fnLevel, "if")
									// ifMode = append(ifMode, 1)
									ifMode[len(ifMode)-1] = 1
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
					if selfClose == 1 {
						continue
					}

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

					res, e := callFunc("Each", &args, nil, &opts, true)
					if e != nil {
						return "", e
					}

					rt := reflect.TypeOf(res)
					if rt == goutil.VarType["byteArray"] {
						// return normal func for compiler
						write(regex.JoinBytes([]byte("{{#each:"), len(fnLevel), ' ', res, []byte("}}")))
						fnLevel = append(fnLevel, "each")
					}else if rt == reflect.TypeOf(funcs.EachList{}) {
						// get content to run in each loop
						fnCont = append(fnCont, fnData{tag: []byte("each"), each: res.(funcs.EachList), cont: []byte{}})
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

					ind := 0
					for key, arg := range elm {
						if arg.val == nil {
							args[strconv.Itoa(ind)] = []byte(key)
							ind++
						}else{
							args[key] = arg.val
						}
					}

					if selfClose == 1 {
						res, e := callFunc(string(fnName), &args, nil, &opts, true)
						if e != nil {
							return "", e
						}

						if res != nil {
							write(goutil.ToByteArray(res))
						}
					}else{
						fnCont = append(fnCont, fnData{tag: elm["TAG"].val, args: args, fnName: fnName, cont: []byte{}})
					}
				}
			}else if mode == 2 {
				if selfClose == 2 && len(fnCont) != 0 && fnCont[len(fnCont)-1].isComponent && bytes.Equal(elm["TAG"].val, fnCont[len(fnCont)-1].tag) {
					fn := fnCont[len(fnCont)-1]
					fnCont = fnCont[:len(fnCont)-1]

					compOpts, e := goutil.DeepCopyJson(opts)
					if e != nil {
						return "", e
					}

					for key, val := range fn.args {
						compOpts[key] = val
					}

					var comp []byte
					cPath, compErr := PreCompile(string(fn.fnName), compOpts, append(componentOf, fullPath)...)
					if compErr == nil {
						if c, e := os.ReadFile(cPath); e == nil {
							comp = c
						}

						pathCache.ForEach(func(key, val string) bool {
							if cPath == val {
								pathCache.Del(key)
								return false
							}

							return true
						})
						os.Remove(cPath)
					}

					if comp == nil || compErr != nil {
						if debugMode {
							fmt.Println(compErr)
							if compErr != nil {
								write([]byte("{{#Error: Turbx Component Failed: '"+string(fn.fnName)+"'\n"+compErr.Error()+"}}"))
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
					if arg.val == nil {
						args[strconv.Itoa(ind)] = []byte(key)
						ind++
					}else{
						if bytes.HasPrefix(arg.val, []byte("{{")) && bytes.HasSuffix(arg.val, []byte("}}")) {
							//todo: handle {{$vars}} within components
							// will also need to handle in a similar way for html elements
							// html elements may be easier to start with
							args[key] = arg.val
						}else{
							args["$"+key] = arg.val
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
					compOpts, e := goutil.DeepCopyJson(opts)
					if e != nil {
						return "", e
					}

					for key, val := range args {
						compOpts[key] = val
					}

					var comp []byte
					cPath, compErr := PreCompile(string(fnName), compOpts, append(componentOf, fullPath)...)
					if compErr == nil {
						if c, e := os.ReadFile(cPath); e == nil {
							comp = c
						}

						pathCache.ForEach(func(key, val string) bool {
							if cPath == val {
								pathCache.Del(key)
								return false
							}

							return true
						})
						os.Remove(cPath)
					}

					if comp == nil || compErr != nil {
						if debugMode {
							fmt.Println(compErr)
							if compErr != nil {
								write([]byte("{{#Error: Turbx Component Failed: '"+string(fnName)+"'\n"+compErr.Error()+"}}"))
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

				//todo: handle {{$vars}} within html elements
				// will also need to handle in a similar way for components

				// sort html args
				argSort := []string{}
				for key := range elm {
					if !regex.Compile(`^([A-Z]+|[0-9]+)$`).Match([]byte(key)) {
						argSort = append(argSort, key)
					}
				}
				sort.Strings(argSort)


				if publicPath != "" {
					link := ""
					if _, ok := elm["src"]; ok {
						link = "src"
					}else if _, ok := elm["href"]; ok && bytes.Equal(elm["TAG"].val, []byte("link")) {
						link = "href"
					}

					if link != "" && bytes.HasPrefix(elm[link].val, []byte{'/'}) {
						if regex.Compile(`(?<!\.min)\.(\w+)$`).Match(elm[link].val) {
							minSrc := regex.Compile(`\.(\w+)$`).RepStrComplex(elm[link].val, []byte(".min.$1"))
							if path, e := goutil.JoinPath(publicPath, string(minSrc)); e == nil {
								if _, e := os.Stat(path); e == nil {
									elm[link] = elmVal{elm[link].ind, minSrc}
								}
							}else if bytes.HasSuffix(elm[link].val, []byte(".js")) {
								//todo: auto minify js
							}else if bytes.HasSuffix(elm[link].val, []byte(".css")) {
								//todo: auto minify css
							}
						}
					}
				}

				res := regex.JoinBytes('<', elm["TAG"].val)
				for _, arg := range argSort {
					res = regex.JoinBytes(res, ' ', arg)
					if elm[arg].val != nil {
						val := regex.Compile(`([\\"])`).RepStrComplex(elm[arg].val, []byte(`\$1`))
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


		//todo: handle {{$vars}}


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
	for err != nil && regex.Compile(`[\s\r\n]`).MatchRef(b) {
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


func callFunc(name string, args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool) (interface{}, error) {
	name = string(regex.Compile(`[^\w_]`).RepStr([]byte(name), []byte{}))

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
	name = string(regex.Compile(`[^\w_]`).RepStr([]byte(name), []byte{}))

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
			b = regex.Compile(`[^\w_-]`).RepStr(b, exclude[1])
		}else{
			b = regex.Compile(`[%1]`, string(exclude[0])).RepStr(b, exclude[1])
		}
	}else if len(exclude) >= 1 {
		if exclude[0] == nil || len(exclude[0]) == 0 {
			b = regex.Compile(`[^\w_-]`).RepStr(b, []byte{})
		}else{
			b = regex.Compile(`[%1]`, string(exclude[0])).RepStr(b, []byte{})
		}
	}

	for len(b) < size {
		a := make([]byte, size)
		rand.Read(a)
		a = []byte(base64.URLEncoding.EncodeToString(a))
	
		if len(exclude) >= 2 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.Compile(`[^\w_-]`).RepStr(a, exclude[1])
			}else{
				a = regex.Compile(`[%1]`, string(exclude[0])).RepStr(a, exclude[1])
			}
		}else if len(exclude) >= 1 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.Compile(`[^\w_-]`).RepStr(a, []byte{})
			}else{
				a = regex.Compile(`[%1]`, string(exclude[0])).RepStr(a, []byte{})
			}
		}

		b = append(b, a...)
	}

	return b[:size]
}
