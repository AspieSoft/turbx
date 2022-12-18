package main

import (
	"bytes"
	"fmt"
	"strconv"
	"turbx/compiler"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v3"
)

var debugMode bool = false

var encKey []byte

func main(){
	defer compiler.Close()

	args := goutil.MapArgs()

	if args["debug"] == "true" {
		debugMode = true
	}

	if args["enc"] != "" && args["enc"] != "true" {
		encKey = goutil.CleanByte([]byte(args["enc"]))
	}

	config := compiler.Config{Root: args["0"]}

	if args["ext"] != "" && args["ext"] != "true" {
		config.Ext = args["ext"]
	}

	if args["components"] != "" && args["components"] != "true" {
		config.Components = args["components"]
	}else if args["component"] != "" && args["component"] != "true" {
		config.Components = args["component"]
	}

	if args["layout"] != "" && args["layout"] != "true" {
		if args["layout"] == "false" || args["layout"] == "0" || args["layout"] == "null" {
			config.Layout = "!"
		}else{
			config.Layout = args["layout"]
		}
	}

	if args["public"] != "" && args["public"] != "true" {
		config.Public = args["public"]
	}

	if args["opts"] != "" && args["opts"] != "true" {
		opts := []byte(args["opts"])
		if dec, err := Decode(opts); err == nil {
			opts = dec
		}

		if opts, err := goutil.ParseJson(opts); err == nil {
			config.ConstOpts = opts
		}
	}

	if args["cache"] != "" && args["cache"] != "true" {
		config.Cache = args["cache"]
	}

	err := compiler.SetConfig(config)
	if err != nil {
		panic(err)
	}


	userInput := make(chan string)
	go readInput(userInput)

	for {
		input := <-userInput

		if input == "ping" {
			fmt.Println("pong")
		}else{
			if encKey == nil && (input == "stop" || input == "exit") {
				compiler.Close()
				return
			}

			if inp, err := Decode([]byte(input)); err == nil {
				if bytes.Equal(inp, []byte("ping")) {
					fmt.Println("pong")
				}else if bytes.Equal(inp, []byte("stop")) || bytes.Equal(inp, []byte("exit")) {
					compiler.Close()
					return
				}else{
					inpArgs := bytes.SplitN(inp, []byte(":"), 4)
					if len(inpArgs) >= 3 {
						if bytes.Equal(inpArgs[0], []byte("comp")) || bytes.Equal(inpArgs[0], []byte("pre")) {
							opts := map[string]interface{}{}
							if len(inpArgs) >= 4 {
								if json, err := goutil.ParseJson(inpArgs[3]); err == nil {
									opts = json
								}
							}

							if bytes.Equal(inpArgs[1], []byte("comp")) {
								res, err := compiler.Compile(string(inpArgs[2]), opts)
								if err != nil {
									send(inpArgs[1], []byte("error"), []byte(err.Error()))
								}else{
									send(inpArgs[1], []byte("res"), res)
								}
							} else {
								err := compiler.PreCompile(string(inpArgs[2]), opts)
								if err != nil {
									send(inpArgs[1], []byte("error"), []byte(err.Error()))
								}else{
									send(inpArgs[1], []byte("res"), []byte("success"))
								}
							}
						}else if bytes.Equal(inpArgs[0], []byte("has")) {
							send(inpArgs[1], []byte("res"), []byte(strconv.FormatBool(compiler.HasPreCompile(string(inpArgs[2])))))
						}else if bytes.Equal(inpArgs[0], []byte("opts")) {
							if opts, err := goutil.ParseJson(inpArgs[1]); err == nil {
								compiler.SetConfig(compiler.Config{ConstOpts: opts})
							}
						}
					}
				}
			}
		}
	}


	return


	//temp: will remove
	// will use "html" ext in production (or use md with custom markdown support by default)
	// may create my own markdown compiler to fix issues with modules lack of html support
	compiler.SetConfig(compiler.Config{Ext: "xhtml"})

	//temp: test
	err = compiler.PreCompile("index", map[string]interface{}{
		"$test": 3,
		"key": "value",
		"$list": map[string]interface{}{
			"item1": "value a",
			"item2": "value b",
			"item3": "value c",
		},
	})

	if err != nil {
		panic(err)
	}


	res, err := compiler.Compile("index", map[string]interface{}{
		"$test": 3,
		"key": "value",
		"$list": map[string]interface{}{
			"item1": "value a",
			"item2": "value b",
			"item3": "value c",
		},
	})

	if err != nil {
		panic(err)
	}

	// fmt.Println(compData)

	// path is the temp dir to be stored in the cache (do not use ttlcache, the file will need to be removed when an object expires)
	// may update ttlCache to accept an optional OnExpire callback
	// _ = path
	// fmt.Println(path)


	_ = res
	// fmt.Println("\n----------\n\n"+string(res))
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


func send(resToken []byte, resType []byte, b []byte){
	res := regex.JoinBytes(resToken, ':', resType, ':', b)
	res, err := Encode(res)
	if err != nil {
		fmt.Println(regex.JoinBytes(resToken, ':', []byte("error:failed_to_encode_response")))
	}else{
		fmt.Println(res)
	}
}

func Encode(b []byte) ([]byte, error) {
	if encKey != nil {
		return goutil.Encrypt(b, encKey)
	}

	res, err := goutil.Compress(b)
	if err != nil && debugMode {
		return b, nil
	}
	return res, err
}

func Decode(b []byte) ([]byte, error) {
	if encKey != nil {
		return goutil.Decrypt(b, encKey)
	}

	res, err := goutil.Decompress(b)
	if err != nil && debugMode {
		return b, nil
	}
	return res, err
}
