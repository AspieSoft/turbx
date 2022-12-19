package compiler

import (
	"bytes"
	"errors"
	"reflect"
	"sort"
	"strconv"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v3"
	lorem "github.com/drhodes/golorem"
)

type compFN struct {}

var funcs compFN = compFN{}

var userFuncList map[string]func(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal)(interface{}, error) = map[string]func(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal)(interface{}, error){}


// KeyVal is used to allow key:value lists to be sorted in an array
type KeyVal struct {
	Key []byte
	Val interface{}
}

type eachList struct {
	List *[]KeyVal
	As []byte
	Of []byte
}


// compiler funcs
func (t *compFN) If(args *[][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
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

		if !hasArg2 {
			arg1Val, arg1ok := GetOpt(arg1, opts, pre, addVars)

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
			arg1Val, arg1ok := GetOpt(arg1, opts, pre, addVars)

			var arg2Val interface{} = nil
			arg2ok := false

			if sign == 6 {
				arg2Val = goutil.ToString(arg2)
				arg2ok = true
			}else{
				arg2Val, arg2ok = GetOpt(arg2, opts, pre, addVars)
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
					unsolved[grp] = append(unsolved[grp], modeB, []byte{'^'}, arg1, signB, regex.JoinBytes('"', goutil.EscapeHTMLArgs(arg2), '"'))
					inv[grp] = false
				}else{
					unsolved[grp] = append(unsolved[grp], modeB, arg1, signB, regex.JoinBytes('"', goutil.EscapeHTMLArgs(arg2), '"'))
				}

				continue
			}

			p := false
			t := uint8(0)

			if sign != 6 {
				if reflect.TypeOf(arg1Val) == goutil.VarType["byteArray"] {
					arg1Val = string(arg1Val.([]byte))
				}
				if reflect.TypeOf(arg2Val) == goutil.VarType["byteArray"] {
					arg2Val = string(arg2Val.([]byte))
				}
			}

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
					if regex.Compile(arg2Val.(string)).Match(arg1Val.([]byte)) {
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

func (t *compFN) Each(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
	var from int
	var to int
	if (*args)["range"] != nil && len((*args)["range"]) != 0 {
		r := regex.Compile(`[^0-9\-]+`).Split((*args)["range"])
		if i, err := strconv.Atoi(string(bytes.TrimSpace(r[0]))); err == nil {
			from = i
		}
		if i, err := strconv.Atoi(string(bytes.TrimSpace(r[1]))); err == nil {
			to = i
		}
	}else{
		if (*args)["from"] != nil && len((*args)["from"]) != 0 {
			if i, err := strconv.Atoi(string(bytes.TrimSpace((*args)["from"]))); err == nil {
				from = i
			}
		}
	
		if (*args)["to"] != nil && len((*args)["to"]) != 0 {
			if i, err := strconv.Atoi(string(bytes.TrimSpace((*args)["to"]))); err == nil {
				to = i
			}
		}
	}

	if (*args)["1"] == nil || len((*args)["1"]) == 0 {
		res := []KeyVal{}
		if from <= to {
			for i := from; i <= to; i++ {
				res = append(res, KeyVal{[]byte(strconv.Itoa(i)), i})
			}
		}else{
			for i := from; i >= to; i-- {
				res = append(res, KeyVal{[]byte(strconv.Itoa(i)), i})
			}
		}

		resData := eachList{List: &res}

		if (*args)["as"] != nil && len((*args)["as"]) != 0 {
			resData.As = (*args)["as"]
		}
		if (*args)["of"] != nil && len((*args)["of"]) != 0 {
			resData.Of = (*args)["of"]
		}

		return resData, nil
	}

	list, ok := GetOpt((*args)["1"], opts, pre, addVars)
	if !ok {
		if bytes.HasPrefix((*args)["1"], []byte{'$'}) {
			return nil, nil
		}

		res := (*args)["1"]
		if (*args)["as"] != nil && len((*args)["as"]) != 0 {
			res = regex.JoinBytes(res, []byte(" as "), (*args)["as"])
		}
		if (*args)["of"] != nil && len((*args)["of"]) != 0 {
			res = regex.JoinBytes(res, []byte(" of "), (*args)["of"])
		}

		return res, nil
	}

	var res *[]KeyVal

	lt := reflect.TypeOf(list)
	if lt == goutil.VarType["map"] {
		resList := make([]KeyVal, len(list.(map[string]interface{})))
		res = &resList
		ind := 0

		for k, v := range list.(map[string]interface{}) {
			(*res)[ind] = KeyVal{[]byte(k), v}
			ind++
		}

		sort.Slice(*res, func(i, j int) bool {
			k1 := []byte((*res)[i].Key)
			k2 := []byte((*res)[j].Key)
			l := len(k1)
			if len(k2) < l {
				l = len(k2)
			}

			r := 0
			for i := 0; i < l; i++ {
				if k1[i] < k2[i] {
					r = 1
					break
				}else if k1[i] > k2[i] {
					r = -1
					break
				}
			}

			if from > to {
				if r == 0 {
					return len(k1) > len(k2)
				}
				return r == -1
			}

			if r == 0 {
				return len(k1) < len(k2)
			}
			return r == 1
		})

	}else if lt == goutil.VarType["array"] {
		resList := make([]KeyVal, len(list.([]interface{})))
		res = &resList
		ind := 0
		for i, v := range list.([]interface{}) {
			(*res)[ind] = KeyVal{[]byte(strconv.Itoa(i)), v}
			ind++
		}

		if from > to {
			sort.Slice(*res, func(i, j int) bool {
				return i > j
			})
		}
	}else{
		return nil, nil
	}

	resData := eachList{List: res}

	if (*args)["as"] != nil && len((*args)["as"]) != 0 {
		resData.As = (*args)["as"]
	}
	if (*args)["of"] != nil && len((*args)["of"]) != 0 {
		resData.Of = (*args)["of"]
	}

	return resData, nil
}

func (t *compFN) Json(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
	if val, ok := GetOpt((*args)["1"], opts, pre, addVars); ok {
		json, err := goutil.StringifyJSON(val, 0, 2)
		if err != nil {
			return nil, err
		}

		return json, nil
	}

	return nil, errors.New("var not found")
}

func (t *compFN) Lorem(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, error) {
	wType := byte('p')
	if (*args)["type"] != nil && len((*args)["type"]) != 0 {
		wType = (*args)["type"][0]
	}else{
		for i := 1; i < len(*args); i++ {
			iStr := strconv.Itoa(i)
			if (*args)[iStr] != nil && len((*args)[iStr]) != 0 && !regex.Compile(`^[0-9]+$`).Match((*args)[iStr]) {
				wType = (*args)[iStr][0]
				break
			}
		}
	}

	rep := 1
	minLen := 2
	maxLen := 10
	used := 0
	minSet := false

	if (*args)["rep"] != nil && regex.Compile(`^[0-9]+$`).Match((*args)["rep"]) {
		if n, err := strconv.Atoi(string((*args)["rep"])); err == nil {
			rep = n
		}
	}else{
		for i := used + 1; i < len(*args); i++ {
			iStr := strconv.Itoa(i)
			if (*args)[iStr] != nil && len((*args)[iStr]) != 0 && regex.Compile(`^[0-9]+$`).Match((*args)[iStr]) {
				if n, err := strconv.Atoi(string((*args)[iStr])); err == nil {
					used = i
					rep = n
					break
				}
			}
		}
	}

	if (*args)["min"] != nil && regex.Compile(`^[0-9]+$`).Match((*args)["min"]) {
		if n, err := strconv.Atoi(string((*args)["min"])); err == nil {
			minSet = true
			minLen = n
		}
	}else{
		for i := used + 1; i < len(*args); i++ {
			iStr := strconv.Itoa(i)
			if (*args)[iStr] != nil && len((*args)[iStr]) != 0 && regex.Compile(`^[0-9]+$`).Match((*args)[iStr]) {
				if n, err := strconv.Atoi(string((*args)[iStr])); err == nil {
					used = i
					minSet = true
					minLen = n
					break
				}
			}
		}
	}

	if (*args)["max"] != nil && regex.Compile(`^[0-9]+$`).Match((*args)["max"]) {
		if n, err := strconv.Atoi(string((*args)["max"])); err == nil {
			maxLen = n
		}
	}else{
		maxSet := false
		for i := used + 1; i < len(*args); i++ {
			iStr := strconv.Itoa(i)
			if (*args)[iStr] != nil && len((*args)[iStr]) != 0 && regex.Compile(`^[0-9]+$`).Match((*args)[iStr]) {
				if n, err := strconv.Atoi(string((*args)[iStr])); err == nil {
					used = i
					maxSet = true
					maxLen = n
					break
				}
			}
		}

		if !maxSet && minSet {
			maxLen = minLen
		}
	}

	if wType == 'p' {
		res := [][]byte{}
		for i := 0; i < rep; i++ {
			res = append(res, []byte("<p>"+lorem.Paragraph(minLen, maxLen)+"</p>"))
		}
		return bytes.Join(res, []byte("\n\n")), nil
	} else if wType == 'w' {
		res := [][]byte{}
		for i := 0; i < rep; i++ {
			res = append(res, []byte(lorem.Word(minLen, maxLen)))
		}
		return bytes.Join(res, []byte(" ")), nil
	} else if wType == 's' {
		res := [][]byte{}
		for i := 0; i < rep; i++ {
			res = append(res, []byte(lorem.Sentence(minLen, maxLen)))
		}
		return bytes.Join(res, []byte(" ")), nil
	} else if wType == 'h' {
		return []byte(lorem.Host()), nil
	} else if wType == 'e' {
		return []byte(lorem.Email()), nil
	} else if wType == 'u' {
		return []byte(lorem.Url()), nil
	}

	res := [][]byte{}
	for i := 0; i < rep; i++ {
		res = append(res, []byte("<p>"+lorem.Paragraph(minLen, maxLen)+"</p>"))
	}
	return bytes.Join(res, []byte("\n\n")), nil
}


// handle opts
func convertOpt(arg []byte, opts *map[string]interface{}, pre *bool, addVars *[]KeyVal) (interface{}, bool) {
	if regex.Compile(`^(["'\'])(.*)\1$`).MatchRef(&arg) {
		arg = regex.Compile(`^(["'\'])(.*)\1$`).RepStrComplexRef(&arg, []byte("$2"))
		
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

	// handle additional vars
	for i := len(*addVars)-1; i >= 0; i-- {
		if bytes.Equal((*addVars)[i].Key, arg) {
			return (*addVars)[i].Val, true
		}
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

func getOptObj(arg []byte, opts *map[string]interface{}, pre *bool, addVars *[]KeyVal) (interface{}, bool) {
	args := regex.Compile(`\.|(\[(?:"(?:\\[\\"]|.)*?"|'(?:\\[\\']|.)*?'|\'(?:\\[\\\']|.)*?\'|.)*?\])`).SplitRef(&arg)
	// args := regex.Compile(`(\[[\w_]+\])|\.`).SplitRef(&arg)

	res, ok := convertOpt(args[0], opts, pre, addVars)
	if !ok {
		return res, false
	}
	args = args[1:]

	for _, arg := range args {
		if bytes.HasPrefix(arg, []byte{'['}) && bytes.HasSuffix(arg, []byte{']'}) {
			arg = arg[1:len(arg)-1]
			v, ok := GetOpt(arg, opts, *pre, addVars)
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
			val, ok := convertOpt(arg, &r, pre, addVars)
			if !ok {
				return val, false
			}
			res = val
		}else if rType == goutil.VarType["array"] {
			r := map[string]interface{}{}
			for i, v := range res.([]interface{}) {
				r[strconv.Itoa(i)] = v
			}
			val, ok := convertOpt(arg, &r, pre, addVars)
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
			val, ok := convertOpt(arg, &r, pre, addVars)
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

// GetOpt is used to handle grabing an option from the user options that were passed
//
// this method accepts the arg as a simple text like []byte("myOption")
//
// this method can also handle complex options like {{this|'that'}} (with optional or statements) and even {{class="myClass"}} vars
func GetOpt(arg []byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal) (interface{}, bool) {
	var key []byte
	arg = regex.Compile(`^{{{?([\w_-]+)=(["'\']|)(.*)\2}}}?$`).RepFuncRef(&arg, func(data func(int) []byte) []byte {
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

			val, ok := getOptObj(b, opts, &pre, addVars)
			if ok {
				if key != nil {
					return KeyVal{key, val}, true
				}
				return val, true
			}
			b = []byte{}

			if pre && val == true {
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
					if regex.Compile(`[A-Za-z]`).MatchRef(&[]byte{arg[i]}) {
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
							if regex.Compile(`[A-Za-z]`).MatchRef(&[]byte{arg[i]}) {
								b = append(b, arg[i])
							}
							i++
						}
		
						b = append(b, arg[i])
					}
					// continue
				}else if arg[i] == '\\' {
					if regex.Compile(`[A-Za-z]`).MatchRef(&[]byte{arg[i]}) {
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
		if val, ok := getOptObj(b, opts, &pre, addVars); ok {
			if key != nil {
				return KeyVal{key, val}, true
			}
			return val, true
		}
	}

	return nil, false
}


// other methods

// NewFunc can be used to create custom functions for the compiler
//
// these user defined functions will only run after the default functions have been resolved
func NewFunc(name string, fn func(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool, addVars *[]KeyVal)(interface{}, error)) error {
	name = string(regex.Compile(`[^\w_]`).RepStr([]byte(name), []byte{}))

	if name == "If" || name == "Else" || name == "Elif" || name == "Each" {
		return errors.New("the function '"+name+"' is one of the major core functions set by the compiler")
	}

	m := reflect.ValueOf(&funcs).MethodByName(name)
	if !goutil.IsZeroOfUnderlyingType(m) {
		return errors.New("the function '"+name+"' has already been set by the compiler")
	}

	if _, ok := userFuncList[name]; !ok {
		userFuncList[name] = fn
		return nil
	}

	return errors.New("you cannot set 2 functions with the same name")
}