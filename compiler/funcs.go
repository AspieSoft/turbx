package compiler

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
)

type tagFuncs struct {
	list map[string]func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool)[]byte
}
var TagFuncs tagFuncs = tagFuncs{}

// AddFN adds a new function to the compiler
//
// @name: the name of your function
//
// @cb: a callback function where you can return a []byte with html to be added to the file
//
// cb - @opts: contains the list of options that were passed into the compiler (recommended: use `turbx.GetOpt` when retrieving an option)
//
// cb - @args: arguments that the template passed into the function (you may want to pass some of these into the `@name` arg for `turbx.GetOpt`)
//
// cb - @eachArgs: a list of arguments that were defined by an each loop (the last eachAre should take priority over the first one)
//
// cb - @precompile: returns true if the function was called by the precompiler, false if called by the final compiler
//
// cb - @return: nil = no content
//
// cb - @return: []byte("html") = html result
//
// cb - @return: append([]byte{0}, []byte("args")...) = pass to compiler
//
// cb - @return: append([]byte{1}, []byte("error msg")...) = return error
//
// @useSync (optional): by default all functions run concurrently, if you need the compiler to wait for your function to finish, you can set this to `true`
func (funcs *tagFuncs) AddFN(name string, cb func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool)[]byte, useSync ...bool) error {
	if _, _, err := getCoreTagFunc([]byte(name)); err != nil {
		return errors.New("the method '"+name+"' is already in use by the core system")
	}

	if _, ok := funcs.list[name]; ok {
		return errors.New("the method '"+name+"' is already in use")
	}else if _, ok := funcs.list[name+"_SYNC"]; ok {
		return errors.New("the method '"+name+"' is already in use")
	}

	if len(useSync) != 0 && useSync[0] {
		name += "_SYNC"
	}

	funcs.list[name] = cb

	return nil
}


