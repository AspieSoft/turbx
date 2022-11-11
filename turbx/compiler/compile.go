package compiler

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"turbx/compiler/funcs"

	"github.com/AspieSoft/go-regex/v3"
	"github.com/AspieSoft/goutil/v3"
)

var rootPath string
var rootTmpPath string
var fileExt string = "html"

func SetRoot(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	tmp, err := goutil.JoinPath(rootPath, "tmp")
	if err != nil {
		return err
	}

	rootPath = path
	rootTmpPath = tmp

	return nil
}

func SetExt(ext string) {
	if ext != "" {
		fileExt = string(regex.RepStr([]byte(ext), regex.Compile(`[^\w_-]`), []byte{}))
	}
}

type Test struct {
	Key string
	Value []byte
	Fn func(t int)
}

func PreCompile(path string, args map[string]interface{}) (string, error) {
	if rootPath == "" || rootTmpPath == "" {
		return "", errors.New("a root path was never chosen")
	}

	path, err := goutil.JoinPath(rootPath, path + "." + fileExt)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(path, rootTmpPath) {
		return "", errors.New("path leaked into tmp cache")
	}

	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer func(){
		file.Close()
	}()

	reader := bufio.NewReader(file)
	_ = reader

	// b, _ := reader.Peek(10)
	// fmt.Println(string(b))

	//todo: return temp file path (write result in temp folder)
	// also verify precompile is not calling in the temp folder
	return "", nil
}

func callFunc(name string, args *map[string][]byte, cont *[]byte, opts *map[string]interface{}) (interface{}, error) {
	name = string(regex.RepStr([]byte(name), regex.Compile(`[^\w_]`), []byte{}))

	var val []reflect.Value
	if opts == nil {
		var t funcs.Pre
		m := reflect.ValueOf(&t).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}

		val = m.Call([]reflect.Value{
			reflect.ValueOf(args),
			reflect.ValueOf(cont),
		})
	}else{
		var t funcs.Comp
		m := reflect.ValueOf(&t).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}

		val = m.Call([]reflect.Value{
			reflect.ValueOf(args),
			reflect.ValueOf(cont),
			reflect.ValueOf(opts),
		})
	}

	var data interface{}
	var err error
	if val[0].CanInterface() {
		data = val[0].Interface()
	}
	if val[1].CanInterface() {
		if e := val[1].Interface(); e != nil {
			err = e.(error)
		}
	}

	return data, err
}
