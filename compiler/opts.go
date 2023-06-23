package compiler

import (
	"bytes"
	"reflect"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
)

// GetOpt handles grabbing and escaping an option (or eachArgs as needed)
//
// escape: 0 = raw, 1 = raw arg, 2 = html, 3 = arg, 4 = html arg key
func GetOpt(name []byte, opts *map[string]interface{}, eachArgs *[]EachArgs, escape uint8, precomp bool, stringsOnly bool) interface{} {
	regWord := `(?:[\w_\-$]+|'(?:\\[\\']|[^'])*'|"(?:\\[\\"]|[^"])*"|\'(?:\\[\\\']|[^\'])*\')+`
	nameVars := regex.Comp(`((?:` + regWord + `|\.` + regWord + `|\[` + regWord + `\])+)`).SplitRef(&name)

	varList := [][]byte{}
	newVar := []byte{}
	for _, v := range nameVars {
		if len(v) == 1 && v[0] == '|' && len(newVar) != 0 {
			varList = append(varList, newVar)
			newVar = []byte{}
		} else if len(v) != 0 {
			newVar = append(newVar, v...)
		}
	}
	if len(newVar) != 0 {
		varList = append(varList, newVar)
	}
	newVar = nil
	nameVars = nil

	// a list of vars to move to the compiler (this list should be joined with '|')
	var varComp [][]byte
	if precomp {
		varComp = [][]byte{}
	}

	for _, varName := range varList {
		// handle strings
		if len(varName) >= 2 && ((varName[0] == '\'' && varName[len(varName)-1] == '\'') || (varName[0] == '"' && varName[len(varName)-1] == '"') || (varName[0] == '`' && varName[len(varName)-1] == '`')) {
			return regex.Comp(`\\([\\'"\'])`).RepStrComp(varName[1:len(varName)-1], []byte("$1"))
		}

		// handle basic var names
		if regex.Comp(`^[\w_\-$]+$`).MatchRef(&varName) {
			if hasVarOpt(varName, opts, eachArgs, escape, precomp) {
				val := getVarOpt(varName, opts, eachArgs, escape, precomp)

				if goutil.IsZeroOfUnderlyingType(val) {
					return nil
				}

				if stringsOnly && val != nil {
					return escapeVarVal(goutil.Conv.ToBytes(val), escape)
				}

				return escapeVarVal(val, escape)
			}
			if precomp {
				varComp = append(varComp, varName)
			}
			continue
		}

		objNameList := regex.Comp(`(\[` + regWord + `\])|\.(` + regWord + `|)`).SplitRef(&varName)

		objList := [][]byte{}
		for _, v := range objNameList {
			if len(v) != 0 {
				objList = append(objList, v)
			}
		}

		if len(objList) == 0 {
			continue
		}

		var val interface{}

		if !hasVarOpt(objList[0], opts, eachArgs, escape, precomp) {
			if precomp {
				varComp = append(varComp, varName)
			}
			continue
		}

		val = getVarOpt(objList[0], opts, eachArgs, escape, precomp)

		endLoop := false
		for i := 1; i < len(objList); i++ {
			t := reflect.TypeOf(val)
			if t != goutil.VarType["map[string]interface{}"] && t != goutil.VarType["[]interface{}"] {
				endLoop = true
				break
			}

			n := objList[i]
			if len(n) >= 2 && n[0] == '[' && n[len(n)-1] == ']' {
				n = n[1 : len(n)-1]
				if len(n) >= 2 && ((n[0] == '\'' && n[len(n)-1] == '\'') || (n[0] == '"' && n[len(n)-1] == '"') || (n[0] == '`' && n[len(n)-1] == '`')) {
					n = regex.Comp(`\\([\\'"\'])`).RepStrComp(n[1:len(n)-1], []byte("$1"))
				} else {
					if !hasVarOpt(n, opts, eachArgs, escape, precomp) {
						varComp = append(varComp, varName)
						endLoop = true
						break
					}
					n = goutil.Conv.ToBytes(escapeVarVal(getVarOpt(n, opts, eachArgs, escape, precomp), escape))
				}
			}

			if len(n) == 0 {
				endLoop = true
				break
			}

			if t == goutil.VarType["map[string]interface{}"] {
				if v, ok := val.(map[string]interface{})[string(n)]; ok {
					val = v
				} else if v, ok := val.(map[string]interface{})["$"+string(n)]; ok && n[0] != '$' {
					val = v
				} else {
					endLoop = true
					break
				}
			} else if t == goutil.VarType["[]interface{}"] {
				nI := goutil.Conv.ToInt(n)
				if len(val.([]interface{})) > nI {
					val = val.([]interface{})[nI]
				} else {
					endLoop = true
					break
				}
			}
		}
		if endLoop {
			continue
		}

		if goutil.IsZeroOfUnderlyingType(val) {
			return nil
		}

		if stringsOnly && val != nil {
			return escapeVarVal(goutil.Conv.ToBytes(val), escape)
		}

		return escapeVarVal(val, escape)
	}

	if precomp && len(varComp) != 0 {
		return getVarStr(bytes.Join(varComp, []byte{'|'}), escape)
	}

	return nil
}