// note: the method 'If', is a unique tag func, with different args and return values
func (funcs *tagFuncs) If(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) ([]byte, bool) {
	passCompArgs := map[int][]byte{}

	res := []uint8{}

	newArgs := map[string][]byte{}
	newArgsInd := []string{}
	ind := 0
	grpLevel := 0

	inv := false

	for _, key := range args.ind {
		arg := args.args[key]
		if len(arg) < 2 {
			if len(arg) == 0 {
				res = append(res, 0)
			}else{
				val := GetOpt(arg, opts, eachArgs, 0, precomp, false)
				if reflect.TypeOf(val) == goutil.VarType["[]byte"] && len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
					retArg := args.args[key]
					if inv {
						retArg = append([]byte{'!', ' '}, retArg...)
					}
					passCompArgs[len(res)] = retArg
					if (!inv && precomp) || (inv && !precomp) {
						res = append(res, 5)
					}else{
						res = append(res, 4)
					}
					continue
				}

				if (!inv && !goutil.IsZeroOfUnderlyingType(val)) || (inv && goutil.IsZeroOfUnderlyingType(val)) {
					res = append(res, 1)
				}else{
					res = append(res, 0)
				}
			}
			continue
		}

		if arg[0] == 5 && arg[1] == '(' {
			if grpLevel != 0 {
				s := strconv.Itoa(ind)
				newArgs[s] = arg
				newArgsInd = append(newArgsInd, s)
				ind++
			}
			grpLevel++
		}else if grpLevel != 0 {
			if arg[0] == 5 && arg[1] == ')' {
				grpLevel--
				if grpLevel != 0 {
					s := strconv.Itoa(ind)
					newArgs[s] = arg
					newArgsInd = append(newArgsInd, s)
					ind++
				}else{
					passComp, ok := TagFuncs.If(opts, &htmlArgs{args: newArgs, ind: newArgsInd}, eachArgs, precomp)
					
					newArgs = map[string][]byte{}
					newArgsInd = []string{}
					
					if inv {
						res = append(res, 8)
					}
					if passComp != nil && len(passComp) != 0 {
						passCompArgs[len(res)] = passComp
					}
					if !precomp && passComp != nil && len(passComp) != 0 {
						if !inv {
							res = append(res, 6)
						}else{
							res = append(res, 7)
						}
					}else if (!inv && ok) || (inv && !ok) {
						res = append(res, 7)
					}else{
						res = append(res, 6)
					}
				}
			}else{
				if _, err := strconv.Atoi(key); err == nil {
					s := strconv.Itoa(ind)
					newArgs[s] = arg
					newArgsInd = append(newArgsInd, s)
					ind++
				}else{
					newArgs[key] = arg
					newArgsInd = append(newArgsInd, key)
				}
			}
		}else if arg[0] == 5 && arg[1] == '!' {
			inv = !inv
		}else if arg[0] == 5 && arg[1] == '&' {
			res = append(res, 2)
			inv = false
		}else if arg[0] == 5 && arg[1] == '|' {
			res = append(res, 3)
			inv = false
		}else if arg[0] != 5 {
			if arg[0] != 0 && arg[0] <= 5 {
				arg = arg[1:]
				arg = regex.Comp(`^\{\{\{?[=:]?(.*)\}\}\}?$`).RepStrCompRef(&arg, []byte("$1"))
			}else if arg[0] == 0 {
				arg = arg[1:]
			}

			sign := uint8(0)
			if arg[0] == '=' {
				arg = arg[1:]
			}else if arg[0] == '!' {
				sign = 1
				arg = arg[1:]
			}else if arg[0] == '<' {
				sign = 2
				arg = arg[1:]
				if len(arg) != 0 {
					if arg[0] == '=' {
						sign = 3
						arg = arg[1:]
					}
				}
			}else if arg[0] == '>' {
				sign = 4
				arg = arg[1:]
				if len(arg) != 0 {
					if arg[0] == '=' {
						sign = 5
						arg = arg[1:]
					}
				}
			}else if arg[0] == '/' && regex.Comp(`^/(.*)/([ismxISMX]*)$`).MatchRef(&arg) { // regex
				sign = 6
				arg = regex.Comp(`^/(.*)/([ismxISMX]*)$`).RepFuncRef(&arg, func(data func(int) []byte) []byte {
					if len(data(2)) != 0 {
						flags := []byte("(?)")
						for _, f := range data(2) {
							fl := bytes.ToLower([]byte{f})[0]
							if f == fl {
								flags = append(flags, f)
							}else{
								flags = append(flags, '-', f)
							}
						}
						return regex.JoinBytes('(', '?', flags, ')', data(1))
					}
					return data(1)
				})
			}

			var isStr byte
			arg = regex.Comp(`^(["'\'])(.*)\1$`).RepFuncRef(&arg, func(data func(int) []byte) []byte {
				isStr = data(1)[0]
				return data(2)
			})

			// if lone arg key
			if _, err := strconv.Atoi(key); err == nil {
				t := inv
				if sign == 1 {
					t = !t
				}
				if isStr != 0 {
					if (!t && len(arg) != 0) || (t && len(arg) == 0) {
						res = append(res, 1)
					}else{
						res = append(res, 0)
					}
				}else{
					val := GetOpt(arg, opts, eachArgs, 0, precomp, false)
					if reflect.TypeOf(val) == goutil.VarType["[]byte"] && len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
						retArg := args.args[key]
						if t {
							if sign == 2 {
								retArg = append([]byte{'>'}, retArg...)
							}else if sign == 3 {
								retArg = append([]byte{'>', '='}, retArg...)
							}else if sign == 4 {
								retArg = append([]byte{'<'}, retArg...)
							}else if sign == 5 {
								retArg = append([]byte{'<', '='}, retArg...)
							}else{
								retArg = append([]byte{'!', ' '}, retArg...)
							}
						}else{
							if sign == 2 {
								retArg = append([]byte{'<'}, retArg...)
							}else if sign == 3 {
								retArg = append([]byte{'<', '='}, retArg...)
							}else if sign == 4 {
								retArg = append([]byte{'>'}, retArg...)
							}else if sign == 5 {
								retArg = append([]byte{'>', '='}, retArg...)
							}
						}
						if sign == 6 {
							retArg = regex.JoinBytes('/', retArg, '/')
						}
						passCompArgs[len(res)] = retArg
						if (!t && precomp) || (t && !precomp) {
							res = append(res, 5)
						}else{
							res = append(res, 4)
						}
						continue
					}

					if (!t && !goutil.IsZeroOfUnderlyingType(val)) || (t && goutil.IsZeroOfUnderlyingType(val)) {
						res = append(res, 1)
					}else{
						res = append(res, 0)
					}
				}
				continue
			}

			// if comparing arg
			val1 := GetOpt([]byte(key), opts, eachArgs, 0, precomp, false)
			if reflect.TypeOf(val1) == goutil.VarType["[]byte"] && len(val1.([]byte)) != 0 && val1.([]byte)[0] == 0 {
				retArg := arg
				if isStr != 0 {
					retArg = regex.JoinBytes(isStr, retArg, isStr)
				}
				if sign == 1 {
					retArg = append([]byte{'!'}, retArg...)
				}else if sign == 2 {
					retArg = append([]byte{'<'}, retArg...)
				}else if sign == 3 {
					retArg = append([]byte{'<', '='}, retArg...)
				}else if sign == 4 {
					retArg = append([]byte{'>'}, retArg...)
				}else if sign == 5 {
					retArg = append([]byte{'>', '='}, retArg...)
				}else if sign == 6 {
					retArg = regex.JoinBytes('/', retArg, '/')
				}
				if inv {
					retArg = regex.Comp(`^([!<>]|)`).RepFuncRef(&retArg, func(data func(int) []byte) []byte {
						if len(data(1)) != 0 {
							if data(1)[0] == '<' {
								return []byte{'>'}
							}else if data(1)[0] == '>' {
								return []byte{'<'}
							}
							return []byte{}
						}
						return []byte{'!'}
					})
				}
				passCompArgs[len(res)] = regex.JoinBytes(key, '=', '"', goutil.HTML.EscapeArgs(retArg, '"'), '"')
				if (!inv && precomp) || (inv && !precomp) {
					res = append(res, 5)
				}else{
					res = append(res, 4)
				}
				continue
			}

			var val2 interface{}
			if isStr != 0 || sign == 6 /* regex */ {
				val2 = arg
			}else{
				val2 = GetOpt(arg, opts, eachArgs, 0, precomp, false)
				if reflect.TypeOf(val2) == goutil.VarType["[]byte"] && len(val2.([]byte)) != 0 && val2.([]byte)[0] == 0 {
					retArg := args.args[key]
					if isStr != 0 {
						retArg = regex.JoinBytes(isStr, retArg, isStr)
					}
					if sign == 1 {
						retArg = append([]byte{'!'}, retArg...)
					}else if sign == 2 {
						retArg = append([]byte{'<'}, retArg...)
					}else if sign == 3 {
						retArg = append([]byte{'<', '='}, retArg...)
					}else if sign == 4 {
						retArg = append([]byte{'>'}, retArg...)
					}else if sign == 5 {
						retArg = append([]byte{'>', '='}, retArg...)
					}else if sign == 6 {
						retArg = regex.JoinBytes('/', retArg, '/')
					}
					if inv {
						retArg = regex.Comp(`^([!<>]|)`).RepFuncRef(&retArg, func(data func(int) []byte) []byte {
							if len(data(1)) != 0 {
								if data(1)[0] == '<' {
									return []byte{'>'}
								}else if data(1)[0] == '>' {
									return []byte{'<'}
								}
								return []byte{}
							}
							return []byte{'!'}
						})
					}
					passCompArgs[len(res)] = regex.JoinBytes(key, '=', '"', goutil.HTML.EscapeArgs(retArg, '"'), '"')
					if (!inv && precomp) || (inv && !precomp) {
						res = append(res, 5)
					}else{
						res = append(res, 4)
					}
					continue
				}
			}

			if sign == 0 {
				if (!inv && goutil.TypeEqual(val1, val2)) || (inv && !goutil.TypeEqual(val1, val2)) {
					res = append(res, 1)
				}else{
					res = append(res, 0)
				}
			}else if sign == 1 {
				if (!inv && !goutil.TypeEqual(val1, val2)) || (inv && goutil.TypeEqual(val1, val2)) {
					res = append(res, 1)
				}else{
					res = append(res, 0)
				}
			}else if sign == 2 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 0)
					}else{
						res = append(res, 1)
					}
				}else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 0)
					}else{
						res = append(res, 0)
					}
				}else{
					//todo: handle <
				}
			}else if sign == 3 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 1)
					}else{
						res = append(res, 1)
					}
				}else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 1)
					}else{
						res = append(res, 0)
					}
				}else{
					//todo: handle <=
				}
			}else if sign == 4 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 0)
					}else{
						res = append(res, 0)
					}
				}else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 0)
					}else{
						res = append(res, 1)
					}
				}else{
					//todo: handle >
				}
			}else if sign == 5 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 1)
					}else{
						res = append(res, 0)
					}
				}else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 1)
					}else{
						res = append(res, 1)
					}
				}else{
					//todo: handle >=
				}
			}else if sign == 6 {
				//todo: verify regex is safe
				regex.Comp(string(arg)).Match(goutil.Conv.ToBytes(val1))
			}
		}
	}

	modeAND := true
	r := true
	preCompRes := []byte{}

	for i, v := range res {
		if v == 1 || v == 5 || v == 7 {
			r = true
			if v == 5 || v == 7 {
				if a, ok := passCompArgs[i]; ok {
					if i != 0 {
						if res[i-1] == 2 {
							preCompRes = append(preCompRes, ' ', '&')
						}else if res[i-1] == 3 {
							preCompRes = append(preCompRes, ' ', '|')
						}else if res[i-1] == 8 {
							if i > 1 {
								if res[i-2] == 2 {
									preCompRes = append(preCompRes, ' ', '&')
								}else if res[i-2] == 3 {
									preCompRes = append(preCompRes, ' ', '|')
								}
							}
							preCompRes = append(preCompRes, ' ', '!')
						}
						if v == 7 {
							preCompRes = append(preCompRes, ' ', '(', ' ')
						}
						preCompRes = append(preCompRes, ' ')
					}else if v == 7 {
						preCompRes = append(preCompRes, '(', ' ')
					}
					preCompRes = append(preCompRes, a...)
					if v == 7 {
						preCompRes = append(preCompRes, ' ', ')')
					}
				}
			}
			if !modeAND && (!precomp || v == 1) {
				break
			}
		}else if v == 0 || v == 4 || v == 6 {
			r = false
			if v == 4 || v == 6 {
				if a, ok := passCompArgs[i]; ok {
					if i != 0 {
						if res[i-1] == 2 {
							preCompRes = append(preCompRes, ' ', '&')
						}else if res[i-1] == 3 {
							preCompRes = append(preCompRes, ' ', '|')
						}else if res[i-1] == 8 {
							if i > 1 {
								if res[i-2] == 2 {
									preCompRes = append(preCompRes, ' ', '&')
								}else if res[i-2] == 3 {
									preCompRes = append(preCompRes, ' ', '|')
								}
							}
							preCompRes = append(preCompRes, ' ', '!')
						}
						if v == 6 {
							preCompRes = append(preCompRes, ' ', '(', ' ')
						}
						preCompRes = append(preCompRes, ' ')
					}else if v == 6 {
						preCompRes = append(preCompRes, '(', ' ')
					}
					preCompRes = append(preCompRes, a...)
					if v == 6 {
						preCompRes = append(preCompRes, ' ', ')')
					}
				}
			}
			if modeAND && (!precomp || v == 0) {
				break
			}
		}else if v == 2 {
			modeAND = true
		}else if v == 3 {
			modeAND = false
		}
	}

	if precomp && len(preCompRes) != 0 {
		return preCompRes, true
	}

	// return nil, false (absolute false)
	// return nil, true (absolute true)
	// return []byte("args"), true (push to compiler)
	// @[]byte: precomp result, @bool: if true
	return nil, r
}


func (funcs *tagFuncs) Myfn(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// do stuff concurrently

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}

// add "_SYNC" if this function should run in sync, rather than running async on a seperate channel
func (funcs *tagFuncs) Myfn_SYNC(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// do stuff in sync

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}
