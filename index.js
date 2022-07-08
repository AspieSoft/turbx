const fs = require('fs');
const {join} = require('path');
// const crypto = require('crypto');
const zlib = require('zlib');
// const memoryCache = require('@aspiesoft/obj-memory-cache');
const multiTaskQueue = require('@aspiesoft/multi-task-queue');
const {spawn} = require('child_process');

const deviceRateLimit = requireOptional('express-device-rate-limit');


function warn(){
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

  console.warn(...args, '\x1b[0m', ' '.repeat(100));
}

warn('[red]', 'Notice: Version 0.1 does not yet support if statements and other _functions\nIf you need these features, please use version 0.0 for now');


function requireOptional(path){
  try {
    return require(path);
  } catch(e) {
    return undefined;
  }
}


const common = require('./common');
const {
  sleep,
  randomToken,
  clean,
  toTimeMillis,
} = common;

const {tagFunctions, addTagFunction, runTagFunction} = require('./functions.old');


const taskQueue = multiTaskQueue(10);


let ExpressApp = undefined;


const ROOT = (function() {
  if(require.main.filename) {
    return clean(require.main.filename.toString()).replace(/[\\\/][^\\\/]+[\\\/]?$/, '');
  }
  if(require.main.path) {
    return clean(require.main.path.toString());
  }
  if(process.cwd && process.cwd()){
    return clean(process.cwd());
  }
  return clean(join(__dirname).toString().replace(/[\/\\]node_modules[\/\\][^\\\/]+[\\\/]?$/, ''));
})();

const OPTS = {};


const goCompiledResults = {};
let goCompiler;
let goCompilerLastInit = 0;

const goRecentId = {};
setInterval(function(){
  const now = Date.now();
  const id = Object.keys(goRecentId);
  for(let i = 0; i < id.length; i++){
    if(now - goRecentId[id] > 20000){
      delete goRecentId[id];
    }
  }
}, 20000);

function initGoCompiler(){
  goCompilerLastInit = Date.now();
  // goCompiler = spawn('go', ['run', 'compiler/main.go']);
  goCompiler = spawn('./compiler/compiler');
  goCompiler.stdout.on('data', async (data) => {
    data = data.toString().trim();
  
    if(data.startsWith('debug:')){
      console.log(data);
      return;
    }
  
    let idToken = undefined;
    data = data.replace(/^([\w_-]+):/, (_, id) => {
      idToken = id;
      return '';
    });
    if(!idToken){
      return;
    }

    const now = Date.now();
    if(now - goRecentId[idToken] < 20000){
      return;
    }

    goRecentId[idToken] = now;

    goCompiledResults[idToken] = data;
  });
  goCompiler.stderr.on('end', () => {
    initGoCompiler();
  });
}
initGoCompiler();

function goCompilerSetOpt(key, value){
  goCompiler.stdin.write('set:' + key.toString().replace(/[^\w_-]/g, '') + '=' + value.toString().replace(/[\r\n\v]/g, '') + '\n');
}

async function goCompilerPreCompile(file){
  const token = randomToken(64);
  goCompiler.stdin.write('pre:' + token + ':' + file.toString().replace(/[\r\n\v]/g, '') + '\n');

  const updateSpeed = Number(OPTS.updateSpeed) || 10;

  let loops = (toTimeMillis(OPTS.timeout) || 30000) / updateSpeed;
  while(loops-- > 0){
    if(goCompiledResults[token]){
      break;
    }
    await sleep(updateSpeed);
  }

  const res = goCompiledResults[token];
  delete goCompiledResults[token];
  return res;
}

