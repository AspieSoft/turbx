package funcs

import (
	"bytes"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex/v3"
	"github.com/AspieSoft/goutil/v3"
)

type Pre struct {}
type Comp struct {}


type keyVal struct {
	key []byte
	val interface{}
}


func convertOpt(arg []byte, opts *map[string]interface{}, pre *bool) (interface{}, bool) {
	if regex.MatchRef(&arg, regex.Compile(`^(["'\'])(.*)\1$`)) {
		arg = regex.RepStrComplexRef(&arg, regex.Compile(`^(["'\'])(.*)\1$`), []byte("$2"))
		
		if bytes.Equal(arg, []byte("true")) {
			return true, true
		}else if bytes.Equal(arg, []byte("false")) {
			return false, true
		}else if bytes.Equal(arg, []byte("nil")) || bytes.Equal(arg, []byte("null")) || bytes.Equal(arg, []byte("undefined")) {
			return nil, true
		}else if v, err := strconv.Atoi(string(arg)); err == nil {
			return v, true
		}else if v, err := strconv.ParseFloat(string(arg), 64); err == nil {
			return v, true
		}

		return []byte(arg), true
	}

	if *pre {
		if arg[0] == '$' {
			if val, ok := (*opts)[string(arg)]; ok {
				return val, true
			}else if val, ok := (*opts)[string(arg[1:])]; ok {
				return val, true
			}
		}else if val, ok := (*opts)["$"+string(arg)]; ok {
			return val, true
		}

		// first param true with a false 2nd param is used to break the loop in the getOpt method that calls this method because a const var did not exist in pre compile mode
		return true, false
	}

	if val, ok := (*opts)[string(arg)]; ok {
		return val, true
	}else if arg[0] == '$' {
		if val, ok := (*opts)[string(arg[1:])]; ok {
			return val, true
		}
	}else{
		if val, ok := (*opts)["$"+string(arg)]; ok {
			return val, true
		}
	}

	return nil, false
}

func getOptObj(arg []byte, opts *map[string]interface{}, pre *bool) (interface{}, bool) {
	args := regex.SplitRef(&arg, regex.Compile(`\.|(\[(?:"(?:\\[\\"]|.)*?"|'(?:\\[\\']|.)*?'|\'(?:\\[\\\']|.)*?\'|.)*?\])`))
	// args := regex.SplitRef(&arg, regex.Compile(`(\[[\w_]+\])|\.`))

	res, ok := convertOpt(args[0], opts, pre)
	if !ok {
		return res, false
	}
	args = args[1:]

	for _, arg := range args {
		if bytes.HasPrefix(arg, []byte{'['}) && bytes.HasSuffix(arg, []byte{']'}) {
			arg = arg[1:len(arg)-1]
			v, ok := getOpt(arg, opts, *pre)
			if !ok {
				return v, false
			}
			arg = goutil.ToByteArray(v)
			if arg == nil || len(arg) == 0 {
				if *pre {
					return true, false
				}
				return nil, false
			}
		}

		rType := reflect.TypeOf(res)
		if rType == goutil.VarType["map"] {
			r := (res.(map[string]interface{}))
			val, ok := convertOpt(arg, &r, pre)
			if !ok {
				return val, false
			}
			res = val
		}else if rType == goutil.VarType["array"] {
			r := map[string]interface{}{}
			for i, v := range res.([]interface{}) {
				r[strconv.Itoa(i)] = v
			}
			val, ok := convertOpt(arg, &r, pre)
			if !ok {
				return val, false
			}
			res = val
		}else if rType == goutil.VarType["byteArray"] || rType == goutil.VarType["string"] {
			if rType == goutil.VarType["string"] {
				res = []byte(res.(string))
			}
			r := map[string]interface{}{}
			for i, v := range res.([]byte) {
				r[strconv.Itoa(i)] = v
			}
			val, ok := convertOpt(arg, &r, pre)
			if !ok {
				return val, false
			}
			res = val
		}else{
			return nil, false
		}
	}

	return res, true
}

