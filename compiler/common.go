package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/AspieSoft/go-regex"
)

var varType map[string]reflect.Type

func init(){
	varType = map[string]reflect.Type{}

	varType["array"] = reflect.TypeOf([]interface{}{})
	varType["arrayByte"] = reflect.TypeOf([][]byte{})
	varType["map"] = reflect.TypeOf(map[string]interface{}{})
	varType["arrayEachFnObj"] = reflect.TypeOf([]eachFnObj{})

	varType["int"] = reflect.TypeOf(int(0))
	varType["float64"] = reflect.TypeOf(float64(0))
	varType["float32"] = reflect.TypeOf(float32(0))

	varType["string"] = reflect.TypeOf("")
	varType["byteArray"] = reflect.TypeOf([]byte{})
	varType["byte"] = reflect.TypeOf([]byte{0}[0])

	// int 32 returned instead of byte
	varType["int32"] = reflect.TypeOf(' ')

	varType["func"] = reflect.TypeOf(func(){})
	varType["tagFunc"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{} {return nil})
	varType["preTagFunc"] = reflect.TypeOf(func(map[string][]byte, int, fileData) interface{} {return nil})
}

func debug(msg ...interface{}) {
	fmt.Println("debug:", msg)
}

func joinPath(path ...string) (string, error) {
	resPath, err := filepath.Abs(path[0])
	if err != nil {
		return "", err
	}
	for i := 1; i < len(path); i++ {
		p := filepath.Join(resPath, path[i])
		if p == resPath || !strings.HasPrefix(p, resPath) {
			return "", errors.New("path leaked outside of root")
		}
		resPath = p
	}
	return resPath, nil
}

func contains(search []string, value string) bool {
	for _, v := range search {
		if v == value {
			return true
		}
	}
	return false
}

func containsMap(search map[string][]byte, value []byte) bool {
	for _, v := range search {
		if bytes.Equal(v, value) {
			return true
		}
	}
	return false
}

func toString(res interface{}) string {
	switch reflect.TypeOf(res) {
		case varType["string"]:
			return res.(string)
		case varType["byteArray"]:
			return string(res.([]byte))
		case varType["byte"]:
			return string(res.(byte))
		case varType["int32"]:
			return string(res.(int32))
		case varType["int"]:
			return strconv.Itoa(res.(int))
		case varType["float64"]:
			return strconv.FormatFloat(res.(float64), 'f', -1, 64)
		case varType["float32"]:
			return strconv.FormatFloat(float64(res.(float32)), 'f', -1, 32)
		default:
			return ""
	}
}

func IsZeroOfUnderlyingType(x interface{}) bool {
	// return x == nil || x == reflect.Zero(reflect.TypeOf(x)).Interface()
	return x == nil || reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func escapeHTML(html []byte) []byte {
	html = regex.RepFunc(html, `[<>&]`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("<")) {
			return []byte("&lt;")
		} else if bytes.Equal(data(0), []byte(">")) {
			return []byte("&gt;")
		}
		return []byte("&amp;")
	})
	return regex.RepStr(html, `&amp;(amp;)*`, []byte("&amp;"))
}

func escapeHTMLArgs(html []byte) []byte {
	return regex.RepFunc(html, `[\\"'\']`, func(data func(int) []byte) []byte {
		return append([]byte("\\"), data(0)...)
	})
}

func stringifyJSON(data interface{}) ([]byte, error) {
	json, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}
	json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	return json, nil
}

func stringifyJSONSpaces(data interface{}, ind int, pre int) ([]byte, error) {
	json, err := json.MarshalIndent(data, strings.Repeat(" ", pre), strings.Repeat(" ", ind))
	if err != nil {
		return []byte{}, err
	}
	json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	return json, nil
}

func compress(msg string) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(msg)); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func decompress(str string) string {
	data, _ := base64.StdEncoding.DecodeString(str)
	rdata := bytes.NewReader(data)
	r, _ := gzip.NewReader(rdata)
	s, _ := ioutil.ReadAll(r)
	return string(s)
}
