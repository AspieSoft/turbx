# Turbx

![npm version](https://img.shields.io/npm/v/turbx)
![dependency status](https://img.shields.io/librariesio/release/npm/turbx)
![gitHub top language](https://img.shields.io/github/languages/top/aspiesoft/turbx)
![npm license](https://img.shields.io/npm/l/turbx)

![npm weekly downloads](https://img.shields.io/npm/dw/turbx)
![npm monthly downloads](https://img.shields.io/npm/dm/turbx)

[![donation link](https://img.shields.io/badge/buy%20me%20a%20coffee-paypal-blue)](https://paypal.me/shaynejrtaylor?country.x=US&locale.x=en_US)

A Fast and Easy To Use View Engine, Compiled In Go.

## Whats New

- Rebuilt the compiler in go from the ground up

## Installation

```shell script

sudo apt-get install libpcre3-dev

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
  before: function(opts){
    // do stuff before res.render
  },
  after: function(opts, html /* html string containing the compiled output */){
    // do stuff after res.render
  },
}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'md');

app.use(function(req, res, next){
  res.render('index', {
    title: 'example',
    content: '<h2>Hello, World!</h2>',

    // const vars can be used to precompile a var, and not need to compile it again
    // a const var is defined by starting with a '$' in the key name
    $GoogleAuthToken: 'This Value Will Never Change',
  });
});

// pre compiling constant vars
app.use(async function(req, res, next){
  let preCompiled = await res.inCache('index');
  if(!preCompiled){
    const SomethingConsistant = await someLongProcess();

    await res.preRender('index', {
      $myConstVar: SomethingConsistant,
    });
  }

  res.render('index', {
    title: 'example',
    content: '<h2>Hello, World!</h2>',
  });
});

// pre compiling and overriding the cache
app.use('/fix-cache', async function(req, res, next){
  turbx.preCompile('index', {
    $MyConstOpts: 'new constant option',
  });

  res.render('index', {
    title: 'example',
    content: '<h2>Hello, World!</h2>',
  });
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


<!-- A component is imported by using a capital first letter -->
<!-- The file should also be named with a capital first letter -->
<!-- args can be passed into a component -->
<MyComponent arg1="value 1" arg2="value 2">
  Some body to add to the component
</MyComponent>

<!-- component without a body -->
<MyComponent arg1="value"/>

<!-- file: MyComponent.xhtml -->
{{arg1}} - {{arg2}}
<h1>
  {{{body}}}
</h1>


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

## Other Functions

```xhtml

<!-- random paragraph of lorem ipsum text -->
<_lorem/>
<!-- 2 paragraphs of lorem ipsum text -->
<_lorem 2/>
<!-- 3 sentence of lorem ipsum text with 5-10 words -->
<_lorem s 3 5 10/>
<!-- 1 word of lorem ipsum text with 5-10 letters -->
<_lorem w 1 5 10/>

<!-- embed a youtube video -->
<_json myList/>

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
