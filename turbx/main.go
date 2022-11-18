package main

import (
	"turbx/compiler"

	"github.com/AspieSoft/goutil/v3"
)

func main(){
	defer compiler.Close()

	args := goutil.MapArgs()

	err := compiler.SetRoot(args["0"])
	if err != nil {
		panic(err)
	}

	if args["ext"] != "" && args["ext"] != "true" {
		compiler.SetExt(args["ext"])
	}

	if args["public"] != "" && args["public"] != "true" {
		compiler.SetPublicPath(args["public"])
	}

	//temp: will remove
	// will use "html" ext in production (or use md with custom markdown support by default)
	// may create my own markdown compiler to fix issues with modules lack of html support
	compiler.SetExt("xhtml")

	//temp: test
	path, err := compiler.PreCompile("index", map[string]interface{}{
		"$test": 1,
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
	// path is the temp dir to be stored in the cache (do not use ttlcache, the file will need to be removed when an object expires)
	// may update ttlCache to accept an optional OnExpire callback
	_ = path
	// fmt.Println(path)
}
