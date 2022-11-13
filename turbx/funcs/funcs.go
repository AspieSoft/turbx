package funcs

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex/v3"
	"github.com/AspieSoft/goutil/v3"
)

type Pre struct {}
type Comp struct {}


func getOpt(arg string, opts *map[string]interface{}) (interface{}, bool) {
	//todo: handle object indexes
	if val, ok := (*opts)[arg]; ok {
		return val, true
	}
	return nil, false
}


func (t *Pre) If(args *[][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	pass := []bool{true}
	inv := []bool{false}
	mode := []uint8{0}
	grp := 0

	lastArg := []byte{}

	unsolved := [][]byte{{}}
	_ = unsolved

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
				unsolved = append(unsolved, []byte{})
				grp++
				continue
			}else if (*args)[i][0] == ')' {
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
					inv[grp-1] = false
				}
				pass = pass[:grp]
				mode = mode[:grp]
				grp--
				//todo: have this method also handle the unsolved list

				continue
			}

			//todo: may use '~' for complex regex (note: will also have to update compiler to make this char a seperate arg)
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
			i++
		}

		if hasArg2 && len(*args) > i+1 {
			arg2 = (*args)[i+1]
			i++
		}


		// make '$' unique to const vars for pre compile to handle
		// ignore in regular compiler
		if !hasArg2 {
			var arg1Val interface{} = nil
			arg1ok := false

			if regex.MatchRef(&arg1, regex.Compile(`^(["'\']).*\1$`)) {
				arg1Val = string(arg1[1:len(arg1)-1])
				if arg1Val.(string) == "true" {
					arg1Val = true
					arg1ok = true
				}else if arg1Val.(string) == "false" {
					arg1Val = false
					arg1ok = true
				}else if v, err := strconv.Atoi(arg1Val.(string)); err == nil {
					arg1Val = v
					arg1ok = true
				}else if v, err := strconv.ParseFloat(arg1Val.(string), 64); err == nil {
					arg1Val = v
					arg1ok = true
				}else if arg1Val.(string) == "nil" || arg1Val.(string) == "null" || arg1Val.(string) == "undefined" {
					arg1Val = nil
					arg1ok = true
				}
			}else if arg1[0] == '$' {
				if val, ok := getOpt(string(arg1), opts); ok {
					arg1Val = val
					arg1ok = true
				}else if val, ok := getOpt(string(arg1[1:]), opts); ok {
					arg1Val = val
					arg1ok = true
				}
			}else if val, ok := getOpt("$"+string(arg1), opts); ok {
				arg1Val = val
				arg1ok = true
			}

			if !arg1ok {
				//todo add to unsolved list
				// unsolved[grp] = append(unsolved[grp], )
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
			var arg1Val interface{} = nil
			arg1ok := false
			if regex.MatchRef(&arg1, regex.Compile(`^(["'\']).*\1$`)) {
				arg1Val = string(arg1[1:len(arg1)-1])
				if arg1Val.(string) == "true" {
					arg1Val = true
					arg1ok = true
				}else if arg1Val.(string) == "false" {
					arg1Val = false
					arg1ok = true
				}else if v, err := strconv.Atoi(arg1Val.(string)); err == nil {
					arg1Val = v
					arg1ok = true
				}else if v, err := strconv.ParseFloat(arg1Val.(string), 64); err == nil {
					arg1Val = v
					arg1ok = true
				}else if arg1Val.(string) == "nil" || arg1Val.(string) == "null" || arg1Val.(string) == "undefined" {
					arg1Val = nil
					arg1ok = true
				}
			}else if arg1[0] == '$' {
				if val, ok := getOpt(string(arg1), opts); ok {
					arg1Val = val
					arg1ok = true
				}else if val, ok := getOpt(string(arg1[1:]), opts); ok {
					arg1Val = val
					arg1ok = true
				}
			}else if val, ok := getOpt("$"+string(arg1), opts); ok {
				arg1Val = val
				arg1ok = true
			}

			var arg2Val interface{} = nil
			arg2ok := false
			if regex.MatchRef(&arg2, regex.Compile(`^(["'\']).*\1$`)) {
				arg2Val = string(arg2[1:len(arg2)-1])
				if arg2Val.(string) == "true" {
					arg2Val = true
					arg2ok = true
				}else if arg2Val.(string) == "false" {
					arg2Val = false
					arg2ok = true
				}else if v, err := strconv.Atoi(arg2Val.(string)); err == nil {
					arg2Val = v
					arg2ok = true
				}else if v, err := strconv.ParseFloat(arg2Val.(string), 64); err == nil {
					arg2Val = v
					arg2ok = true
				}else if arg2Val.(string) == "nil" || arg2Val.(string) == "null" || arg2Val.(string) == "undefined" {
					arg2Val = nil
					arg2ok = true
				}
			}else if arg2[0] == '$' {
				if val, ok := getOpt(string(arg2), opts); ok {
					arg2Val = val
					arg2ok = true
				}else if val, ok := getOpt(string(arg2[1:]), opts); ok {
					arg2Val = val
					arg2ok = true
				}
			}else if val, ok := getOpt("$"+string(arg2), opts); ok {
				arg2Val = val
				arg2ok = true
			}

			if !arg1ok || !arg2ok {
				//todo add to unsolved list
				// unsolved[grp] = append(unsolved[grp], )
				continue
			}

			p := false
			t := uint8(0)
			if reflect.TypeOf(arg1Val) != reflect.TypeOf(arg2Val) {
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
				}
			}

			if inv[grp] {
				p = !p
			}

			if p && mode[grp] == 1 {
				pass[grp] = true
			}else if !p && mode[grp] == 0 {
				pass[grp] = false
			}
		}
	}

	fmt.Println(pass)

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