func getOpt(arg []byte, opts *map[string]interface{}, pre ...bool) (interface{}, bool) {
	usePre := false
	if len(pre) != 0 {
		usePre = pre[0]
	}

	var key []byte
	arg = regex.RepFuncRef(&arg, regex.Compile(`^{{{?([\w_-]+)=(["'\']|)(.*)\2}}}?$`), func(data func(int) []byte) []byte {
		key = data(1)
		return data(3)
	})

	arg = bytes.TrimLeft(arg, "{")
	arg = bytes.TrimRight(arg, "}")
	// arg = regex.RepStrComplexRef(&arg, regex.Compile(`^{{{?(.*)}}}?$`), []byte("$1"))

	b := []byte{}
	for i := 0; i < len(arg); i++ {
		if arg[i] == '|' {
			if len(b) == 0 {
				continue
			}

			val, ok := getOptObj(b, opts, &usePre)
			if ok {
				if key != nil {
					return keyVal{key, val}, true
				}
				return val, true
			}
			b = []byte{}

			if usePre && val == true {
				break
			}
			continue
		}else if arg[i] == '"' || arg[i] == '\'' || arg[i] == '`' {
			q := arg[i]
			i++
			b = append(b, q)
			for ; i < len(arg); i++ {
				if arg[i] == q {
					b = append(b, q)
					i++
					break
				}else if arg[i] == '\\' {
					if regex.MatchRef(&[]byte{arg[i]}, regex.Compile(`[A-Za-z]`)) {
						b = append(b, arg[i])
					}
					i++
				}

				b = append(b, arg[i])
			}
			continue
		}else if arg[i] == '[' {
			i++
			b = append(b, '[')
			for ; i < len(arg); i++ {
				if arg[i] == ']' {
					b = append(b, ']')
					i++
					break
				}else if arg[i] == '"' || arg[i] == '\'' || arg[i] == '`' {
					q := arg[i]
					i++
					b = append(b, q)
					for ; i < len(arg); i++ {
						if arg[i] == q {
							b = append(b, q)
							i++
							break
						}else if arg[i] == '\\' {
							if regex.MatchRef(&[]byte{arg[i]}, regex.Compile(`[A-Za-z]`)) {
								b = append(b, arg[i])
							}
							i++
						}
		
						b = append(b, arg[i])
					}
					// continue
				}else if arg[i] == '\\' {
					if regex.MatchRef(&[]byte{arg[i]}, regex.Compile(`[A-Za-z]`)) {
						b = append(b, arg[i])
					}
					i++
				}

				b = append(b, arg[i])
			}

			continue
		}

		b = append(b, arg[i])
	}

	if len(b) != 0 {
		if val, ok := getOptObj(b, opts, &usePre); ok {
			if key != nil {
				return keyVal{key, val}, true
			}
			return val, true
		}
	}

	//todo: handle object indexes and nested objects
	// also handle strings and optionally '|' seperators (ensure precompiled methods recognize all values and get disabled for reaching string values and non constant values)
	/* if val, ok := (*opts)[arg]; ok {
		return val, true
	} */
	return nil, false
}



