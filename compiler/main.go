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
	"time"

	"github.com/AspieSoft/go-regex"
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

var regHtmlTag string = `(?:[\w_\-.$!:][\\/][\w_\-.$!:]|[\w_\-.$!:])`

var singleTagList map[string]bool = map[string]bool{
	"br": true,
	"hr": true,
	"wbr": true,
	"meta": true,
	"link": true,
	"param": true,
	"base": true,
	"input": true,
	"img": true,
	"area": true,
	"col": true,
	"command": true,
	"embed": true,
	"keygen": true,
	"source": true,
	"track": true,
}


//todo: add functions to go
var tagFuncs map[string]interface{} = map[string]interface{} {
	"if": tagFuncIf,

	"each": func(args map[string][]byte, cont []byte, opts map[string]interface{}, level int, file fileData) interface{} {
		return nil
	},
}

func tagFuncIf(args map[string][]byte, cont []byte, opts map[string]interface{}, level int, file fileData) interface{} {
	isTrue := false
	lastArg := []byte{}

	if len(args) == 0 {
		return cont
	}

	for i := 0; i < len(args); i++ {
		arg := args[strconv.Itoa(i)]
		if bytes.Equal(arg, []byte("&")) {
			if isTrue {
				continue
			}
			break
		} else if bytes.Equal(arg, []byte("|")) {
			if isTrue {
				break
			}
			continue
		}

		arg1, sign, arg2 := []byte{}, "", []byte{}
		var arg1Any interface{}
		var arg2Any interface{}
		a1, ok1 := args[strconv.Itoa(i+1)]
		if ok1 && regex.Match(a1, `^[!<>]?=|[<>]$`) {
			arg1 = arg
			sign = string(a1)
			arg2 = args[strconv.Itoa(i+2)]
			lastArg = arg1
		} else if ok1 && regex.Match(arg, `^[!<>]?=|[<>]$`) {
			arg1 = lastArg
			sign = string(arg)
			arg2 = a1
			i++
		} else if len(args) == 1 {
			pos := true
			if bytes.HasPrefix(arg, []byte("!")) {
				pos = false
				arg1 = arg[1:]
			}

			if bytes.Equal(arg, []byte("")) {
				arg1 = a1
				i++
			} else if bytes.Equal(arg, []byte("!")) {
				arg1 = a1
				i++
				pos = true
			}

			if bytes.HasPrefix(arg1, []byte("!")) {
				pos = !pos
				arg1 = arg1[1:]
			}

			lastArg = arg1

			if regex.Match(arg1, `^["'\'](.*)["'\']$`) {
				arg1 = regex.RepFunc(arg1, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
					return data(1)
				})
				if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
					arg1Any = arg1N
				} else {
					arg1Any = arg1
				}
			} else if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
				arg1Any = arg1N
			} else {
				arg1Any = getOpt(opts, string(arg1), false)
			}

			isTrue = IsZeroOfUnderlyingType(arg1Any)
			if pos {
				isTrue = !isTrue
			}
			
			continue
		} else {
			continue
		}

		if regex.Match(arg1, `^["'\'](.*)["'\']$`) {
			arg1 = regex.RepFunc(arg1, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
				return data(1)
			})
			if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
				arg1Any = arg1N
			} else {
				arg1Any = arg1
			}
		} else if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
			arg1Any = arg1N
		} else {
			arg1Any = getOpt(opts, string(arg1), false)
			if reflect.TypeOf(arg1Any) == varType["string"] {
				if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
					arg1Any = arg1N
				}
			}
		}

		if len(arg2) == 0 {
			arg2Any = nil
		} else if regex.Match(arg2, `^["'\'](.*)["'\']$`) {
			arg2 = regex.RepFunc(arg2, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
				return data(1)
			})
			if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
				arg2Any = arg2N
			} else {
				arg2Any = arg2
			}
		} else if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
			arg2Any = arg2N
		} else {
			arg2Any = getOpt(opts, string(arg2), false)
			if reflect.TypeOf(arg2Any) == varType["string"] {
				if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
					arg2Any = arg2N
				}
			}
		}

		lastArg = arg1

		arg1Type := reflect.TypeOf(arg1Any)
		if arg1Type == varType["int"] {
			arg1Any = float64(arg1Any.(int))
		}else if arg1Type == varType["float32"] {
			arg1Any = float64(arg1Any.(float32))
		}else if arg1Type == varType["int32"] {
			arg1Any = float64(arg1Any.(int32))
		}else if arg1Type == varType["byteArray"] {
			arg1Any = string(arg1Any.([]byte))
		}else if arg1Type == varType["byte"] {
			arg1Any = string(arg1Any.(byte))
		}

		arg2Type := reflect.TypeOf(arg2Any)
		if arg2Type == varType["int"] {
			arg2Any = float64(arg2Any.(int))
		}else if arg2Type == varType["float32"] {
			arg2Any = float64(arg2Any.(float32))
		}else if arg2Type == varType["int32"] {
			arg2Any = float64(arg2Any.(int32))
		}else if arg1Type == varType["byteArray"] {
			arg2Any = string(arg2Any.([]byte))
		}else if arg1Type == varType["byte"] {
			arg2Any = string(arg2Any.(byte))
		}

		switch sign {
		case "=":
			isTrue = (arg1Any == arg2Any)
			break
		case "!=":
		case "!":
			isTrue = (arg1Any != arg2Any)
			break
		case ">=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == varType["float64"] {
				isTrue = (arg1Any.(float64) >= arg2Any.(float64))
			}
			break
		case "<=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == varType["float64"] {
				isTrue = (arg1Any.(float64) <= arg2Any.(float64))
			}
			break
		case ">":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == varType["float64"] {
				isTrue = (arg1Any.(float64) > arg2Any.(float64))
			}
			break
		case "<":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == varType["float64"] {
				isTrue = (arg1Any.(float64) < arg2Any.(float64))
			}
			break
		default:
			break
		}

		i += 2
	}

	elseOpt := regex.Match(cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`)
	if elseOpt && isTrue {
		return regex.RepStr(cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`, []byte(""))
	} else if elseOpt {
		blankElse := false
		newArgs, newCont := map[string][]byte{}, []byte{}
		regex.RepFunc(cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`, func(data func(int) []byte) []byte {
			argInt, err := strconv.Atoi(string(regex.RepStr(data(2), `\s`, []byte{})))
			if err != nil {
				blankElse = true
			}else{
				newArgs = file.args[argInt]
			}
			newCont = data(3)
			return nil
		}, true)

		if blankElse {
			return newCont
		}

		// https://stackoverflow.com/questions/61830637/how-to-self-reference-a-function
		// return runTagFunc("if", newArgs, newCont, opts, level, file)
		return tagFuncIf(newArgs, newCont, opts, level, file)
	} else if isTrue {
		return cont
	}

	return []byte{}
}


var OPTS map[string]string = map[string]string{}

func main() {

	initVarTypes()

	userInput := make(chan string)
	go readInput(userInput)

	for {
		input := <-userInput

		if input == "stop" || input == "exit" {
			break
		} else if strings.HasPrefix(input, "set:") && strings.ContainsRune(input, '=') {
			opt := strings.SplitN(strings.SplitN(input, ":", 2)[1], "=", 2)
			setOPT(opt[0], opt[1])
		} else if strings.HasPrefix(input, "pre:") {
			pre := strings.SplitN(input, ":", 2)[1]
			go runPreCompile(pre)
		} else if strings.ContainsRune(input, ':') {
			go runCompile(input)
		}
	}
}

var writingOpts int = 0
var readingOpts int = 0

func setOPT(key string, val string){
	writingOpts++
	for readingOpts != 0 {
		time.Sleep(1000)
	}
	OPTS[key] = val
	writingOpts--
}

func getOPT(key string) string {
	for writingOpts != 0 {
		time.Sleep(1000)
	}
	readingOpts++
	opt := OPTS[key]
	readingOpts--
	return opt
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

func containsMap(search map[string][]byte, value []byte) bool {
	for _, v := range search {
		if bytes.Equal(v, value) {
			return true
		}
	}
	return false
}

func toString(res interface{}) string {
	switch reflect.TypeOf(res) {
		case varType["string"]:
			return res.(string)
		case varType["byteArray"]:
			return string(res.([]byte))
		case varType["byte"]:
			return string(res.(byte))
		case varType["int32"]:
			return string(res.(int32))
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

func IsZeroOfUnderlyingType(x interface{}) bool {
	// return x == nil || x == reflect.Zero(reflect.TypeOf(x)).Interface()
	return x == nil || reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func initVarTypes() {
	varType = map[string]reflect.Type{}

	varType["array"] = reflect.TypeOf([]interface{}{})
	varType["arrayByte"] = reflect.TypeOf([][]byte{})
	varType["map"] = reflect.TypeOf(map[string]interface{}{})

	varType["int"] = reflect.TypeOf(int(0))
	varType["float64"] = reflect.TypeOf(float64(0))
	varType["float32"] = reflect.TypeOf(float32(0))

	varType["string"] = reflect.TypeOf("")
	varType["byteArray"] = reflect.TypeOf([]byte{})
	varType["byte"] = reflect.TypeOf([]byte{0}[0])

	// int 32 returned instead of byte
	varType["int32"] = reflect.TypeOf(' ')

	varType["func"] = reflect.TypeOf(func(){})
	varType["tagFunc"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{} {return nil})
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
		args := regex.Split(regex.RepStr([]byte(arg), `\s+`, []byte{}), `\.|(\[.*?\])`)
		for _, a := range args {
			if regex.Match(a, `^%![0-9]+!%$`) {
				return string(a)
			}

			if bytes.HasPrefix(a, []byte("[")) && bytes.HasSuffix(a, []byte("]")) {
				a = a[1 : len(a)-2]
				if reflect.TypeOf(res) != varType["array"] || !regex.Match(a, `^[0-9]+$`) {
					a = []byte(getOpt(opts, string(a), true).(string))
				}
			}

			if reflect.TypeOf(res) == varType["array"] && regex.Match(a, `^[0-9]+$`) {
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
			} else {
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
		case varType["int32"]:
			return string(res.(int32))
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

	_, err := getFile(inputData[1], false, true)
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

	file, err := getFile(inputData[2], false, true)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	out := compile(file, opts, true, true)

	resOut, err := compress(string(out))
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	fmt.Println(inputData[0] + ":" + resOut)
}

func getFile(filePath string, component bool, allowImport bool) (fileData, error) {

	// init options
	root := getOPT("root")
	if root == "" {
		return fileData{}, errors.New("root not found")
	}

	ext := "xhtml"
	if getOPT("ext") != "" {
		ext = "xhtml"
	}

	compRoot := "components"
	if getOPT("components") != "" {
		compRoot = getOPT("components")
	}

	var html []byte = nil
	var path string
	var err error

	// try files
	if component {
		path, err = joinPath(root, compRoot, filePath+"."+ext)
		if err == nil {
			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}

		if html == nil {
			path, err = joinPath(root, filePath+"."+ext)
			if err == nil {
				html, err = ioutil.ReadFile(path)
				if err != nil {
					html = nil
				}
			}
		}
	}

	if html == nil && allowImport {
		path, err = joinPath(root, filePath+"."+ext)
		if err == nil {
			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	if html == nil {
		return fileData{}, err
	}

	// pre compile
	file, err := preCompile(html)
	if err != nil {
		return fileData{}, err
	}

	//todo: cache file and listen for changes

	return file, nil
}

func preCompile(html []byte) (fileData, error) {
	html = append([]byte("\n"), html...)
	html = append(html, []byte("\n")...)

	html = encodeEncoding(html)

	objStrings := []stringObj{}
	stringList := [][]byte{}

	// extract strings and comments
	html = regex.RepFunc(html, `(?s)(<!--.*?-->|/\*.*?\*/|\r?\n//.*?\r?\n)|(["'`+"`"+`])((?:\\[\\"'`+"`"+`]|.)*?)\2`, func(data func(int) []byte) []byte {
		if len(data(1)) != 0 {
			return []byte{}
		}
		objStrings = append(objStrings, stringObj{s: decodeEncoding(data(3)), q: data(2)[0]})
		return regex.JoinBytes([]byte("%!s"), len(objStrings)-1, []byte("!%"))
	})

	decodeStrings := func(html []byte, mode int) []byte {
		return decodeEncoding(regex.RepFunc(html, `%!s([0-9]+)!%`, func(data func(int) []byte) []byte {
			i, err := strconv.Atoi(string(data(1)))
			if err != nil || len(objStrings) <= i {
				return []byte{}
			}
			str := objStrings[i]

			if mode == 1 && regex.Match(str.s, `^-?[0-9]+(\.[0-9]+|)$`) {
				return str.s
			} else if mode == 2 {
				return str.s
			} else if mode == 3 {
				if regex.Match(str.s, `^-?[0-9]+(\.[0-9]+|)$`) {
					return str.s
				} else {
					stringList = append(stringList, str.s)
					return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
				}
			} else if mode == 4 {
				stringList = append(stringList, str.s)
				return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
			}

			return regex.JoinBytes(str.q, str.s, str.q)
		}))
	}

	// extract scripts
	objScripts := []scriptObj{}
	html = regex.RepFunc(html, `(?s)<(script|js|style|css|less|markdown|md|text|txt|raw)(\s+.*?|)>(.*?)</\1>`, func(data func(int) []byte) []byte {
		cont := decodeEncoding(decodeStrings(data(3), 0))

		var tag byte
		if regex.Match(data(1), `^(markdown|md)$`) {
			tag = 'm'
			cont = compileMD(cont)
		} else if regex.Match(data(1), `^(text|txt|raw)$`) {
			tag = 't'
			cont = escapeHTML(cont)
		} else if bytes.Equal(data(1), []byte("raw")) {
			tag = 'r'
		} else if regex.Match(data(1), `^(script|js)$`) {
			tag = 'j'
			cont = compileJS(cont)
		} else if regex.Match(data(1), `^(style|css|less)$`) {
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
	html = regex.RepFunc(html, `(?s)<(/|)(`+regHtmlTag+`+)(\s+.*?|)\s*(/|)>`, func(data func(int) []byte) []byte {
		argStr := regex.RepStr(regex.RepStr(data(3), `^\s+`, []byte{}), `\s+`, []byte(" "))

		newArgs := map[string][]byte{}

		ind := 0
		vInd := -1

		if len(argStr) != 0 {
			if regex.Match(data(2), `^_(el(if|se)|if)$`) {
				argStr = regex.RepFunc(argStr, `\s*([!<>=]|)\s*(=)|(&)\s*(&)|(\|)\s*(\|)|([<>&|])\s*`, func(data func(int) []byte) []byte {
					return regex.JoinBytes(' ', data(0), ' ')
				})
				argStr = regex.RepStr(argStr, `\s+`, []byte(" "))
			} else {
				argStr = regex.RepStr(argStr, `\s*=\s*`, []byte("="))
			}

			args := bytes.Split(argStr, []byte(" "))

			if regex.Match(data(2), `^_(el(if|se)|if)$`) {
				for _, v := range args {
					newArgs[strconv.Itoa(ind)] = decodeStrings(v, 3)
					ind++
				}
			} else {
				for _, v := range args {
					if regex.Match(v, `^(\{\{\{?)(.*?)(\}\}\}?)$`) {
						if bytes.Contains(v, []byte("=")) {
							esc := true
							v = regex.RepFunc(v, `^(\{\{\{?)(.*?)(\}\}\}?)$`, func(data func(int) []byte) []byte {
								if len(data(1)) == 3 && len(data(3)) == 3 {
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

								// newVal := append(append(key, []byte(`=`)...), decodeStrings(val[1], 1)...)
								newVal := regex.JoinBytes(key, '=', decodeStrings(val[1], 1))
								newVal = regex.RepFunc(newVal, `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
									stringList = append(stringList, data(2))
									return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
								})

								if esc {
									newArgs[strconv.Itoa(vInd)] = regex.JoinBytes([]byte("{{"), newVal, []byte("}}"))
									vInd--
								} else {
									newArgs[strconv.Itoa(vInd)] = regex.JoinBytes([]byte("{{{"), newVal, []byte("}}}"))
									vInd--
								}
							} else {
								decompVal := regex.RepFunc(decodeStrings(val[1], 1), `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
									stringList = append(stringList, data(2))
									return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
								})
								newVal := regex.JoinBytes(decodeStrings(val[0], 2), '=', decompVal)

								if esc {
									newArgs[strconv.Itoa(vInd)] = regex.JoinBytes([]byte("{{"), newVal, []byte("}}"))
									vInd--
								} else {
									newArgs[strconv.Itoa(vInd)] = regex.JoinBytes([]byte("{{{"), newVal, []byte("}}}"))
									vInd--
								}
							}
						} else {
							decompVal := regex.RepFunc(decodeStrings(v, 1), `(?s)(['`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
								stringList = append(stringList, data(2))
								return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
							})

							newArgs[strconv.Itoa(ind)] = decompVal
							ind++
						}
					} else if bytes.Contains(v, []byte("=")) {
						val := bytes.Split(v, []byte("="))

						decompVal := decodeStrings(val[1], 1)

						if regex.Match(decompVal, `^\{\{\{?.*?\}\}\}?$`) {
							decompVal = regex.RepFunc(decodeStrings(val[1], 1), `(?s)(['"`+"`"+`])((?:\\[\\'`+"`"+`]|.)*?)\1`, func(data func(int) []byte) []byte {
								stringList = append(stringList, data(2))
								return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
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
		if !regex.Match(data(2), `^[A-Z_]`) {
			if len(newArgs) > 0 {
				args1, args2, args3 := []byte{}, []byte{}, []byte{}
				for key, val := range newArgs {
					if i, err := strconv.Atoi(key); err == nil {
						if i < 0 {
							args2 = append(args2, regex.JoinBytes(' ', val)...)
						} else {
							args3 = append(args3, regex.JoinBytes(' ', val)...)
						}
					} else {
						args1 = append(args1, regex.JoinBytes(' ', key, '=', val)...)
					}
				}
				newArgsBasic = regex.JoinBytes(args1, args2, args3)
			} else {
				newArgsBasic = []byte{}
			}
		}

		if len(data(4)) != 0 || singleTagList[string(data(2))] {
			// self closing tag
			if newArgsBasic != nil {
				return regex.JoinBytes('<', data(2), newArgsBasic, []byte("/>"))
			}

			if len(newArgs) > 0 {
				argList = append(argList, newArgs)
				return regex.JoinBytes('<', data(2), ':', tagIndex, ' ', len(argList)-1, []byte("/>"))
			}
			return regex.JoinBytes('<', data(2), ':', tagIndex, []byte("/>"))
		} else if len(data(1)) != 0 {
			// closing tag
			if newArgsBasic != nil {
				return regex.JoinBytes("</", data(2), newArgsBasic, '>')
			}

			if tagIndex > 0 {
				tagIndex--
			}
			return regex.JoinBytes([]byte("</"), data(2), ':', tagIndex, '>')
		} else {
			// opening tag
			if newArgsBasic != nil {
				return regex.JoinBytes('<', data(2), newArgsBasic, '>')
			}

			tagIndex++

			if len(newArgs) > 0 {
				argList = append(argList, newArgs)
				return regex.JoinBytes('<', data(2), ':', tagIndex-1, " ", len(argList)-1, '>')
			}
			return regex.JoinBytes('<', data(2), ':', tagIndex-1, '>')
		}
	})

	// move var strings to seperate list
	html = regex.RepFunc(html, `(?s)(\{\{\{?)(.*?)(\}\}\}?)`, func(data func(int) []byte) []byte {
		esc := true
		if len(data(1)) == 3 && len(data(3)) == 3 {
			esc = false
		}

		val := regex.RepFunc(data(2), `%!s[0-9]+!%`, func(data func(int) []byte) []byte {
			stringList = append(stringList, decodeStrings(data(0), 2))
			return regex.JoinBytes([]byte("%!"), len(stringList)-1, []byte("!%"))
		})

		if esc {
			return regex.JoinBytes([]byte("{{"), val, []byte("}}"))
		}
		return regex.JoinBytes([]byte("{{{"), val, []byte("}}}"))
	})

	html = decodeStrings(html, 0)
	html = decodeEncoding(html)

	// preload components
	/* go regex.RepFunc(html, `(?s)<([A-Z]`+regHtmlTag+`+):[0-9]+(\s+[0-9]+|)/?>`, func(data func(int) []byte) []byte {
		name := string(data(1))

		getFile(name, true, true)
		return []byte{}
	}, true) */

	return fileData{html: html, args: argList, str: stringList, script: objScripts}, nil
}

func compileLayout(res *[]byte, opts map[string]interface{}, allowImport bool){
	layout := []byte("<BODY/>")
	
	template := "layout"
	if opts["template"] != nil && reflect.TypeOf(opts["template"]) == varType["string"] {
		template = opts["template"].(string)
	} else if getOPT("template") != "" {
		template = getOPT("template")
	}

	preLayout, err := getFile(template, false, allowImport)
	if err != nil {
		*res = layout
		return
	}
	preLayout.html = regex.RepStr(preLayout.html, `(?i){{{?\s*body\s*}}}?|<body\s*/>`, []byte("<BODY/>"))

	layout = compile(preLayout, opts, false, allowImport)

	//todo: smartly auto insert body tag if missing

	*res = layout
}

func compile(file fileData, opts map[string]interface{}, includeTemplate bool, allowImport bool) []byte {

	hasLayout := false
	// layoutReady := false
	var layout []byte = nil
	if includeTemplate && (opts["template"] != nil || getOPT("template") != "") {
		hasLayout = true

		go compileLayout(&layout, opts, allowImport)

		_ = (func() {
			template := "layout"
			if opts["template"] != nil && reflect.TypeOf(opts["template"]) == varType["string"] {
				template = opts["template"].(string)
			} else if getOPT("template") != "" {
				template = getOPT("template")
			}

			preLayout, err := getFile(template, false, allowImport)
			if err != nil {
				layout = []byte("<BODY/>")
				return
			}
			preLayout.html = regex.RepStr(preLayout.html, `(?i){{{?\s*body\s*}}}?|<body\s*/>`, []byte("<BODY/>"))

			layout = compile(preLayout, opts, false, allowImport)

			//todo: smartly auto insert body tag if missing

			// layoutReady = true
		})
	}

	// handle functions, components, and imports with content
	file.html = runFuncs(file.html, opts, 0, file, allowImport)

	// handle functions without content
	file.html = regex.RepFunc(file.html, `(?s)<_([\w_\-.$!]`+regHtmlTag+`*):([0-9]+)(\s+[0-9]+|)/>`, func(data func(int) []byte) []byte {

		// get function
		var fn func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{}
		funcs := tagFuncs[string(data(1))]

		for reflect.TypeOf(funcs) == varType["string"] {
			funcs = tagFuncs[funcs.(string)]
		}

		if reflect.TypeOf(funcs) == varType["tagFunc"] {
			fn = funcs.(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{})
			} else {
			return []byte{}
		}

		// get args
		args := map[string][]byte{}
		argI, err := strconv.Atoi(strings.TrimSpace(string(data(3))))
		if err == nil {
			args = file.args[argI]
		}

		// get level
		level, err := strconv.Atoi(string(data(2)))
		if err != nil {
			level = -1
		}

		cont := fn(args, []byte{}, opts, level + 1, file)

		if cont == nil {
			return []byte{}
		}

		res := []byte{}
		if reflect.TypeOf(cont) == varType["arrayByte"] {
			for _, v := range cont.([][]byte) {
				res = append(res, runFuncs(v, opts, level + 1, file, allowImport)...)
			}
		}else{
			res = runFuncs([]byte(toString(cont)), opts, level + 1, file, allowImport)
		}

		return res
	})

	// handle components and imports with content
	file.html = regex.RepFunc(file.html, `(?s)<([A-Z]|_:)(`+regHtmlTag+`*):[0-9]+(\s+[0-9]+|)/>`, func(data func(int) []byte) []byte {

		// get args
		args := map[string][]byte{}
		argI, err := strconv.Atoi(strings.TrimSpace(string(data(3))))
		if err == nil {
			args = file.args[argI]
		}

		canImport := allowImport
		if args["_noimport"] != nil {
			canImport = false
		}

		compOpts := opts
		for key, val := range args {
			if regex.Match(val, `^(\{\{\{?)(.*?)(\}\}\}?)$`) {
				esc := true
				val = regex.RepFunc(val, `^(\{\{\{?)(.*?)(\}\}\}?)$`, func(d func(int) []byte) []byte {
					if len(d(1)) == 3 && len(d(3)) == 3 {
						esc = false
					}
					return d(2)
				})

				if bytes.ContainsRune(val, '=') {
					arg := strings.SplitN(string(val), "=", 2)
					v := getOpt(opts, arg[1], false)
					if esc {
						vt := reflect.TypeOf(v)
						if vt == varType["string"] {
							v = string(escapeHTMLArgs([]byte(v.(string))))
						}else if vt == varType["byteArray"] {
							v = escapeHTMLArgs(v.([]byte))
						}else if vt == varType["byte"] {
							v = escapeHTMLArgs([]byte{v.(byte)})[0]
						}
					}
					compOpts[arg[0]] = v
				}else{
					v := getOpt(opts, string(val), true)
					if esc {
						vs := string(escapeHTMLArgs([]byte(v.(string))))
						if strings.ContainsRune(vs, '=') {
							arg := strings.SplitN(vs, "=", 2)
							compOpts[arg[0]] = arg[1]
						}else{
							compOpts[vs] = true
						}
					}
				}
			} else if regex.Match([]byte(key), `^[0-9]+(\.[0-9]+|)$`) {
				compOpts[string(val)] = true
			} else {
				compOpts[key] = val
			}
		}

		// if component
		if len(data(1)) == 1 {
			fileName := string(append(data(1), data(2)...))
			comp, err := getFile(fileName, true, canImport)
			if err != nil {
				return []byte{}
			}

			return compile(comp, compOpts, false, canImport)
		}

		if canImport {
			comp, err := getFile(string(data(2)), false, canImport)
			if err != nil {
				return []byte{}
			}

			return compile(comp, compOpts, false, canImport)
		}

		return []byte{}
	})

	// handle html args with vars
	file.html = regex.RepFunc(file.html, `<(`+regHtmlTag+`+)\s+(.*?)\s*(/?)>`, func(data func(int) []byte) []byte {
		args := bytes.Split(data(2), []byte(" "))

		hasChanged := false
		for i, arg := range args {
			if regex.Match(arg, `(\{\{\{?)(.*?)(\}\}\}?)`) {
				hasChanged = true

				var key []byte
				var argV []byte
				key = regex.RepFunc(arg, `(\{\{\{?)(.*?)(\}\}\}?)`, func(d func(int) []byte) []byte {
					esc := true
					if len(d(1)) == 3 && len(d(3)) == 3 {
						esc = false
					}

					val := regex.RepStr(d(2), `(?s)["'\']`, []byte{})

					if bytes.ContainsRune(val, '=') {
						v := bytes.SplitN(val, []byte("="), 2)
						argV = []byte(getOpt(opts, string(v[1]), true).(string))
						if esc {
							argV = escapeHTMLArgs(argV)
						}

						return append(v[0], '=')
					}

					argV = []byte(getOpt(opts, string(val), true).(string))
					if esc {
						argV = escapeHTMLArgs(argV)
					}

					return []byte{}
				})

				if len(argV) == 0 {
					args[i] = []byte{}
				}else{
					if len(key) == 0 {
						args[i] = argV
					} else {
						args[i] = append(key, argV...)
					}
				}
			}
		}

		if hasChanged {
			return regex.JoinBytes('<', data(1), ' ', bytes.Join(args, []byte(" ")), data(3), '>')
		}

		return data(0)
	})

	// merge layout with content
	if hasLayout {
		for layout == nil {
			time.Sleep(1000)
		}
		file.html = regex.RepStr(layout, `<BODY/>`, file.html)
	}

	// handle other vars
	file.html = regex.RepFunc(file.html, `(\{\{\{?)(.*?)(\}\}\}?)`, func(data func(int) []byte) []byte {
		esc := true
		if len(data(1)) == 3 && len(data(3)) == 3 {
			esc = false
		}

		val := getOpt(opts, string(data(2)), true).(string)

		if esc {
			return escapeHTML([]byte(val))
		}

		return []byte(val)
	})

	// put back scripts
	file.html = regex.RepFunc(file.html, `<!_script\s+([0-9]+)/>`, func(data func(int) []byte) []byte {
		i, err := strconv.Atoi(string(data(1)))
		if err != nil {
			return []byte{}
		}

		script := file.script[i]

		class := "undefined"
		if script.tag == 'j' {
			return regex.JoinBytes([]byte("<script"), script.args, '>', script.cont, []byte("</script>"))
		}else if script.tag == 'c' {
			return regex.JoinBytes([]byte("<style"), script.args, '>', script.cont, []byte("</style>"))
		}else if script.tag == 'm' {
			class = "markdown"
		}else if script.tag == 't' {
			class = "text"
		}else if script.tag == 'r' {
			class = "raw"
		}

		return regex.JoinBytes([]byte("<div class=\""), class, []byte("\""), script.args, '>', script.cont, []byte("</div>"))
	})

	// put back strings
	file.html = regex.RepFunc(file.html, `%!([0-9]+)!%`, func(data func(int) []byte) []byte {
		i, err := strconv.Atoi(string(data(1)))
		if err != nil {
			return []byte{}
		}

		return file.str[i]
	})

	// set css and js vars from public opts
	if reflect.TypeOf(opts["public"]) == varType["map"] {
		publicOpts := opts["public"].(map[string]interface{})

		if reflect.TypeOf(publicOpts["js"]) == varType["map"] && len(publicOpts["js"].(map[string]interface{})) != 0 {
			jsVars := []byte("<script>")

			for key, val := range publicOpts["js"].(map[string]interface{}) {
				json, err := stringifyJSON(val)
				if err != nil {
					continue
				}
				json = regex.RepStr(json, `\n`, []byte(`\n`))
				key = string(regex.RepStr([]byte(key), `-`, []byte("_")))
				key = string(regex.RepStr([]byte(key), `[^\w_]`, []byte{}))
				jsVars = append(jsVars, regex.JoinBytes([]byte(";const "), key, '=', json, ';')...)
			}

			jsVars = append(jsVars, []byte("</script>")...)

			if regex.Match(file.html, `</head>`) {
				regex.RepStr(file.html, `</head>`, append(jsVars, []byte("</head>")...))
			}else{
				file.html = append(jsVars, file.html...)
			}
		}

		if reflect.TypeOf(publicOpts["css"]) == varType["map"] && len(publicOpts["css"].(map[string]interface{})) != 0 {
			cssVars := []byte("<style>:root{")

			for key, val := range publicOpts["css"].(map[string]interface{}) {
				key = string(regex.RepStr([]byte(key), `[^\w_-]`, []byte{}))
				cssVars = append(cssVars, regex.JoinBytes([]byte("--"), key, ':', val, ';')...)
			}

			cssVars = append(cssVars, []byte("}</style>")...)

			if regex.Match(file.html, `</head>`) {
				regex.RepStr(file.html, `</head>`, append(cssVars, []byte("</head>")...))
			}else{
				file.html = append(cssVars, file.html...)
			}
		}
	}

	return file.html
}

func runFuncs(html []byte, opts map[string]interface{}, level int, file fileData, allowImport bool) []byte {
	levelStr := strconv.Itoa(level)

	// handle functions with content
	html = regex.RepFunc(html, `(?s)<_([\w_\-.$!]`+regHtmlTag+`*):`+levelStr+`(\s+[0-9]+|)>(.*?)</_\1:`+levelStr+`>`, func(data func(int) []byte) []byte {

		// get function
		var fn func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{}
		funcs := tagFuncs[string(data(1))]

		for reflect.TypeOf(funcs) == varType["string"] {
			funcs = tagFuncs[funcs.(string)]
		}

		if reflect.TypeOf(funcs) == varType["tagFunc"] {
			fn = funcs.(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{})
		} else {
			return []byte{}
		}

		// get args
		args := map[string][]byte{}
		argI, err := strconv.Atoi(strings.TrimSpace(string(data(2))))
		if err == nil {
			args = file.args[argI]
		}

		cont := fn(args, data(3), opts, level + 1, file)

		if cont == nil {
			return []byte{}
		}

		res := []byte{}
		if reflect.TypeOf(cont) == varType["arrayByte"] {
			for _, v := range cont.([][]byte) {
				res = append(res, runFuncs(v, opts, level + 1, file, allowImport)...)
			}
		}else{
			res = runFuncs([]byte(toString(cont)), opts, level + 1, file, allowImport)
		}

		return res
	})

	// handle components and imports with content
	html = regex.RepFunc(html, `(?s)<([A-Z]|_:)(`+regHtmlTag+`*):`+levelStr+`(\s+[0-9]+|)>(.*?)</\1\2:`+levelStr+`>`, func(data func(int) []byte) []byte {

		// get args
		args := map[string][]byte{}
		argI, err := strconv.Atoi(strings.TrimSpace(string(data(3))))
		if err == nil {
			args = file.args[argI]
		}

		canImport := allowImport
		if args["_noimport"] != nil {
			canImport = false
		}

		compOpts := opts
		for key, val := range args {
			if regex.Match(val, `^(\{\{\{?)(.*?)(\}\}\}?)$`) {
				esc := true
				val = regex.RepFunc(val, `^(\{\{\{?)(.*?)(\}\}\}?)$`, func(d func(int) []byte) []byte {
					if len(d(1)) == 3 && len(d(3)) == 3 {
						esc = false
					}
					return d(2)
				})

				if bytes.ContainsRune(val, '=') {
					arg := strings.SplitN(string(val), "=", 2)
					v := getOpt(opts, arg[1], false)
					if esc {
						vt := reflect.TypeOf(v)
						if vt == varType["string"] {
							v = string(escapeHTMLArgs([]byte(v.(string))))
						}else if vt == varType["byteArray"] {
							v = escapeHTMLArgs(v.([]byte))
						}else if vt == varType["byte"] {
							v = escapeHTMLArgs([]byte{v.(byte)})[0]
						}
					}
					compOpts[arg[0]] = v
				}else{
					v := getOpt(opts, string(val), true)
					if esc {
						vs := string(escapeHTMLArgs([]byte(v.(string))))
						if strings.ContainsRune(vs, '=') {
							arg := strings.SplitN(vs, "=", 2)
							compOpts[arg[0]] = arg[1]
						}else{
							compOpts[vs] = true
						}
					}
				}
			} else if regex.Match([]byte(key), `^[0-9]+(\.[0-9]+|)$`) {
				compOpts[string(val)] = true
			} else {
				compOpts[key] = val
			}
		}

		compOpts["body"] = data(4)

		// if component
		if len(data(1)) == 1 {
			fileName := string(append(data(1), data(2)...))
			comp, err := getFile(fileName, true, canImport)
			if err != nil {
				return []byte{}
			}

			return compile(comp, compOpts, false, canImport)
		}

		if canImport {
			comp, err := getFile(string(data(2)), false, canImport)
			if err != nil {
				return []byte{}
			}

			return compile(comp, compOpts, false, canImport)
		}

		return []byte{}
	})

	return html
}

func escapeHTML(html []byte) []byte {
	html = regex.RepFunc(html, `[<>&]`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("<")) {
			return []byte("&lt;")
		} else if bytes.Equal(data(0), []byte(">")) {
			return []byte("&gt;")
		}
		return []byte("&amp;")
	})
	return regex.RepStr(html, `&amp;(amp;)*`, []byte("&amp;"))
}

func escapeHTMLArgs(html []byte) []byte {
	return regex.RepFunc(html, `[\\"'`+"`"+`]`, func(data func(int) []byte) []byte {
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
	return regex.RepFunc(html, `%!|!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("%!")) {
			return []byte("%!o!%")
		}
		return []byte("%!c!%")
	})
}

func decodeEncoding(html []byte) []byte {
	return regex.RepFunc(html, `%!([oc])!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(1), []byte("o")) {
			return []byte("%!")
		}
		return []byte("!%")
	})
}

func stringifyJSON(data interface{}) ([]byte, error) {
	json, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}
	json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	return json, nil
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
