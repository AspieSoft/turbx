package compiler

import (
	"bytes"
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
)

type Config struct {
	Root string
	Ext string
	Static string
	StaticUrl string
	DebugMode bool
}

var compilerConfig Config

func SetConfig(config Config) error {
	if config.Root != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}
		compilerConfig.Root = path
	}

	if config.Static != "" {
		path, err := filepath.Abs(config.Root)
		if err != nil {
			return err
		}
		compilerConfig.Static = path
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

	compilerConfig.DebugMode = config.DebugMode

	return nil
}

func init(){
	root, err := filepath.Abs("views")
	if err != nil {
		root = "views"
	}

	static, err := filepath.Abs("public")
	if err != nil {
		root = "public"
	}

	compilerConfig = Config{
		Root: root,
		Ext: "html",
		Static: static,
		StaticUrl: "",
		DebugMode: false,
	}
}

func Close(){
	// time.Sleep(3 * time.Second)
}


type tagData struct {
	tag []byte
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

type htmlArgs struct {
	args map[string][]byte
	ind []string
	tag []byte
	close uint8

	passToComp bool
}

type EachArgs struct {
	listMap map[string]interface{}
	listArr []interface{}
	key []byte
	val []byte
	ind uint
	size uint

	passToComp bool
}

type htmlChanList struct {
	tag chan handleHtmlData
	comp chan handleHtmlData
	fn chan handleHtmlData

	running *uint8
}

type handleHtmlData struct {
	html *[]byte
	options *map[string]interface{}
	arguments *htmlArgs
	eachArgs []EachArgs
	compileError *error
	componentList [][]byte

	fn *func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte
	preComp bool

	stopChan bool
}

func PreCompile(path string, opts map[string]interface{}) error {
	path, err := goutil.FS.JoinPath(compilerConfig.Root, path + "." + compilerConfig.Ext)
	if err != nil {
		if compilerConfig.DebugMode {
			fmt.Println(err)
		}
		return err
	}

	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		if compilerConfig.DebugMode {
			fmt.Println(err)
		}
		return err
	}

	htmlChan := newPreCompileChan()

	html := []byte{0}
	preCompile(path, &opts, &htmlArgs{}, &html, &err, &htmlChan, nil, nil)
	if err != nil {
		if compilerConfig.DebugMode {
			fmt.Println(err)
			html = append(html, regex.JoinBytes([]byte("<!--{{#error: "), regex.Comp(`%1`, compilerConfig.Root).RepStr([]byte(err.Error()), []byte{}), []byte("}}-->"))...)
		}else{
			return err
		}
	}

	//todo: add precompiled file to temp cache
	fmt.Println("----------\n", string(html[1:]))

	if err != nil {
		return err
	}
	return nil
}