func (t *Pre) If(args *[][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	pass := []bool{true}
	inv := []bool{false}
	mode := []uint8{0}
	grp := 0

	lastArg := []byte{}

	unsolved := [][][]byte{{}}

	for i := 0; i < len(*args); i++ {
		if len((*args)[i]) == 1 {
			if (*args)[i][0] == '^' {
				inv[grp] = !inv[grp]
				continue
			}else if (*args)[i][0] == '&' {
				mode[grp] = 0
				continue
			}else if (*args)[i][0] == '|' {
				mode[grp] = 1
				continue
			}else if (*args)[i][0] == '(' {
				pass = append(pass, true)
				inv = append(inv, false)
				mode = append(mode, 0)
				unsolved = append(unsolved, [][]byte{})
				grp++
				continue
			}else if (*args)[i][0] == ')' {
				if grp == 0 {
					continue
				}

				if !inv[grp] {
					if mode[grp-1] == 0 && !pass[grp] {
						pass[grp-1] = false
					}else if mode[grp-1] == 1 && pass[grp] {
						pass[grp-1] = true
					}
				}else{
					if mode[grp-1] == 0 && pass[grp] {
						pass[grp-1] = false
					}else if mode[grp-1] == 1 && !pass[grp] {
						pass[grp-1] = true
					}
					// inv[grp-1] = false
				}

				pass = pass[:grp]
				mode = mode[:grp]
				// grp--

				// handle the unsolved list

				var modeB []byte
				switch mode[grp-1] {
					case 0:
						modeB = []byte{'&'}
					case 1:
						modeB = []byte{'|'}
				}

				if (!pass[grp-1] && unsolved[grp][0][0] == '&') || (pass[grp-1] && unsolved[grp][0][0] == '|') {
					unsolved[grp] = unsolved[grp][1:]
				}

				if inv[grp-1] {
					unsolved[grp-1] = append(unsolved[grp-1], modeB, []byte{'^', '('})
					inv[grp-1] = false
				}else{
					unsolved[grp-1] = append(unsolved[grp-1], modeB, []byte{'('})
				}

				if len(unsolved[grp][0]) == 1 && unsolved[grp][0][0] == '&' {
					unsolved[grp] = unsolved[grp][1:]
				}

				unsolved[grp-1] = append(unsolved[grp-1], unsolved[grp]...)
				unsolved[grp-1] = append(unsolved[grp-1], []byte{')'})
				unsolved = unsolved[:grp]

				inv = inv[:grp]

				grp--
				continue
			}
		}

		arg1 := (*args)[i]
		var sign uint8
		var arg2 []byte

		hasArg2 := false
		if len(arg1) == 1 {
			if arg1[0] == '=' {
				sign = 0
				hasArg2 = true
			}else if arg1[0] == '!' {
				sign = 1
				hasArg2 = true
			}else if arg1[0] == '<' {
				sign = 2
				hasArg2 = true
			}else if arg1[0] == '>' {
				sign = 3
				hasArg2 = true
			}else if arg1[0] == '~' {
				sign = 6
				hasArg2 = true
			}
		}else if len(arg1) == 2 && arg1[1] == '=' {
			if arg1[0] == '<' {
				sign = 4
				hasArg2 = true
			}else if arg1[0] == '>' {
				sign = 5
				hasArg2 = true
			}
		}

		if hasArg2 {
			arg1 = lastArg
		}else{
			lastArg = arg1

			if len(*args) > i+1 {
				if len((*args)[i+1]) == 1 {
					if (*args)[i+1][0] == '=' {
						sign = 0
						hasArg2 = true
					}else if (*args)[i+1][0] == '!' {
						sign = 1
						hasArg2 = true
					}else if (*args)[i+1][0] == '<' {
						sign = 2
						hasArg2 = true
					}else if (*args)[i+1][0] == '>' {
						sign = 3
						hasArg2 = true
					}else if (*args)[i+1][0] == '~' {
						sign = 6
						hasArg2 = true
					}
				}else if len((*args)[i+1]) == 2 && (*args)[i+1][1] == '=' {
					if (*args)[i+1][0] == '<' {
						sign = 4
						hasArg2 = true
					}else if (*args)[i+1][0] == '>' {
						sign = 5
						hasArg2 = true
					}
				}
			}
			// i++
		}

		if hasArg2 && len(*args) > i+2 {
			arg2 = (*args)[i+2]
			// i++
			i += 2
		}


		// make '$' unique to const vars for pre compile to handle
		// ignore in regular compiler
		if !hasArg2 {
			arg1Val, arg1ok := getOpt(arg1, opts, true)

			if !arg1ok {
				// add to unsolved list
				var modeB []byte
				switch mode[grp] {
					case 0:
						modeB = []byte{'&'}
					case 1:
						modeB = []byte{'|'}
				}

				if inv[grp] {
					unsolved[grp] = append(unsolved[grp], modeB, []byte{'^'}, arg1)
					inv[grp] = false
				}else{
					unsolved[grp] = append(unsolved[grp], modeB, arg1)
				}

				continue
			}

			if (!inv[grp] && !goutil.IsZeroOfUnderlyingType(arg1Val)) || (inv[grp] && goutil.IsZeroOfUnderlyingType(arg1Val)) {
				if mode[grp] == 1 {
					pass[grp] = true
				}
			}else if mode[grp] == 0 {
				pass[grp] = false
			}
			inv[grp] = false
		}else{
			arg1Val, arg1ok := getOpt(arg1, opts, true)

			var arg2Val interface{} = nil
			arg2ok := false

			if sign == 6 {
				arg2Val = goutil.ToString(arg2)
				arg2ok = true
			}else{
				arg2Val, arg2ok = getOpt(arg2, opts, true)
			}

			if !arg1ok || !arg2ok {
				// add to unsolved list
				var modeB []byte
				switch mode[grp] {
					case 0:
						modeB = []byte{'&'}
					case 1:
						modeB = []byte{'|'}
				}

				var signB []byte
				switch sign {
					case 0:
						signB = []byte{'='}
					case 1:
						signB = []byte{'!'}
					case 2:
						signB = []byte{'<'}
					case 3:
						signB = []byte{'>'}
					case 4:
						signB = []byte{'<', '='}
					case 5:
						signB = []byte{'>', '='}
					case 6:
						signB = []byte{'~'}
				}

				// unsolved[grp] = append(unsolved[grp], arg1, signB, arg2)
				if inv[grp] {
					unsolved[grp] = append(unsolved[grp], modeB, []byte{'^'}, arg1, signB, arg2)
					inv[grp] = false
				}else{
					unsolved[grp] = append(unsolved[grp], modeB, arg1, signB, arg2)
				}

				continue
			}

			p := false
			t := uint8(0)

			if sign == 6 {
				// regex
				arg1Val = goutil.ToByteArray(arg1Val)
				arg2Val = goutil.ToString(arg2Val)
				t = 6
			}else if reflect.TypeOf(arg1Val) != reflect.TypeOf(arg2Val) {
				if reflect.TypeOf(arg1Val) == goutil.VarType["string"] {
					arg2Val = goutil.ToString(arg2Val)
					t = 1
				}else if reflect.TypeOf(arg1Val) == goutil.VarType["bool"] {
					if v, err := strconv.ParseBool(goutil.ToString(arg2Val)); err == nil {
						arg2Val = v
						t = 2
					}
				}else if reflect.TypeOf(arg1Val) == goutil.VarType["int"] {
					if v, err := strconv.Atoi(goutil.ToString(arg2Val)); err == nil {
						arg2Val = v
						t = 3
					}
				}else if reflect.TypeOf(arg1Val) == goutil.VarType["float"] {
					if v, err := strconv.ParseFloat(goutil.ToString(arg2Val), 64); err == nil {
						arg2Val = v
						t = 4
					}
				}
			}else if reflect.TypeOf(arg1Val) == goutil.VarType["string"]{
				t = 1
			}else if reflect.TypeOf(arg1Val) == goutil.VarType["bool"]{
				t = 2
			}else if reflect.TypeOf(arg1Val) == goutil.VarType["int"]{
				t = 3
			}else if reflect.TypeOf(arg1Val) == goutil.VarType["float"]{
				t = 4
			}else if arg1Val == nil {
				t = 5
			}

			if t != 0 {
				if sign == 0 && arg1Val == arg2Val {
					p = true
				} else if sign == 1 && arg1Val != arg2Val {
					p = true
				} else if sign == 2 {
					if t == 1 && arg1Val.(string) < arg2Val.(string) {
						p = true
					}else if t == 2 && !arg1Val.(bool) && arg2Val.(bool) {
						p = true
					}else if t == 3 && arg1Val.(int) < arg2Val.(int) {
						p = true
					}else if t == 4 && arg1Val.(float64) < arg2Val.(float64) {
						p = true
					}
				} else if sign == 3 {
					if t == 1 && arg1Val.(string) > arg2Val.(string) {
						p = true
					}else if t == 2 && arg1Val.(bool) && !arg2Val.(bool) {
						p = true
					}else if t == 3 && arg1Val.(int) > arg2Val.(int) {
						p = true
					}else if t == 4 && arg1Val.(float64) > arg2Val.(float64) {
						p = true
					}
				} else if sign == 4 {
					if t == 1 && arg1Val.(string) <= arg2Val.(string) {
						p = true
					}else if t == 2 {
						p = true
					}else if t == 3 && arg1Val.(int) <= arg2Val.(int) {
						p = true
					}else if t == 4 && arg1Val.(float64) <= arg2Val.(float64) {
						p = true
					}
				}else if sign == 5 {
					if t == 1 && arg1Val.(string) >= arg2Val.(string) {
						p = true
					}else if t == 2 {
						p = true
					}else if t == 3 && arg1Val.(int) >= arg2Val.(int) {
						p = true
					}else if t == 4 && arg1Val.(float64) >= arg2Val.(float64) {
						p = true
					}
				}else if sign == 6 && t == 6 {
					if regex.Match(arg1Val.([]byte), regex.Compile(arg2Val.(string))) {
						p = true
					}
				}
			}

			if inv[grp] {
				p = !p
				inv[grp] = false
			}

			if p && mode[grp] == 1 {
				pass[grp] = true
			}else if !p && mode[grp] == 0 {
				pass[grp] = false
			}
		}
	}

	if len(unsolved[grp]) != 0 {
		if !pass[0] && unsolved[grp][0][0] == '&' {
			return false, nil
		}else if pass[0] && unsolved[grp][0][0] == '|' {
			return true, nil
		}else{
			unsolved[grp] = unsolved[grp][1:]
			return bytes.Join(unsolved[0], []byte{' '}), nil
		}
	}

	return pass[0], nil
}

func (t *Pre) Each(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {

	//todo: setup pre compiled each loop
	//todo: have functions provide optional pointer to temp storage

	if val, ok := (*args)["0"]; ok {
		_ = val
		// v, ok := getOpt(val, opts)
	}else{
		return nil, nil
	}

	return nil, nil
}


func (t *Comp) If(args *[][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	//todo: setup normal if handler without an unsolved list
	return nil, nil
}


func (t *Pre) PreFn(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (t *Comp) CompFn(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	return nil, nil
}
