package compiler

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"turbx/funcs"

	"github.com/AspieSoft/go-regex/v3"
	"github.com/AspieSoft/goutil/v3"
)

var rootPath string
var fileExt string = "html"

var cacheTmpPath string

func init(){
	dir, err := os.MkdirTemp("", "turbx-cache." + string(randBytes(16, nil)) + ".")
	if err != nil {
		panic(err)
	}
	cacheTmpPath = dir

	SetRoot("views")

	go clearTmpCache()
}

func Close(){
	os.RemoveAll(cacheTmpPath)
}

func clearTmpCache(){
	if cacheTmpPath == "" {
		return
	}

	if dir, err := os.ReadDir(cacheTmpPath); err == nil {
		for _, file := range dir {
			if p, err := goutil.JoinPath(cacheTmpPath, file.Name()); err != nil {
				os.RemoveAll(p)
			}
		}
	}
}

func SetRoot(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	rootPath = path

	go clearTmpCache()

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

func PreCompile(path string, opts map[string]interface{}) (string, error) {
	if rootPath == "" || cacheTmpPath == "" {
		return "", errors.New("a root path was never chosen")
	}

	path, err := goutil.JoinPath(rootPath, path + "." + fileExt)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(path, cacheTmpPath) {
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

	tmpFile, tmpPath, err := tmpPath()
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	writer := bufio.NewWriter(tmpFile)

	_ = reader
	_ = writer

	// b, _ := reader.Peek(10)
	// fmt.Println(string(b))

	//todo: compile components and pre funcs
	//todo: convert other funcs to use {{#if}} in place of <_if>
	//todo: compile const vars (leave unhandled vars for main compile method)

	//// may read multiple bytes at a time, and check if they contain '<' in the first check (define read size with a const var)
	//todo: compile markdown while reading file (may ignore above comment for this idea)

	//todo: return temp file path (write result in temp folder)
	return tmpPath, nil
}

func callFunc(name string, args *map[string][]byte, cont *[]byte, opts *map[string]interface{}, pre bool) (interface{}, error) {
	name = string(regex.RepStr([]byte(name), regex.Compile(`[^\w_]`), []byte{}))

	var m reflect.Value
	if pre {
		var t funcs.Pre
		m := reflect.ValueOf(&t).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}
	}else{
		var t funcs.Comp
		m := reflect.ValueOf(&t).MethodByName(name)
		if goutil.IsZeroOfUnderlyingType(m) {
			return nil, errors.New("method does not exist in Pre Compiled Functions")
		}
	}

	val := m.Call([]reflect.Value{
		reflect.ValueOf(args),
		reflect.ValueOf(cont),
		reflect.ValueOf(opts),
	})

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

var addingTmpPath int = 0
func tmpPath(tries ...int) (*os.File, string, error) {
	for addingTmpPath >= 10 {
		time.Sleep(100 * time.Nanosecond)
	}
	addingTmpPath++
	time.Sleep(10 * time.Nanosecond)
	if addingTmpPath > 10 {
		addingTmpPath--

		if len(tries) != 0 && tries[0] > 10000 {
			return nil, "", errors.New("failed to queue the generation of a unique tmp cache path within 10000 tries")
		}

		time.Sleep(1000 * time.Nanosecond)

		if len(tries) != 0 {
			return tmpPath(tries[0] + 1)
		}
		return tmpPath(1)
	}

	now := time.Now().UnixNano()
	t := strconv.Itoa(int(now))
	t = t[len(t)-12:]

	time.Sleep(10 * time.Nanosecond)
	addingTmpPath--

	tmp := randBytes(32, nil)

	path, err := goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)

	loops := 0
	for err != nil {
		if _, e := os.Stat(path); e != nil {
			break
		}

		loops++
		if loops > 10000 {
			return nil, "", errors.New("failed to generate a unique tmp cache path within 10000 tries")
		}

		tmp = randBytes(32, nil)
		path, err = goutil.JoinPath(cacheTmpPath, string(tmp) + "." + t + "." + fileExt)
	}

	if _, err := os.Stat(path); err == nil {
		return nil, "", errors.New("failed to generate a unique tmp cache path")
	}

	// err = os.WriteFile(path, []byte{}, 1600)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 1600)
	if err != nil {
		return nil, path, err
	}

	return file, path, nil
}

func randBytes(size int, exclude ...[]byte) []byte {
	b := make([]byte, size)
	rand.Read(b)
	b = []byte(base64.URLEncoding.EncodeToString(b))

	if len(exclude) >= 2 {
		if exclude[0] == nil || len(exclude[0]) == 0 {
			b = regex.RepStr(b, regex.Compile(`[^\w_-]`), exclude[1])
		}else{
			b = regex.RepStr(b, regex.Compile(`[%1]`, string(exclude[0])), exclude[1])
		}
	}else if len(exclude) >= 1 {
		if exclude[0] == nil || len(exclude[0]) == 0 {
			b = regex.RepStr(b, regex.Compile(`[^\w_-]`), []byte{})
		}else{
			b = regex.RepStr(b, regex.Compile(`[%1]`, string(exclude[0])), []byte{})
		}
	}

	for len(b) < size {
		a := make([]byte, size)
		rand.Read(a)
		a = []byte(base64.URLEncoding.EncodeToString(a))
	
		if len(exclude) >= 2 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.RepStr(a, regex.Compile(`[^\w_-]`), exclude[1])
			}else{
				a = regex.RepStr(a, regex.Compile(`[%1]`, string(exclude[0])), exclude[1])
			}
		}else if len(exclude) >= 1 {
			if exclude[0] == nil || len(exclude[0]) == 0 {
				a = regex.RepStr(a, regex.Compile(`[^\w_-]`), []byte{})
			}else{
				a = regex.RepStr(a, regex.Compile(`[%1]`, string(exclude[0])), []byte{})
			}
		}

		b = append(b, a...)
	}

	return b[:size]
}
