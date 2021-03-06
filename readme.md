# Turbx

![npm version](https://img.shields.io/npm/v/turbx)
![dependency status](https://img.shields.io/librariesio/release/npm/turbx)
![gitHub top language](https://img.shields.io/github/languages/top/aspiesoft/turbx)
![npm license](https://img.shields.io/npm/l/turbx)

![npm weekly downloads](https://img.shields.io/npm/dw/turbx)
![npm monthly downloads](https://img.shields.io/npm/dm/turbx)

[![donation link](https://img.shields.io/badge/buy%20me%20a%20coffee-square-blue)](https://buymeacoffee.aspiesoft.com)

A Fast and Easy To Use View Engine, Compiled In Go.

> Notice: This View Engine Is Currently In Beta

## Whats New

- Module now compiles templates in [go](https://go.dev/)
- golang compiler now has improved crash recovery

## Installation

```shell script
npm install turbx
```

## Setup

```js

const express = require('express');
const {join} = require('path');
const turbx = require('turbx');

const app = express();

app.engine('xhtml', turbx({
  /* global options */
  template: 'layout',
  opts: {default: 'some default options for res.render'},
  before: function(data /* json string containing the pre-compiled (only partly compiled) output */, opts){
    // do stuff before res.render
    return data || undefined; // returning undefined changes nothing
  },
  after: function(html /* html string containing the compiled output */, opts){
    // do stuff after res.render
    return html || undefined; // returning undefined changes nothing
  },
}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'xhtml');

app.use(function(req, res, next){
  res.render('index', {title: 'example', content: '<h2>Hello, World!</h2>'});
});

```

## Usage

```xhtml

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


<!-- functions start with an _ -->
<_if var1 & var2 = 'b' | var2 = 'c' | !var3>
  do stuff...
<_elif !var1 & var2 = 'a'/>
  do other stuff...
<_else/>
  do final stuff...
</_if>

<_each myObj as value of key in index>
  {{key}}: {{value}}
  array index: {{index}}
</_each>

<!-- random paragraph of lorem ipsum text -->
<_lorem/>
<!-- paragraph of lorem ipsum text with 2 sentences -->
<_lorem 2/>
<!-- sentence of lorem ipsum text with 3-5 words -->
<_lorem s 3 5/>
<!-- word of lorem ipsum text with 5-10 letters -->
<_lorem w 5 10/>


<!-- A component is imported by using a capital first letter -->
<!-- The file should also be named with a capital first letter -->
<!-- args can be passed into a component -->
<MyComponent arg1="value 1" arg2="value 2" type="h1">
  Some body to add to the component
</MyComponent>

<!-- component without a body -->
<MyComponent arg1="value"/>

<!-- file: MyComponent.xhtml -->
{{arg1}} - {{arg2}}
<{{type}}>
  {{{body}}}
</{{type}}>


<!-- file: layout.xhtml -->
<html>
  <head></head>
  <body>
    <header></header>
    <main>
      <!-- Insert Body -->
      <Body/>
    </main>
    <footer>
  </body>
</html>

```

## Additional Features

```js

// add basic rate limiting
// this function uses the express-device-rate-limit module made my AspieSoft
turbx.rateLimit(app, {opts /* for express-device-rate-limit module */});

// auto render views as pages
turbx.renderPages(app, {opts});
// also recognizes folders with an index.xhtml file
// ignores the components folder and tries to ignore root files named after error codes (or in an "error" folder)

```
