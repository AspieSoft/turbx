package compiler

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v5"
	lorem "github.com/drhodes/golorem"
)

type tagFuncs struct {
	list map[string]func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte
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
func (funcs *tagFuncs) AddFN(name string, cb func(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte, useSync ...bool) error {
	if _, _, err := getCoreTagFunc([]byte(name)); err != nil {
		return errors.New("the method '" + name + "' is already in use by the core system")
	}

	if _, ok := funcs.list[name]; ok {
		return errors.New("the method '" + name + "' is already in use")
	} else if _, ok := funcs.list[name+"_SYNC"]; ok {
		return errors.New("the method '" + name + "' is already in use")
	}

	if len(useSync) != 0 && useSync[0] {
		name += "_SYNC"
	}

	funcs.list[name] = cb

	return nil
}

// note: the method 'If', is a unique tag func, with different args and return values than normal tag funcs
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
			} else {
				val := GetOpt(arg, opts, eachArgs, 0, precomp, false)
				if reflect.TypeOf(val) == goutil.VarType["[]byte"] && len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
					retArg := args.args[key]
					if inv {
						retArg = append([]byte{'!', ' '}, retArg...)
					}
					passCompArgs[len(res)] = retArg
					if (!inv && precomp) || (inv && !precomp) {
						res = append(res, 5)
					} else {
						res = append(res, 4)
					}
					continue
				}

				if (!inv && !goutil.IsZeroOfUnderlyingType(val)) || (inv && goutil.IsZeroOfUnderlyingType(val)) {
					res = append(res, 1)
				} else {
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
		} else if grpLevel != 0 {
			if arg[0] == 5 && arg[1] == ')' {
				grpLevel--
				if grpLevel != 0 {
					s := strconv.Itoa(ind)
					newArgs[s] = arg
					newArgsInd = append(newArgsInd, s)
					ind++
				} else {
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
						} else {
							res = append(res, 7)
						}
					} else if (!inv && ok) || (inv && !ok) {
						res = append(res, 7)
					} else {
						res = append(res, 6)
					}
				}
			} else {
				if _, err := strconv.Atoi(key); err == nil {
					s := strconv.Itoa(ind)
					newArgs[s] = arg
					newArgsInd = append(newArgsInd, s)
					ind++
				} else {
					newArgs[key] = arg
					newArgsInd = append(newArgsInd, key)
				}
			}
		} else if arg[0] == 5 && arg[1] == '!' {
			inv = !inv
		} else if arg[0] == 5 && arg[1] == '&' {
			res = append(res, 2)
			inv = false
		} else if arg[0] == 5 && arg[1] == '|' {
			res = append(res, 3)
			inv = false
		} else if arg[0] != 5 {
			if arg[0] != 0 && arg[0] <= 5 {
				arg = arg[1:]
				arg = regex.Comp(`^\{\{\{?[=:]?(.*)\}\}\}?$`).RepStrCompRef(&arg, []byte("$1"))
			} else if arg[0] == 0 {
				arg = arg[1:]
			}

			sign := uint8(0)
			if arg[0] == '=' {
				arg = arg[1:]
			} else if arg[0] == '!' {
				sign = 1
				arg = arg[1:]
			} else if arg[0] == '<' {
				sign = 2
				arg = arg[1:]
				if len(arg) != 0 {
					if arg[0] == '=' {
						sign = 3
						arg = arg[1:]
					}
				}
			} else if arg[0] == '>' {
				sign = 4
				arg = arg[1:]
				if len(arg) != 0 {
					if arg[0] == '=' {
						sign = 5
						arg = arg[1:]
					}
				}
			} else if arg[0] == '/' && regex.Comp(`^/(.*)/([ismxISMX]*)$`).MatchRef(&arg) { // regex
				sign = 6
				arg = regex.Comp(`^/(.*)/([ismxISMX]*)$`).RepFuncRef(&arg, func(data func(int) []byte) []byte {
					if len(data(2)) != 0 {
						flags := []byte("(?)")
						for _, f := range data(2) {
							fl := bytes.ToLower([]byte{f})[0]
							if f == fl {
								flags = append(flags, f)
							} else {
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
					} else {
						res = append(res, 0)
					}
				} else {
					val := GetOpt(arg, opts, eachArgs, 0, precomp, false)
					if reflect.TypeOf(val) == goutil.VarType["[]byte"] && len(val.([]byte)) != 0 && val.([]byte)[0] == 0 {
						retArg := args.args[key]
						if t {
							if sign == 2 {
								retArg = append([]byte{'>'}, retArg...)
							} else if sign == 3 {
								retArg = append([]byte{'>', '='}, retArg...)
							} else if sign == 4 {
								retArg = append([]byte{'<'}, retArg...)
							} else if sign == 5 {
								retArg = append([]byte{'<', '='}, retArg...)
							} else {
								retArg = append([]byte{'!', ' '}, retArg...)
							}
						} else {
							if sign == 2 {
								retArg = append([]byte{'<'}, retArg...)
							} else if sign == 3 {
								retArg = append([]byte{'<', '='}, retArg...)
							} else if sign == 4 {
								retArg = append([]byte{'>'}, retArg...)
							} else if sign == 5 {
								retArg = append([]byte{'>', '='}, retArg...)
							}
						}
						if sign == 6 {
							retArg = regex.JoinBytes('/', retArg, '/')
						}
						passCompArgs[len(res)] = retArg
						if (!t && precomp) || (t && !precomp) {
							res = append(res, 5)
						} else {
							res = append(res, 4)
						}
						continue
					}

					if (!t && !goutil.IsZeroOfUnderlyingType(val)) || (t && goutil.IsZeroOfUnderlyingType(val)) {
						res = append(res, 1)
					} else {
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
				} else if sign == 2 {
					retArg = append([]byte{'<'}, retArg...)
				} else if sign == 3 {
					retArg = append([]byte{'<', '='}, retArg...)
				} else if sign == 4 {
					retArg = append([]byte{'>'}, retArg...)
				} else if sign == 5 {
					retArg = append([]byte{'>', '='}, retArg...)
				} else if sign == 6 {
					retArg = regex.JoinBytes('/', retArg, '/')
				}
				if inv {
					retArg = regex.Comp(`^([!<>]|)`).RepFuncRef(&retArg, func(data func(int) []byte) []byte {
						if len(data(1)) != 0 {
							if data(1)[0] == '<' {
								return []byte{'>'}
							} else if data(1)[0] == '>' {
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
				} else {
					res = append(res, 4)
				}
				continue
			}

			var val2 interface{}
			if isStr != 0 || sign == 6 /* regex */ {
				val2 = arg
			} else {
				val2 = GetOpt(arg, opts, eachArgs, 0, precomp, false)
				if reflect.TypeOf(val2) == goutil.VarType["[]byte"] && len(val2.([]byte)) != 0 && val2.([]byte)[0] == 0 {
					retArg := args.args[key]
					if isStr != 0 {
						retArg = regex.JoinBytes(isStr, retArg, isStr)
					}
					if sign == 1 {
						retArg = append([]byte{'!'}, retArg...)
					} else if sign == 2 {
						retArg = append([]byte{'<'}, retArg...)
					} else if sign == 3 {
						retArg = append([]byte{'<', '='}, retArg...)
					} else if sign == 4 {
						retArg = append([]byte{'>'}, retArg...)
					} else if sign == 5 {
						retArg = append([]byte{'>', '='}, retArg...)
					} else if sign == 6 {
						retArg = regex.JoinBytes('/', retArg, '/')
					}
					if inv {
						retArg = regex.Comp(`^([!<>]|)`).RepFuncRef(&retArg, func(data func(int) []byte) []byte {
							if len(data(1)) != 0 {
								if data(1)[0] == '<' {
									return []byte{'>'}
								} else if data(1)[0] == '>' {
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
					} else {
						res = append(res, 4)
					}
					continue
				}
			}

			if sign == 0 {
				if (!inv && goutil.TypeEqual(val1, val2)) || (inv && !goutil.TypeEqual(val1, val2)) {
					res = append(res, 1)
				} else {
					res = append(res, 0)
				}
			} else if sign == 1 {
				if (!inv && !goutil.TypeEqual(val1, val2)) || (inv && goutil.TypeEqual(val1, val2)) {
					res = append(res, 1)
				} else {
					res = append(res, 0)
				}
			} else if sign == 2 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 0)
					} else {
						res = append(res, 1)
					}
				} else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 0)
					} else {
						res = append(res, 0)
					}
				} else {
					//todo: handle <
					if goutil.Conv.ToFloat(val1) < goutil.Conv.ToFloat(val2) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				}
			} else if sign == 3 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 1)
					} else {
						res = append(res, 1)
					}
				} else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				} else {
					//todo: handle <=
					if goutil.Conv.ToFloat(val1) <= goutil.Conv.ToFloat(val2) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				}
			} else if sign == 4 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 0)
					} else {
						res = append(res, 0)
					}
				} else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 0)
					} else {
						res = append(res, 1)
					}
				} else {
					//todo: handle >
					if goutil.Conv.ToFloat(val1) > goutil.Conv.ToFloat(val2) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				}
			} else if sign == 5 {
				val2 = goutil.ToVarTypeInterface(val2, val1)
				if val1 == nil {
					if (!inv && val2 == nil) || (inv && val2 != nil) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				} else if val2 == nil {
					if (!inv && val1 == nil) || (inv && val1 != nil) {
						res = append(res, 1)
					} else {
						res = append(res, 1)
					}
				} else {
					//todo: handle >=
					if goutil.Conv.ToFloat(val1) >= goutil.Conv.ToFloat(val2) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				}
			} else if sign == 6 {
				if regex.IsValidRef(&arg) {
					rB := regex.Comp(string(arg)).Match(goutil.Conv.ToBytes(val1))
					if (!inv && rB) || (inv && !rB) {
						res = append(res, 1)
					} else {
						res = append(res, 0)
					}
				} else {
					res = append(res, 0)
				}
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
						} else if res[i-1] == 3 {
							preCompRes = append(preCompRes, ' ', '|')
						} else if res[i-1] == 8 {
							if i > 1 {
								if res[i-2] == 2 {
									preCompRes = append(preCompRes, ' ', '&')
								} else if res[i-2] == 3 {
									preCompRes = append(preCompRes, ' ', '|')
								}
							}
							preCompRes = append(preCompRes, ' ', '!')
						}
						if v == 7 {
							preCompRes = append(preCompRes, ' ', '(', ' ')
						}
						preCompRes = append(preCompRes, ' ')
					} else if v == 7 {
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
		} else if v == 0 || v == 4 || v == 6 {
			r = false
			if v == 4 || v == 6 {
				if a, ok := passCompArgs[i]; ok {
					if i != 0 {
						if res[i-1] == 2 {
							preCompRes = append(preCompRes, ' ', '&')
						} else if res[i-1] == 3 {
							preCompRes = append(preCompRes, ' ', '|')
						} else if res[i-1] == 8 {
							if i > 1 {
								if res[i-2] == 2 {
									preCompRes = append(preCompRes, ' ', '&')
								} else if res[i-2] == 3 {
									preCompRes = append(preCompRes, ' ', '|')
								}
							}
							preCompRes = append(preCompRes, ' ', '!')
						}
						if v == 6 {
							preCompRes = append(preCompRes, ' ', '(', ' ')
						}
						preCompRes = append(preCompRes, ' ')
					} else if v == 6 {
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
		} else if v == 2 {
			modeAND = true
		} else if v == 3 {
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

// Rand sets a var option to crypto random bytes
//
// note: this method needs to be in sync
//
// add "_SYNC" if this function should run in sync, rather than running async on a seperate channel
func (funcs *tagFuncs) Rand_SYNC(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// args.args first byte:
	// 0 = normal arg "arg"
	// 1 = escaped option {{arg}}
	// 2 = raw option {{{arg}}}

	if len(args.args["0"]) != 0 && args.args["0"][0] == 0 {
		varName := args.args["0"][1:]

		if !regex.Comp(`^[\w_\-\$]+$`).MatchRef(&varName) {
			return nil
		}

		prefix := []byte{}
		if len(args.args["1"]) != 0 && args.args["1"][0] == 0 {
			prefix = args.args["1"][1:]
			if !regex.Comp(`^[\w_\-\$]+$`).MatchRef(&prefix) {
				prefix = []byte{}
			}
		}

		if precomp && varName[0] != '$' {
			if len(prefix) != 0 {
				return append([]byte{0}, regex.JoinBytes('0', '=', '"', varName, '"', '1', '=', '"', prefix, '"')...)
			}else{
				return append([]byte{0}, regex.JoinBytes('0', '=', '"', varName, '"')...)
			}
		}

		size := 64
		if len(args.args["size"]) != 0 && args.args["size"][0] == 0 {
			size = goutil.Conv.ToInt(args.args["size"][1:])
			if size == 0 {
				size = 64
			}
		}

		var exclude []byte = nil
		if len(args.args["exclude"]) != 0 && args.args["exclude"][0] == 0 {
			exclude = goutil.Conv.ToBytes(args.args["exclude"][1:])
		}

		var r []byte
		if exclude != nil {
			r = goutil.Crypt.RandBytes(size, exclude)
		} else {
			r = goutil.Crypt.RandBytes(size)
		}

		(*opts)[string(varName)] = append(prefix, r...)
	}

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}

// Json returns an option as a json string
func (funcs *tagFuncs) Json(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// args.args first byte:
	// 0 = normal arg "arg"
	// 1 = escaped option {{arg}}
	// 2 = raw option {{{arg}}}

	if len(args.args["0"]) != 0 && args.args["0"][0] == 0 {
		varName := args.args["0"][1:]

		if !regex.Comp(`^[\w_\-\$]+$`).MatchRef(&varName) {
			return nil
		}

		if hasVarOpt(varName, opts, eachArgs, 0, precomp) {
			val := GetOpt(varName, opts, eachArgs, 0, precomp, false)
			if b, ok := val.([]byte); ok && len(b) != 0 {
				if b[0] == 0 {
					b = b[1:]
				}
				return b
			} else if !goutil.IsZeroOfUnderlyingType(val) {
				if v, ok := val.(string); ok {
					return regex.JoinBytes('"', goutil.HTML.EscapeArgs([]byte(v), '"'), '"')
				}
				return toBytesOrJson(val)
			}
		} else {
			return append([]byte{0}, regex.JoinBytes('0', '=', '"', varName, '"')...)
		}
	}

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}

func (funcs *tagFuncs) Lorem(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// args.args first byte:
	// 0 = normal arg "arg"
	// 1 = escaped option {{arg}}
	// 2 = raw option {{{arg}}}

	argInd := 0

	wType := byte('p')
	if len(args.args["type"]) != 0 && args.args["type"][0] == 0 {
		wType = goutil.ToType[byte](args.args["type"][1:])
		if wType == 0 {
			wType = 'p'
		}
	} else if strI := strconv.Itoa(argInd); len(args.args[strI]) != 0 && args.args[strI][0] == 0 && !regex.Compile(`^[0-9]+$`).Match(args.args[strI][1:]) {
		wType = goutil.ToType[byte](args.args[strI][1:])
		if wType == 0 {
			wType = 'p'
		} else {
			argInd++
		}
	}

	rep := 1
	minLen := 2
	maxLen := 10
	minSet := false

	if len(args.args["rep"]) != 0 && args.args["rep"][0] == 0 {
		rep = goutil.Conv.ToInt(args.args["rep"][1:])
	} else if strI := strconv.Itoa(argInd); len(args.args[strI]) != 0 && args.args[strI][0] == 0 && regex.Comp(`^[0-9]+$`).Match(args.args[strI][1:]) {
		rep = goutil.Conv.ToInt(args.args[strI][1:])
		argInd++
	}

	if len(args.args["min"]) != 0 && args.args["min"][0] == 0 {
		minLen = goutil.Conv.ToInt(args.args["min"][1:])
	} else if strI := strconv.Itoa(argInd); len(args.args[strI]) != 0 && args.args[strI][0] == 0 && regex.Comp(`^[0-9]+$`).Match(args.args[strI][1:]) {
		minLen = goutil.Conv.ToInt(args.args[strI][1:])
		argInd++
		minSet = true
	}

	if len(args.args["max"]) != 0 && args.args["max"][0] == 0 {
		maxLen = goutil.Conv.ToInt(args.args["max"][1:])
	} else if strI := strconv.Itoa(argInd); len(args.args[strI]) != 0 && args.args[strI][0] == 0 && regex.Comp(`^[0-9]+$`).Match(args.args[strI][1:]) {
		maxLen = goutil.Conv.ToInt(args.args[strI][1:])
		argInd++
	} else if minSet {
		maxLen = minLen
	}

	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}

	res := []byte{}
	if wType == 'p' {
		resList := [][]byte{}
		for i := 0; i < rep; i++ {
			resList = append(resList, []byte("<p>"+lorem.Paragraph(minLen, maxLen)+"</p>"))
		}
		res = bytes.Join(resList, []byte("\n\n"))
	} else if wType == 'w' {
		resList := [][]byte{}
		for i := 0; i < rep; i++ {
			resList = append(resList, []byte(lorem.Word(minLen, maxLen)))
		}
		res = bytes.Join(resList, []byte(" "))
	} else if wType == 's' {
		resList := [][]byte{}
		for i := 0; i < rep; i++ {
			resList = append(resList, []byte(lorem.Sentence(minLen, maxLen)))
		}
		res = bytes.Join(resList, []byte(" "))
	} else if wType == 'h' {
		res = []byte(lorem.Host())
	} else if wType == 'e' {
		res = []byte(lorem.Email())
	} else if wType == 'u' {
		res = []byte(lorem.Url())
	}

	if len(res) != 0 {
		return res
	}

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}

// Set sets a var value to an option
//
// note: this method needs to be in sync
//
// add "_SYNC" if this function should run in sync, rather than running async on a seperate channel
//
// Set also pretends not to be a precomp func
func (funcs *tagFuncs) Set_SYNC(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) []byte {
	// args.args first byte:
	// 0 = normal arg "arg"
	// 1 = escaped option {{arg}}
	// 2 = raw option {{{arg}}}

	for _, arg := range args.ind {
		varName := []byte(arg)
		
		if !regex.Comp(`^[\w_\-\$]+$`).MatchRef(&varName) {
			continue
		}

		(*opts)[string(varName)] = GetOpt(args.args[arg][1:], opts, eachArgs, 0, false, false)
	}

	// return nil = return nothing
	// []byte("result html") = return basic html
	// append([]byte{0}, []byte("args")...) = pass function to compiler
	// append([]byte{1}, []byte("error message")...) = return error
	return nil
}
