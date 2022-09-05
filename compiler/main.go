package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex"
	"github.com/AspieSoft/go-ttlcache"
	"github.com/AspieSoft/goutil"
	"github.com/alphadose/haxmap"
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

var DebugMode bool = false

//todo: update GithubAssetURL version when updating module
var GithubAssetURL = "https://cdn.jsdelivr.net/gh/AspieSoft/turbx@0.4.4/assets"

func main() {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--enc=") {
			encKey, _ = goutil.Decompress(strings.Replace(arg, "--enc=", "", 1))
		} else if strings.HasPrefix(arg, "--debug") {
			DebugMode = true
		}
	}

	if DebugMode {
		GithubAssetURL = "/assets"
	}

	freeMem = goutil.FormatMemoryUsage(memory.FreeMemory())
	go (func() {
		for {
			time.Sleep(1 * time.Millisecond)
			freeMem = goutil.FormatMemoryUsage(memory.FreeMemory())
		}
	})()

	goutil.VarType["arrayEachFnObj"] = reflect.TypeOf([]eachFnObj{})
	goutil.VarType["tagFunc"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData) interface{} { return nil })
	goutil.VarType["tagFuncPre"] = reflect.TypeOf(func(map[string][]byte, []byte, map[string]interface{}, int, fileData, int) (interface{}, bool) {
		return nil, false
	})
	goutil.VarType["preTagFunc"] = reflect.TypeOf(func(map[string][]byte, int, fileData, bool) interface{} { return nil })

	cacheTime := 2 * time.Hour
	if cache := getOPT("cache"); cache != "" {
		if c, err := time.ParseDuration(cache); err == nil {
			cacheTime = c * time.Millisecond
		}
	}

	fileCache = ttlcache.New[string, fileData](
		cacheTime,
		4 * time.Hour,
	)

	userInput := make(chan string)
	go readInput(userInput)

	for {
		input := <-userInput

		if input == "ping" {
			fmt.Println("pong")
		}

		if encKey != "" {
			dec, err := goutil.Decrypt(input, encKey)
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
			}else if opt[0] == "cache" {
				if i, err := strconv.Atoi(opt[1]); err == nil {
					fileCache.TTL(time.Duration(i) * time.Millisecond)
				}
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

func debug(msg ...interface{}) {
	fmt.Println("debug:", msg)
}

func watchViews(root string){
	goutil.WatchDir(root, &goutil.Watcher{
		FileChange: func(path, op string) {
			path = strings.Replace(strings.Replace(path, root, "", 1), "/", "", 1)

			fileCache.Del(path)
			fileCache.Del(path + ".pre")

			path = string(regex.RepStr([]byte(path), `\.[\w]+$`, []byte{}))
			fileCache.Del(path)
			fileCache.Del(path + ".pre")
		},
		Remove: func(path, op string) (removeWatcher bool) {
			path = strings.Replace(strings.Replace(path, root, "", 1), "/", "", 1)
			fileCache.Del(path)
			path = regex.RepStr(path, `\.[\w]+$`, "")
			fileCache.Del(path)
			return true
		},
	})
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
				if reflect.TypeOf(res) != goutil.VarType["array"] || !regex.Match(a, `^[0-9]+$`) {
					a = []byte(getOpt(opts, string(a), true).(string))
				}
			}

			if reflect.TypeOf(res) == goutil.VarType["array"] && regex.Match(a, `^[0-9]+$`) {
				i, err := strconv.Atoi(string(a))
				if err == nil && reflect.TypeOf(res) == goutil.VarType["array"] && len(res.([]interface{})) > i {
					res = res.([]interface{})[i]
				}
			} else if reflect.TypeOf(res) == goutil.VarType["map"] {
				res = res.(map[string]interface{})[string(a)]
			} else {
				res = nil
				break
			}

			if t := reflect.TypeOf(res); t != goutil.VarType["map"] && t != goutil.VarType["array"] {
				break
			}
		}

		if res != nil && res != false {
			if stringOutput {
				if t := reflect.TypeOf(res); t != goutil.VarType["map"] && t != goutil.VarType["array"] {
					break
				}
			} else {
				break
			}
		}
	}

	if stringOutput {
		switch reflect.TypeOf(res) {
		case goutil.VarType["string"]:
			return string(res.(string))
		case goutil.VarType["byteArray"]:
			return string(res.([]byte))
		case goutil.VarType["byte"]:
			return string(res.(byte))
		case goutil.VarType["int32"]:
			return string(res.(int32))
		case goutil.VarType["int"]:
			return strconv.Itoa(res.(int))
		case goutil.VarType["float64"]:
			return strconv.FormatFloat(res.(float64), 'f', -1, 64)
		case goutil.VarType["float32"]:
			return strconv.FormatFloat(float64(res.(float32)), 'f', -1, 32)
		default:
			return ""
		}
	}

	return res
}

func sendRes(res string) {
	if encKey != "" {
		enc, err := goutil.Encrypt([]byte(res), encKey)
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

	_, err := getFile(inputData[1], false, true, false)
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	sendRes(inputData[0] + ":success")
}

func checkCompiledCache(input string) {
	inputData := strings.SplitN(input, ":", 2)

	if _, ok := fileCache.Get(inputData[1] + ".pre"); ok {
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

	optStr, err := goutil.Decompress(inputData[1])
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
	}else if reflect.TypeOf(opts["const"]) == goutil.VarType["map"] {
		pre = 1
		// preCompileConst = goutil.CopyMap(opts["const"].(map[string]interface{}))
		preCompileConst = opts["const"].(map[string]interface{})
		for key, val := range preCompileConst {
			if _, ok := opts[key]; !ok {
				opts[key] = val
			}
		}
		preCompileConst["PreCompile"] = true
		delete(opts, "const")
	}

	file, err := getFile(inputData[2], false, true, true)
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

	resOut, err := goutil.Compress(string(out))
	if err != nil {
		sendRes(inputData[0] + ":error")
		return
	}

	sendRes(inputData[0] + ":" + resOut)
}

func getFile(filePath string, component bool, allowImport bool, fastMode bool) (fileData, error) {
	if !fastMode {
		if cache, ok := fileCache.Get(filePath); ok {
			return cache, nil
		}
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
		path, err = goutil.JoinPath(root, compRoot, filePath+"."+ext)
		if err == nil {
			// html, err = ioutil.ReadFile(path)
			html, err = os.ReadFile(path)
			if err != nil {
				html = nil
			}
		}

		if html == nil {
			path, err = goutil.JoinPath(root, filePath+"."+ext)
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
		path, err = goutil.JoinPath(root, filePath+"."+ext)
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
	file, err := preCompile(html, filePath, fastMode)
	if err != nil {
		return fileData{}, err
	}

	fileCache.Set(filePath, file)

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
