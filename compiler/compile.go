package main

import (
	"bytes"
	"compiler/common"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex"
)

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

//todo: add option to ignore specific option vars (for example: to compile them later and cache the other vars)

func preCompile(html []byte) (fileData, error) {
	html = append([]byte("\n"), html...)
	html = append(html, []byte("\n")...)

	html = encodeEncoding(html)

	objStrings := []stringObj{}
	stringList := [][]byte{}

	// extract strings and comments
	html = regex.RepFunc(html, `(?s)(<!--.*?-->|/\*.*?\*/|\r?\n//.*?\r?\n)|(["'\'])((?:\\[\\"'\']|.)*?)\2`, func(data func(int) []byte) []byte {
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
			cont = common.EscapeHTML(cont)
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
								newVal = regex.RepFunc(newVal, `(?s)(['\'])((?:\\[\\'\']|.)*?)\1`, func(data func(int) []byte) []byte {
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
								decompVal := regex.RepFunc(decodeStrings(val[1], 1), `(?s)(['\'])((?:\\[\\'\']|.)*?)\1`, func(data func(int) []byte) []byte {
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
							decompVal := regex.RepFunc(decodeStrings(v, 1), `(?s)(['\'])((?:\\[\\'\']|.)*?)\1`, func(data func(int) []byte) []byte {
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
							decompVal = regex.RepFunc(decodeStrings(val[1], 1), `(?s)(['"\'])((?:\\[\\'\']|.)*?)\1`, func(data func(int) []byte) []byte {
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

	// precompile static functions
	html = regex.RepFunc(html, `(?s)<_([\w_\-.$!]`+regHtmlTag+`*):([0-9]+)(\s+[0-9]+|)/>`, func(data func(int) []byte) []byte {

		// get function
		var fn func(map[string][]byte, int, fileData) interface{}
		funcs := preTagFuncs[string(data(1))]

		for reflect.TypeOf(funcs) == common.VarType["string"] {
			funcs = preTagFuncs[funcs.(string)]
		}

		if reflect.TypeOf(funcs) == common.VarType["preTagFunc"] {
			fn = funcs.(func(map[string][]byte, int, fileData) interface{})
		} else {
			return []byte{}
		}

		// get args
		args := map[string][]byte{}
		argI, err := strconv.Atoi(strings.TrimSpace(string(data(3))))
		if err == nil {
			args = argList[argI]
		}

		// get level
		level, err := strconv.Atoi(string(data(2)))
		if err != nil {
			level = -1
		}

		cont := fn(args, level + 1, fileData{html: html, args: argList, str: stringList, script: objScripts})

		if cont == nil {
			return []byte{}
		}

		return []byte(common.ToString(cont))
	})


	// preload components
	go (func(){
		preCompiledComponent := map[string]bool{}
		regex.RepFunc(html, `(?s)<([A-Z]`+regHtmlTag+`+):[0-9]+(\s+[0-9]+|)/?>`, func(data func(int) []byte) []byte {
			name := string(data(1))
	
			if !preCompiledComponent[name] {
				preCompiledComponent[name] = true
				getFile(name, true, true)
			}
	
			return []byte{}
		}, true)
	})()

	return fileData{html: html, args: argList, str: stringList, script: objScripts}, nil
}

func compileLayout(res *[]byte, opts map[string]interface{}, allowImport bool){
	layout := []byte("<BODY/>")
	
	template := "layout"
	if opts["template"] != nil && reflect.TypeOf(opts["template"]) == common.VarType["string"] {
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
	var layout []byte = nil
	if includeTemplate && (opts["template"] != nil || getOPT("template") != "") {
		hasLayout = true

		go compileLayout(&layout, opts, allowImport)
	}

	// handle functions, components, and imports with content
	file.html = runFuncs(file.html, opts, 0, file, allowImport)

	// handle functions without content
	file.html = regex.RepFunc(file.html, `(?s)<_([\w_\-.$!]`+regHtmlTag+`*):([0-9]+)(\s+[0-9]+|)/>`, func(data func(int) []byte) []byte {

		// get function
		var fn func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{}
		funcs := tagFuncs[string(data(1))]

		for reflect.TypeOf(funcs) == common.VarType["string"] {
			funcs = tagFuncs[funcs.(string)]
		}

		if reflect.TypeOf(funcs) == common.VarType["tagFunc"] {
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
		contType := reflect.TypeOf(cont)
		if contType == common.VarType["arrayEachFnObj"] {
			for _, v := range cont.([]eachFnObj) {
				res = append(res, runFuncs(v.html, v.opts, level + 1, file, allowImport)...)
			}
		}else if contType == common.VarType["arrayByte"] {
			for _, v := range cont.([][]byte) {
				res = append(res, runFuncs(v, opts, level + 1, file, allowImport)...)
			}
		}else{
			res = runFuncs([]byte(common.ToString(cont)), opts, level + 1, file, allowImport)
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
						if vt == common.VarType["string"] {
							v = string(common.EscapeHTMLArgs([]byte(v.(string))))
						}else if vt == common.VarType["byteArray"] {
							v = common.EscapeHTMLArgs(v.([]byte))
						}else if vt == common.VarType["byte"] {
							v = common.EscapeHTMLArgs([]byte{v.(byte)})[0]
						}
					}
					compOpts[arg[0]] = v
				}else{
					v := getOpt(opts, string(val), true)
					if esc {
						vs := string(common.EscapeHTMLArgs([]byte(v.(string))))
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
							argV = common.EscapeHTMLArgs(argV)
						}

						return append(v[0], '=')
					}

					argV = []byte(getOpt(opts, string(val), true).(string))
					if esc {
						argV = common.EscapeHTMLArgs(argV)
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
			time.Sleep(1 * time.Millisecond)
		}
		file.html = regex.RepStr(layout, `(?i)<BODY/>`, file.html)
	}

	// handle other vars
	file.html = regex.RepFunc(file.html, `(\{\{\{?)(.*?)(\}\}\}?)`, func(data func(int) []byte) []byte {
		esc := true
		if len(data(1)) == 3 && len(data(3)) == 3 {
			esc = false
		}

		val := getOpt(opts, string(data(2)), true).(string)

		if esc {
			return common.EscapeHTML([]byte(val))
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
	if reflect.TypeOf(opts["public"]) == common.VarType["map"] {
		publicOpts := opts["public"].(map[string]interface{})

		if reflect.TypeOf(publicOpts["js"]) == common.VarType["map"] && len(publicOpts["js"].(map[string]interface{})) != 0 {
			jsVars := []byte("<script>")

			for key, val := range publicOpts["js"].(map[string]interface{}) {
				json, err := common.StringifyJSON(val)
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

		if reflect.TypeOf(publicOpts["css"]) == common.VarType["map"] && len(publicOpts["css"].(map[string]interface{})) != 0 {
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

		for reflect.TypeOf(funcs) == common.VarType["string"] {
			funcs = tagFuncs[funcs.(string)]
		}

		if reflect.TypeOf(funcs) == common.VarType["tagFunc"] {
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
		contType := reflect.TypeOf(cont)
		if contType == common.VarType["arrayEachFnObj"] {
			for _, v := range cont.([]eachFnObj) {
				res = append(res, runFuncs(v.html, v.opts, level + 1, file, allowImport)...)
			}
		}else if contType == common.VarType["arrayByte"] {
			for _, v := range cont.([][]byte) {
				res = append(res, runFuncs(v, opts, level + 1, file, allowImport)...)
			}
		}else{
			res = runFuncs([]byte(common.ToString(cont)), opts, level + 1, file, allowImport)
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
						if vt == common.VarType["string"] {
							v = string(common.EscapeHTMLArgs([]byte(v.(string))))
						}else if vt == common.VarType["byteArray"] {
							v = common.EscapeHTMLArgs(v.([]byte))
						}else if vt == common.VarType["byte"] {
							v = common.EscapeHTMLArgs([]byte{v.(byte)})[0]
						}
					}
					compOpts[arg[0]] = v
				}else{
					v := getOpt(opts, string(val), true)
					if esc {
						vs := string(common.EscapeHTMLArgs([]byte(v.(string))))
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

func compileJS(script []byte) []byte {
	//todo: minify js (also allow top level async/await)
	return script
}

func compileCSS(style []byte) []byte {
	//todo: compile less to css
	return style
}

func compileMD(md []byte) []byte {
	//todo: compile markdown
	return md
}