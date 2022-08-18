package main

import (
	"bytes"
	"compiler/common"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex"
	lorem "github.com/drhodes/golorem"
)

var preTagFuncs map[string]interface{} = map[string]interface{} {
	"lorem": func(args map[string][]byte, level int, file fileData) interface{} {

		wType := byte('p')
		if len(args["type"]) != 0 {
			wType = args["type"][0]
		}else if len(args["0"]) != 0 && !regex.Match(args["0"], `^[0-9]+$`) {
			wType = args["0"][0]
		}else if len(args["1"]) != 0 && !regex.Match(args["1"], `^[0-9]+$`) {
			wType = args["1"][0]
		}else if len(args["2"]) != 0 && !regex.Match(args["2"], `^[0-9]+$`) {
			wType = args["2"][0]
		}
		
		minLen := 1
		maxLen := 10
		used := -1

		if len(args["min"]) != 0 {
			used = -2
			i, err := strconv.Atoi(string(args["min"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["0"]) != 0 && regex.Match(args["0"], `^[0-9]+$`) {
			used = 0
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["1"]) != 0 && regex.Match(args["1"], `^[0-9]+$`) {
			used = 1
			i, err := strconv.Atoi(string(args["1"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["2"]) != 0 && regex.Match(args["2"], `^[0-9]+$`) {
			used = 2
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}

		if len(args["max"]) != 0 {
			i, err := strconv.Atoi(string(args["max"]))
			if err == nil {
				minLen = i
			}
		}else if used != 0 && len(args["0"]) != 0 && regex.Match(args["0"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if used != 1 && len(args["1"]) != 0 && regex.Match(args["1"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["1"]))
			if err == nil {
				minLen = i
			}
		}else if used != 2 && len(args["2"]) != 0 && regex.Match(args["2"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if used != -1 {
			maxLen = minLen
		}

		if wType == 'p' {
			return []byte(lorem.Paragraph(minLen, maxLen))
		} else if wType == 'w' {
			return []byte(lorem.Word(minLen, maxLen))
		} else if wType == 's' {
			return []byte(lorem.Sentence(minLen, maxLen))
		} else if wType == 'h' {
			return []byte(lorem.Host())
		} else if wType == 'e' {
			return []byte(lorem.Email())
		} else if wType == 'u' {
			return []byte(lorem.Url())
		}

		return []byte(lorem.Paragraph(minLen, maxLen))
	},
}

var tagFuncs map[string]interface{} = map[string]interface{} {
	"if": tagFuncIf,

	"each": func(args map[string][]byte, cont []byte, opts map[string]interface{}, level int, file fileData) interface{} {
		//todo: fix each function outputing the same var

		if len(args) == 0 {
			return []byte{}
		}

		var argObj string = ""
		var argAs []byte = nil
		var argOf []byte = nil
		var argIn []byte = nil
		argType := 0

		for i, v := range args {
			if i == "0" || i == "value" {
				argObj = string(v)
				continue
			}

			if i == "as" {
				argAs = v
				continue
			}else if i == "of" {
				argOf = v
				continue
			}else if i == "in" {
				argIn = v
				continue
			}

			if bytes.Equal(v, []byte("as")) {
				argType = 1
				continue
			}else if bytes.Equal(v, []byte("of")) {
				argType = 2
				continue
			}else if bytes.Equal(v, []byte("in")) {
				argType = 3
				continue
			}

			if argType == 1 {
				argAs = v
				argType = 0
				continue
			}else if argType == 2 {
				argOf = v
				argType = 0
				continue
			}else if argType == 3 {
				argIn = v
				argType = 0
				continue
			}

			if argAs == nil {
				argAs = v
			}else if argOf == nil {
				argOf = v
			}else if argIn == nil {
				argIn = v
			}
		}

		obj := getOpt(opts, argObj, false)
		res := []eachFnObj{}

		objType := reflect.TypeOf(obj)
		if objType != common.VarType["map"] && objType != common.VarType["array"] {
			return []byte{}
		}

		if objType == common.VarType["map"] {
			n := 0
			for i, v := range obj.(map[string]interface{}) {
				opt := opts
				if argAs != nil {
					opt[string(argAs)] = v
				}else{
					opt[argObj] = v
				}
				if argOf != nil {
					opt[string(argOf)] = i
				}
				if argIn != nil {
					opt[string(argIn)] = n
				}
				res = append(res, eachFnObj{html: cont, opts: opt})
				n++
			}
		}else if objType == common.VarType["array"] {
			n := 0
			for i, v := range obj.([]interface{}) {
				opt := opts
				if argAs != nil {
					opt[string(argAs)] = v
				}else{
					opt[argObj] = v
				}
				if argOf != nil {
					opt[string(argOf)] = i
				}
				if argIn != nil {
					opt[string(argIn)] = n
				}
				res = append(res, eachFnObj{html: cont, opts: opt})
				n++
			}
		}

		return res
	},

	"json": func(args map[string][]byte, cont []byte, opts map[string]interface{}, level int, file fileData) interface{} {
		var json interface{} = nil
		if val, ok := args["0"]; ok {
			json = getOpt(opts, string(val), false)
		}else{
			return []byte{}
		}

		var spaces int = 0
		if val, ok := args["indent"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}else if val, ok := args["1"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}

		var prefix int = 0
		if val, ok := args["prefix"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}else if val, ok := args["2"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				prefix = 0
			}else{
				prefix = sp
			}
		}

		res, err := common.StringifyJSONSpaces(json, spaces, prefix)
		if err != nil {
			return []byte{}
		}

		return res
	},
}

func tagFuncIf(args map[string][]byte, cont []byte, opts map[string]interface{}, level int, file fileData, pre bool) (interface{}, bool) {
	isTrue := false
	lastArg := []byte{}

	if len(args) == 0 {
		return cont, false
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

			if len(arg1) == 0 {
				arg1 = arg
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
				if pre && arg1Any == nil {
					return nil, true
				}
			}

			isTrue = common.IsZeroOfUnderlyingType(arg1Any)
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
			if pre && arg1Any == nil {
				return nil, true
			}

			if reflect.TypeOf(arg1Any) == common.VarType["string"] {
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
			if pre && arg2Any == nil {
				return nil, true
			}

			if reflect.TypeOf(arg2Any) == common.VarType["string"] {
				if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
					arg2Any = arg2N
				}
			}
		}

		lastArg = arg1

		arg1Type := reflect.TypeOf(arg1Any)
		if arg1Type == common.VarType["int"] {
			arg1Any = float64(arg1Any.(int))
		}else if arg1Type == common.VarType["float32"] {
			arg1Any = float64(arg1Any.(float32))
		}else if arg1Type == common.VarType["int32"] {
			arg1Any = float64(arg1Any.(int32))
		}else if arg1Type == common.VarType["byteArray"] {
			arg1Any = string(arg1Any.([]byte))
		}else if arg1Type == common.VarType["byte"] {
			arg1Any = string(arg1Any.(byte))
		}

		arg2Type := reflect.TypeOf(arg2Any)
		if arg2Type == common.VarType["int"] {
			arg2Any = float64(arg2Any.(int))
		}else if arg2Type == common.VarType["float32"] {
			arg2Any = float64(arg2Any.(float32))
		}else if arg2Type == common.VarType["int32"] {
			arg2Any = float64(arg2Any.(int32))
		}else if arg1Type == common.VarType["byteArray"] {
			arg2Any = string(arg2Any.([]byte))
		}else if arg1Type == common.VarType["byte"] {
			arg2Any = string(arg2Any.(byte))
		}

		switch sign {
		case "=":
			isTrue = (arg1Any == arg2Any)
		case "!=":
		case "!":
			isTrue = (arg1Any != arg2Any)
		case ">=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == common.VarType["float64"] {
				isTrue = (arg1Any.(float64) >= arg2Any.(float64))
			}
		case "<=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == common.VarType["float64"] {
				isTrue = (arg1Any.(float64) <= arg2Any.(float64))
			}
		case ">":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == common.VarType["float64"] {
				isTrue = (arg1Any.(float64) > arg2Any.(float64))
			}
		case "<":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == common.VarType["float64"] {
				isTrue = (arg1Any.(float64) < arg2Any.(float64))
			}
		}

		i += 2
	}

	elseOpt := regex.Match(cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`)
	if elseOpt && isTrue {
		return regex.RepStr(cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`, []byte("")), false
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
			return newCont, false
		}

		return tagFuncIf(newArgs, newCont, opts, level, file, pre)
	} else if isTrue {
		return cont, false
	}

	return []byte{}, false
}
