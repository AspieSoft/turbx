package compiler

type tagFuncs struct {}
var TagFuncs tagFuncs = tagFuncs{}

var testInd int = 0

func (funcs *tagFuncs) If(opts *map[string]interface{}, args *htmlArgs, precomp bool) ([]byte, bool) {
	//todo: handle and reduce if statement args
	// if precomp == true, return leftover args
	// if precomp == false, assume leftover args are false

	// return nil, false (absolute false)
	// return nil, true (absolute true)
	// return []byte("args"), true (push to compiler)
	// @[]byte: precomp result, @bool: if true
	return nil, false
}
