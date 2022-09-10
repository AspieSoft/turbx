package main

import (
	"bytes"
	"net/http"
	"reflect"
	"strconv"

	"github.com/AspieSoft/go-regex/v2"
	"github.com/AspieSoft/goutil/v2"
	lorem "github.com/drhodes/golorem"
)

var preTagFuncs map[string]interface{} = map[string]interface{} {
	"lorem": func(arg *map[string][]byte, level int, file *fileData, fastMode bool) interface{} {
		args := *arg

		wType := byte('p')
		if len(args["type"]) != 0 {
			wType = args["type"][0]
		}else if len(args["0"]) != 0 && !regex.Match(args["0"], `^[0-9]+$`) {
			wType = args["0"][0]
		}else if len(args["1"]) != 0 && !regex.Match(args["1"], `^[0-9]+$`) {
			wType = args["1"][0]
		}else if len(args["2"]) != 0 && !regex.Match(args["2"], `^[0-9]+$`) {
			wType = args["2"][0]
		}

		minLen := 1
		maxLen := 10
		used := -1

		if len(args["min"]) != 0 {
			used = -2
			i, err := strconv.Atoi(string(args["min"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["0"]) != 0 && regex.Match(args["0"], `^[0-9]+$`) {
			used = 0
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["1"]) != 0 && regex.Match(args["1"], `^[0-9]+$`) {
			used = 1
			i, err := strconv.Atoi(string(args["1"]))
			if err == nil {
				minLen = i
			}
		}else if len(args["2"]) != 0 && regex.Match(args["2"], `^[0-9]+$`) {
			used = 2
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}

		if len(args["max"]) != 0 {
			i, err := strconv.Atoi(string(args["max"]))
			if err == nil {
				minLen = i
			}
		}else if used != 0 && len(args["0"]) != 0 && regex.Match(args["0"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if used != 1 && len(args["1"]) != 0 && regex.Match(args["1"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["1"]))
			if err == nil {
				minLen = i
			}
		}else if used != 2 && len(args["2"]) != 0 && regex.Match(args["2"], `^[0-9]+$`) {
			i, err := strconv.Atoi(string(args["0"]))
			if err == nil {
				minLen = i
			}
		}else if used != -1 {
			maxLen = minLen
		}

		if wType == 'p' {
			return []byte(lorem.Paragraph(minLen, maxLen))
		} else if wType == 'w' {
			return []byte(lorem.Word(minLen, maxLen))
		} else if wType == 's' {
			return []byte(lorem.Sentence(minLen, maxLen))
		} else if wType == 'h' {
			return []byte(lorem.Host())
		} else if wType == 'e' {
			return []byte(lorem.Email())
		} else if wType == 'u' {
			return []byte(lorem.Url())
		}

		return []byte(lorem.Paragraph(minLen, maxLen))
	},

	"youtube": func(args *map[string][]byte, level int, file *fileData, fastMode bool) interface{} {
		url := (*args)["url"]
		url = regex.RepFuncRef(&url, `^(["'\'])(.*)\1$`, func(data func(int) []byte) []byte {
			return data(2)
		})
		url = regex.RepFuncRef(&url, `^(?:https?://|)(?:www\.|)(?:youtube\.com|youtu\.be)/(?:watch/?\?v=|playlist/?\?list=|channel/?|)(.*)$`, func(data func(int) []byte) []byte {
			return data(1)
		})
		if regex.MatchRef(&url, `^.*?&list=`) {
			url = regex.RepStrRef(&url, `^.*?&list=`, []byte{})
		}
		url = regex.RepStrRef(&url, `(\?|&).*$`, []byte{})

		if fastMode {
			return regex.JoinBytes([]byte(`<div class="youtube-embed youtube-embed-client" src="`), url, []byte(`"><img class="youtube-embed-play-btn" src="`+GithubAssetURL+`/youtube.png"/></div>`))
		}

		//todo: consider running "http.Get" as client fetch call, rather than taking up server resources and slowing down for requests to youtube
		// may also run this as a goroutine

		videoData := map[string][]byte{}
		if bytes.HasPrefix(url, []byte("c/")) {
			//todo: get embed url for custom channel url
			// url = bytes.Replace(url, []byte("c/"), []byte{}, 1)
			return []byte{}
		}else if bytes.HasPrefix(url, []byte("UC")) || bytes.HasPrefix(url, []byte("UU")) {
			var vidUrl string
			if bytes.HasPrefix(url, []byte("UC")) {
				vidUrl = string(bytes.Replace(url, []byte("UC"), []byte{}, 1))
			} else {
				vidUrl = string(bytes.Replace(url, []byte("UU"), []byte{}, 1))
			}

			//todo: fix checking if video can be embedded
			// for this below method (commented out): youtube always returns 200 ok responce (this method is also slow)
			/* resHead, err := http.Head("https://www.youtube.com/embed/?list=UU" + vidUrl)
			if err != nil {
				return []byte{}
			}

			if resHead.StatusCode == 200 {
				vidUrl = "UU" + vidUrl
			}else{
				vidUrl = "PU" + vidUrl
			} */

			vidUrl = "UU" + vidUrl

			res, err := http.Get("https://www.youtube.com/oembed?url=https://www.youtube.com/playlist?list="+vidUrl+"&format=json")
			if err != nil {
				return []byte{}
			}
			body, _ := goutil.DecodeJSON(res.Body)

			videoData["url"] = append([]byte("https://www.youtube.com/embed/?list="), url...)

			if reflect.TypeOf(body["thumbnail_url"]) == goutil.VarType["string"] {
				videoData["img"] = []byte(body["thumbnail_url"].(string))
			}
			if reflect.TypeOf(body["title"]) == goutil.VarType["string"] {
				videoData["title"] = []byte(body["title"].(string))
			}
			if body["width"] != nil && body["height"] != nil {
				videoData["ratio"] = []byte(goutil.ToString(body["width"])+":"+goutil.ToString(body["height"]))
			}
		}else if bytes.HasPrefix(url, []byte("PU")) || bytes.HasPrefix(url, []byte("PL")) {
			res, err := http.Get("https://www.youtube.com/oembed?url=https://www.youtube.com/playlist?list="+string(url)+"&format=json")
			if err != nil {
				return []byte{}
			}
			body, _ := goutil.DecodeJSON(res.Body)

			videoData["url"] = append([]byte("https://www.youtube.com/embed/?list="), url...)

			if reflect.TypeOf(body["thumbnail_url"]) == goutil.VarType["string"] {
				videoData["img"] = []byte(body["thumbnail_url"].(string))
			}
			if reflect.TypeOf(body["title"]) == goutil.VarType["string"] {
				videoData["title"] = []byte(body["title"].(string))
			}
			if body["width"] != nil && body["height"] != nil {
				videoData["ratio"] = []byte(goutil.ToString(body["width"])+":"+goutil.ToString(body["height"]))
			}
		}else{
			res, err := http.Get("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v="+string(url)+"&format=json")
			if err != nil {
				return []byte{}
			}
			body, _ := goutil.DecodeJSON(res.Body)

			videoData["url"] = append([]byte("https://www.youtube.com/embed/"), url...)

			if reflect.TypeOf(body["thumbnail_url"]) == goutil.VarType["string"] {
				videoData["img"] = []byte(body["thumbnail_url"].(string))
			}
			if reflect.TypeOf(body["title"]) == goutil.VarType["string"] {
				videoData["title"] = []byte(body["title"].(string))
			}
			if body["width"] != nil && body["height"] != nil {
				videoData["ratio"] = []byte(goutil.ToString(body["width"])+":"+goutil.ToString(body["height"]))
			}
		}

		if videoData["url"] == nil {
			return []byte{}
		}

		res := regex.JoinBytes([]byte(`<div class="youtube-embed" href="`), videoData["url"], []byte(`"`))
		if videoData["ratio"] != nil {
			res = regex.JoinBytes(res, []byte(` ratio="`), videoData["ratio"], []byte(`">`))
		}else{
			res = append(res, '>')
		}
		if videoData["img"] != nil {
			res = regex.JoinBytes(res, []byte(`<img src="`), videoData["img"], []byte(`" alt="YouTube Embed"/>`))
		}
		if videoData["title"] != nil {
			res = regex.JoinBytes(res, []byte(`<h1>`), videoData["title"], []byte(`</h1>`))
		}
		res = append(res, []byte(`<img class="youtube-embed-play-btn" src="`+GithubAssetURL+`/youtube.png"/></div>`)...)

		return res
	},

	"yt": "youtube",
}

var tagFuncs map[string]interface{} = map[string]interface{} {
	"if": tagFuncIf,

	"each": func(arg *map[string][]byte, cont []byte, opts *map[string]interface{}, level int, file *fileData) interface{} {
		args := *arg

		//todo: fix each function outputing the same var

		if len(args) == 0 {
			return []byte{}
		}

		var argObj string = ""
		var argAs []byte = nil
		var argOf []byte = nil
		var argIn []byte = nil
		argType := 0

		for i, v := range args {
			if i == "0" || i == "value" {
				argObj = string(v)
				continue
			}

			if i == "as" {
				argAs = v
				continue
			}else if i == "of" {
				argOf = v
				continue
			}else if i == "in" {
				argIn = v
				continue
			}

			if bytes.Equal(v, []byte("as")) {
				argType = 1
				continue
			}else if bytes.Equal(v, []byte("of")) {
				argType = 2
				continue
			}else if bytes.Equal(v, []byte("in")) {
				argType = 3
				continue
			}

			if argType == 1 {
				argAs = v
				argType = 0
				continue
			}else if argType == 2 {
				argOf = v
				argType = 0
				continue
			}else if argType == 3 {
				argIn = v
				argType = 0
				continue
			}

			if argAs == nil {
				argAs = v
			}else if argOf == nil {
				argOf = v
			}else if argIn == nil {
				argIn = v
			}
		}

		obj := getOpt(*opts, argObj, false)
		res := []eachFnObj{}

		objType := reflect.TypeOf(obj)
		if objType != goutil.VarType["map"] && objType != goutil.VarType["array"] {
			return []byte{}
		}

		if objType == goutil.VarType["map"] {
			n := 0
			for i, v := range obj.(map[string]interface{}) {
				opt, err := goutil.DeepCopyJson(*opts)
				if err != nil {
					opt = map[string]interface{}{}
				}
				if argAs != nil {
					opt[string(argAs)] = v
				}else{
					opt[argObj] = v
				}
				if argOf != nil {
					opt[string(argOf)] = i
				}
				if argIn != nil {
					opt[string(argIn)] = n
				}
				res = append(res, eachFnObj{html: cont, opts: opt})
				n++
			}
		}else if objType == goutil.VarType["array"] {
			n := 0
			for i, v := range obj.([]interface{}) {
				opt, err := goutil.DeepCopyJson(*opts)
				if err != nil {
					opt = map[string]interface{}{}
				}
				if argAs != nil {
					opt[string(argAs)] = v
				}else{
					opt[argObj] = v
				}
				if argOf != nil {
					opt[string(argOf)] = i
				}
				if argIn != nil {
					opt[string(argIn)] = n
				}
				res = append(res, eachFnObj{html: cont, opts: opt})
				n++
			}
		}

		return res
	},

	"json": func(arg *map[string][]byte, cont []byte, opts *map[string]interface{}, level int, file *fileData) interface{} {
		args := *arg

		var json interface{} = nil
		if val, ok := args["0"]; ok {
			json = getOpt(*opts, string(val), false)
		}else{
			return []byte{}
		}

		var spaces int = 0
		if val, ok := args["indent"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}else if val, ok := args["1"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}

		var prefix int = 0
		if val, ok := args["prefix"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				spaces = 0
			}else{
				spaces = sp
			}
		}else if val, ok := args["2"]; ok {
			sp, err := strconv.Atoi(string(val))
			if err != nil {
				prefix = 0
			}else{
				prefix = sp
			}
		}

		res, err := goutil.StringifyJSON(json, spaces, prefix)
		if err != nil {
			return []byte{}
		}

		return res
	},
}

func tagFuncIf(arg *map[string][]byte, cont []byte, opts *map[string]interface{}, level int, file *fileData, pre int) (interface{}, bool) {
	args := *arg
	
	isTrue := false
	lastArg := []byte{}

	if len(args) == 0 {
		return cont, false
	}

	for i := 0; i < len(args); i++ {
		arg := args[strconv.Itoa(i)]
		if bytes.Equal(arg, []byte("&")) {
			if isTrue {
				continue
			}
			break
		} else if bytes.Equal(arg, []byte("|")) {
			if isTrue {
				break
			}
			continue
		}

		arg1, sign, arg2 := []byte{}, "", []byte{}
		var arg1Any interface{}
		var arg2Any interface{}
		a1, ok1 := args[strconv.Itoa(i+1)]
		if ok1 && regex.MatchRef(&a1, `^[!<>]?=|[<>]$`) {
			arg1 = arg
			sign = string(a1)
			arg2 = args[strconv.Itoa(i+2)]
			lastArg = arg1
		} else if ok1 && regex.MatchRef(&arg, `^[!<>]?=|[<>]$`) {
			arg1 = lastArg
			sign = string(arg)
			arg2 = a1
			i++
		} else if len(args) == 1 {
			pos := true
			if bytes.HasPrefix(arg, []byte("!")) {
				pos = false
				arg1 = arg[1:]
			}

			if bytes.Equal(arg, []byte("")) {
				arg1 = a1
				i++
			} else if bytes.Equal(arg, []byte("!")) {
				arg1 = a1
				i++
				pos = true
			}

			if bytes.HasPrefix(arg1, []byte("!")) {
				pos = !pos
				arg1 = arg1[1:]
			}

			if len(arg1) == 0 {
				arg1 = arg
			}

			lastArg = arg1

			if regex.MatchRef(&arg1, `^["'\'](.*)["'\']$`) {
				arg1 = regex.RepFuncRef(&arg1, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
					return data(1)
				})
				if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
					arg1Any = arg1N
				} else {
					arg1Any = arg1
				}
			} else if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
				arg1Any = arg1N
			} else {
				arg1Any = getOpt(*opts, string(arg1), false)
				if pre == 1 && arg1Any == nil {
					return nil, true
				}
			}

			isTrue = goutil.IsZeroOfUnderlyingType(arg1Any)
			if pos {
				isTrue = !isTrue
			}
			
			continue
		} else {
			continue
		}

		if regex.MatchRef(&arg1, `^["'\'](.*)["'\']$`) {
			arg1 = regex.RepFuncRef(&arg1, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
				return data(1)
			})
			if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
				arg1Any = arg1N
			} else {
				arg1Any = arg1
			}
		} else if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
			arg1Any = arg1N
		} else {
			arg1Any = getOpt(*opts, string(arg1), false)
			if pre == 1 && arg1Any == nil {
				return nil, true
			}

			if reflect.TypeOf(arg1Any) == goutil.VarType["string"] {
				if arg1N, err := strconv.Atoi(string(arg1)); err == nil {
					arg1Any = arg1N
				}
			}
		}

		if len(arg2) == 0 {
			arg2Any = nil
		} else if regex.MatchRef(&arg2, `^["'\'](.*)["'\']$`) {
			arg2 = regex.RepFuncRef(&arg2, `^["'\'](.*)["'\']$`, func(data func(int) []byte) []byte {
				return data(1)
			})
			if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
				arg2Any = arg2N
			} else {
				arg2Any = arg2
			}
		} else if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
			arg2Any = arg2N
		} else {
			arg2Any = getOpt(*opts, string(arg2), false)
			if pre == 1 && arg2Any == nil {
				return nil, true
			}

			if reflect.TypeOf(arg2Any) == goutil.VarType["string"] {
				if arg2N, err := strconv.Atoi(string(arg2)); err == nil {
					arg2Any = arg2N
				}
			}
		}

		lastArg = arg1

		arg1Type := reflect.TypeOf(arg1Any)
		if arg1Type == goutil.VarType["int"] {
			arg1Any = float64(arg1Any.(int))
		}else if arg1Type == goutil.VarType["float32"] {
			arg1Any = float64(arg1Any.(float32))
		}else if arg1Type == goutil.VarType["int32"] {
			arg1Any = float64(arg1Any.(int32))
		}else if arg1Type == goutil.VarType["byteArray"] {
			arg1Any = string(arg1Any.([]byte))
		}else if arg1Type == goutil.VarType["byte"] {
			arg1Any = string(arg1Any.(byte))
		}

		arg2Type := reflect.TypeOf(arg2Any)
		if arg2Type == goutil.VarType["int"] {
			arg2Any = float64(arg2Any.(int))
		}else if arg2Type == goutil.VarType["float32"] {
			arg2Any = float64(arg2Any.(float32))
		}else if arg2Type == goutil.VarType["int32"] {
			arg2Any = float64(arg2Any.(int32))
		}else if arg1Type == goutil.VarType["byteArray"] {
			arg2Any = string(arg2Any.([]byte))
		}else if arg1Type == goutil.VarType["byte"] {
			arg2Any = string(arg2Any.(byte))
		}

		switch sign {
		case "=":
			isTrue = (arg1Any == arg2Any)
		case "!=":
		case "!":
			isTrue = (arg1Any != arg2Any)
		case ">=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == goutil.VarType["float64"] {
				isTrue = (arg1Any.(float64) >= arg2Any.(float64))
			}
		case "<=":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == goutil.VarType["float64"] {
				isTrue = (arg1Any.(float64) <= arg2Any.(float64))
			}
		case ">":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == goutil.VarType["float64"] {
				isTrue = (arg1Any.(float64) > arg2Any.(float64))
			}
		case "<":
			if arg1Type == reflect.TypeOf(arg2Any) && arg1Type == goutil.VarType["float64"] {
				isTrue = (arg1Any.(float64) < arg2Any.(float64))
			}
		}

		i += 2
	}

	elseOpt := regex.MatchRef(&cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`)
	if elseOpt && isTrue {
		return regex.RepStrRef(&cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`, []byte("")), false
	} else if elseOpt {
		blankElse := false
		newArgs, newCont := map[string][]byte{}, []byte{}
		regex.RepFuncRef(&cont, `(?s)<_el(if|se):`+strconv.Itoa(level)+`(\s+[0-9]+|)/>(.*)$`, func(data func(int) []byte) []byte {
			argInt, err := strconv.Atoi(string(regex.RepStr(data(2), `\s`, []byte{})))
			if err != nil {
				blankElse = true
			}else{
				newArgs = file.args[argInt]
			}
			newCont = data(3)
			return nil
		}, true)

		if blankElse {
			return newCont, false
		}

		return tagFuncIf(&newArgs, newCont, opts, level, file, pre)
	} else if isTrue {
		return cont, false
	}

	return []byte{}, false
}
