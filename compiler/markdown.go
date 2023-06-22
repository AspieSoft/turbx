package compiler

import (
	"bytes"
	"strconv"

	"github.com/AspieSoft/go-liveread"
	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
)

var reLinkMD *regex.Regexp = regex.Comp(`(\!|)\[((?:"(?:\\[\\"'\']|\.)*"|'(?:\\[\\"'\']|\.)*'|\'(?:\\[\\"'\']|\.)*\'|.)*?)\]\(((?:"(?:\\[\\"'\']|\.)*"|'(?:\\[\\"'\']|\.)*'|\'(?:\\[\\"'\']|\.)*\'|.)*?)\)`)

type mdListData struct {
	tab uint
	listType byte
}

func compileMarkdown(reader *liveread.Reader[uint8], write *func(b []byte, raw ...bool), firstChar *bool, spaces *uint, mdStore *map[string]interface{}) bool {

	//todo: handle markdown

	buf, err := reader.Peek(1)
	if err == nil {
		if *firstChar {
			if buf[0] == '#' {
				level := uint(1)
				buf, err = reader.Get(level, 1)
				for err == nil && buf[0] == '#' && level < 6 {
					level++
					buf, err = reader.Get(level, 1)
				}
				reader.Discard(level)

				buf, err = reader.Peek(1)
				if err == nil && buf[0] == ' ' {
					reader.Discard(1)
					buf, err = reader.Peek(1)
				}

				cont := []byte{}
				for err == nil && buf[0] != '\n' {
					cont = append(cont, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(1)
				}

				(*write)(regex.JoinBytes([]byte("<h"), int(level), '>', mdHandleFonts(cont), []byte("</h"), int(level), '>'))

				return true
			}else if buf[0] == '-' {
				level := uint(1)
				buf, err = reader.Get(level, 1)
				for err == nil && buf[0] == '-' {
					level++
					buf, err = reader.Get(level, 1)
				}

				if level >= 3 && (err != nil || buf[0] == '\r' || buf[0] == '\n') {
					reader.Discard(level)
					(*write)([]byte("<hr/>"))
					return true
				}

				buf, err = reader.Peek(1)
			}

			// handle list
			buf, err = reader.Peek(1)
			if buf[0] == '-' || buf[0] == '*' || buf[0] == '~' || regex.Comp(`^[0-9]`).MatchRef(&buf) {
				ind := uint(1)

				skipList := false
				listType := uint8(0)
				listInd := 1
				if regex.Comp(`^[0-9]`).MatchRef(&buf) {
					listType = 1

					listKey := []byte{buf[0]}
					buf, err = reader.Get(ind, 1)
					for err == nil && regex.Comp(`^[0-9]`).MatchRef(&buf) {
						listKey = append(listKey, buf[0])
						ind++
						buf, err = reader.Get(ind, 1)
					}

					if buf[0] != '.' {
						skipList = true
						buf, err = reader.Peek(1)
					}else{
						ind++
						buf, err = reader.Get(ind, 1)

						if i, e := strconv.Atoi(string(listKey)); e == nil {
							listInd = i
						}
					}
				}

				if !skipList {
					reader.Discard(ind)
					buf, err = reader.Peek(1)
		
					if buf[0] == ' ' {
						reader.Discard(1)
						buf, err = reader.Peek(1)
					}
		
					cont := []byte{}
					for err == nil && buf[0] != '\n' {
						cont = append(cont, buf[0])
						reader.Discard(1)
						buf, err = reader.Peek(1)
					}

					cont = mdHandleFonts(cont)

					closing := uint8(0)
					ind = 1
					buf, err = reader.Get(ind, 1)

					/* for regex.Comp(`^[ \t]`).MatchRef(&buf) {
						ind++
						buf, err = reader.Get(ind, 1)
					} */

					if err != nil || buf[0] == '\n' {
						closing = 1
					}

					if (*mdStore)["listTab"] == nil {
						(*mdStore)["listTab"] = []mdListData{}
					}

					if len((*mdStore)["listTab"].([]mdListData)) == 0 || (*mdStore)["listTab"].([]mdListData)[len((*mdStore)["listTab"].([]mdListData))-1].tab < *spaces {
						if closing == 0 && listType == 1 {
							sp := uint(0)
							for err == nil && buf[0] != '\n' {
								if sp > *spaces {
									for err == nil && buf[0] != '\n' {
										ind++
										buf, err = reader.Get(ind, 1)
									}
									ind++
									buf, err = reader.Get(ind, 1)
									if err != nil {
										closing = 1
										break
									}

									sp = 0
									continue
								}else if regex.Comp(`^[0-9]`).MatchRef(&buf) {
									break
								}

								sp++
								ind++
								buf, err = reader.Get(ind, 1)
							}

							if closing == 0 {
								if err != nil || buf[0] == '\n' {
									closing = 1
								}else if sp < *spaces {
									closing = 2
								}else{
									key := []byte{}
									for err == nil && regex.Comp(`^[0-9]`).MatchRef(&buf) {
										key = append(key, buf[0])
										ind++
										buf, err = reader.Get(ind, 1)
									}
									if err == nil {
										if i, e := strconv.Atoi(string(key)); e == nil && i < listInd {
											listType = 2
										}
									}
								}
							}
						}
						
						(*mdStore)["listTab"] = append((*mdStore)["listTab"].([]mdListData), mdListData{
							tab: *spaces,
							listType: listType,
						})

						if listType == 0 {
							(*write)([]byte("<ul>"))
						}else if listType == 1 {
							(*write)([]byte("<ol>"))
						}else if listType == 0 {
							(*write)([]byte("<ol reversed>"))
						}

						(*write)(regex.JoinBytes([]byte("<li>"), cont, []byte("</li>")))
					}else{
						for (*mdStore)["listTab"].([]mdListData)[len((*mdStore)["listTab"].([]mdListData))-1].tab > *spaces {
							if (*mdStore)["listTab"].([]mdListData)[len((*mdStore)["listTab"].([]mdListData))-1].listType == 0 {
								(*write)([]byte("</ul>"))
							}else{
								(*write)([]byte("</ol>"))
							}
							(*mdStore)["listTab"] = (*mdStore)["listTab"].([]mdListData)[:len((*mdStore)["listTab"].([]mdListData))-1]
						}

						(*write)(regex.JoinBytes([]byte("<li>"), cont, []byte("</li>")))
					}

					if closing == 1 {
						for len((*mdStore)["listTab"].([]mdListData)) != 0 {
							if (*mdStore)["listTab"].([]mdListData)[len((*mdStore)["listTab"].([]mdListData))-1].listType == 0 {
								(*write)([]byte("</ul>"))
							}else{
								(*write)([]byte("</ol>"))
							}
							(*mdStore)["listTab"] = (*mdStore)["listTab"].([]mdListData)[:len((*mdStore)["listTab"].([]mdListData))-1]
						}
					}else if closing == 2 {
						if (*mdStore)["listTab"].([]mdListData)[len((*mdStore)["listTab"].([]mdListData))-1].listType == 0 {
							(*write)([]byte("</ul>"))
						}else{
							(*write)([]byte("</ol>"))
						}
						(*mdStore)["listTab"] = (*mdStore)["listTab"].([]mdListData)[:len((*mdStore)["listTab"].([]mdListData))-1]
					}

					return true
				}

				buf, err = reader.Peek(1)
			}

			//todo: handle tables


			//todo: handle blockquotes
			if buf[0] == '>' {
				*firstChar = false

				if (*mdStore)["inBlockquote"] == nil || (*mdStore)["inBlockquote"] == 0 {
					(*mdStore)["inBlockquote"] = 1
					(*write)([]byte("<blockquote>"))
				}

				reader.Discard(1)
				buf, err = reader.Peek(1)
				if buf[0] == ' ' {
					reader.Discard(1)
					buf, err = reader.Peek(1)
				}

				ind := uint(0)
				for err == nil && buf[0] != '\n' {
					ind++
					buf, err = reader.Get(ind, 1)
				}
				ind++
				buf, err = reader.Get(ind, 1)

				for regex.Comp(`^[ \t]`).MatchRef(&buf) {
					ind++
					buf, err = reader.Get(ind, 1)
				}
				
				if buf[0] != '>' {
					(*mdStore)["inBlockquote"] = 2
				}

				return true
			}
		}

		*firstChar = false

		if buf[0] == '*' || buf[0] == '_' || buf[0] == '~' || buf[0] == '-' {
			firstByte := buf[0]
			
			ind := uint(1)
			level := []byte{buf[0]}
			buf, err = reader.Get(ind, 1)
			for err == nil && (buf[0] == '*' || buf[0] == '_' || buf[0] == '~' || buf[0] == '-') {
				level = append(level, buf[0])
				ind++
				buf, err = reader.Get(ind, 1)
			}

			if firstByte == '*' || len(level) > 1 {
				buf, err = reader.Get(ind, 1)
				if err == nil && buf[0] == ' ' {
					ind++
					buf, err = reader.Get(ind, 1)
				}
	
				levelEnd := level
				cont := []byte{}
				for err == nil && buf[0] != '\n' {
					if len(levelEnd) == 0 {
						break
					}else if buf[0] == levelEnd[len(levelEnd)-1] {
						levelEnd = levelEnd[:len(levelEnd)-1]
					}
	
					cont = append(cont, buf[0])
					ind++
					buf, err = reader.Get(ind, 1)
				}
	
				(*write)(levelEnd)
				reader.Discard(uint(len(levelEnd)))
	
				if len(levelEnd) != len(level) {
					level = level[len(levelEnd):]
	
					(*write)(mdHandleFonts(append(level, cont...)))
	
					reader.Discard(uint(len(cont) + len(level)))
					return true
				}
				return false
			}else{
				buf, err = reader.Peek(1)
			}
		}

		if buf[0] == '`' {
			buf, err := reader.Peek(3)
			if err == nil && buf[1] == '`' && buf[2] == '`' {
				reader.Discard(3)

				buf, err = reader.Peek(1)
				lang := []byte{}
				for err == nil && regex.Comp(`^[\w_\- ]`).MatchRef(&buf) {
					lang = append(lang, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(1)
				}

				cont := []byte{}
				buf, err = reader.Peek(3)
				for err == nil && !(buf[0] == '`' && buf[1] == '`' && buf[2] == '`') {
					cont = append(cont, buf[0])
					reader.Discard(1)
					buf, err = reader.Peek(3)
				}

				if err == nil {
					reader.Discard(3)
				}

				if len(lang) == 0 {
					(*write)(regex.JoinBytes([]byte("<pre>"), cont, []byte("</pre>")), true)
				}else{
					(*write)(regex.JoinBytes([]byte("<code lang=\""), lang, []byte("\">"), cont, []byte("</code>")), true)
				}

				return true
			}else{
				ind := uint(1)
				buf, err = reader.Get(ind, 1)
				cont := []byte{}
				for err == nil && !(buf[0] == '`' || buf[0] == '\n') {
					cont = append(cont, buf[0])
					ind++
					buf, err = reader.Get(ind, 1)
				}

				if err == nil && buf[0] == '`' {
					(*write)(regex.JoinBytes([]byte("<pre>"), cont, []byte("</pre>")))

					reader.Discard(ind+1)
					return true
				}
			}

			return false
		}

		ind := uint(0)
		isEmbed := false
		if buf[0] == '!' {
			isEmbed = true

			ind++
			buf, err = reader.Get(ind, 1)
			if err != nil {
				return false
			}
		}

		if buf[0] == '[' {
			data1 := []byte{}
			innerLink := uint(0)
			ind++
			buf, err = reader.Get(ind, 1)
			for err == nil && (buf[0] != ']' || innerLink != 0) {
				if buf[0] == '\n' {
					break
				}

				data1 = append(data1, buf[0])

				// handle strings
				if buf[0] == '"' || buf[0] == '\'' || buf[0] == '`' {
					q := buf[0]
					ind++
					buf, err = reader.Get(ind, 1)
					for err == nil && buf[0] != q {
						data1 = append(data1, buf[0])
						if buf[0] == '\\' {
							ind++
							buf, err = reader.Get(ind, 1)
							data1 = append(data1, buf[0])
						}
						ind++
						buf, err = reader.Get(ind, 1)
					}

					data1 = append(data1, q)
				}else if buf[0] == '[' {
					innerLink++
				}else if innerLink != 0 && buf[0] == ']' {
					innerLink--
				}

				ind++
				buf, err = reader.Get(ind, 1)
			}

			if err != nil || buf[0] != ']' {
				if isEmbed {
					(*write)([]byte{'!', '['})
					reader.Discard(2)
					return true
				}
				return false
			}

			ind++
			buf, err = reader.Get(ind, 1)
			
			var data2 []byte = nil
			if buf[0] == '(' {
				back := ind
				data2 = []byte{}
				ind++
				buf, err = reader.Get(ind, 1)
				for err == nil && buf[0] != ')' {
					if buf[0] == '\n' {
						break
					}
	
					data2 = append(data2, buf[0])
	
					// handle strings
					if buf[0] == '"' || buf[0] == '\'' || buf[0] == '`' {
						q := buf[0]
						ind++
						buf, err = reader.Get(ind, 1)
						for err == nil && buf[0] != q {
							data2 = append(data2, buf[0])
							if buf[0] == '\\' {
								ind++
								buf, err = reader.Get(ind, 1)
								data2 = append(data2, buf[0])
							}
							ind++
							buf, err = reader.Get(ind, 1)
						}

						data2 = append(data2, q)
					}
	
					ind++
					buf, err = reader.Get(ind, 1)
				}

				if err != nil || buf[0] != ')' {
					data2 = nil
					ind = back
					buf, err = reader.Get(ind, 1)
				}else{
					ind++
					buf, err = reader.Get(ind, 1)
				}
			}

			var htmlArgs []byte = nil

			buf, err = reader.Get(ind, 3)
			if buf[0] == '{' && buf[1] != '{' && !(buf[1] == '\\' && buf[2] == '{') {
				back := ind
				args := map[string][]byte{}
				css := map[string][]byte{}
				argKeys := []string{}
				cssKeys := []string{}

				ind++
				buf, err = reader.Get(ind, 1)

				nextArg := []byte{}
				key := ""
				argMode := uint8(0)
				argInd := 0
				for err == nil && buf[0] != '}' {
					if argMode == 0 && buf[0] == '=' {
						key = string(nextArg)
						nextArg = []byte{}
						argMode = 1

						if key == "" {
							key = strconv.Itoa(argInd)
							argInd++
						}

						ind++
						buf, err = reader.Get(ind, 1)
						continue
					}else if argMode == 0 && buf[0] == ':' {
						key = string(nextArg)
						nextArg = []byte{}
						argMode = 2

						if key == "" {
							key = strconv.Itoa(argInd)
							argInd++
						}

						ind++
						buf, err = reader.Get(ind, 1)

						for err == nil && regex.Comp(`^[ \t]`).MatchRef(&buf) {
							ind++
							buf, err = reader.Get(ind, 1)
						}
						continue
					}else if argMode == 0 && regex.Comp(`^[ \t]`).MatchRef(&buf) {
						key = strconv.Itoa(argInd)
						argInd++

						args[key] = nextArg
						argKeys = append(argKeys, key)
						key = ""
						nextArg = []byte{}
						argMode = 0
						
						ind++
						buf, err = reader.Get(ind, 1)
						continue
					}else if argMode == 1 && regex.Comp(`^[\s;]`).MatchRef(&buf) {
						args[key] = nextArg
						argKeys = append(argKeys, key)
						key = ""
						nextArg = []byte{}
						argMode = 0

						ind++
						buf, err = reader.Get(ind, 1)
						continue
					}else if argMode == 2 && buf[0] == ';' {
						css[key] = nextArg
						cssKeys = append(cssKeys, key)
						key = ""
						nextArg = []byte{}
						argMode = 0

						ind++
						buf, err = reader.Get(ind, 1)
						continue
					}else if argMode == 2 && regex.Comp(`^[\r\n]`).MatchRef(&buf) {
						if buf[0] != '\r' {
							nextArg = append(nextArg, ' ')
						}
						ind++
						buf, err = reader.Get(ind, 1)
						continue
					}

					nextArg = append(nextArg, buf[0])
	
					// handle strings
					if buf[0] == '"' || buf[0] == '\'' || buf[0] == '`' {
						q := buf[0]
						ind++
						buf, err = reader.Get(ind, 1)
						for err == nil && buf[0] != q {
							if argMode == 2 && regex.Comp(`^[\r\n]`).MatchRef(&buf) {
								if buf[0] != '\r' {
									nextArg = append(nextArg, '\\', 'n')
								}
								ind++
								buf, err = reader.Get(ind, 1)
								continue
							}

							nextArg = append(nextArg, buf[0])
							if buf[0] == '\\' {
								ind++
								buf, err = reader.Get(ind, 1)
								nextArg = append(nextArg, buf[0])
							}
							ind++
							buf, err = reader.Get(ind, 1)
						}

						nextArg = append(nextArg, q)
					}
	
					ind++
					buf, err = reader.Get(ind, 1)
				}

				if err != nil || buf[0] != '}' {
					ind = back
					buf, err = reader.Get(ind, 1)
				}else{
					ind++
					buf, err = reader.Get(ind, 1)

					if len(nextArg) != 0 {
						if argMode == 0 {
							args[strconv.Itoa(argInd)] = nextArg
						}else if argMode == 1 {
							args[key] = nextArg
							argKeys = append(argKeys, key)
						}else if argMode == 2 {
							css[key] = nextArg
							cssKeys = append(cssKeys, key)
						}
					}

					// sort css and args
					if len(cssKeys) != 0 {
						if v, ok := args["style"]; !ok || v == nil {
							args["style"] = []byte{}
							argKeys = append(argKeys, "style")
						}else if args["style"][len(args["style"])-1] != ';' {
							args["style"] = append(args["style"], ';')
						}
						args["style"] = regex.Comp(`^(["'\'])(.*)\1$`).RepStrComp(args["style"], []byte("$2"))
	
						sortStrings(&cssKeys)
						for _, key := range cssKeys {
							args["style"] = append(args["style"], regex.JoinBytes(key, ':', css[key], ';')...)
						}
	
						args["style"] = regex.JoinBytes('"', goutil.HTML.EscapeArgs(args["style"], '"'), '"')
					}

					htmlArgs = []byte{}
					sortStrings(&argKeys)
					for i, key := range argKeys {
						if i != 0 {
							htmlArgs = append(htmlArgs, ' ')
						}

						if regex.Comp(`^[0-9]+$`).Match([]byte(key)) {
							htmlArgs = append(htmlArgs, regex.JoinBytes(args[key])...)
						}else{
							htmlArgs = append(htmlArgs, regex.JoinBytes(key, '=', args[key])...)
						}
					}

					htmlArgs = append([]byte{' '}, bytes.TrimLeft(htmlArgs, " ")...)
				}
			}

			// [data1](data2){htmlArgs}
			if data1 != nil && data2 != nil {
				if reLinkMD.MatchRef(&data1) {
					data1 = reLinkMD.RepFuncRef(&data1, func(data func(int) []byte) []byte {
						d1 := data(2)
						d2 := data(3)
						if len(data(1)) != 0 {
							return mdHandleEmbed(&d1, &d2, nil)
						}else{
							return mdHandleLink(&d1, &d2, nil)
						}
					})
				}

				if isEmbed {
					(*write)(mdHandleEmbed(&data1, &data2, &htmlArgs))
				}else{
					(*write)(mdHandleLink(&data1, &data2, &htmlArgs))
				}
			}else if data1 != nil {
				(*write)(mdHandleInput(&data1, &htmlArgs))
			}

			reader.Discard(ind)
			return true
		}

		if isEmbed {
			(*write)([]byte{'!'})
			reader.Discard(1)
			buf, err = reader.Peek(1)
			return true
		}
	}

	// return true to continue if markdown found
	// return false to allow precompiler to handle as default
	return false
}

// markdownCompilerNextLine runs when the main compiler finds a line break
//
// note: this method only runs if this is not already set to the firstChar, but is transitioning to the firstChar
func compileMarkdownNextLine(reader *liveread.Reader[uint8], write *func(b []byte, raw ...bool), firstChar *bool, spaces *uint, mdStore *map[string]interface{}){
	if (*mdStore)["inBlockquote"] == 2 {
		(*mdStore)["inBlockquote"] = 0
		(*write)([]byte("</blockquote>"))
	}
}


func mdHandleLink(name *[]byte, url *[]byte, htmlArgs *[]byte) []byte {
	return regex.JoinBytes([]byte("<a href=\""), goutil.HTML.EscapeArgs(*url, '"'), '"', *htmlArgs, '>', *name, []byte("</a>"))
}

func mdHandleEmbed(embedType *[]byte, url *[]byte, htmlArgs *[]byte) []byte {
	//todo: handle embed

	return []byte{}
}

func mdHandleInput(data *[]byte, htmlArgs *[]byte) []byte {
	//todo: handle form input

	return []byte{}
}

func mdHandleFonts(data []byte) []byte {
	data = regex.Comp(`(\*{1,3})(.*?)\1`).RepFuncRef(&data, func(data func(int) []byte) []byte {
		if len(data(1)) == 3 {
			return regex.JoinBytes([]byte("<strong><em>"), data(2), []byte("</em></strong>"))
		}else if len(data(1)) == 2 {
			return regex.JoinBytes([]byte("<strong>"), data(2), []byte("</strong>"))
		}else if len(data(1)) == 1 {
			return regex.JoinBytes([]byte("<em>"), data(2), []byte("</em>"))
		}

		return data(0)
	})

	data = regex.Comp(`(__)(.*?)\1`).RepFuncRef(&data, func(data func(int) []byte) []byte {
		return regex.JoinBytes([]byte("<u>"), data(2), []byte("</u>"))
	})

	data = regex.Comp(`(--|~~)(.*?)\1`).RepFuncRef(&data, func(data func(int) []byte) []byte {
		if data(1)[0] == '-' {
			return regex.JoinBytes([]byte("<del>"), data(2), []byte("</del>"))
		}
		return regex.JoinBytes([]byte("<s>"), data(2), []byte("</s>"))
	})

	return data
}
