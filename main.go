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
		
	})
}
