package compiler

type tagFuncs struct {}
var TagFuncs tagFuncs = tagFuncs{}

// note: If is a unique tag func, with different args and return values
func (funcs *tagFuncs) If(opts *map[string]interface{}, args *htmlArgs, precomp bool) ([]byte, bool) {
	//todo: handle and reduce if statement args
	// if precomp == true, return leftover args
	// if precomp == false, assume leftover args are false

	// return nil, false (absolute false)
	// return nil, true (absolute true)
	// return []byte("args"), true (push to compiler)
	// return append([]byte{0}, []byte("args")...), true (rerun in pre compiler if args.fnContArgs were found) (uses {{%&if}} instead of {{%if}})
	// @[]byte: precomp result, @bool: if true
	return nil, false
}

func (funcs *tagFuncs) Each_INIT(opts *map[string]interface{}, args *htmlArgs, precomp bool) [][]byte {
	//todo: return list of vars to ignore within function


	
	//? if preComp and not a constant var
	// return [][]byte{append([]byte{0}, []byte("myList as valueVar of keyVar")...), []byte("valueVar"), []byte("keyVar")}

	//? if we have the var and can run out func
	// return [][]byte{[]byte("valueVar"), []byte("keyVar")}

	// return list of args used by this func that should be ignored by the compiler
	return nil
}

// add "_SYNC" if this function should run in sync, rather than running async on a seperate channel
func (funcs *tagFuncs) Each_SYNC(opts *map[string]interface{}, args *htmlArgs, precomp bool) []byte {
	//todo: run each loop with args.args["body"] as the content

	//todo: find a way to handle if statements inside each loops and with args set by init
	// may need to add an if statement handler for use inside functions (handling {{%if value}}action1{{%else}}action2{{%/if}} compiler functions)

	//? if preComp and not a constant var
	// return append([]byte{0}, []byte("myList as valueVar of keyVar")...)

	//? if returning an error and stopping the compiler
	// return append([]byte{1}, []byte("error message")...)

	// return result content
	return nil
}
