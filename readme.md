# Turbx

![npm](https://img.shields.io/npm/v/turbx)
![Libraries.io dependency status for latest release](https://img.shields.io/librariesio/release/npm/turbx)
![GitHub top language](https://img.shields.io/github/languages/top/aspiesoft/turbx)
![NPM](https://img.shields.io/npm/l/turbx)

![npm](https://img.shields.io/npm/dw/turbx)
![npm](https://img.shields.io/npm/dm/turbx)

[![paypal](https://img.shields.io/badge/buy%20me%20a%20coffee-paypal-blue)](https://buymeacoffee.aspiesoft.com)

A Fast and Easy To Use View Engine, Compiled With Regex.

> Notice: This View Engine Is Currently In Alpha

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
  opts: {default: 'some default options for res.render'}
}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'xhtml');

app.use(function(req, res, next){
  res.render('index', {title: 'example', content: '<h2>Hello, World!</h2>'});
});

```
