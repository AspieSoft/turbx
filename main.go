package main

import (
	"github.com/AspieSoft/turbx/compiler"
)

var debugMode bool = false

var encKey []byte

func main(){
	defer compiler.Close()

	compiler.SetConfig(compiler.Config{
		Root: "node/test/views",
		Static: "node/test/public",
		DebugMode: true,
	})

	compiler.PreCompile("index", map[string]interface{}{
		"$test": 1,
		"$var": "MyVar",
		"$list": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	})
}