func preCompile(path string, options *map[string]interface{}, arguments *htmlArgs, html *[]byte, compileError *error, htmlChan *htmlChanList, eachArgsList []EachArgs, componentList [][]byte){
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
					k = "$"+k
				}
				
				if k == "$body" {
					opts[k] = v
					continue
				}

				if v != nil && len(v) != 0 && v[0] == 0 {
					v = v[1:]

					if len(v) != 0 && bytes.HasPrefix(v, []byte("{{")) && bytes.HasSuffix(v, []byte("}}")) {
						v = v[2:len(v)-2]
	
						if len(v) == 0 {
							continue
						}
	
						esc := uint8(2)
	
						if len(v) >= 2 && v[0] == '{' && v[1] == '}' {
							esc = 0
							v = v[1:len(v)-1]
						}else if v[0] == '{' {
							v = v[1:]
						}else if v[len(v)-1] == '}' {
							v = v[:len(v)-1]
						}
	
						if val := GetOpt(v, options, &eachArgsList, esc, true, false); val != nil {
							opts[k] = val
						}
					}else{
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


	htmlRes := []byte{}
	htmlTags := []*[]byte{}
	htmlTagsErr := []*error{}

	htmlContTemp := [][]byte{}
	htmlContTempTag := []htmlArgs{}
	write := func(b []byte){
		if len(htmlContTempTag) != 0 {
			htmlContTemp[len(htmlContTempTag)-1] = append(htmlContTemp[len(htmlContTempTag)-1], b...)
		}else{
			htmlRes = append(htmlRes, b...)
		}
	}

	ifTagLevel := []uint8{}

	var buf byte
	for err == nil {
		buf, err = reader.PeekByte(0)
		if buf == 0 {
			break
		}

		if buf == '<' { // handle html tags
			args := htmlArgs{
				args: map[string][]byte{},
				ind: []string{},
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
					}else if b == '>' {
						if args.close == 0 {
							args.close = 3
						}
						break
					}else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
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
						}else if b == '>' {
							if args.close == 0 {
								args.close = 3
							}
							break
						}else if b == '&' || b == '|' || b == '(' || b == ')' {
							i := strconv.Itoa(argInd)
							args.args[i] = []byte{5, b}
							args.ind = append(args.ind, i)
							argInd++
							continue
						}else if regex.Comp(`[\s\r\n]`).MatchRef(&[]byte{b}) {
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
								key = key[2:len(key)-2]
								isVar++
	
								if bytes.HasPrefix(key, []byte("{")) && bytes.HasSuffix(key, []byte("}")) {
									key = key[1:len(key)-1]
									isVar++
								}else if bytes.HasPrefix(key, []byte("{")) {
									key = key[1:]
								}else if bytes.HasSuffix(key, []byte("}")) {
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
								}else{
									isVar = 1
								}
							}else if len(val) >= 2 && val[len(val)-2] == '}' && val[len(val)-1] == '}' {
								val = val[:len(val)-2]
								if len(val) >= 1 && val[len(val)-1] == '}' {
									val = val[:len(val)-1]
								}else{
									isVar = 1
								}
							}else if isVar == 2 {
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
						}else{
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
									}else{
										// add string for compiler result and check else content
										write(regex.JoinBytes([]byte("{{%if "), precompStr, []byte("}}")))
										ifTagLevel = append(ifTagLevel, 2)
									}
								}else{
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
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
											}
										}else if ib == '<' {
											ibTag, ie := reader.Peek(8)
											if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
												break
											}else if ie == nil && regex.Comp(`^</?_?(if)[\s/>:]`).MatchRef(&ibTag) {
												if ibTag[1] == '/' {
													ifLevel--
													if ifLevel < 0 {
														break
													}
												}else{
													ifLevel++
												}
											}
										}

										reader.Discard(1)
										ib, ie = reader.PeekByte(0)
									}
								}
							}else if args.close == 1 && len(ifTagLevel) != 0 && (bytes.Equal(args.tag, []byte("_if")) || bytes.Equal(args.tag, []byte("if"))) {
								if ifTagLevel[len(ifTagLevel)-1] == 1 || ifTagLevel[len(ifTagLevel)-1] == 2 {
									write([]byte("{{%/if}}"))
								}else{
									removeLineBreak(reader)
								}
								ifTagLevel = ifTagLevel[:len(ifTagLevel)-1]
							}else if len(ifTagLevel) != 0 && regex.Comp(`(?i)^_?(el(?:se|if)|else_?if)$`).MatchRef(&args.tag) {
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
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
											}
										}else if ib == '<' {
											ibTag, ie := reader.Peek(8)
											if ie == nil && ifLevel == 0 && regex.Comp(`^</_?(if)[\s/>:]`).MatchRef(&ibTag) {
												break
											}else if ie == nil && regex.Comp(`^</?_?(if)[\s/>:]`).MatchRef(&ibTag) {
												if ibTag[1] == '/' {
													ifLevel--
													if ifLevel < 0 {
														ifLevel = 0
													}
												}else{
													ifLevel++
												}
											}
										}

										reader.Discard(1)
										ib, ie = reader.PeekByte(0)
									}
								}else if ifTagLevel[len(ifTagLevel)-1] == 2 { // string if statement
									if precompStr, ok := TagFuncs.If(options, &args, &eachArgsList, true); ok {
										if precompStr == nil {
											// grab content and skip next else content
											ifTagLevel[len(ifTagLevel)-1] = 1
											write([]byte("{{%else}}"))
										}else{
											// add string for compiler result and check else content
											write(regex.JoinBytes([]byte("{{%else "), precompStr, []byte("}}")))
										}
									}else{
										// skip if content and move on to next else tag
										ib, ie := reader.PeekByte(0)
										ifLevel := 0
										for ie == nil {
											if ib == '"' || ib == '\'' || ib == '`' {
												q := ib
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
												for ie == nil && ib != q {
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
												}
											}else if ib == '<' {
												ibTag, ie := reader.Peek(8)
												if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
													break
												}else if ie == nil && regex.Comp(`^</?_?(if)[\s/>:]`).MatchRef(&ibTag) {
													if ibTag[1] == '/' {
														ifLevel--
														if ifLevel < 0 {
															break
														}
													}else{
														ifLevel++
													}
												}
											}

											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
										}
									}
								}else if ifTagLevel[len(ifTagLevel)-1] == 3 { // false if statement
									if precompStr, ok := TagFuncs.If(options, &args, &eachArgsList, true); ok {
										if precompStr == nil {
											// grab if content and skip else content
											ifTagLevel[len(ifTagLevel)-1] = 0
											removeLineBreak(reader)
										}else{
											// add string for compiler result and check else content
											ifTagLevel[len(ifTagLevel)-1] = 2
											write(regex.JoinBytes([]byte("{{%if "), precompStr, []byte("}}")))
										}
									}else{
										// skip if content and move on to next else tag
										ib, ie := reader.PeekByte(0)
										ifLevel := 0
										for ie == nil {
											if ib == '"' || ib == '\'' || ib == '`' {
												q := ib
												reader.Discard(1)
												ib, ie = reader.PeekByte(0)
												for ie == nil && ib != q {
													reader.Discard(1)
													ib, ie = reader.PeekByte(0)
												}
											}else if ib == '<' {
												ibTag, ie := reader.Peek(8)
												if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(el(?:se|if)|else_?if)[\s/>:]`).MatchRef(&ibTag) {
													break
												}else if ie == nil && regex.Comp(`^</?_?(if)[\s/>:]`).MatchRef(&ibTag) {
													if ibTag[1] == '/' {
														ifLevel--
														if ifLevel < 0 {
															break
														}
													}else{
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
						}else if regex.Comp(`(?i)^_?(each|for|for_?each)$`).MatchRef(&args.tag) {
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
											}else if t == goutil.VarType["[]interface{}"] && len(listArg.([]interface{})) != 0 {
												eachArgs.listArr = listArg.([]interface{})
											}else{
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
															reader.Discard(1)
															ib, ie = reader.PeekByte(0)
														}
													}else if ib == '<' {
														ibTag, ie := reader.Peek(8)
														if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
															break
														}else if ie == nil && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
															if ibTag[1] == '/' {
																ifLevel--
																if ifLevel < 0 {
																	break
																}
															}else{
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
											}else if args.args["of"] != nil && len(args.args["of"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.key = args.args["of"][1:]
											}

											if args.args["value"] != nil && len(args.args["value"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.val = args.args["value"][1:]
											}else if args.args["as"] != nil && len(args.args["as"]) != 0 && args.args["0"][0] == 0 {
												eachArgs.val = args.args["as"][1:]
											}

											eachArgsList = append(eachArgsList, eachArgs)
											reader.Save()

											removeLineBreak(reader)
											continue
										}
									}else{
										// return new each function to run in compiler
										argStr := args.args["0"][1:]
										eachArgs := EachArgs{passToComp: true}

										if args.args["value"] != nil && len(args.args["value"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["value"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										}else if args.args["val"] != nil && len(args.args["val"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["val"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										}else if args.args["as"] != nil && len(args.args["as"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.val = args.args["as"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" as=\""), eachArgs.val, '"')
										}

										if args.args["key"] != nil && len(args.args["key"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.key = args.args["key"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" of=\""), eachArgs.key, '"')
										}else if args.args["of"] != nil && len(args.args["of"]) != 0 && args.args["0"][0] == 0 {
											eachArgs.key = args.args["of"][1:]
											argStr = regex.JoinBytes(argStr, []byte(" of=\""), eachArgs.key, '"')
										}

										eachArgsList = append(eachArgsList, eachArgs)
										write(regex.JoinBytes([]byte("{{%each"), ' ', argStr, []byte("}}")))

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
											reader.Discard(1)
											ib, ie = reader.PeekByte(0)
										}
									}else if ib == '<' {
										ibTag, ie := reader.Peek(8)
										if ie == nil && ifLevel == 0 && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
											break
										}else if ie == nil && regex.Comp(`^</?_?(each|for|for_?each)[\s/>:]`).MatchRef(&ibTag) {
											if ibTag[1] == '/' {
												ifLevel--
												if ifLevel < 0 {
													break
												}
											}else{
												ifLevel++
											}
										}
									}

									reader.Discard(1)
									ib, ie = reader.PeekByte(0)
								}
							}else if args.close == 1 {
								if len(eachArgsList) != 0 {
									if eachArgsList[len(eachArgsList)-1].passToComp {
										write([]byte("{{%/each}}"))
										eachArgsList = eachArgsList[:len(eachArgsList)-1]
									}else if eachArgsList[len(eachArgsList)-1].ind < eachArgsList[len(eachArgsList)-1].size-1 {
										if eachArgsList[len(eachArgsList)-1].ind == 0 {
											reader.Restore()
											removeLineBreak(reader)
										}else{
											reader.RestoreReset()
											removeLineBreak(reader)
										}
										eachArgsList[len(eachArgsList)-1].ind++
									}else{
										reader.DelSave()
										removeLineBreak(reader)
										eachArgsList = eachArgsList[:len(eachArgsList)-1]
									}
								}
							}
						}else if args.tag[0] == '_' && len(args.tag) > 1 {
							args.tag = bytes.ToLower(args.tag)
							args.tag[1] = bytes.ToUpper([]byte{args.tag[1]})[0]

							if args.close == 3 {
								htmlContTempTag = append(htmlContTempTag, args)
								htmlContTemp = append(htmlContTemp, []byte{})
							}else if args.close == 1 && len(htmlContTempTag) != 0 {
								for i := len(htmlContTempTag)-1; i >= 0; i-- {
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
											htmlChan.fn <- handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList}
										}else{
											handleHtmlFunc(handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList})
										}
										write([]byte{0})
									}else{
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
							}else if args.close == 2 {
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
										htmlChan.fn <- handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList}
									}else{
										handleHtmlFunc(handleHtmlData{fn: &fn, preComp: true, html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList})
									}
									write([]byte{0})
								}
							}
						}else if args.tag[0] == bytes.ToUpper([]byte{args.tag[0]})[0] {
							if args.close == 3 {
								htmlContTempTag = append(htmlContTempTag, args)
								htmlContTemp = append(htmlContTemp, []byte{})
							}else if args.close == 1 && bytes.Equal(args.tag, htmlContTempTag[len(htmlContTemp)-1].tag) {
								for k, v := range htmlContTempTag[len(htmlContTemp)-1].args {
									args.args[k] = v
								}
								args.args["body"] = htmlContTemp[len(htmlContTempTag)-1]

								htmlContTemp = htmlContTemp[:len(htmlContTempTag)-1]
								htmlContTempTag = htmlContTempTag[:len(htmlContTempTag)-1]

								htmlCont := []byte{0}
								var compErr error
								htmlTags = append(htmlTags, &htmlCont)
								htmlTagsErr = append(htmlTagsErr, &compErr)

								if htmlChan != nil {
									htmlChan.comp <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList}
								}else{
									handleHtmlComponent(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList})
								}
								write([]byte{0})
							}else if args.close == 2 {
								htmlCont := []byte{0}
								var compErr error
								htmlTags = append(htmlTags, &htmlCont)
								htmlTagsErr = append(htmlTagsErr, &compErr)

								if htmlChan != nil {
									htmlChan.comp <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList}
								}else{
									handleHtmlComponent(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr, componentList: componentList})
								}
								write([]byte{0})
							}
						}else{
							// handle normal tags
							if (args.close == 3 || args.close == 1) && goutil.Contains(singleHtmlTags, bytes.ToLower(args.tag)) {
								args.close = 2
							}

							htmlCont := []byte{0}
							var compErr error
							htmlTags = append(htmlTags, &htmlCont)
							htmlTagsErr = append(htmlTagsErr, &compErr)

							// pass through channel instead of a goroutine (like a queue)
							if htmlChan != nil {
								htmlChan.tag <- handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr}
							}else{
								handleHtmlTag(handleHtmlData{html: &htmlCont, options: options, arguments: &args, eachArgs: cloneArr(eachArgsList), compileError: &compErr})
							}
							write([]byte{0})
						}

						continue
					}
				}
			}
		}else if buf == '{' { // handle html vars
			ind := uint(3)
			if b, e := reader.Peek(3); e == nil {
				if b[0] == '{' && b[1] == '{' {
					esc := uint8(2)
					if b[2] == '{' {
						esc = 0
					}else{
						ind--
					}

					b, e = reader.Get(ind, 2)
					for e == nil && !(b[0] == '}' && b[1] == '}') && b[0] != '\r' && b[0] != '\n' {
						if b[0] == '"' || b[0] == '\'' || b[0] == '`' {
							q := b[0]
							ind++
							b, e = reader.Get(ind, 2)
							for e == nil && b[0] != q {
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
						}else{
							reader.Discard(2)
						}

						if esc == 0 {
							b, e = reader.Peek(ind-3)
							reader.Discard(ind-1)
						}else{
							b, e = reader.Peek(ind-2)
							reader.Discard(ind)
						}

						if p, e := reader.PeekByte(0); e == nil {
							if p == '}' {
								reader.Discard(1)
							}else{
								esc = 2
							}
						}else{
							esc = 2
						}

						val := GetOpt(b, options, &eachArgsList, esc, true, true)
						if !goutil.IsZeroOfUnderlyingType(val) {
							write(goutil.Conv.ToBytes(val))
						}

						continue
					}
				}
			}
		}

		//todo: add optional shortcode handler (ie: {{#shortcode@plugin}} {{#priorityShortcode}}) ("@plugin" should be optional)
		// may add in a "#shortcode" option to options, and pass in a list of functions that return html/markdown
		// may also add a mothod for shortcodes to run other shortcodes (apart from themselves)
		// may have shortcodes run in elixir or another lightweight programming language (may also add subfolder for shortcodes)

		//todo: consider using 'AspieSoft/go-memshare' module if a funcs.go file is detected in the $PWD directory and link it to the TagFuncs.AddFN method

		//todo: handle markdown

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
	htmlTagsInd := uint(0)
	i := bytes.IndexByte(htmlRes, 0)
	for i != -1 {
		*html = append(*html, htmlRes[:i]...)
		htmlRes = htmlRes[i+1:]

		if htmlTagsInd >= uint(len(htmlTags)) {
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

		*html = append(*html, (*htmlCont)[1:]...)
		htmlTagsInd++

		i = bytes.IndexByte(htmlRes, 0)
	}

	*html = append(*html, htmlRes...)
	(*html)[0] = 1
}

