package main

import (
	"bytes"
	"compiler/common"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex"
	"github.com/alphadose/haxmap"
	"github.com/fsnotify/fsnotify"
	"github.com/jellydator/ttlcache/v3"
	"github.com/pbnjay/memory"
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
	path   string
}

type eachFnObj struct {
	html []byte
	opts map[string]interface{}
}

var fileCache *ttlcache.Cache[string, fileData]

// var OPTS map[string]string = map[string]string{}
var OPTS *haxmap.HashMap[string, string] = haxmap.New[string, string]()

var encKey string
var freeMem float64

func main() {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--enc=") {
			encKey, _ = common.Decompress(strings.Replace(arg, "--enc=", "", 1))
		}
	}

	freeMem = common.FormatMemoryUsage(memory.FreeMemory())
	go (func() {
		for {
			time.Sleep(1 * time.Millisecond)
			freeMem = common.FormatMemoryUsage(memory.FreeMemory())
		}
	})()

	common.VarType["arrayEachFnObj"] = reflect.TypeOf([]eachFnObj{})
	common.VarType["tagFunc"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{} { return nil })
	common.VarType["tagFuncPre"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData, int) (interface{}, bool) {
		return nil, false
	})
	common.VarType["preTagFunc"] = reflect.TypeOf(func(map[string][]byte, int, fileData) interface{} { return nil })

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
		}

		if encKey != "" {
			dec, err := common.Decrypt(input, encKey)
			if err != nil {
				continue
			}
			input = string(dec)
		}

		if input == "ping" {
			sendRes("pong")
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
		} else if strings.HasPrefix(input, "has:") {
			pre := strings.SplitN(input, ":", 2)[1]
			go checkCompiledCache(pre)
		} else if strings.ContainsRune(input, ':') {
			go runCompile(input)
		}
	}
}

func watchViewsReadSubFolder(watcher *fsnotify.Watcher, dir string) {
	// files, err := ioutil.ReadDir(dir)
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			if path, err := common.JoinPath(dir, file.Name()); err == nil {
				watcher.Add(path)
				watchViewsReadSubFolder(watcher, path)
			}
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
				filePath := event.Name

				stat, err := os.Stat(filePath)
				if err != nil {
					watcher.Remove(filePath)
					filePath = strings.Replace(strings.Replace(filePath, root, "", 1), "/", "", 1)
					fileCache.Delete(filePath)
					filePath = string(regex.RepStr([]byte(filePath), `\.[\w]+$`, []byte{}))
					fileCache.Delete(filePath)
				} else if stat.IsDir() {
					watcher.Add(filePath)
				} else {
					filePath = strings.Replace(strings.Replace(filePath, root, "", 1), "/", "", 1)

					fileCache.Delete(filePath)
					fileCache.Delete(filePath + ".pre")

					filePath = string(regex.RepStr([]byte(filePath), `\.[\w]+$`, []byte{}))
					fileCache.Delete(filePath)
					fileCache.Delete(filePath + ".pre")
				}

			}
		}
	}()

	err = watcher.Add(root)
	if err != nil {
		return
	}

	watchViewsReadSubFolder(watcher, root)

	<-done
}

func setOPT(key string, val string) {
	OPTS.Set(key, val)
}

