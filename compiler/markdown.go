package compiler

import "github.com/AspieSoft/go-liveread"

func compileMarkdown(reader *liveread.Reader[uint8], write *func(b []byte)) bool {

	//todo: handle markdown

	// return true to continue if markdown found
	// return false to allow precompiler to handle as default
	return false
}
