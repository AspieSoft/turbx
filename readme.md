# Turbx

![npm version](https://img.shields.io/npm/v/turbx)
![dependency status](https://img.shields.io/librariesio/release/npm/turbx)
![gitHub top language](https://img.shields.io/github/languages/top/aspiesoft/turbx)
![npm license](https://img.shields.io/npm/l/turbx)

![npm weekly downloads](https://img.shields.io/npm/dw/turbx)
![npm monthly downloads](https://img.shields.io/npm/dm/turbx)

[![donation link](https://img.shields.io/badge/buy%20me%20a%20coffee-paypal-blue)](https://paypal.me/shaynejrtaylor?country.x=US&locale.x=en_US)

A Fast and Easy To Use View Engine, Compiled In Go.

> Note: this is the info for the Go module.
> Click [here](https://github.com/AspieSoft/turbx/tree/master/node) to find info on the NodeJS module.

---

> Notice: the Go module v2 is ready, but the NodeJS module v2 has Not been setup yet.
> The html templates between v1 and v2 are slightly different.
> For the NodeJS module, you should continue to use v1.
> This is only a pre-release of v2 for Go.

## Whats New

- Rebuild to module to replace my old spaghetti code (and created new spaghetti code)
- Performance and Stability improvements.

## Installation

```shell script

sudo apt-get install libpcre3-dev

go get github.com/AspieSoft/turbx/v2

```

## Setup

```go

package main

import (
  turbx "github.com/AspieSoft/turbx/v2/compiler"
)

func Test(t *testing.T){
  defer turbx.Close()

  turbx.SetConfig(turbx.Config{
    Root: "views",
    Static: "public",
    Ext: "html",
    IncludeMD: true,
    DebugMode: true,
  })

  // note: if 'turbx.SetConfig' is never called, you will need to run 'turbx.InitDefault' in its place
  // this runs some initial code that cannot run before the config is set
  turbx.InitDefault()

  startTime := time.Now().UnixNano()

  html, path, comp, err := turbx.Compile("index", map[string]interface{}{
    "@compress": []string{"br", "gz"}, // pass the browser compression options from the client
    "@cache": true,

    "key": "MyKey",
    "name": "MyName",

    "$myConstantVar": "this var will run in the precompiler",

    "test": 1,
    "var": "MyVar",
    "list": map[string]interface{}{
      "key1": "value1",
      "key2": "value2",
      "key3": "value3",
    },
  })

  if err != nil {
    // this method will only log errors if debug mode is enabled
    turbx.LogErr(err)
    return
  }

  endTime := time.Now().UnixNano()


  // a path will be provided if we precompiled a static file
  // you can send this file directly to the user (it will automatically choose a compressed file as needed)
  // if there is no path (path == ""), we will have an html output instead (which will also be compressed as needed)
  if path != "" {
    html, err = os.ReadFile(path)
    if err != nil {
      turbx.LogErr(err)
      return
    }
  }

  if comp == 1 {
    if html, err = goutil.BROTLI.UnZip(html); err != nil {
      turbx.LogErr(err)
      return
    }
  }else if comp == 2 {
    if html, err = goutil.GZIP.UnZip(html); err != nil {
      turbx.LogErr(err)
      return
    }
  }

  fmt.Println("----------")
  fmt.Println(string(html))
  fmt.Println("----------")

  fmt.Println(float64(endTime - startTime) / float64(time.Millisecond), "ms")
}

```

## Usage

```html

<!-- this is a comment -->
/* this is also a comment */
// this is an inline comment


<!-- output a variable with escaped HTML -->
{{title}}

<!-- output a variable and allow HTML -->
{{{content}}}

<!-- output a variable with a fallback -->
{{title|name|'Default Title'}}


<!-- output a variable as an attribute (this method will remove the attribute if the value is undefined) -->
<a {{href="url"}}>Link</a>

<!-- this is a shortcut if the attribute name matches the variable name -->
<a {{="href"}}>Link</a>


<!-- you can safely insert object keys (if an object is undefined, this will return blank, and will Not crash) -->
{{obj.key}}

<!-- use dot notation for array indexes -->
{{arr.0}}

<!-- use the value of the "key" var as the key of "obj" -->
{{obj[key]}}
{{obj[myVar]}}


<!-- using vars as if they were constant -->
{{$normalVal|'make this constant anyway, even if not sent as a constant'}}


<!-- functions start with an _ -->
<_if var1 & var2="'b'" | var2="'c'" | !var3 | (group & group1.test1)>
  do stuff...
<_else !var1 & var2="var3" | (var3=">=3" & var3="!0")/>
  do other stuff...
<_else/>
  do final stuff...
</_if>

<_each myObj as="value" of="key">
  {{key}}: {{value}}
</_each>

<!-- 'if/else' statements and 'each' loops are a special kind of function, and do not need the '_' prefix -->
<if test>
  {{test}}
<else/>
  no test
</if>

<each myObj as="value" of="key">
  {{key}}: {{value}}
</each>


<!-- A component is imported by using a capital first letter -->
<!-- The file should also be named with a capital first letter -->
<!-- args can be passed into a component -->
<MyComponent arg1="value 1" arg2="value 2">
  Some body to add to the component
</MyComponent>

<!-- component without a body -->
<MyComponent arg1="value"/>

<!-- file: MyComponent.html -->
{{arg1}} - {{arg2}}
<h1>
  {{{body}}}

  <!-- escaped html body -->
  {{body}}
</h1>


<!-- file: layout.html -->
<html>
  <head></head>
  <body>
    <header></header>
    <main>
      <!-- Insert Body -->
      {{{body}}}
    </main>
    <footer>
  </body>
</html>

```

## Other Functions

```html

<!-- set a new variable to crypto random bytes -->
<_rand randBytes/>
{{randBytes}}

<!-- define a size -->
<_rand rand size="64"/>
{{rand}}

<!-- random paragraph of lorem ipsum text -->
<_lorem/>
<!-- 2 paragraphs of lorem ipsum text -->
<_lorem 2/>
<!-- 3 sentence of lorem ipsum text with 5-10 words -->
<_lorem s 3 5 10/>
<!-- 1 word of lorem ipsum text with 5-10 letters -->
<_lorem w 1 5 10/>

<!-- output json as a string -->
<_json myList/>

```