func getOPT(key string) string {
	if val, ok := OPTS.Get(key); ok {
		return val
	}
	return ""
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
				if reflect.TypeOf(res) != common.VarType["array"] || !regex.Match(a, `^[0-9]+$`) {
					a = []byte(getOpt(opts, string(a), true).(string))
				}
			}

			if reflect.TypeOf(res) == common.VarType["array"] && regex.Match(a, `^[0-9]+$`) {
				i, err := strconv.Atoi(string(a))
				if err == nil && reflect.TypeOf(res) == common.VarType["array"] && len(res.([]interface{})) > i {
					res = res.([]interface{})[i]
				}
			} else if reflect.TypeOf(res) == common.VarType["map"] {
				res = res.(map[string]interface{})[string(a)]
			} else {
				res = nil
				break
			}

			if t := reflect.TypeOf(res); t != common.VarType["map"] && t != common.VarType["array"] {
				break
			}
		}

		if res != nil && res != false {
			if stringOutput {
				if t := reflect.TypeOf(res); t != common.VarType["map"] && t != common.VarType["array"] {
					break
				}
			} else {
				break
			}
		}
	}

	if stringOutput {
		switch reflect.TypeOf(res) {
		case common.VarType["string"]:
			return string(res.(string))
		case common.VarType["byteArray"]:
			return string(res.([]byte))
		case common.VarType["byte"]:
			return string(res.(byte))
		case common.VarType["int32"]:
			return string(res.(int32))
		case common.VarType["int"]:
			return strconv.Itoa(res.(int))
		case common.VarType["float64"]:
			return strconv.FormatFloat(res.(float64), 'f', -1, 64)
		case common.VarType["float32"]:
			return strconv.FormatFloat(float64(res.(float32)), 'f', -1, 32)
		default:
			return ""
		}
	}

	return res
}

func sendRes(res string) {
	if encKey != "" {
		enc, err := common.Encrypt([]byte(res), encKey)
		if err != nil {
			return
		}
		fmt.Println(enc)
		return
	}
	fmt.Println(res)
}

func runPreCompile(input string) {
	inputData := strings.SplitN(input, ":", 2)

	for freeMem < 10 {
		time.Sleep(1 * time.Millisecond)
	}

	_, err := getFile(inputData[1], false, true)
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	sendRes(inputData[0] + ":success")
}

func checkCompiledCache(input string) {
	inputData := strings.SplitN(input, ":", 2)

	cache := fileCache.Get(inputData[1] + ".pre")
	if cache != nil {
		sendRes(inputData[0] + ":true")
	} else {
		sendRes(inputData[0] + ":false")
	}
}

func runCompile(input string) {
	inputData := strings.SplitN(input, ":", 3)

	for freeMem < 10 {
		time.Sleep(1 * time.Millisecond)
	}

	optStr, err := common.Decompress(inputData[1])
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	opts := map[string]interface{}{}
	err = json.Unmarshal([]byte(optStr), &opts)
	if err != nil {
		opts = map[string]interface{}{}
	}

	pre := 0
	var preCompileConst map[string]interface{}
	if opts["PreCompile"] == true {
		pre = 1
	}else if reflect.TypeOf(opts["const"]) == common.VarType["map"] {
		pre = 1
		// preCompileConst = common.CopyMap(opts["const"].(map[string]interface{}))
		preCompileConst = opts["const"].(map[string]interface{})
		for key, val := range preCompileConst {
			if _, ok := opts[key]; !ok {
				opts[key] = val
			}
		}
		preCompileConst["PreCompile"] = true
		delete(opts, "const")
	}

	file, err := getFile(inputData[2], false, true)
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	var out []byte
	if preCompileConst != nil {
		out = compile(file, opts, true, true, 0)
		go compile(file, preCompileConst, true, true, pre)
	}else{
		out = compile(file, opts, true, true, pre)
	}

	resOut, err := common.Compress(string(out))
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	sendRes(inputData[0] + ":" + resOut)
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
		path, err = common.JoinPath(root, compRoot, filePath+"."+ext)
		if err == nil {
			// html, err = ioutil.ReadFile(path)
			html, err = os.ReadFile(path)
			if err != nil {
				html = nil
			}
		}

		if html == nil {
			path, err = common.JoinPath(root, filePath+"."+ext)
			if err == nil {
				// html, err = ioutil.ReadFile(path)
				html, err = os.ReadFile(path)
				if err != nil {
					html = nil
				}
			}
		}
	}

	if html == nil && allowImport {
		path, err = common.JoinPath(root, filePath+"."+ext)
		if err == nil {
			// html, err = ioutil.ReadFile(path)
			html, err = os.ReadFile(path)
			if err != nil {
				html = nil
			}
		}
	}

	if html == nil {
		return fileData{}, err
	}

	// pre compile
	file, err := preCompile(html, filePath)
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