async function goCompilerCompile(file, opts){
  const token = randomToken(64);

  let zippedOpts = undefined;

  zlib.gzip(JSON.stringify(opts), (err, buffer) => {
    if(err){
      goCompiledResults[token] = 'error';
      zippedOpts = null;
      return;
    }

    zippedOpts = buffer.toString('base64');

    // goCompiler.stdin.write(token + ':' + buffer.toString('base64') + ':' + file.toString().replace(/[\r\n\v]/g, '') + '\n');
  });

  const updateSpeed = Number(OPTS.updateSpeed) || 10;

  let loops = (toTimeMillis(OPTS.timeout) || 30000) / updateSpeed;
  while(loops-- > 0){
    if(zippedOpts != undefined){
      break;
    }
    await sleep(updateSpeed);
  }

  if(!zippedOpts){
    return zippedOpts;
  }

  let reqStarted = Date.now();
  goCompiler.stdin.write(token + ':' + zippedOpts + ':' + file.toString().replace(/[\r\n\v]/g, '') + '\n');

  while(loops-- > 0){
    if(goCompiledResults[token]){
      break;
    }

    if(reqStarted < goCompilerLastInit){
      reqStarted = Date.now();
      goCompiler.stdin.write(token + ':' + zippedOpts + ':' + file.toString().replace(/[\r\n\v]/g, '') + '\n');
    }

    await sleep(updateSpeed);
  }

  const res = goCompiledResults[token];
  delete goCompiledResults[token];

  let output = undefined;
  if(res){
    zlib.gunzip(Buffer.from(res, 'base64'), (err, res) => {
      if(err){
        output = null;
        return;
      }
      output = res.toString();
    });

    while(loops-- > 0){
      if(output !== undefined){
        break;
      }
      await sleep(updateSpeed);
    }

    return output;
  }

  return undefined;
}


function exitHandler(options, exitCode) {
  if(options.cleanup){
    goCompiler.stdin.write('stop');
  }
  if(options.exit){
    process.exit(exitCode);
  }
}

process.on('exit', exitHandler.bind(null,{cleanup:true}));

//catches ctrl+c event
process.on('SIGINT', exitHandler.bind(null, {exit:true}));

// catches "kill pid" (for example: nodemon restart)
process.on('SIGUSR1', exitHandler.bind(null, {exit:true}));
process.on('SIGUSR2', exitHandler.bind(null, {exit:true}));

//catches uncaught exceptions
process.on('uncaughtException', exitHandler.bind(null, {exit:true}));


