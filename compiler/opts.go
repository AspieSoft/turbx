package compiler

import (
	"bytes"
	"fmt"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
)

func GetOpt(name []byte, opts *map[string]interface{}, escape uint8, precomp bool, stringsOnly bool) interface{} {
	// escape: 0 = raw, 1 = raw arg, 2 = html, 3 = arg, 4 = html arg key

	//todo: handle & | operators and .obj[key] objects
	// fmt.Println(escape)
	// fmt.Println(string(name))

	regWord := `(?:[\w_\-$]+|'(?:\\[\\']|[^'])*'|"(?:\\[\\"]|[^"])*"|\'(?:\\[\\\']|[^\'])*\')+`
	nameVars := regex.Comp(`((?:`+regWord+`|\.`+regWord+`|\[`+regWord+`\])+)`).SplitRef(&name)

	varList := [][]byte{}
	newVar := []byte{}
	for _, v := range nameVars {
		if len(v) == 1 && v[0] == '|' && len(newVar) != 0 {
			varList = append(varList, newVar)
			newVar = []byte{}
		}else if len(v) != 0 {
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
			if hasVarOpt(varName, opts, escape, precomp) {
				val := getVarOpt(varName, opts, escape, precomp)

				if goutil.IsZeroOfUnderlyingType(val) {
					return nil
				}

				if stringsOnly && val != nil {
					return goutil.Conv.ToBytes(val)
				}

				return val
			}
			if precomp {
				varComp = append(varComp, varName)
			}
			continue
		}

		//todo: handle complex var objects
		// example: .obj[key]
		// No recursive function needed ([key] only accepts basic var names and strings)
		// note: for precomp, if the base var exists, and its key doesn't, it should not be added to the varComp method, otherwise it should be added if the base var does not exist
		// may also add to varComp if a [key] doesn't exist even when nested

		fmt.Println(string(varName))

		objNameList := regex.Comp(`(\[`+regWord+`\])|\.(`+regWord+`|)`).SplitRef(&varName)

		objList := [][]byte{}
		for _, v := range objNameList {
			if len(v) != 0 {
				objList = append(objList, v)
			}
		}

		for i, v := range objList {
			fmt.Println(i, ":", string(v))
		}
		fmt.Println("")
	}

	if precomp && len(varComp) != 0 {
		return getVarStr(bytes.Join(varComp, []byte{'|'}), escape)
	}

	return nil
}

func hasVarOpt(name []byte, opts *map[string]interface{}, escape uint8, precomp bool) bool {
	if len(name) == 0 {
		return false
	}

	if precomp && name[0] != '$' {
		name = append([]byte{'$'}, name...)
	}

	if (*opts)[string(name)] == nil && !precomp {
		return (*opts)["$"+string(name)] != nil
	}

	return (*opts)[string(name)] != nil
}

func getVarOpt(name []byte, opts *map[string]interface{}, escape uint8, precomp bool) interface{} {
	if len(name) == 0 {
		return nil
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

	val := goutil.Clean.JSON((*opts)[string(checkName)])
	if escape == 0 || escape == 1 {
		return val
	}else if escape == 2 {
		return goutil.HTML.Escape(goutil.Conv.ToBytes(val))
	}else if escape == 3 {
		//todo: sanitize arg from xss attacks (example: remove 'data:' from val)
		return goutil.HTML.EscapeArgs(goutil.Conv.ToBytes(val))
	}else if escape == 4 {
		return regex.Comp(`[^\w_-]+`).RepStr(goutil.Conv.ToBytes(val), []byte{})
	}

	return nil
}

func getVarStr(name []byte, escape uint8) []byte {
	if bytes.HasPrefix(name, []byte{'$'}) {
		return nil
	}else if escape == 0 {
		// pass with fist byte as 0 to authorize passing a var
		return regex.JoinBytes([]byte{0}, []byte("{{{"), name, []byte("}}}"))
	}else if escape == 1 {
		return regex.JoinBytes([]byte{0}, []byte("{{{="), name, []byte("}}}"))
	}else if escape == 2 {
		return regex.JoinBytes([]byte{0}, []byte("{{"), name, []byte("}}"))
	}else if escape == 3 {
		return regex.JoinBytes([]byte{0}, []byte("{{="), name, []byte("}}"))
	}else if escape == 4 {
		return regex.JoinBytes([]byte{0}, []byte("{{:"), name, []byte("}}"))
	}

	return nil
}