// getEachArg returns a value from an eachArg if it exists
//
// returns nil, if no matching args are found (this is when you should check the `opts` list for the `name` arg)
//
// returns []byte{0} if the found arg should be passed to the compiler
func getEachArg(name []byte, eachArgs *[]EachArgs) interface{} {
	if len(name) == 0 {
		return nil
	}
	if name[0] == '$' {
		name = name[1:]
		if len(name) == 0 {
			return nil
		}
	}

	nameConst := append([]byte{'$'}, name...)

	for i := len(*eachArgs) - 1; i >= 0; i-- {
		if bytes.Equal(name, (*eachArgs)[i].key) || bytes.Equal(nameConst, (*eachArgs)[i].key) {
			if (*eachArgs)[i].passToComp {
				return []byte{0}
			} else if (*eachArgs)[i].listMap != nil {
				return (*eachArgs)[i].listArr[(*eachArgs)[i].ind]
			} else {
				return (*eachArgs)[i].ind
			}
		} else if bytes.Equal(name, (*eachArgs)[i].val) || bytes.Equal(nameConst, (*eachArgs)[i].val) {
			if (*eachArgs)[i].passToComp {
				return []byte{0}
			} else if (*eachArgs)[i].listMap != nil {
				key := goutil.Conv.ToString((*eachArgs)[i].listArr[(*eachArgs)[i].ind])
				return (*eachArgs)[i].listMap[key]
			} else {
				return (*eachArgs)[i].listArr[(*eachArgs)[i].ind]
			}
		}
	}

	return nil
}

func hasVarOpt(name []byte, opts *map[string]interface{}, eachArgs *[]EachArgs, escape uint8, precomp bool) bool {
	if len(name) == 0 {
		return false
	}

	if v := getEachArg(name, eachArgs); v != nil {
		return true
	}

	if precomp && name[0] != '$' {
		name = append([]byte{'$'}, name...)
	}

	if (*opts)[string(name)] == nil && !precomp {
		return (*opts)["$"+string(name)] != nil
	}

	return (*opts)[string(name)] != nil
}

func getVarOpt(name []byte, opts *map[string]interface{}, eachArgs *[]EachArgs, escape uint8, precomp bool) interface{} {
	if len(name) == 0 {
		return nil
	}

	if v := getEachArg(name, eachArgs); v != nil {
		if reflect.TypeOf(v) == goutil.VarType["[]byte"] && v.([]byte)[0] == 0 {
			return getVarStr(name, escape)
		}

		return goutil.Clean.JSON(v)
	}

	checkName := name
	if precomp && name[0] != '$' {
		checkName = append([]byte{'$'}, name...)
	}

	if (*opts)[string(checkName)] == nil && !precomp {
		checkName = append([]byte{'$'}, name...)
	}

	if (*opts)[string(checkName)] == nil {
		return getVarStr(name, escape)
	}

	return goutil.Clean.JSON((*opts)[string(checkName)])
}

func getVarStr(name []byte, escape uint8) []byte {
	if bytes.HasPrefix(name, []byte{'$'}) {
		return nil
	} else if escape == 0 {
		// pass with fist byte as 0 to authorize passing a var
		return regex.JoinBytes([]byte{0}, []byte("{{{"), name, []byte("}}}"))
	} else if escape == 1 {
		return regex.JoinBytes([]byte{0}, []byte("{{{="), name, []byte("}}}"))
	} else if escape == 2 {
		return regex.JoinBytes([]byte{0}, []byte("{{"), name, []byte("}}"))
	} else if escape == 3 {
		return regex.JoinBytes([]byte{0}, []byte("{{="), name, []byte("}}"))
	} else if escape == 4 {
		return regex.JoinBytes([]byte{0}, []byte("{{:"), name, []byte("}}"))
	}

	return nil
}

func escapeVarVal(val interface{}, escape uint8) interface{} {
	if escape == 0 || escape == 1 {
		return val
	} else if escape == 2 {
		return goutil.HTML.Escape(toBytesOrJson(val))
	} else if escape == 3 {
		valB := goutil.HTML.EscapeArgs(toBytesOrJson(val), '"')

		// prevent xss injection
		if regex.Comp(`(?i)(\b)(on\S+)(\s*)=|(javascript|data|vbscript):|(<\s*)(\/*)script|style(\s*)=|(<\s*)meta|\*(.*?)[\r\n]*(.*?)\*`).MatchRef(&valB) {
			return []byte("{{#warning: xss injection was detected}}")
		}

		return valB
	} else if escape == 4 {
		return regex.Comp(`[^\w_-]+`).RepStr(toBytesOrJson(val), []byte{})
	}

	return nil
}

func toBytesOrJson(val interface{}) []byte {
	t := reflect.TypeOf(val)
	if t == goutil.VarType["map[string]interface{}"] || t == goutil.VarType["[]interface{}"] {
		if json, err := goutil.JSON.Stringify(val, 2); err == nil {
			return json
		}
	} else {
		return goutil.Conv.ToBytes(val)
	}

	return []byte{}
}
