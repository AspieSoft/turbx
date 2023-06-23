package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AspieSoft/goutil/v5"
	"github.com/AspieSoft/turbx/v2/compiler"
)

func Test(t *testing.T) {
	defer compiler.Close()

	compiler.SetConfig(compiler.Config{
		Root: "node/test/views",
		Static: "node/test/public",
		DomainFolder: 1,
		IncludeMD: true,
		DebugMode: true,
	})

	startTime := time.Now().UnixNano()

	html, path, comp, err := compiler.Compile("index", map[string]interface{}{
		"@compress": []string{"br", "gz"},
		"@cache": false,
		"@tab": 4, // how many spaces does a tab equal (for markdown)

		"key": "MyKey",
		"name": "MyName",

		"test": 1,
		"var": "MyVar",
		"list": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	})

	if err != nil {
		compiler.LogErr(err)
		return
	}

	endTime := time.Now().UnixNano()

	if path != "" {
		html, err = os.ReadFile(path)
		if err != nil {
			compiler.LogErr(err)
			return
		}
	}

	if comp == 1 {
		if html, err = goutil.BROTLI.UnZip(html); err != nil {
			compiler.LogErr(err)
			return
		}
	} else if comp == 2 {
		if html, err = goutil.GZIP.UnZip(html); err != nil {
			compiler.LogErr(err)
			return
		}
	}

	fmt.Println("----------")
	fmt.Println(string(html))
	fmt.Println("----------")

	fmt.Println(float64(endTime-startTime)/float64(time.Millisecond), "ms")
}
