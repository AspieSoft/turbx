package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/AspieSoft/turbx/compiler"

	"github.com/AspieSoft/go-regex/v4"
	"github.com/AspieSoft/goutil/v4"
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
		encKey = []byte(args["enc"])
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


	//temp
	/* if args["debug"] != "" {
		err = compiler.PreCompile("index", map[string]interface{}{})
		if err != nil {
			fmt.Println(err)
		}
		return
	} */


	userInput := make(chan string)
	go readInput(userInput)

	for {
		input := <-userInput

		if input == "ping" {
			fmt.Println("pong")
		}else if input == "stop" || input == "exit" {
			compiler.Close()
			return
		}else{
			var inp []byte
			if dec, err := Decode([]byte(input)); err == nil {
				inp = dec
			}else if debugMode {
				inp = []byte(input)
			}

			if inp != nil {
				if bytes.Equal(inp, []byte("ping")) {
					if b, err := Encode([]byte("pong")); err == nil {
						fmt.Println(string(b))
					}
				}else if bytes.Equal(inp, []byte("stop")) || bytes.Equal(inp, []byte("exit")) {
					compiler.Close()
					return
				}else{
					go handleInput(inp)
				}
			}
		}
	}
}


func handleInput(input []byte){
	inpArgs := bytes.SplitN(input, []byte(":"), 5)

	// get compression type
	compType := uint8(0)
	for i := 0; i < len(inpArgs); i++ {
		if bytes.Equal(inpArgs[i], []byte("gzip")) {
			compType = 1
			inpArgs = append(inpArgs[:i], inpArgs[i+1:]...)
			break
		}else if bytes.Equal(inpArgs[i], []byte("brotli")) {
			compType = 2
			inpArgs = append(inpArgs[:i], inpArgs[i+1:]...)
			break
		}
	}

	if len(inpArgs) >= 2 && bytes.Equal(inpArgs[0], []byte("opts")) {
		opts := inpArgs[1]
		if dec, err := Decode(opts); err == nil {
			opts = dec
		}

		if opts, err := goutil.ParseJson(opts); err == nil {
			compiler.SetConfig(compiler.Config{ConstOpts: opts})
		}
	}else if len(inpArgs) >= 3 {
		if bytes.Equal(inpArgs[0], []byte("comp")) || bytes.Equal(inpArgs[0], []byte("pre")) {
			opts := map[string]interface{}{}
			if len(inpArgs) >= 4 {
				if json, err := goutil.ParseJson(inpArgs[3]); err == nil {
					opts = json
				}
			}

			if bytes.Equal(inpArgs[0], []byte("comp")) {
				res, err := compiler.Compile(string(inpArgs[2]), opts, compType)
				if err != nil {
					send(inpArgs[1], []byte("error"), []byte(err.Error()))
				}else{
					/* b := goutil.ToType[[]interface{}](res).([]interface{})
					if r, err := goutil.StringifyJSON(b); err == nil {
						// res = r
						send(inpArgs[1], []byte("res"), r)
					}else{
						r, err = goutil.Compress(res)
						if err != nil {
							send(inpArgs[1], []byte("error"), []byte(err.Error()))
						}else{
							// res = r
							send(inpArgs[1], []byte("res"), r)
						}
					} */
					if compType != 0 {
						base64.StdEncoding.EncodeToString(res)
						send(inpArgs[1], []byte("res"), []byte(base64.StdEncoding.EncodeToString(res)))
					}else {
						r, err := goutil.Compress(res)
						if err != nil {
							send(inpArgs[1], []byte("error"), []byte(err.Error()))
						}else{
							send(inpArgs[1], []byte("res"), r)
						}
					}

					// send(inpArgs[1], []byte("res"), res)
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
			if len(inpArgs) >= 4 {
				send(inpArgs[1], []byte("res"), []byte(strconv.FormatBool(compiler.HasPreCompile(string(inpArgs[2]), string(inpArgs[3])))))
			}else{
				send(inpArgs[1], []byte("res"), []byte(strconv.FormatBool(compiler.HasPreCompile(string(inpArgs[2]), ""))))
			}
		}else if bytes.Equal(inpArgs[0], []byte("set")) {
			if bytes.Equal(inpArgs[1], []byte("root")) {
				compiler.SetConfig(compiler.Config{Root: string(inpArgs[2])})
			}else if bytes.Equal(inpArgs[1], []byte("components")) || bytes.Equal(inpArgs[1], []byte("component")) {
				compiler.SetConfig(compiler.Config{Components: string(inpArgs[2])})
			}else if bytes.Equal(inpArgs[1], []byte("layout")) {
				compiler.SetConfig(compiler.Config{Layout: string(inpArgs[2])})
			}else if bytes.Equal(inpArgs[1], []byte("ext")) {
				compiler.SetConfig(compiler.Config{Ext: string(inpArgs[2])})
			}else if bytes.Equal(inpArgs[1], []byte("public")) {
				compiler.SetConfig(compiler.Config{Public: string(inpArgs[2])})
			}else if bytes.Equal(inpArgs[1], []byte("cache")) {
				compiler.SetConfig(compiler.Config{Cache: string(inpArgs[2])})
			}
		}
	}
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
		fmt.Println(string(regex.JoinBytes(resToken, ':', []byte("error:failed_to_encode_response"))))
	}else{
		fmt.Println(string(res))
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
