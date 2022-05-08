# Turbx

![npm](https://img.shields.io/npm/v/turbx)
![Libraries.io dependency status for latest release](https://img.shields.io/librariesio/release/npm/turbx)
![GitHub top language](https://img.shields.io/github/languages/top/aspiesoft/turbx)
![NPM](https://img.shields.io/npm/l/turbx)

![npm](https://img.shields.io/npm/dw/turbx)
![npm](https://img.shields.io/npm/dm/turbx)

[![paypal](https://img.shields.io/badge/buy%20me%20a%20coffee-paypal-blue)](https://buymeacoffee.aspiesoft.com)

A Fast and Easy To Use View Engine, Compiled With Regex.

> Notice: This View Engine Is Currently In Beta

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

<!-- 2 paragraphs of lorem ipsum text -->
<_lorem 2/>
  <!-- alias -->
  <_p 2/>
  <_paragraph 2/>
<!-- 3 sentences of lorem ipsum text -->
<_lorem 3 s/>
  <!-- alias -->
  <_s 3 s/>
  <_sentence 3 s/>
<!-- 5 words of lorem ipsum text -->
<_lorem 5 w/>
  <!-- alias -->
  <_w 5 w/>
  <_word 5 w/>


<!-- A component is imported by using a capital first letter -->
<!-- The file should still be named in lowercase like any other file -->
<!-- args can be passed into a component -->
<MyComponent arg1="value 1" arg2="value 2" type="h1">
  Some body to add to the component
</MyComponent>

<!-- component without a body -->
<MyComponent arg1="value"/>

<!-- file: myComponent.xhtml -->
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
