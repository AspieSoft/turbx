package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/GRbit/go-pcre"
)

type stringObj struct {
	s []byte
	q byte
}

type scriptObj struct {
	tag  byte
	args []byte
	cont []byte
}

type fileData struct {
	html   []byte
	args   []map[string][]byte
	str    [][]byte
	script []scriptObj
}

var varType map[string]reflect.Type

var regCache map[string]pcre.Regexp

var singleTagList map[string]bool

var OPTS map[string]string


func main() {

	initVarTypes()

	regCache = map[string]pcre.Regexp{}

	singleTagList = map[string]bool{
		"br": true,
		"hr": true,
	}

	OPTS = map[string]string{}

	userInput := make(chan string)
	go readInput(userInput)

	for true {
		input := <-userInput

		//todo: make seperate method for pre compile, and store pre compiled file in system
		// also include some files or components in memory depending on the amount of space
		// may pre compile components into files, for cache file

		if input == "stop" || input == "exit" {
			break
		} else if strings.HasPrefix(input, "set:") && strings.ContainsRune(input, '=') {
			opt := strings.SplitN(strings.SplitN(input, ":", 2)[1], "=", 2)
			OPTS[opt[0]] = opt[1]
		} else if strings.HasPrefix(input, "pre:") {
			pre := strings.SplitN(input, ":", 2)[1]
			go runPreCompile(pre)
		} else if strings.ContainsRune(input, ':') {
			go runCompile(input)
		}

	}
}

func debug(msg ...interface{}) {
	fmt.Println("debug:", msg)
}

func joinPath(path ...string) (string, error) {
	resPath, err := filepath.Abs(path[0])
	if err != nil {
		return "", err
	}
	for i := 1; i < len(path); i++ {
		p := filepath.Join(resPath, path[i])
		if p == resPath || !strings.HasPrefix(p, resPath) {
			return "", errors.New("path leaked outside of root")
		}
		resPath = p
	}
	return resPath, nil
}

func contains(search []string, value string) bool {
	for _, v := range search {
		if v == value {
			return true
		}
	}
	return false
}

func initVarTypes(){
	varType = map[string]reflect.Type{}

	varType["array"] = reflect.TypeOf([]interface{}{})
	varType["map"] = reflect.TypeOf(map[string]interface{}{})

	varType["int"] = reflect.TypeOf(int(0))
	varType["float64"] = reflect.TypeOf(float64(0))
	varType["float32"] = reflect.TypeOf(float32(0))

	varType["string"] = reflect.TypeOf("")
	varType["byteArray"] = reflect.TypeOf([]byte(""))
	varType["byte"] = reflect.TypeOf(' ')
}

func readInput(input chan<- string) {
	for {
		var u string
		_, err := fmt.Scanf("%s\n", &u)
		if err == nil {
			input <- u
		}
	}
}

func getOpt(opts map[string]interface{}, arg string, stringOutput bool) interface{} {
	var res interface{}
	res = nil

	argOpts := strings.Split(arg, "|")
	for _, arg := range argOpts {
		res = opts
		args := regSplit(regRepStr([]byte(arg), `\s+`, []byte("")), `\.|(\[.*?\])`)
		for _, a := range args {
			if regMatch(a, `^%![0-9]+!%$`) {
				return string(a)
			}

			if bytes.HasPrefix(a, []byte("[")) && bytes.HasSuffix(a, []byte("]")) {
				a = a[1:len(a)-2]
				if reflect.TypeOf(res) != varType["array"] || !regMatch(a, `^[0-9]+$`) {
					a = []byte(getOpt(opts, string(a), true).(string))
				}
			}

			if reflect.TypeOf(res) == varType["array"] && regMatch(a, `^[0-9]+$`) {
				i, err := strconv.Atoi(string(a))
				if err == nil && reflect.TypeOf(res) == varType["array"] && len(res.([]interface{})) > i {
					res = res.([]interface{})[i]
				}
			} else if reflect.TypeOf(res) == varType["map"] {
				res = res.(map[string]interface{})[string(a)]
			} else {
				res = nil
				break
			}

			if t := reflect.TypeOf(res); t != varType["map"] && t != varType["array"] {
				break
			}
		}

		
		if res != nil && res != false {
			if stringOutput {
				if t := reflect.TypeOf(res); t != varType["map"] && t != varType["array"] {
					break
				}
			}else{
				break
			}
		}
	}

	if stringOutput {
		switch reflect.TypeOf(res) {
			case varType["string"]:
				return string(res.(string))
			case varType["byteArray"]:
				return string(res.([]byte))
			case varType["byte"]:
				return string(res.(byte))
			case varType["int"]:
				return strconv.Itoa(res.(int))
			case varType["float64"]:
				return strconv.FormatFloat(res.(float64), 'f', -1, 64)
			case varType["float32"]:
				return strconv.FormatFloat(float64(res.(float32)), 'f', -1, 32)
			default:
				return ""
		}
	}

	return res
}

