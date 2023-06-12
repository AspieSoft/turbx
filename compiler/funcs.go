package compiler

import (
	"errors"
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


// note: If is a unique tag func, with different args and return values
func (funcs *tagFuncs) If(opts *map[string]interface{}, args *htmlArgs, eachArgs *[]EachArgs, precomp bool) ([]byte, bool) {
	//todo: handle and reduce if statement args
	// if precomp == true, return leftover args
	// if precomp == false, assume leftover args are false

	//! Note (for precomp): the folowing chars `&|()` should have spaces seperating them from normal var tags, when being returned as compiler args

	// return nil, false (absolute false)
	// return nil, true (absolute true)
	// return []byte("args"), true (push to compiler)
	// @[]byte: precomp result, @bool: if true
	return nil, false
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
