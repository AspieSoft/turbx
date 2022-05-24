package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
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
	html []byte
	args []map[string][]byte
	str [][]byte
	script []scriptObj
}

var regCache map[string]*regexp.Regexp

var singleTagList map[string]bool

func main() {
	regCache = map[string]*regexp.Regexp{}

	singleTagList = map[string]bool{
		"br": true,
		"hr": true,
	}

	//todo: use escapeHTMLArgs where needed in preCompile

	file := preCompile(os.Args[1])
	_ = file

	//todo: return output to js
	//todo (later): make go handle final compile, and just output html

	args := []map[string]string{}

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
		"html": string(file.html),
		"args": args,
		"strings": str,
		"scripts": script,
	}

	json, err := json.Marshal(res)
	_ = json
	if err != nil {
		panic(err)
	}

	out, err := compress(string(json))
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func preCompile(file string) fileData {
	html, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)
	}

	html = append([]byte("\n"), html...)
	html = append(html, []byte("\n")...)

	html = encodeEncoding(html)

	objStrings := []stringObj{}
	stringList := [][]byte{}

	html = regRepFunc(html, `(?s)(<!--.*?-->|/\*.*?\*/|\r?\n//.*?\r?\n)|"((?:\\[\\"]|.)*?)"|'((?:\\[\\']|.)*?)'|`+"`((?:\\\\[\\\\`]|.)*?)`", func(data [][]byte) []byte {
		if !bytes.Equal(data[1], []byte("")) {
			return []byte("")
		} else if !bytes.Equal(data[2], []byte("")) {
			objStrings = append(objStrings, stringObj{s: decodeEncoding(data[2]), q: '"'})
		} else if !bytes.Equal(data[3], []byte("")) {
			objStrings = append(objStrings, stringObj{s: decodeEncoding(data[3]), q: '\''})
		} else if !bytes.Equal(data[4], []byte("")) {
			objStrings = append(objStrings, stringObj{s: decodeEncoding(data[4]), q: '`'})
		}
		return []byte("%!s" + strconv.Itoa(len(objStrings)-1) + "!%")
	})

	decodeStrings := func(html []byte, mode int) []byte {
		return decodeEncoding(regRepFunc(html, `%!s([0-9]+)!%`, func(data [][]byte) []byte {
			i, err := strconv.Atoi(string(data[1]))
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
					return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
				}
			}else if mode == 4 {
				stringList = append(stringList, str.s)
				return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
			}

			return append(append([]byte{str.q}, str.s...), str.q)
		}))
	}

	objScripts := []scriptObj{}

	tags := `script|js|style|css|less|markdown|md|text|txt|raw`
	html = regRepFunc(html, `(?s)<(`+tags+`)(\s+.*?|)>(.*?)</(`+tags+`)>`, func(data [][]byte) []byte {
		cont := decodeStrings(data[3], 0)

		var tag byte
		if regMatch(data[1], `^(markdown|md)$`) {
			tag = 'm'
			cont = compileMD(cont)
		} else if regMatch(data[1], `^(text|txt|raw)$`) {
			tag = 't'
			cont = escapeHTML(cont)
		} else if bytes.Equal(data[1], []byte("raw")) {
			tag = 'r'
		} else if regMatch(data[1], `^(script|js)$`) {
			tag = 'j'
			cont = compileJS(cont)
		} else if regMatch(data[1], `^(style|css|less)$`) {
			tag = 'c'
			cont = compileCSS(cont)
		}

		args := decodeStrings(data[2], 0)

		objScripts = append(objScripts, scriptObj{tag, args, cont})
		i := strconv.Itoa(len(objScripts) - 1)

		return []byte("<!_script " + i + "/>")
	})

	// move html args to list
	/*
		!	note: if you find a bug with changed html args
		*	this function moves them to the closing tag to merge opening and closing tag args
		*	the next function moves them back to the opening tag, but it could be buggy
	*/
	fullArgList := []map[string][]byte{}
	tagIndex := []map[string][]byte{}
	maxTagIndex := 0
	html = regRepFunc(html, `(?s)<(/|)([\w_\-\.$!:]+)(\s+.*?|)\s*(/|)>`, func(data [][]byte) []byte {
		argStr := regRepStr(regRepStr(data[3], `^\s+`, []byte("")), `\s+`, []byte(" "))

		newArgs := map[string][]byte{}

		ind := 0
		vInd := -1

		if len(argStr) != 0 {
			if regMatch(data[2], `^_(el(if|se)|if)$`) {
				argStr = regRepFunc(argStr, `\s*([!<>=]|)\s*(=)|(&)\s*(&)|(\|)\s*(\|)|([<>&|])\s*`, func(data [][]byte) []byte {
					return append(append([]byte(" "), data[0]...), ' ')
				})
				argStr = regRepStr(argStr, `\s+`, []byte(" "))
			} else {
				argStr = regRepStr(argStr, `\s*=\s*`, []byte("="))
			}

			args := bytes.Split(argStr, []byte(" "))

			if regMatch(data[2], `^_(el(if|se)|if)$`) {
				for _, v := range args {
					newArgs[strconv.Itoa(ind)] = decodeStrings(v, 3)
					ind++
				}
			} else {
				for _, v := range args {
					if regMatch(v, `^\{\{\{?.*?\}\}\}?$`) {
						if bytes.Contains(v, []byte("=")) {
							esc := true
							v = regRepFunc(v, `(\{\{\{?)(.*?)(\}\}\}?)`, func(data [][]byte) []byte {
								if bytes.Equal(data[1], []byte("{{{")) || bytes.Equal(data[3], []byte("}}}")) {
									esc = false
								}
								return data[2]
							})
							val := bytes.Split(v, []byte("="))

							if len(val[0]) == 0 {
								key := decodeStrings(val[1], 2)
								key = bytes.Split(key, []byte("|"))[0]
								keyObj := bytes.Split(key, []byte("."))
								key = keyObj[len(keyObj)-1]

								newVal := append(append(key, []byte(`=`)...), decodeStrings(val[1], 1)...)
								newVal = regRepFunc(newVal, `(?s)'((?:\\[\\']|.)*?)'|` + "`((?:\\\\[\\\\`]|.)*?)`", func(b [][]byte) []byte {
									if len(data[1]) != 0 {
										stringList = append(stringList, data[1])
									} else if len(data[2]) != 0 {
										stringList = append(stringList, data[2])
									}
									return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
								})

								if esc {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{"), newVal...), []byte("}}")...)
									vInd--
								} else {
									newArgs[strconv.Itoa(vInd)] = append(append([]byte("{{{"), newVal...), []byte("}}}")...)
									vInd--
								}
							} else {
								decompVal := regRepFunc(decodeStrings(val[1], 1), `(?s)'((?:\\[\\']|.)*?)'|` + "`((?:\\\\[\\\\`]|.)*?)`", func(b [][]byte) []byte {
									if len(data[1]) != 0 {
										stringList = append(stringList, data[1])
									} else if len(data[2]) != 0 {
										stringList = append(stringList, data[2])
									}
									return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
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
							decompVal := regRepFunc(decodeStrings(v, 1), `(?s)'((?:\\[\\']|.)*?)'|` + "`((?:\\\\[\\\\`]|.)*?)`", func(b [][]byte) []byte {
								if len(data[1]) != 0 {
									stringList = append(stringList, data[1])
								} else if len(data[2]) != 0 {
									stringList = append(stringList, data[2])
								}
								return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
							})

							newArgs[strconv.Itoa(ind)] = decompVal
							ind++
						}
					} else if bytes.Contains(v, []byte("=")) {
						val := bytes.Split(v, []byte("="))

						decompVal := decodeStrings(val[1], 1)
						
						if regMatch(decompVal, `^\{\{\{?.*?\}\}\}?$`) {
							decompVal = regRepFunc(decodeStrings(val[1], 1), `(?s)'((?:\\[\\']|.)*?)'|"((?:\\[\\"]|.)*?)"|` + "`((?:\\\\[\\\\`]|.)*?)`", func(b [][]byte) []byte {
								if len(data[1]) != 0 {
									stringList = append(stringList, data[1])
								} else if len(data[2]) != 0 {
									stringList = append(stringList, data[2])
								}
								return []byte("%!"+strconv.Itoa(len(stringList)-1)+"!%")
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

		if len(data[4]) != 0 || singleTagList[string(data[2])] {
			// self closing tag
			if len(newArgs) > 0 {
				fullArgList = append(fullArgList, newArgs)
				return append(append([]byte("<"), data[2]...), []byte(":"+strconv.Itoa(len(tagIndex))+" "+strconv.Itoa(len(fullArgList)-1)+"/>")...)
			}
			return append(append([]byte("<"), data[2]...), []byte(":"+strconv.Itoa(len(tagIndex))+"/>")...)
		} else if len(data[1]) != 0 {
			// closing tag
			if len(tagIndex) > 0 {
				firstArgs := tagIndex[len(tagIndex)-1]
				tagIndex = tagIndex[:len(tagIndex)-1]

				// merge open tag args
				for key, val := range firstArgs {
					if i, err := strconv.Atoi(key); err == nil {
						if i < 0 {
							i += vInd
						} else {
							i += ind
						}
						s := strconv.Itoa(i)
						if _, ok := newArgs[s]; !ok {
							newArgs[s] = val
						}
					} else {
						if _, ok := newArgs[key]; !ok {
							newArgs[key] = val
						}
					}
				}

				if len(newArgs) > 0 {
					fullArgList = append(fullArgList, newArgs)
					return append(append([]byte("</"), data[2]...), []byte(":"+strconv.Itoa(len(tagIndex))+" "+strconv.Itoa(len(fullArgList)-1)+">")...)
				}
				return append(append([]byte("</"), data[2]...), []byte(":"+strconv.Itoa(len(tagIndex))+">")...)
			}
		} else {
			// opening tag
			tagIndex = append(tagIndex, newArgs)
			l := len(tagIndex)
			if l > maxTagIndex {
				maxTagIndex = l
			}
			return append(append([]byte("<"), data[2]...), []byte(":"+strconv.Itoa(l-1)+">")...)
		}

		return []byte("")
	})

	// fix arg pos
	/*
		!	note: this function could be buggy
		*	this is that next function previously mentioned in the above functions comment
		*	this moves the args reference back from the closing tag, to the opening tag
		*	this is not the most efficient method, but it runs
		* if you can merge this function with the above function, that may fix a bug, or improve performance
	*/
	for i := 0; i < maxTagIndex; i++ {
		iStr := strconv.Itoa(i)
		html = regRepFunc(html, `(?s)<([\w_\-\.$!:]+):`+iStr+`>(.*?)</([\w_\-\.$!:]+):`+iStr+`(\s+[0-9]+|)>`, func(data [][]byte) []byte {
			if !bytes.Equal(data[1], data[3]) || len(data[4]) == 0 {
				return data[0]
			}
			return append(append(append(append(append([]byte("<"), data[1]...), append([]byte(":"+iStr), data[4]...)...), append([]byte(">"), data[2]...)...), append([]byte("</"), data[3]...)...), []byte(":"+iStr+">")...)
		})
	}

	// put back non-function and non-component args
	/*
		?	note: to improve performance
		*	if the above 2 functions are merged
		*	this function may also be able to be merged with them
	*/
	argList := []map[string][]byte{}
	html = regRepFunc(html, `(?s)(</?)([\w_\-\.$!:]+)(:[0-9]+)(\s+[0-9]+|)(/?>)`, func(data [][]byte) []byte {
		iS := bytes.Trim(data[4], " ")
		i := -1
		if len(iS) != 0 {
			var err error
			i, err = strconv.Atoi(string(iS))
			if err != nil {
				if !regMatch(data[2], `^[A-Z_]`) {
					return append(append(data[1], data[2]...), data[5]...)
				}
				return append(append(data[1], data[2]...), append(data[3], data[5]...)...)
			}
		}

		if i == -1 || i >= len(fullArgList) {
			if !regMatch(data[2], `^[A-Z_]`) {
				return append(append(data[1], data[2]...), data[5]...)
			}
			return append(append(data[1], data[2]...), append(data[3], data[5]...)...)
		}

		if !regMatch(data[2], `^[A-Z_]`) {
			args1, args2, args3 := []byte{}, []byte{}, []byte{}
			for key, val := range fullArgList[i] {
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
			args := append(append(args1, args2...), args3...)

			return append(append(data[1], data[2]...), append(args, data[5]...)...)
		}

		argList = append(argList, fullArgList[i])

		return append(append(append(data[1], data[2]...), data[3]...), append(append([]byte(" "), []byte(strconv.Itoa(len(argList)-1))...), data[5]...)...)
	})

	// move var strings to seperate list
	html = regRepFunc(html, `(\{\{\{?)(.*?)(\}\}\}?)`, func(data [][]byte) []byte {
		esc := true
		if bytes.Equal(data[1], []byte("{{{")) || bytes.Equal(data[3], []byte("}}}")) {
			esc = false
		}

		val := regRepFunc(data[2], `%!s[0-9]+!%`, func(b [][]byte) []byte {
			stringList = append(stringList, decodeStrings(data[0], 2))
			return []byte("%!" + strconv.Itoa(len(stringList)-1) + "!%")
		})

		if esc {
			return append(append([]byte("{{"), val...), []byte("}}")...)
		}
		return append(append([]byte("{{{"), val...), []byte("}}}")...)
	})

	html = decodeStrings(html, 0)
	html = decodeEncoding(html)

	return fileData{html: html, args: argList, str: stringList, script: objScripts}
}

func escapeHTML(html []byte) []byte {
	html = regRepFunc(html, `[<>&]`, func(data [][]byte) []byte {
		if bytes.Equal(data[0], []byte("<")) {
			return []byte("&lt;")
		} else if bytes.Equal(data[0], []byte(">")) {
			return []byte("&gt;")
		}
		return []byte("&amp;")
	})
	return regRepStr(html, `&amp;(amp;)*`, []byte("&amp;"))
}

func escapeHTMLArgs(html []byte) []byte {
	return regRepFunc(html, `[\\"'`+"`"+`]`, func(data [][]byte) []byte {
		return append([]byte("\\"), data[0]...)
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
	return regRepFunc(html, `%!|!%`, func(data [][]byte) []byte {
		if bytes.Equal(data[0], []byte("%!")) {
			return []byte("%!o!%")
		}
		return []byte("%!c!%")
	})
}

func decodeEncoding(html []byte) []byte {
	return regRepFunc(html, `%!([oc])!%`, func(data [][]byte) []byte {
		if bytes.Equal(data[1], []byte("o")) {
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

func regRepFunc(str []byte, re string, rep func(b [][]byte) []byte) []byte {
	var reg *regexp.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = regexp.MustCompile(re)
		regCache[re] = reg
	}

	return reg.ReplaceAllFunc(str, func(b []byte) []byte {
		return rep(reg.FindSubmatch(b))
	})
}

func regRepStr(str []byte, re string, rep []byte) []byte {
	var reg *regexp.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = regexp.MustCompile(re)
		regCache[re] = reg
	}

	return reg.ReplaceAll(str, rep)
}

func regMatch(str []byte, re string) bool {
	var reg *regexp.Regexp

	if val, ok := regCache[re]; ok {
		reg = val
	} else {
		reg = regexp.MustCompile(re)
		regCache[re] = reg
	}

	return reg.Match(str)
}