func runPreCompile(input string) {
	inputData := strings.SplitN(input, ":", 2)

	_, err := getFile(inputData[1], false, nil)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	fmt.Println(inputData[0] + ":success")
}

func runCompile(input string) {
	inputData := strings.SplitN(input, ":", 3)

	optStr := decompress(inputData[1])

	opts := map[string]interface{}{}
	err := json.Unmarshal([]byte(optStr), &opts)
	if err != nil {
		opts = map[string]interface{}{}
	}

	file, err := getFile(inputData[2], false, nil)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	//todo: compile file and return html

	// fmt.Println(file)

	compile(file, opts)

	//temp
	// fmt.Println(inputData[0] + ":error")
	return

	//todo: return output to js
	//todo (later): make go handle final compile, and just output html

	/* args := []map[string]string{}

	for i, v := range file.args {
		args = append(args, map[string]string{})
		for key, val := range v {
			args[i][key] = string(val)
		}
	}

	str := []string{}
	for _, v := range file.str {
		str = append(str, string(v))
	}

	script := []map[string]string{}
	for i, v := range file.script {
		script = append(script, map[string]string{})
		script[i]["tag"] = string(v.tag)
		script[i]["args"] = string(v.args)
		script[i]["cont"] = string(v.cont)
	}

	res := map[string]interface{}{
		"html":    string(file.html),
		"args":    args,
		"strings": str,
		"scripts": script,
	}

	json, err := json.Marshal(res)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	} */

	/* out, err := compress(string(json))
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	} */

	/* json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	fmt.Println(inputData[0] + ":" + string(json)) */
}

