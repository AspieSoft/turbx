package main

import (
	"turbx/compiler"

	"github.com/AspieSoft/goutil/v3"
)

func main(){
	defer compiler.Close()

	args := goutil.MapArgs()

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
		//todo: decompress and convert to json object
		// config.ConstOpts = args["opts"]
	}

	err := compiler.SetConfig(config)
	if err != nil {
		panic(err)
	}


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
