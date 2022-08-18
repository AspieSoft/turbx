const express = require('express')
const app = express()
const {join} = require('path')
const turbx = require('../index')

const sleep = (waitTimeInMs) => new Promise(resolve => setTimeout(resolve, waitTimeInMs));

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
//* js: 43ms 51ms, 59ms 53ms, 58ms 55ms, 58ms 64ms
//* go: 83ms 74ms, 82ms 75ms, 82ms 85ms, 85ms 88ms (default regex)
//* go: 82ms 32ms, 83ms 22ms, 84ms 33ms, 77ms 31ms (pcre regex "github.com/GRbit/go-pcre")
//* go: 44ms 32ms, 44ms 31ms, 45ms 42ms, 43ms 31ms (full compile, without functions or cache)
//* go: 34ms 33ms, 43ms 32ms, 43ms 33ms, 43ms 30ms (with cache)
//* go: 20ms  8ms, 18ms 11ms, 20ms 12ms, 18ms  9ms (faster update speed)

//? average
//* js: 40-50ms
//* go: 20ms then 10ms (first is 12ms without nodemon) (can also hit 5ms)
//! note: decreasing the updateSpeed value makes things faster. an updateSpeed of 1 can result in 10ms compile time


app.engine('xhtml', turbx(app, {
  template: 'layout',
  components: 'components',
  timeout: '3s',
  // updateSpeed: 10,
  updateSpeed: 1,
  before: function(opts){
    opts.startTime = new Date().getTime();
  },
  after: function(opts){
    log('[purple]', 'Compiled', '[yellow]', opts.settings.filename, '[purple]', 'In', '[cyan]', (new Date().getTime()) - opts.startTime, '~ms');
  },
}));
app.set('views', join(__dirname, 'views'));
app.set('view engine', 'xhtml');


// firewall rate limiting
turbx.rateLimit();


app.get('/', async (req, res) => {
  let preCompiled = await res.preCompiled('index');
  console.log(preCompiled);

  res.render('index', {
    // PreCompile: true,

    const: {
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
    },
  });
});

app.get('/cache', async (req, res) => {
  let preCompiled = await res.preCompiled('index');
  console.log(preCompiled);

  let opts = {};
  if(!preCompiled){
    opts = {
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
    };
  }

  res.render('index', {
    ...opts,
    emptyVar: 'This var was not from the cache!',
  });
});

app.get('/slow', async (req, res, next) => {
  let preCompiled = await res.preCompiled('basic');

  if(!preCompiled){
    const SomethingConsistant = await someLongProcess();

    await res.preRender('basic', {
      myConstVar: SomethingConsistant,
    });
  }

  res.render('basic', {
    title: 'example',
    content: '<h2>Hello, World!</h2>',
  });
});

async function someLongProcess(){
  await sleep(3000);
  return 'test 1';
}


// auto set all views to public pages
turbx.renderPages({
  test: 1,
});


app.listen(3000)