func getFile(filePath string, component bool, parents []string) (fileData, error) {

	// init options
	root := OPTS["root"]
	if root == "" {
		return fileData{}, errors.New("root not found")
	}

	ext := "xhtml"
	if OPTS["ext"] != "" {
		ext = "xhtml"
	}

	compRoot := "components"
	if OPTS["components"] != "" {
		compRoot = OPTS["components"]
	}

	var html []byte = nil
	var path string
	var err error

	// try files

	// component current parent
	if component && parents != nil {
		par := filepath.Join(parents[len(parents)-1], "..")
		if strings.HasSuffix(parents[len(parents)-1], "/" + compRoot) {
			path, err = joinPath(par, filePath + "." + ext)
		}else{
			path, err = joinPath(par, compRoot, filePath + "." + ext)
		}
		if err == nil {
			if contains(parents, path) {
				return fileData{}, errors.New("infinite loop detected")
			}

			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	// component root parent
	if component && parents != nil {
		par := filepath.Join(parents[0], "..")
		if strings.HasPrefix(parents[len(parents)-1], par) {
			path, err = joinPath(par, compRoot, filePath + "." + ext)
			if err == nil {
				if contains(parents, path) {
					return fileData{}, errors.New("infinite loop detected")
				}
	
				html, err = ioutil.ReadFile(path)
				if err != nil {
					html = nil
				}
			}
		}
	}

	// component root
	if html == nil && component {
		path, err = joinPath(root, compRoot, filePath + "." + ext)
		if err == nil {
			if parents != nil && contains(parents, path) {
				return fileData{}, errors.New("infinite loop detected")
			}

			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	// current parent
	if html == nil && parents != nil {
		par := filepath.Join(parents[len(parents)-1], "..")
		path, err = joinPath(par, filePath + "." + ext)
		if err == nil {
			if contains(parents, path) {
				return fileData{}, errors.New("infinite loop detected")
			}

			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	// root parent
	if html == nil && parents != nil {
		par := filepath.Join(parents[0], "..")
		if strings.HasPrefix(parents[len(parents)-1], par) {
			path, err = joinPath(par, filePath + "." + ext)
			if err == nil {
				if contains(parents, path) {
					return fileData{}, errors.New("infinite loop detected")
				}
	
				html, err = ioutil.ReadFile(path)
				if err != nil {
					html = nil
				}
			}
		}
	}

	// root file
	if html == nil {
		path, err = joinPath(root, filePath + "." + ext)
		if err == nil {
			if parents != nil && contains(parents, path) {
				return fileData{}, errors.New("infinite loop detected")
			}

			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	if html == nil {
		return fileData{}, err
	}

	// add parent
	if parents != nil {
		parents = append(parents, path)
	}else{
		parents = []string{path}
	}

	// pre compile
	file, err := preCompile(html, parents)
	if err != nil {
		return fileData{}, err
	}

	//todo: cache file and listen for changes

	return file, nil
}

func preCompile(html []byte, parents []string) (fileData, error) {
	html = append([]byte("\n"), html...)
	html = append(html, []byte("\n")...)

	html = encodeEncoding(html)

	objStrings := []stringObj{}
	stringList := [][]byte{}

	// extract strings and comments
	html = regRepFunc(html, `(?s)(<!--.*?-->|/\*.*?\*/|\r?\n//.*?\r?\n)|(["'`+"`"+`])((?:\\[\\"'`+"`"+`]|.)*?)\2`, func(data func(int) []byte) []byte {
		if len(data(1)) != 0 {
			return []byte("")
		}
		objStrings = append(objStrings, stringObj{s: decodeEncoding(data(3)), q: data(2)[0]})
		return []byte("%!s" + strconv.Itoa(len(objStrings)-1) + "!%")
	})

	decodeStrings := func(html []byte, mode int) []byte {
		return decodeEncoding(regRepFunc(html, `%!s([0-9]+)!%`, func(data func(int) []byte) []byte {
			i, err := strconv.Atoi(string(data(1)))
			if err != nil || len(objStrings) <= i {
				return []byte("")
			}
			str := objStrings[i]

			if mode == 1 && regMatch(str.s, `^-?[0-9]+(\.[0-9]+|)$`) {
				return str.s
			} else if mode == 2 {
				return str.s
			} else if mode == 3 {
				if regMatch(str.s, `^-?[0-9]+(\.[0-9]+|)$`) {
					return str.s
				} else {
					stringList = append(stringList, str.s)
					return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
				}
			} else if mode == 4 {
				stringList = append(stringList, str.s)
				return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
			}

			return append(append([]byte{str.q}, str.s...), str.q)
		}))
	}

	// extract scripts
	objScripts := []scriptObj{}
	html = regRepFunc(html, `(?s)<(script|js|style|css|less|markdown|md|text|txt|raw)(\s+.*?|)>(.*?)</\1>`, func(data func(int) []byte) []byte {
		cont := decodeStrings(data(3), 0)

		var tag byte
		if regMatch(data(1), `^(markdown|md)$`) {
			tag = 'm'
			cont = compileMD(cont)
		} else if regMatch(data(1), `^(text|txt|raw)$`) {
			tag = 't'
			cont = escapeHTML(cont)
		} else if bytes.Equal(data(1), []byte("raw")) {
			tag = 'r'
		} else if regMatch(data(1), `^(script|js)$`) {
			tag = 'j'
			cont = compileJS(cont)
		} else if regMatch(data(1), `^(style|css|less)$`) {
			tag = 'c'
			cont = compileCSS(cont)
		}

		args := decodeStrings(data(2), 0)

		objScripts = append(objScripts, scriptObj{tag, args, cont})
		i := strconv.Itoa(len(objScripts) - 1)

		return []byte("<!_script " + i + "/>")
	})

	// move html args to list
	argList := []map[string][]byte{}
	tagIndex := 0
	html = regRepFunc(html, `(?s)<(/|)([\w_\-\.$!:]+)(\s+.*?|)\s*(/|)>`, func(data func(int) []byte) []byte {
		argStr := regRepStr(regRepStr(data(3), `^\s+`, []byte("")), `\s+`, []byte(" "))

		newArgs := map[string][]byte{}

		ind := 0
		vInd := -1

		if len(argStr) != 0 {
			if regMatch(data(2), `^_(el(if|se)|if)$`) {
				argStr = regRepFunc(argStr, `\s*([!<>=]|)\s*(=)|(&)\s*(&)|(\|)\s*(\|)|([<>&|])\s*`, func(data func(int) []byte) []byte {
					return append(append([]byte(" "), data(0)...), ' ')
				})
				argStr = regRepStr(argStr, `\s+`, []byte(" "))
			} else {
				argStr = regRepStr(argStr, `\s*=\s*`, []byte("="))
			}

			args := bytes.Split(argStr, []byte(" "))

			if regMatch(data(2), `^_(el(if|se)|if)$`) {
				for _, v := range args {
					newArgs[strconv.Itoa(ind)] = decodeStrings(v, 3)
					ind++
				}
			} else {
				for _, v := range args {
					if regMatch(v, `^\{\{\{?.*?\}\}\}?$`) {
						if bytes.Contains(v, []byte("=")) {
							esc := true
							v = regRepFunc(v, `(\{\{\{?)(.*?)(\}\}\}?)`, func(data func(int) []byte) []byte {
								if bytes.Equal(data(1), []byte("{{{")) || bytes.Equal(data(3), []byte("}}}")) {
									esc = false
								}
								return data(2)
							})
							val := bytes.Split(v, []byte("="))

							if len(val[0]) == 0 {
								key := decodeStrings(val[1], 2)
								key = bytes.Split(key, []byte("|"))[0]
								keyObj := bytes.Split(key, []byte("."))
								key = keyObj[len(keyObj)-1]

								newVal := append(append(key, []byte(`=`)...), decodeStrings(val[1], 1)...)
								newVal = regRepFunc(newVal, `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
									stringList = append(stringList, data(2))
									return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
								})

								if esc {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{"), newVal...), []byte("}}")...)
									vInd--
								} else {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{{"), newVal...), []byte("}}}")...)
									vInd--
								}
							} else {
								decompVal := regRepFunc(decodeStrings(val[1], 1), `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
									stringList = append(stringList, data(2))
									return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
								})
								newVal := append(append(decodeStrings(val[0], 2), []byte(`=`)...), decompVal...)

								if esc {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{"), newVal...), []byte("}}")...)
									vInd--
								} else {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{{"), newVal...), []byte("}}}")...)
									vInd--
								}
							}
						} else {
							decompVal := regRepFunc(decodeStrings(v, 1), `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
								stringList = append(stringList, data(2))
								return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
							})

							newArgs[strconv.Itoa(ind)] = decompVal
							ind++
						}
					} else if bytes.Contains(v, []byte("=")) {
						val := bytes.Split(v, []byte("="))

						decompVal := decodeStrings(val[1], 1)

						if regMatch(decompVal, `^\{\{\{?.*?\}\}\}?$`) {
							decompVal = regRepFunc(decodeStrings(val[1], 1), `(?s)(['"`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
								stringList = append(stringList, data(2))
								return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
							})
						}

						newArgs[string(decodeStrings(val[0], 2))] = decompVal
					} else {
						newArgs[strconv.Itoa(ind)] = decodeStrings(v, 3)
						ind++
					}
				}
			}
		}


		// handle non-function and non-component args for putting back
		var newArgsBasic []byte = nil
		if !regMatch(data(2), `^[A-Z_]`) {
			if len(newArgs) > 0 {
				args1, args2, args3 := []byte{}, []byte{}, []byte{}
				for key, val := range newArgs {
					if i, err := strconv.Atoi(key); err == nil {
						if i < 0 {
							args2 = append(args2, append([]byte(" "), val...)...)
						} else {
							args3 = append(args3, append([]byte(" "), val...)...)
						}
					} else {
						args1 = append(args1, append([]byte(" "), append(append([]byte(key), '='), val...)...)...)
					}
				}
				newArgsBasic = append(append(args1, args2...), args3...)
			}else{
				newArgsBasic = []byte{}
			}
		}


		if len(data(4)) != 0 || singleTagList[string(data(2))] {
			// self closing tag
			if newArgsBasic != nil {
				return append(append([]byte("<"), data(2)...), append(newArgsBasic, []byte("/>")...)...)
			}

			if len(newArgs) > 0 {
				argList = append(argList, newArgs)
				return append(append([]byte("<"), data(2)...), []byte(":"+strconv.Itoa(tagIndex)+" "+strconv.Itoa(len(argList)-1)+"/>")...)
			}
			return append(append([]byte("<"), data(2)...), []byte(":"+strconv.Itoa(tagIndex)+"/>")...)
		} else if len(data(1)) != 0 {
			// closing tag
			if newArgsBasic != nil {
				return append(append([]byte("<"), data(2)...), append(newArgsBasic, []byte("/>")...)...)
			}

			if tagIndex > 0 {
				tagIndex--
			}
			return append(append([]byte("</"), data(2)...), []byte(":"+strconv.Itoa(tagIndex)+">")...)
		} else {
			// opening tag
			if newArgsBasic != nil {
				return append(append([]byte("<"), data(2)...), append(newArgsBasic, []byte(">")...)...)
			}

			tagIndex++

			if len(newArgs) > 0 {
				argList = append(argList, newArgs)
				return append(append([]byte("<"), data(2)...), []byte(":"+strconv.Itoa(tagIndex-1)+" "+strconv.Itoa(len(argList)-1)+">")...)
			}
			return append(append([]byte("<"), data(2)...), []byte(":"+strconv.Itoa(tagIndex-1)+">")...)
		}
	})

	// move var strings to seperate list
	html = regRepFunc(html, `(?s)(\{\{\{?)(.*?)(\}\}\}?)`, func(data func(int) []byte) []byte {
		esc := true
		if bytes.Equal(data(1), []byte("{{{")) || bytes.Equal(data(3), []byte("}}}")) {
			esc = false
		}

		val := regRepFunc(data(2), `%!s[0-9]+!%`, func(data func(int) []byte) []byte {
			stringList = append(stringList, decodeStrings(data(0), 2))
			return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
		})

		if esc {
			return append(append([]byte("{{"), val...), []byte("}}")...)
		}
		return append(append([]byte("{{{"), val...), []byte("}}}")...)
	})

	html = decodeStrings(html, 0)
	html = decodeEncoding(html)

	// preload components
	if OPTS["cache_component"] == "embed" {
		html = regRepFunc(html, `(?s)<([A-Z][\w_\-\.$!:]+):([0-9]+)(\s+[0-9]+|)>(.*?)</\1:\2>`, func(data func(int) []byte) []byte {
			name := strings.ToLower(string(data(1)))
			fileData, err := getFile(name, true, parents)
			if err != nil {
				return data(0)
			}

			var args map[string][]byte = nil
			if len(data(3)) > 0 {
				i, err := strconv.Atoi(string(bytes.Trim(data(3), " ")))
				if err != nil {
					return data(0)
				}
				args = argList[i]
			}

			//todo: fill component vars with custom args
			_ = args

			comp := regRepFunc(fileData.html, `%!([0-9]+)!%`, func(data func(int) []byte) []byte {
				i, err := strconv.Atoi(string(data(1)))
				if err != nil {
					return []byte("")
				}

				stringList = append(stringList, fileData.str[i])

				return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
			})

			comp = regRepFunc(comp, `(?s)<(!_script|[A-Z_][\w_\-\.$!:]+:[0-9]+)\s+([0-9]+)(/?)>`, func(data func(int) []byte) []byte {
				i, err := strconv.Atoi(string(data(2)))
				if err != nil {
					return []byte("")
				}

				if bytes.Equal(data(1), []byte("!_script")) {
					objScripts = append(objScripts, fileData.script[i])
					return append(append([]byte("<"), data(1)...), append([]byte(" " + strconv.Itoa(len(objScripts)-1)), append(data(3), []byte(">")...)...)...)
				}

				argList = append(argList, fileData.args[i])
				return append(append([]byte("<"), data(1)...), append([]byte(" " + strconv.Itoa(len(argList)-1)), append(data(3), []byte(">")...)...)...)
			})

			comp = regRepFunc(comp, `(?si)({{{?)\s*body\s*(}}}?)`, func(d func(int) []byte) []byte {
				if bytes.Equal(d(1), []byte("{{")) || bytes.Equal(d(2), []byte("}}")) {
					return escapeHTML(data(4))
				}
				return data(4)
			})

			return comp
		})

		html = regRepFunc(html, `(?s)<([A-Z][\w_\-\.$!:]+):([0-9]+)(\s+[0-9]+|)/>`, func(data func(int) []byte) []byte {
			name := strings.ToLower(string(data(1)))
			fileData, err := getFile(name, true, parents)
			if err != nil {
				return data(0)
			}

			var args map[string][]byte = nil
			if len(data(3)) > 0 {
				i, err := strconv.Atoi(string(bytes.Trim(data(3), " ")))
				if err != nil {
					return data(0)
				}
				args = argList[i]
			}

			//todo: fill component vars with custom args (copy from above replace function)
			_ = args

			comp := regRepFunc(fileData.html, `%!([0-9]+)!%`, func(data func(int) []byte) []byte {
				i, err := strconv.Atoi(string(data(1)))
				if err != nil {
					return []byte("")
				}

				stringList = append(stringList, fileData.str[i])

				return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
			})

			comp = regRepFunc(comp, `(?s)<(!_script|[A-Z_][\w_\-\.$!:]+:[0-9]+)\s+([0-9]+)(/?)>`, func(data func(int) []byte) []byte {
				i, err := strconv.Atoi(string(data(2)))
				if err != nil {
					return []byte("")
				}

				if bytes.Equal(data(1), []byte("!_script")) {
					objScripts = append(objScripts, fileData.script[i])
					return append(append([]byte("<"), data(1)...), append([]byte(" " + strconv.Itoa(len(objScripts)-1)), append(data(3), []byte(">")...)...)...)
				}

				argList = append(argList, fileData.args[i])
				return append(append([]byte("<"), data(1)...), append([]byte(" " + strconv.Itoa(len(argList)-1)), append(data(3), []byte(">")...)...)...)
			})

			return comp
		})
	}else{
		go regRepFunc(html, `(?s)<([A-Z][\w_\-\.$!:]+):[0-9]+(\s+[0-9]+|)/?>`, func(data func(int) []byte) []byte {
			name := strings.ToLower(string(data(1)))
			getFile(name, true, parents)
			return []byte("")
		}, true)
	}

	return fileData{html: html, args: argList, str: stringList, script: objScripts}, nil
}

func compile(file fileData, opts map[string]interface{}) []byte {

	// fmt.Println(string(file.html))

	return []byte("")
}

func escapeHTML(html []byte) []byte {
	html = regRepFunc(html, `[<>&]`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("<")) {
			return []byte("&lt;")
		} else if bytes.Equal(data(0), []byte(">")) {
			return []byte("&gt;")
		}
		return []byte("&amp;")
	})
	return regRepStr(html, `&amp;(amp;)*`, []byte("&amp;"))
}

func escapeHTMLArgs(html []byte) []byte {
	return regRepFunc(html, `[\\"'`+"`"+`]`, func(data func(int) []byte) []byte {
		return append([]byte("\\"), data(0)...)
	})
}

func compileJS(script []byte) []byte {
	return script
}

func compileCSS(style []byte) []byte {
	return style
}

func compileMD(md []byte) []byte {
	return md
}

func encodeEncoding(html []byte) []byte {
	return regRepFunc(html, `%!|!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("%!")) {
			return []byte("%!o!%")
		}
		return []byte("%!c!%")
	})
}

func decodeEncoding(html []byte) []byte {
	return regRepFunc(html, `%!([oc])!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(1), []byte("o")) {
			return []byte("%!")
		}
		return []byte("!%")
	})
}

func compress(msg string) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(msg)); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func decompress(str string) string {
	data, _ := base64.StdEncoding.DecodeString(str)
	rdata := bytes.NewReader(data)
	r, _ := gzip.NewReader(rdata)
	s, _ := ioutil.ReadAll(r)
	return string(s)
}

func regRepFunc(str []byte, re string, rep func(func(int) []byte) []byte, blank ...bool) []byte {
	var reg pcre.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		// reg = pcre.MustCompileJIT(re, pcre.UTF8, pcre.CONFIG_JIT)
		reg = pcre.MustCompile(re, pcre.UTF8)
		regCache[re] = reg
	}

	// ind := reg.FindAllIndex(str, pcre.UTF8)
	ind := reg.FindAllIndex(str, 0)

	res := []byte{}
	trim := 0
	for _, pos := range ind {
		v := str[pos[0]:pos[1]]
		m := reg.Matcher(v, 0)

		if len(blank) != 0 {
			r := rep(func(g int) []byte {
				return m.Group(g)
			})

			if r == nil {
				return nil
			}
		} else {
			if trim == 0 {
				res = append(res, str[:pos[0]]...)
			} else {
				res = append(res, str[trim:pos[0]]...)
			}
			trim = pos[1]

			r := rep(func(g int) []byte {
				return m.Group(g)
			})

			if r == nil {
				res = append(res, str[trim:]...)
				return res
			}

			res = append(res, r...)
		}
	}

	if len(blank) != 0 {
		return nil
	}

	res = append(res, str[trim:]...)

	return res
}

