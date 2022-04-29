const express = require('express')
const app = express()
const {join} = require('path')
const tml = require('../index')

app.engine('xhtml', tml(app, {template: 'layout', timeout: '3s'}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'xhtml');

app.get('/', function (req, res) {
  res.render('index', {
    var1: 'this is a test',
    test: 1,
    test0: false,
    test1: true,
    url: 'https://www.aspiesoft.com',
  });
});

// auto set all views to public pages
tml.renderPages();

app.listen(3000)