func handleHtmlTag(htmlData handleHtmlData){
	//htmlData: html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error

	if htmlData.arguments.close == 1 {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes([]byte{'<', '/'}, htmlData.arguments.tag, '>')...)
		(*htmlData.html)[0] = 1
		return
	}

	sort.Strings(htmlData.arguments.ind)

	for _, v := range htmlData.arguments.ind {
		if htmlData.arguments.args[v][0] == 0 {
			htmlData.arguments.args[v] = htmlData.arguments.args[v][1:]
		}else if htmlData.arguments.args[v][0] == 1 {
			esc := uint8(3)
			if _, err := strconv.Atoi(v); err == nil {
				esc = 4
			}

			arg := GetOpt(htmlData.arguments.args[v][1:], htmlData.options, &htmlData.eachArgs, esc, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(htmlData.arguments.args, v)
				continue
			}else{
				htmlData.arguments.args[v] = goutil.Conv.ToBytes(arg)
			}
		}else if htmlData.arguments.args[v][0] == 2 {
			arg := GetOpt(htmlData.arguments.args[v][1:], htmlData.options, &htmlData.eachArgs, 1, true, true)
			if goutil.IsZeroOfUnderlyingType(arg) {
				delete(htmlData.arguments.args, v)
				continue
			}else{
				htmlData.arguments.args[v] = goutil.Conv.ToBytes(arg)
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
				args = append(args, htmlData.arguments.args[v])
			}else{
				if bytes.HasPrefix(htmlData.arguments.args[v], []byte{0, '{', '{'}) && bytes.HasSuffix(htmlData.arguments.args[v], []byte("}}")) {
					htmlData.arguments.args[v] = htmlData.arguments.args[v][1:]

					size := 2
					if htmlData.arguments.args[v][2] == '{' && htmlData.arguments.args[v][len(htmlData.arguments.args[v])-3] == '}' {
						size = 3
					}

					if htmlData.arguments.args[v][size] == '=' {
						args = append(args, regex.JoinBytes(bytes.Repeat([]byte("{"), size), v, htmlData.arguments.args[v][size:len(htmlData.arguments.args[v])-size], bytes.Repeat([]byte("}"), size)))
					}
				}else{
					htmlData.arguments.args[v] = regex.Comp(`({{+|}}+)`).RepFunc(htmlData.arguments.args[v], func(data func(int) []byte) []byte {
						return bytes.Join(bytes.Split(data(1), []byte{}), []byte{'\\'})
					})

					//todo: check local js and css link args for .min files

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

	//todo: auto fix "emptyContentTags" to closing (ie: <script/> <iframe/>)

	if len(args) == 0 {
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes('<', htmlData.arguments.tag)...)
	}else{
		(*htmlData.html) = append((*htmlData.html), regex.JoinBytes('<', htmlData.arguments.tag, ' ', bytes.Join(args, []byte{' '}))...)
	}

	if htmlData.arguments.close == 2 {
		(*htmlData.html) = append((*htmlData.html), '/', '>')
	}else{
		(*htmlData.html) = append((*htmlData.html), '>')
	}

	(*htmlData.html)[0] = 1
}

func handleHtmlFunc(htmlData handleHtmlData){
	//htmlData: fn *func(/*tag function args*/)[]byte, preComp bool, html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error

	res := (*htmlData.fn)(htmlData.options, htmlData.arguments, &htmlData.eachArgs, htmlData.preComp)
	if res != nil && len(res) != 0 {
		if res[0] == 0 {
			if htmlData.preComp {
				if body, ok := htmlData.arguments.args["body"]; ok {
					*htmlData.html = append(*htmlData.html, regex.JoinBytes([]byte("{{%"), htmlData.arguments.tag[1:], ' ', res[1:], []byte("}}"), body, []byte("{{%/"), htmlData.arguments.tag[1:], []byte("}}"))...)
				}else{
					*htmlData.html = append(*htmlData.html, regex.JoinBytes([]byte("{{%"), htmlData.arguments.tag[1:], ' ', res[1:], []byte("/}}"))...)
				}
			}
		}else if res[0] == 1 {
			*htmlData.compileError = errors.New(string(res[1:]))
			(*htmlData.html)[0] = 2
			return
		}else{
			*htmlData.html = append(*htmlData.html, res...)
		}
	}

	// set first index to 1 to mark as ready
	(*htmlData.html)[0] = 1
}

func handleHtmlComponent(htmlData handleHtmlData){
	//htmlData: html *[]byte, options *map[string]interface{}, arguments *htmlArgs, eachArgs *[]EachArgs, compileError *error, componentList [][]byte

	// note: components cannot wait in the same channel as their parents without possibly getting stuck (ie: waiting for a parent that is also waiting for itself)

	for _, tag := range htmlData.componentList {
		if bytes.Equal(htmlData.arguments.tag, tag) {
			*htmlData.compileError = errors.New("recursion detected in component:\n  in: '"+string(htmlData.componentList[len(htmlData.componentList)-1])+"'\n  with: '"+string(htmlData.arguments.tag)+"'\n  contains:\n    '"+string(bytes.Join(htmlData.componentList, []byte("'\n    '")))+"'\n")
			(*htmlData.html)[0] = 2
			return
		}
	}

	// get component filepath
	path := string(regex.Comp(`\.`).RepStr(regex.Comp(`[^\w_\-\.]`).RepStrRef(&htmlData.arguments.tag, []byte{}), []byte{'/'}))

	path, err := goutil.FS.JoinPath(compilerConfig.Root, path + "." + compilerConfig.Ext)
	if err != nil {
		*htmlData.compileError = err
		(*htmlData.html)[0] = 2
		return
	}

	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		*htmlData.compileError = errors.New("component not found: '"+string(htmlData.arguments.tag)+"'")
		(*htmlData.html)[0] = 2
		return
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
	(*htmlData.html)[0] = 1
}

func newPreCompileChan() htmlChanList {
	tagChan := make(chan handleHtmlData)
	compChan := make(chan handleHtmlData)
	fnChan := make(chan handleHtmlData)

	running := uint8(3)
	mu := sync.Mutex{}

	go func(){
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

	go func(){
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

	go func(){
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
func getCoreTagFunc(name []byte) (func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool)[]byte, bool, error) {
	if name[0] == '_' {
		name = name[1:]
	}
	nameStr := string(regex.Compile(`[^\w_]`).RepStrRef(&name, []byte{}))
	
	isSync := false

	found := true
	m := reflect.ValueOf(&TagFuncs).MethodByName(nameStr)
	if goutil.IsZeroOfUnderlyingType(m) {
		m = reflect.ValueOf(&TagFuncs).MethodByName(nameStr+"_SYNC")
		if goutil.IsZeroOfUnderlyingType(m) {
			found = false
		}else{
			isSync = true
		}
	}

	if !found {
		return nil, false, errors.New("method '"+nameStr+"' does not exist in Compiled Functions")
	}

	if fn, ok := m.Interface().(func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool)[]byte); ok {
		return fn, isSync, nil
	}

	return nil, false, errors.New("method '"+nameStr+"' does not return the expected args")
}


// removeLineBreak removes one extra line break from the compiler
func removeLineBreak[T interface{uint8|uint16}](reader *liveread.Reader[T]) bool {
	b, e := reader.Peek(2)
	if e == nil {
		if b[0] == '\r' && b[1] == '\n' {
			reader.Discard(2)
			return true
		}else if b[0] == '\n' {
			reader.Discard(1)
			return true
		}
	}
	return false
}


// sortStrings will sort a list of strings
//
// this method will also split numbers and return `10 > 2`, rather than seeing `[1,0] < [2,_]`
func sortStrings[T any](list *[]T){
	sort.Slice(*list, func(i, j int) bool {
		l1 := regex.Comp(`([0-9]+)`).Split(goutil.Conv.ToBytes((*list)[i]))
		l2 := regex.Comp(`([0-9]+)`).Split(goutil.Conv.ToBytes((*list)[j]))

		for i := len(l1)-1; i >= 0; i-- {
			if len(l1[i]) == 0 {
				l1 = append(l1[:i], l1[i+1:]...)
			}
		}

		for i := len(l2)-1; i >= 0; i-- {
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
				}else if i1 > i2 {
					return false
				}
			}else if n1 {
				return true
			}else if n2 {
				return false
			}else{
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
					}else if l1[i][j] > l2[i][j] {
						return false
					}
				}

				if small == 1 {
					return true
				}else if small == 2 {
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