func regRepStr(str []byte, re string, rep []byte) []byte {
	var reg pcre.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = pcre.MustCompileJIT(re, pcre.UTF8, pcre.CONFIG_JIT)
		regCache[re] = reg
	}

	// return reg.ReplaceAll(str, rep, pcre.UTF8)
	return reg.ReplaceAll(str, rep, 0)
}

func regMatch(str []byte, re string) bool {
	var reg pcre.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = pcre.MustCompileJIT(re, pcre.UTF8, pcre.CONFIG_JIT)
		regCache[re] = reg
	}

	// return reg.Match(str, pcre.UTF8)
	return reg.Match(str, 0)
}

func regSplit(str []byte, re string) [][]byte {
	var reg pcre.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = pcre.MustCompileJIT(re, pcre.UTF8, pcre.CONFIG_JIT)
		regCache[re] = reg
	}

	ind := reg.FindAllIndex(str, 0)

	res := [][]byte{}
	trim := 0
	for _, pos := range ind {
		v := str[pos[0]:pos[1]]
		m := reg.Matcher(v, 0)
		_ = m

		if trim == 0 {
			res = append(res, str[:pos[0]])
		} else {
			res = append(res, str[trim:pos[0]])
		}
		trim = pos[1]

		for i := 1; i <= m.Groups; i++ {
			g := m.Group(i)
			if len(g) != 0 {
				res = append(res, m.Group(i))
			}
		}
	}

	e := str[trim:]
	if len(e) != 0 {
		res = append(res, str[trim:])
	}

	return res
}
