package common

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/AspieSoft/go-regex"
)

var VarType map[string]reflect.Type

func init(){
	VarType = map[string]reflect.Type{}

	VarType["array"] = reflect.TypeOf([]interface{}{})
	VarType["arrayByte"] = reflect.TypeOf([][]byte{})
	VarType["arrayString"] = reflect.TypeOf([]string{})
	VarType["map"] = reflect.TypeOf(map[string]interface{}{})
	// VarType["arrayEachFnObj"] = reflect.TypeOf([]eachFnObj{})

	VarType["int"] = reflect.TypeOf(int(0))
	VarType["float64"] = reflect.TypeOf(float64(0))
	VarType["float32"] = reflect.TypeOf(float32(0))

	VarType["string"] = reflect.TypeOf("")
	VarType["byteArray"] = reflect.TypeOf([]byte{})
	VarType["byte"] = reflect.TypeOf([]byte{0}[0])

	// int 32 returned instead of byte
	VarType["int32"] = reflect.TypeOf(' ')

	// VarType["tagFunc"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{} {return nil})
	// VarType["preTagFunc"] = reflect.TypeOf(func(map[string][]byte, int, fileData) interface{} {return nil})
	VarType["func"] = reflect.TypeOf(func(){})
}

func Debug(msg ...interface{}) {
	fmt.Println("debug:", msg)
}

func JoinPath(path ...string) (string, error) {
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

func Contains(search []string, value string) bool {
	for _, v := range search {
		if v == value {
			return true
		}
	}
	return false
}

func ContainsMap(search map[string][]byte, value []byte) bool {
	for _, v := range search {
		if bytes.Equal(v, value) {
			return true
		}
	}
	return false
}

func ToString(res interface{}) string {
	switch reflect.TypeOf(res) {
		case VarType["string"]:
			return res.(string)
		case VarType["byteArray"]:
			return string(res.([]byte))
		case VarType["byte"]:
			return string(res.(byte))
		case VarType["int32"]:
			return string(res.(int32))
		case VarType["int"]:
			return strconv.Itoa(res.(int))
		case VarType["float64"]:
			return strconv.FormatFloat(res.(float64), 'f', -1, 64)
		case VarType["float32"]:
			return strconv.FormatFloat(float64(res.(float32)), 'f', -1, 32)
		default:
			return ""
	}
}

func IsZeroOfUnderlyingType(x interface{}) bool {
	// return x == nil || x == reflect.Zero(reflect.TypeOf(x)).Interface()
	return x == nil || reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func FormatMemoryUsage(b uint64) float64 {
	return math.Round(float64(b) / 1024 / 1024 * 100) / 100
}

func EscapeHTML(html []byte) []byte {
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

func EscapeHTMLArgs(html []byte) []byte {
	return regex.RepFunc(html, `[\\"'\']`, func(data func(int) []byte) []byte {
		return append([]byte("\\"), data(0)...)
	})
}

func StringifyJSON(data interface{}) ([]byte, error) {
	json, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}
	json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	return json, nil
}

func StringifyJSONSpaces(data interface{}, ind int, pre int) ([]byte, error) {
	json, err := json.MarshalIndent(data, strings.Repeat(" ", pre), strings.Repeat(" ", ind))
	if err != nil {
		return []byte{}, err
	}
	json = bytes.ReplaceAll(json, []byte("\\u003c"), []byte("<"))
	json = bytes.ReplaceAll(json, []byte("\\u003e"), []byte(">"))

	return json, nil
}

func ParseJson(b []byte) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	err := json.Unmarshal(b, &res)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return res, nil
}

func DecodeJSON(data io.Reader) (map[string]interface{}, error) {
	var res map[string]interface{}
	err := json.NewDecoder(data).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func Compress(msg string) (string, error) {
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

func CompressByte(msg []byte) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(msg); err != nil {
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

func Decompress(str string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	rdata := bytes.NewReader(data)
	r, err := gzip.NewReader(rdata)
	if err != nil {
		return "", err
	}
	// s, err := ioutil.ReadAll(r)
	s, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func Encrypt(plaintext []byte, key string) (string, error) {
	keyHash := sha256.Sum256([]byte(key))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", nil
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(text string, key string) ([]byte, error) {
	keyHash := sha256.Sum256([]byte(key))

	ciphertext, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return []byte{}, err
	}

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return []byte{}, err
	}

	if len(ciphertext) < aes.BlockSize {
		return []byte{}, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)

	stream.XORKeyStream(ciphertext, ciphertext)
	return ciphertext, nil
}

func CleanStr(str string) string {
	//todo: sanitize inputs
	str = strings.ToValidUTF8(str, "")
	return str
}

func CleanArray(data []interface{}) []interface{} {
	cData := []interface{}{}
	for key, val := range data {
		t := reflect.TypeOf(val)
		if t == VarType["string"] {
			cData[key] = CleanStr(val.(string))
		}else if t == VarType["int"] || t == VarType["float64"] || t == VarType["float32"] || t == VarType["bool"] {
			cData[key] = val
		}else if t == VarType["byteArray"] {
			cData[key] = CleanStr(string(val.([]byte)))
		}else if t == VarType["byte"] {
			cData[key] = CleanStr(string(val.(byte)))
		}else if t == VarType["int32"] {
			cData[key] = CleanStr(string(val.(int32)))
		}else if t == VarType["array"] {
			cData[key] = CleanArray(val.([]interface{}))
		}else if t == VarType["map"] {
			cData[key] = CleanMap(val.(map[string]interface{}))
		}
	}
	return cData
}

func CleanMap(data map[string]interface{}) map[string]interface{} {
	cData := map[string]interface{}{}
	for key, val := range data {
		key = CleanStr(key)

		t := reflect.TypeOf(val)
		if t == VarType["string"] {
			cData[key] = CleanStr(val.(string))
		}else if t == VarType["int"] || t == VarType["float64"] || t == VarType["float32"] || t == VarType["bool"] {
			cData[key] = val
		}else if t == VarType["byteArray"] {
			cData[key] = CleanStr(string(val.([]byte)))
		}else if t == VarType["byte"] {
			cData[key] = CleanStr(string(val.(byte)))
		}else if t == VarType["int32"] {
			cData[key] = CleanStr(string(val.(int32)))
		}else if t == VarType["array"] {
			cData[key] = CleanArray(val.([]interface{}))
		}else if t == VarType["map"] {
			cData[key] = CleanMap(val.(map[string]interface{}))
		}
	}

	return cData
}

func CleanJSON(val interface{}) interface{} {
	t := reflect.TypeOf(val)
	if t == VarType["string"] {
		return CleanStr(val.(string))
	}else if t == VarType["int"] || t == VarType["float64"] || t == VarType["float32"] || t == VarType["bool"] {
		return val
	}else if t == VarType["byteArray"] {
		return CleanStr(string(val.([]byte)))
	}else if t == VarType["byte"] {
		return CleanStr(string(val.(byte)))
	}else if t == VarType["int32"] {
		return CleanStr(string(val.(int32)))
	}else if t == VarType["array"] {
		return CleanArray(val.([]interface{}))
	}else if t == VarType["map"] {
		return CleanMap(val.(map[string]interface{}))
	}
	return nil
}