function engine(path, opts, cb){
  path = clean(path);
  opts = clean(opts);

  if(!OPTS.ext){
    OPTS.ext = opts.settings['view engine'] || (path.includes('.') ? path.substring(path.lastIndexOf('.')).replace('.', '') : 'xhtml');
    goCompilerSetOpt('ext', OPTS.ext);
  }

  if(!OPTS.root){
    OPTS.root = opts.settings.views || opts.settings.view || (require.main.filename || process.argv[1] || __dirname).replace(/([\\\/]node_modules[\\\/].*?|[\\\/][^\\\/]*)$/, '/views');
    goCompilerSetOpt('root', OPTS.root);
  }

  if(path.includes('.')){
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  opts.settings.filename = path;

  taskQueue(path, async () => {
    if(OPTS.before){
      OPTS.before(opts);
    }
    if(typeof opts.before === 'function'){
      opts.before(opts);
    }

    const data = await goCompilerCompile(path, opts);

    if(typeof opts.after === 'function'){
      let newData = opts.after(opts, data);
      if(newData !== undefined){
        data = newData;
      }
    }
    if(OPTS.after){
      let newData = OPTS.after(opts, data);
      if(newData !== undefined){
        data = newData;
      }
    }

    if(!data || data === 'error'){
      return cb(null, '<h1>Error 500</h1><h2>Internal Server Error</h2>');
    }

    return cb(null, data);
  });
}


function setOpts(opts){
  let before = opts.before;
  let after = opts.after;
  opts = clean(opts);
  let root = opts.views || opts.view || 'views';

  OPTS.ext = (opts.ext || 'xhtml').replace(/[^\w_\-]/g, '');
  let rootPath = root.replace(/[^\w_\-\\\/\.@$#!]/g, '');
  if(!rootPath.startsWith(ROOT)){
    rootPath = join(ROOT, rootPath);
  }
  OPTS.root = rootPath;

  let componentsOpt = opts.components || opts.component || 'components';
  if(componentsOpt){
    OPTS.components = componentsOpt.replace(/[^\w_\-\\\/\.@$#!]/g, '');
  }

  let template = (opts.template || '').replace(/[^\w_\-\\\/\.@$#!]/g, '');
  if(template && template.trim() !== ''){
    OPTS.template = template;
  }

  OPTS.cache = opts.cache || '2h';
  OPTS.lazyCache = opts.lazyCache || '12h';
  OPTS.timeout = opts.timeout || '30s';

  if(typeof before === 'function'){
    OPTS.before = before;
  }

  if(typeof after === 'function'){
    OPTS.after = after;
  }

  if(typeof opts.static === 'string'){
    OPTS.static = opts.static;
  }else{
    OPTS.static = '/';
  }

  if(typeof opts.opts === 'object'){
    OPTS.opts = opts.opts;
  }else{
    OPTS.opts = {};
  }

  if(['number', 'string'].includes(typeof opts.updateSpeed)){
    updateSpeed = toTimeMillis(opts.updateSpeed);
    if(updateSpeed && updateSpeed > 0){
      OPTS.updateSpeed = updateSpeed;
    }
  }else{
    OPTS.updateSpeed = 10;
  }

  for(let key in OPTS){
    if(OPTS[key] === undefined || OPTS[key] === null){
      continue;
    }
    if(typeof OPTS[key] === 'function'){
      goCompilerSetOpt(key, 'true')
    }else if(typeof OPTS[key] === 'object'){
      goCompilerSetOpt(key, JSON.stringify(OPTS[key]))
    }else if(typeof OPTS[key] === 'string' && OPTS[key].match(/^[0-9]+(\.[0-9]+|)[a-z]{0,3}$/)){
      let opt = toTimeMillis(OPTS[key]);
      if(!Number.isNaN(opt)){
        goCompilerSetOpt(key, opt.toString())
      }else{
        goCompilerSetOpt(key, OPTS[key].toString())
      }
    }else{
      goCompilerSetOpt(key, OPTS[key].toString())
    }
  }
}


function setupExpress(app){
  return;

  app.use('/lazyload/:token/:component', async (req, res, next) => {
    //todo: add lazy loading option
  });
}


function expressFallbackPages(app, opts){
  if(typeof app === 'object'){[app, opts] = [opts, app];}
  if(typeof app !== 'function'){app = ExpressApp;}
  if(typeof app !== 'function'){return;}
  if(typeof opts !== 'object'){opts = {};}


  app.use((req, res, next) => {
    const url = clean(req.url).replace(/^[\\\/]+/, '').replace(/\?.*/, '');
    if(url === OPTS.template || url.match(/^(errors?\/|)[0-9]{3}$/)){
      next();
      return;
    }

    let urlPath = join(OPTS.root, url + '.' + OPTS.ext);
    if(urlPath === OPTS.root || !urlPath.startsWith(OPTS.root) || urlPath.startsWith(OPTS.components)){
      next();
      return;
    }

    if(!fs.existsSync(urlPath)){
      next();
      return;
    }

    try {
      res.render(url, opts);
    } catch(e) {
      next();
    }
  });

  app.use((req, res) => {
    let page404 = join(OPTS.root, 'error/404.' + OPTS.ext);
    if(fs.existsSync(page404)){
      res.status(404).render('error/404', opts);
      return;
    }
    page404 = join(OPTS.root, '404.' + OPTS.ext);
    if(fs.existsSync(page404)){
      res.status(404).render('404', opts);
      return;
    }
    res.status(404).send('<h1>Error 404</h1><h2>Page Not Found</h2>').end();
  });
}

function expressRateLimit(app, opts){
  if(typeof app === 'object'){[app, opts] = [opts, app];}
  if(typeof app !== 'function'){app = ExpressApp;}
  if(typeof app !== 'function'){return;}
  if(typeof opts !== 'object'){opts = {};}

  const rateLimit = deviceRateLimit({
    err: function(req, res){
      let page = join(OPTS.root, 'error/429.' + OPTS.ext);
      if(fs.existsSync(page)){
        res.status(429).render('error/429', opts);
        return;
      }
      page = join(OPTS.root, err + '.' + OPTS.ext);
      if(fs.existsSync(page)){
        res.status(429).render('429', opts);
        return;
      }
      res.status(429).send('<h1>Error 429</h1><h2>Too Many Requests</h2>').end();
    },
    ...opts,
  });

  rateLimit.all(app);
}


module.exports = (function(){
  const exports = function(opts = {
    views: 'views',
    components: 'components',
    ext: 'xhtml',
    template: undefined,
    cache: '2h',
    lazyCache: '12h',
    timeout: '30s',
    before: undefined,
    after: undefined,
    static: '/',
    opts: {},
    updateSpeed: 10,
  }, app){

    if(typeof opts === 'function' || typeof app === 'object'){
      [opts, app] = [app, opts];
    }

    if(typeof opts === 'object'){
      setOpts(opts);
    }else{
      setOpts({});
    }

    if(typeof app === 'function'){
      setupExpress(app);
      ExpressApp = app;
    }

    return engine;
  };

  exports.renderPages = expressFallbackPages;
  exports.rateLimit = expressRateLimit;

  exports.function = {
    ...common,
    add: addTagFunction,
    run: runTagFunction,
  };

  return exports;
})();
