package turbx

import "turbx/compiler"

// Config is used in the SetConfig method to simplify the args and make them optional
type Config compiler.Config

// KeyVal is used to allow key:value lists to be sorted in an array
type KeyVal compiler.KeyVal

// Close handles stoping the compiler and clearing the cache
var Close = compiler.Close

// Compile handles the final output and returns valid html/xhtml that can be passed to the user
//
// this method will automatically call preCompile if needed, and can read the cache file while its being written for an extra performance boost
var Compile = compiler.Compile

// preCompile generates a new pre-compiled file for the cache
//
// this compiles markdown and handles other complex methods
//
// this function is useful if you need to update any constand vars, defined with a "$" as the first char in their key name
var PreCompile = compiler.PreCompile

// HasPreCompile returns true if a file has been pre compiled in the cache and is not expired
var HasPreCompile = compiler.HasPreCompile

// SetConfig can be used to set change the config options provided in the Config struct
//
// this method will also clear the cache
func SetConfig(config Config) error {
	return compiler.SetConfig(compiler.Config{
		Root: config.Root,
		Components: config.Components,
		Layout: config.Layout,
		Ext: config.Ext,
		Public: config.Public,
		Cache: config.Cache,
		ConstOpts: config.ConstOpts,
	})
}

// NewFunc can be used to create custom functions for the compiler
//
// these user defined functions will only run after the default functions have been resolved
var NewFunc = compiler.NewFunc

// GetOpt is used to handle grabing an option from the user options that were passed
//
// this method accepts the arg as a simple text like []byte("myOption")
//
// this method can also handle complex options like {{this|'that'}} (with optional or statements) and even {{class="myClass"}} vars
var GetOpt = compiler.GetOpt
