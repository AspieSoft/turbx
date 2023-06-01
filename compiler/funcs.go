package compiler

type tagFuncs struct {}

func (funcs *tagFuncs) If(opts *map[string]interface{}, args *htmlArgs) bool {
	return false
}
