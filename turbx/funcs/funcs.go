package funcs

type Pre struct {}
type Comp struct {}

func (t *Pre) PreFn(args *map[string][]byte, cont *[]byte) (interface{}, error) {
	return nil, nil
}

func (t *Comp) CompFn(args *map[string][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	return nil, nil
}
