const express = require('express')
const app = express()
const {join} = require('path')
const turbx = require('../index')

function log(){
  let args = [];
  let col = '';
  for(let i = 0; i < arguments.length; i++){
    switch(arguments[i]){
      case '[black]':
        col = '\x1b[30m';
        break;
      case '[red]':
        col = '\x1b[31m';
        break;
      case '[green]':
        col = '\x1b[32m';
        break;
      case '[yellow]':
        col = '\x1b[33m';
        break;
      case '[blue]':
        col = '\x1b[34m';
        break;
      case '[magenta]':
      case '[purple]':
        col = '\x1b[35m';
        break;
      case '[cyan]':
        col = '\x1b[36m';
        break;
      case '[white]':
        col = '\x1b[37m';
        break;
      default:
        if(typeof arguments[i] === 'string' && arguments[i].startsWith('~')){
          args[args.length-1] += arguments[i].replace('~', '');
        }else{
          args.push(col + arguments[i]);
          col = '';
        }
        break;
    }
  }

  console.log(...args, '\x1b[0m', ' '.repeat(100));
}

//? compile speeds
//* 70ms 53ms, 59ms 53ms, 58ms 55ms, 58ms 64ms

app.engine('xhtml', turbx(app, {
  template: 'layout',
  components: 'components',
  timeout: '3s',
  before: function(_, opts){
    opts.startTime = new Date().getTime();
  },
  after: function(_, opts){
    log('[purple]', 'Compiled', '[yellow]', opts.settings.filename, '[purple]', 'In', '[cyan]', (new Date().getTime()) - opts.startTime, '~ms');
  },
}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'xhtml');


// firewall rate limiting
turbx.rateLimit();


app.get('/', function (req, res) {
  res.render('index', {
    var1: 'this is a test',
    test: 1,
    test0: false,
    test1: true,
    url: 'https://www.aspiesoft.com',
    arr: [1, 2, 3],
    obj: {
      test1: 'this is test 1',
      test2: 'this is test 2',
      test3: 'this is test 3',
    },
    testKey: 'test1',
  });
});


// auto set all views to public pages
turbx.renderPages({
  test: 1,
});


app.listen(3000)
