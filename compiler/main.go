package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex"
	"github.com/fsnotify/fsnotify"
	"github.com/jellydator/ttlcache/v3"
)

type stringObj struct {
	s []byte
	q byte
}

type scriptObj struct {
	tag  byte
	args []byte
	cont []byte
}

type fileData struct {
	html   []byte
	args   []map[string][]byte
	str    [][]byte
	script []scriptObj
}

type eachFnObj struct {
	html []byte
	opts map[string]interface{}
}

var fileCache *ttlcache.Cache[string, fileData]

var OPTS map[string]string = map[string]string{}

func main() {

	fileCache = ttlcache.New[string, fileData](
		ttlcache.WithTTL[string, fileData](2 * time.Hour),
	)

	go fileCache.Start()

	go (func() {
		for {
			time.Sleep(2 * time.Hour)
			fileCache.DeleteExpired()
		}
	})()

	userInput := make(chan string)
	go readInput(userInput)

	for {
		input := <-userInput

		if input == "ping" {
			fmt.Println("pong")
		} else if input == "stop" || input == "exit" {
			break
		} else if strings.HasPrefix(input, "set:") && strings.ContainsRune(input, '=') {
			opt := strings.SplitN(strings.SplitN(input, ":", 2)[1], "=", 2)
			setOPT(opt[0], opt[1])
			if opt[0] == "root" {
				go watchViews(opt[1])
			}
		} else if strings.HasPrefix(input, "pre:") {
			pre := strings.SplitN(input, ":", 2)[1]
			go runPreCompile(pre)
		} else if strings.ContainsRune(input, ':') {
			go runCompile(input)
		}
	}
}

func watchViews(root string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			if event, ok := <-watcher.Events; ok {
				filePath := strings.Replace(strings.Replace(event.Name, root, "", 1), "/", "", 1)
				fileCache.Delete(filePath)
				filePath = string(regex.RepStr([]byte(filePath), `\.[\w]+$`, []byte{}))
				fileCache.Delete(filePath)
			}
		}
	}()

	err = watcher.Add(root)
	if err != nil {
		return
	}
	<-done
}

var writingOpts int = 0
var readingOpts int = 0

func setOPT(key string, val string) {
	writingOpts++
	for readingOpts != 0 {
		time.Sleep(1000)
	}
	OPTS[key] = val
	writingOpts--
}

func getOPT(key string) string {
	for writingOpts != 0 {
		time.Sleep(1000)
	}
	readingOpts++
	opt := OPTS[key]
	readingOpts--
	return opt
}

func readInput(input chan<- string) {
	for {
		var u string
		_, err := fmt.Scanf("%s\n", &u)
		if err == nil {
			input <- u
		}
	}
}

func getOpt(opts map[string]interface{}, arg string, stringOutput bool) interface{} {
	var res interface{}
	res = nil

	argOpts := strings.Split(arg, "|")
	for _, arg := range argOpts {
		res = opts
		args := regex.Split(regex.RepStr([]byte(arg), `\s+`, []byte{}), `\.|(\[.*?\])`)
		for _, a := range args {
			if regex.Match(a, `^%![0-9]+!%$`) {
				return string(a)
			}

			if bytes.HasPrefix(a, []byte("[")) && bytes.HasSuffix(a, []byte("]")) {
				a = a[1 : len(a)-2]
				if reflect.TypeOf(res) != varType["array"] || !regex.Match(a, `^[0-9]+$`) {
					a = []byte(getOpt(opts, string(a), true).(string))
				}
			}

			if reflect.TypeOf(res) == varType["array"] && regex.Match(a, `^[0-9]+$`) {
				i, err := strconv.Atoi(string(a))
				if err == nil && reflect.TypeOf(res) == varType["array"] && len(res.([]interface{})) > i {
					res = res.([]interface{})[i]
				}
			} else if reflect.TypeOf(res) == varType["map"] {
				res = res.(map[string]interface{})[string(a)]
			} else {
				res = nil
				break
			}

			if t := reflect.TypeOf(res); t != varType["map"] && t != varType["array"] {
				break
			}
		}

		if res != nil && res != false {
			if stringOutput {
				if t := reflect.TypeOf(res); t != varType["map"] && t != varType["array"] {
					break
				}
			} else {
				break
			}
		}
	}

	if stringOutput {
		switch reflect.TypeOf(res) {
		case varType["string"]:
			return string(res.(string))
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

	return res
}

func runPreCompile(input string) {
	inputData := strings.SplitN(input, ":", 2)

	_, err := getFile(inputData[1], false, true)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	fmt.Println(inputData[0] + ":success")
}

func runCompile(input string) {
	inputData := strings.SplitN(input, ":", 3)

	optStr, err := decompress(inputData[1])
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	opts := map[string]interface{}{}
	err = json.Unmarshal([]byte(optStr), &opts)
	if err != nil {
		opts = map[string]interface{}{}
	}

	file, err := getFile(inputData[2], false, true)
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	out := compile(file, opts, true, true)

	resOut, err := compress(string(out))
	if err != nil {
		fmt.Println(inputData[0] + ":error")
		return
	}

	fmt.Println(inputData[0] + ":" + resOut)
}

func getFile(filePath string, component bool, allowImport bool) (fileData, error) {

	cache := fileCache.Get(filePath)
	if cache != nil {
		return cache.Value(), nil
	}

	// init options
	root := getOPT("root")
	if root == "" {
		return fileData{}, errors.New("root not found")
	}

	ext := getOPT("ext")
	if ext == "" {
		ext = "xhtml"
	}

	compRoot := getOPT("components")
	if compRoot == "" {
		compRoot = "components"
	}

	var html []byte = nil
	var path string
	var err error

	// try files
	if component {
		path, err = joinPath(root, compRoot, filePath+"."+ext)
		if err == nil {
			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}

		if html == nil {
			path, err = joinPath(root, filePath+"."+ext)
			if err == nil {
				html, err = ioutil.ReadFile(path)
				if err != nil {
					html = nil
				}
			}
		}
	}

	if html == nil && allowImport {
		path, err = joinPath(root, filePath+"."+ext)
		if err == nil {
			html, err = ioutil.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	if html == nil {
		return fileData{}, err
	}

	// pre compile
	file, err := preCompile(html)
	if err != nil {
		return fileData{}, err
	}

	cacheTime := ttlcache.DefaultTTL
	if cache := getOPT("cache"); cache != "" {
		if c, err := time.ParseDuration(cache); err == nil {
			cacheTime = c * time.Millisecond
		}
	}
	fileCache.Set(filePath, file, cacheTime)

	return file, nil
}

func encodeEncoding(html []byte) []byte {
	return regex.RepFunc(html, `%!|!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(0), []byte("%!")) {
			return []byte("%!o!%")
		}
		return []byte("%!c!%")
	})
}

func decodeEncoding(html []byte) []byte {
	return regex.RepFunc(html, `%!([oc])!%`, func(data func(int) []byte) []byte {
		if bytes.Equal(data(1), []byte("o")) {
			return []byte("%!")
		}
		return []byte("!%")
	})
}
