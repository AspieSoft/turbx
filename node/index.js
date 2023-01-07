const fs = require('fs');
const {join} = require('path');
const { spawn } = require('child_process');
const zlib = require('zlib');

const { sleep, waitForMemory, randomToken, clean, toTimeMillis, encrypt, decrypt, requireOptional } = require('./common');

const deviceRateLimit = requireOptional('express-device-rate-limit');

const DebugMode = process.argv.includes('--turbx-debug');

const defUpdateSpeed = 1;

const ROOT = (function () {
  if (process.cwd && process.cwd()) {
    return clean(process.cwd());
  }
  if (require.main.filename) {
    return clean(require.main.filename.toString()).replace(/[\\\/][^\\\/]+[\\\/]?$/, '');
  }
  if (require.main.path) {
    return clean(require.main.path.toString());
  }
  return clean(
    join(__dirname)
      .toString()
      .replace(/[\/\\]node_modules[\/\\][^\\\/]+[\\\/]?$/, '')
  );
})();
let UserRoot = null;

const EncKey = randomToken(32);

const CompilerOutput = {};

let Compiler = undefined;
let gettingCompiler = false;
let stoppingCompiler = false;
let pingRes = false;
let lastErr = 0;

function initCompiler(){
  if(gettingCompiler && !stoppingCompiler){
    return;
  }
  gettingCompiler = true;

  const args = [
    OPTS.root,
    `--enc=${EncKey}`,
    `--ext=${OPTS.ext}`,
    `--components=${OPTS.components}`,
    `--layout=${OPTS.layout}`,
    `--public=${OPTS.public}`,
    `--opts=${OPTS.opts}`,
    `--cache=${OPTS.cache}`,
  ];

  if(DebugMode){
    Compiler = spawn('go', ['run', '.', ...args], {cwd: join(__dirname, '../')});
  }else{
    Compiler = spawn('../turbx', [...args], { cwd: __dirname });
  }

  async function onExit(){
    if(!stoppingCompiler){
      let now = Date.now();
      if(now - lastErr < 1000){
        await sleep(10000);
      }
      lastErr = now;

      await sleep(100);
      initCompiler();
    }
  }
  Compiler.on('close', onExit);
  Compiler.stderr.on('end', onExit);

  Compiler.stdout.on('data', async (data) => {
    data = data.toString().trim();

    if(data === 'pong'){
      pingRes = true;
      return;
    }

    if(data.startsWith('debug:')){
      console.log('\x1b[34m'+data.replace(/^debug:\s*/, ''), '\x1b[0m');
      return;
    }

    if(data.startsWith('error:')){
      console.log('\x1b[31m'+data.replace(/^error:\s*/, ''), '\x1b[0m');
      return;
    }

    dec = await decrypt(data, EncKey);
    if(!dec){
      if(DebugMode){
        console.log('\x1b[34m'+data, '\x1b[0m');
      }
      return;
    }else{
      data = clean(dec);
    }

    if(data === 'pong'){
      pingRes = true;
      return;
    }

    data = data.split(':', 3);

    CompilerOutput[data[0]] = {res: data[1], data: data[2]};
  });

  initCompilerOnce();
  gettingCompiler = false;
}

let initCompilerOnceRan = false;
function initCompilerOnce(){
  if(initCompilerOnceRan){
    return;
  }
  initCompilerOnceRan = true;

  setInterval(async function(){
    pingRes = false;
    Compiler.stdin.write('ping\n');
    await sleep(1000);
    if(!pingRes){
      initCompiler();
    }
  }, 10000);
}

async function compilerSend(action, token, msg, opts){
  if(!Compiler){
    return;
  }

  if(token){
    action += ':' + token;
  }
  if(msg){
    action += ':' + msg;
  }
  if(opts){
    action += ':' + opts;
  }

  const res = await encrypt(action, EncKey);
  if(res){
    Compiler.stdin.write(res+'\n');
  }
}


function exitHandler(options, exitCode) {
  if(exitCode && exitCode !== 'SIGINT' && exitCode !== 'SIGUSR1' && exitCode !== 'SIGUSR2'){
    console.log(exitCode);
  }

  if (options.cleanup) {
    stoppingCompiler = true;
    if(Compiler){
      Compiler.stdin.write('stop\n');
    }
  }

  if (options.exit) {
    stoppingCompiler = true;
    if(Compiler){
      Compiler.stdin.write('stop\n');
    }
    process.exit(exitCode);
  }
}

process.on('exit', exitHandler.bind(null, { cleanup: true }));
//catches ctrl+c event
process.on('SIGINT', exitHandler.bind(null, { exit: true }));
// catches "kill pid" (for example: nodemon restart)
process.on('SIGUSR1', exitHandler.bind(null, { exit: true }));
process.on('SIGUSR2', exitHandler.bind(null, { exit: true }));
//catches uncaught exceptions
process.on('uncaughtException', exitHandler.bind(null, { exit: true }));


const OPTS = {};
function setOpt(key, value, def = undefined){
  if(def !== undefined){
    let valueList = value;
    if(!Array.isArray(valueList)){
      valueList = [valueList];
    }
    const t = typeof def;
    const tA = Array.isArray(def);
    for(let i = 0; i < valueList.length; i++){
      if(typeof valueList[i] === t && Array.isArray(valueList[i]) === tA){
        value = valueList[i];
        break;
      }
    }

    if(typeof value !== t || Array.isArray(value) !== tA){
      value = def;
    }
  }

  OPTS[key] = value;

  if(Compiler){
    if(key === 'opts'){
      compilerSend('opts', value);
    }else{
      compilerSend('set', key, value);
    }
  }
}

let ExpressApp = undefined;
function setupExpress(app){
  app.use((req, res, next) => {
    res.inCache = preCompileHasCache;
    res.preRender = preCompile;

    const _render = res.render;
    res.render = async function(view, options, fn){
      if(typeof options !== 'object'){
        options = {};
      }
      let enc = req.header('Accept-Encoding');
      if(typeof enc === 'string'){
        options._gzip = enc.split(',').includes('gzip')
      }

      options._status = function(status, gzip = false){
        res.status(status);
        if(gzip){
          res.set('Content-Encoding', 'gzip');
          res.set('Content-Type', 'text/html');
        }
      };

      options._send = function(data){
        res.send(data).end();
      };


      // fix input to prevent crashing
      view = clean(view);
      if(!options){
        options = {};
      }

      if(!OPTS.ext || !OPTS.root){
        _render.call(this, view, options, fn);
        return true;
      }

      if(view.includes('@')) {
        let data = view.split('@', 2);
        view = data[0];
        options.layout = data[1];
      }

      if(view.includes(':')) {
        let data = view.split(':', 2);
        view = data[0];
        options.cacheID = data[1];
      }

      let ext = (view.includes('.') ? view.substring(view.lastIndexOf('.')).replace('.', '') : undefined);
      if(ext && !options.ext){
        options.ext = ext
      }

      if (view.includes('.')) {
        view = view.substring(0, view.lastIndexOf('.'));
      }

      view = view.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

      const viewPath = join(OPTS.root, view);
      let fullpath = undefined;
      if(options.ext && fs.existsSync(viewPath + '.' + options.ext)){
        fullpath = viewPath + '.' + options.ext;
      }else if(fs.existsSync(viewPath + '.' + OPTS.ext)){
        fullpath = viewPath + '.' + OPTS.ext;
      }

      if(!fullpath){
        let page404 = join(OPTS.root, 'error/404.' + OPTS.ext);
        if (fs.existsSync(page404)) {
          _render.call(this, 'error/404', options, fn);
          return false;
        }
        page404 = join(OPTS.root, '404.' + OPTS.ext);
        if (fs.existsSync(page404)) {
          res.status(404)
          _render.call(this, '404', options, fn);
          return false;
        }
        res.status(404).send('<h1>Error 404</h1><h2>Page Not Found</h2>').end();
        return false;
      }

      let delFile = undefined;
      if(!fs.existsSync(viewPath + '.' + OPTS.ext)){
        // let err = fs.writeFileSync(viewPath + '.' + OPTS.ext, '');
        let wErr = undefined;
        fs.writeFile(viewPath + '.' + OPTS.ext, '', (err) => {
          if(err){
            wErr = err;
            return;
          }
          wErr = null;
        });

        let loops = 1000000;
        while(wErr === undefined && loops-- > 0){
          await sleep(1);
        }

        if(wErr || wErr === undefined){
          let page404 = join(OPTS.root, 'error/404.' + OPTS.ext);
          if (fs.existsSync(page404)) {
            _render.call(this, 'error/404', options, fn);
            return false;
          }
          page404 = join(OPTS.root, '404.' + OPTS.ext);
          if (fs.existsSync(page404)) {
            res.status(404)
            _render.call(this, '404', options, fn);
            return false;
          }
          res.status(404).send('<h1>Error 404</h1><h2>Page Not Found</h2>').end();
          return false;
        }
        
        delFile = viewPath + '.' + OPTS.ext;
      }

      _render.call(this, view, options, fn);

      if(delFile){
        fs.unlink(delFile, (err) => {});
      }

      return true;
    };
    
    next();
  });
}


async function runCompile(method, path, opts){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  const compOpts = clean({...opts});
  const keys = Object.keys(compOpts);
  for(let i = 0; i < keys.length; i++){
    if(keys[i].startsWith('!') || keys[i] === 'settings'){
      delete compOpts[keys[i]];
    }
  }

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  if(opts.settings){
    opts.settings.filename = path;
  }

  if(opts.cacheID){
    path += ':' + opts.cacheID.replace(/[^\w_-]+/, '')
  }

  if(opts.layout){
    path += '@' + opts.layout.replace(/[^\w_\-:\\\/]+/, '')
  }

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    return {error: 'low memory'};
  }

  // handle before functions
  if(method === 'comp' && typeof OPTS.before === 'function'){
    OPTS.before(opts);
  }
  if (typeof opts.before === 'function') {
    opts.before(opts);
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || defUpdateSpeed;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend(method, token, path, JSON.stringify(compOpts))

  // wait for compiler
  const startTime = Date.now();
  let loops = 0;
  let maxLoops = 1000 / updateSpeed;
  while(!CompilerOutput[token]){
    await sleep(updateSpeed);
    if(loops++ > maxLoops){
      loops = 0;
      if(Date.now() - startTime > timeout){
        return;
      }
    }
  }

  // get data if available
  let data = undefined;
  if(CompilerOutput[token]){
    data = CompilerOutput[token]
    delete CompilerOutput[token];
  }

  // handle after functions
  if (typeof opts.after === 'function') {
    let newData = opts.after(opts, data);
    if (newData !== undefined) {
      data = newData;
    }
  }
  if(method === 'comp' && typeof OPTS.after === 'function'){
    let newData = OPTS.after(opts);
    if (newData !== undefined) {
      data = newData;
    }
  }

  return data;
}


async function engine(path, opts, cb){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  if (!OPTS.ext) {
    setOpt('ext', (opts.settings['view engine'] || (path.includes('.') ? path.substring(path.lastIndexOf('.')).replace('.', '') : 'md')).toString())
  }

  if (!OPTS.root) {
    let root = (opts.settings.views || opts.settings.view || 'views').toString();
    if(UserRoot){
      if(!root.startsWith(UserRoot)){
        root = join(UserRoot, root);
      }
    }else if(!root.startsWith(ROOT)){
      root = join(ROOT, root);
    }
    setOpt('root', root);
  }

  const data = await runCompile('comp', path, opts);

  if(!opts._status){
    opts._status = function(){};
  }

  if(!data){
    opts._status(503);
    return cb(null, '<h1>Error 503</h1><h2>Service Unavailable</h2><p>The server failed to complete your request. Please try again or contact a server administrator about this error if it happens frequently.</p>');
  }

  if (data.res === 'error') {
    opts._status(500);
    if(DebugMode || process.env.NODE_ENV !== 'production'){
      if(DebugMode){
        console.error('\x1b[31m'+data.data, '\x1b[0m');
      }
      return cb(null, '<h1>Error 500</h1><h2>Internal Server Error</h2>' + `<p>${data.data}</p>`);
    }
    return cb(null, '<h1>Error 500</h1><h2>Internal Server Error</h2>');
  }


  if(opts._gzip){
    opts._status(200, true);
    return cb(null, Buffer.from(data.data, 'base64'));
  }

  zlib.gunzip(Buffer.from(data.data, 'base64'), (err, html) => {
    if(err){
      opts._status(500);
      return cb(null, '<h1>Error 500</h1><h2>Internal Server Error</h2>');
    }

    opts._status(200, true);
    return cb(null, html.toString());
  });
}

async function compile(path, opts){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  const data = await runCompile('comp', path, opts);

  if(!data){
    return {status: 503, gzip: false, html: '<h1>Error 503</h1><h2>Service Unavailable</h2><p>The server failed to complete your request. Please try again or contact a server administrator about this error if it happens frequently.</p>'};
  }

  if (data.res === 'error') {
    if(DebugMode || process.env.NODE_ENV !== 'production'){
      if(DebugMode){
        console.error('\x1b[31m'+data.data, '\x1b[0m');
      }
      return {status: 500, gzip: false, html: '<h1>Error 500</h1><h2>Internal Server Error</h2>' + `<p>${data.data}</p>`};
    }
    return {status: 500, gzip: false, html: '<h1>Error 500</h1><h2>Internal Server Error</h2>'};
  }


  return {status: 200, gzip: true, html: Buffer.from(data.data, 'base64'), unzip: function(){
    zlib.gunzip(Buffer.from(this.html, 'base64'), (err, html) => {
      if(err){
        return {status: 500, gzip: false, html: '<h1>Error 500</h1><h2>Internal Server Error</h2>'};
      }

      return {status: 200, gzip: false, html: html.toString()};
    });
  }};
}

async function preCompile(path, opts){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  const data = await runCompile('pre', path, opts);

  if(!data){
    return {error: 'failed to complete request'};
  }

  if (data.res === 'error') {
    return {error: data.data};
  }

  return null;
}

async function preCompileHasCache(path, opts){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  if(opts.cacheID){
    path += ':' + opts.cacheID.replace(/[^\w_-]+/, '')
  }

  if(opts.layout){
    path += '@' + opts.layout.replace(/[^\w_\-:\\\/]+/, '')
  }

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    return null;
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || defUpdateSpeed;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend('has', token, path)

  // wait for compiler
  const startTime = Date.now();
  let loops = 0;
  let maxLoops = 1000 / updateSpeed;
  while(!CompilerOutput[token]){
    await sleep(updateSpeed);
    if(loops++ > maxLoops){
      loops = 0;
      if(Date.now() - startTime > timeout){
        break;
      }
    }
  }

  // get data if available
  let data = undefined;
  if(CompilerOutput[token]){
    data = CompilerOutput[token]
    delete CompilerOutput[token];
  }

  if(!data || data.res === 'error'){
    return null;
  }

  return data.data === 'true';
}


function sortAppArgs(app, opts){
  if (typeof app === 'object') {
    [app, opts] = [opts, app];
  }
  if (typeof app !== 'function') {
    app = ExpressApp;
  }
  if (typeof app !== 'function') {
    return [null, null, 'app is not a function'];
  }
  if (typeof ExpressApp !== 'function') {
    ExpressApp = app;
  }

  if (typeof opts !== 'object') {
    opts = {};
  }

  return [app, opts];
}

function renderPages(app, opts){
  [app, opts, error] = sortAppArgs(app, opts);
  if(error){
    return;
  }

  app.use((req, res, next) => {
    const url = clean(req.url)
      .replace(/^[\\\/]+/, '')
      .replace(/\?.*/, '').replace(/[^\w_-]/g, '').toLowerCase();
    if (url === OPTS.layout || url.match(/^(errors?\/|)[0-9]{3}$/)) {
      next();
      return;
    }

    let urlPath = join(OPTS.root, url + '.' + OPTS.ext);
    if (urlPath === OPTS.root || !urlPath.startsWith(OPTS.root) || urlPath.startsWith(OPTS.components)) {
      next();
      return;
    }

    if (!fs.existsSync(urlPath)) {
      next();
      return;
    }

    try {
      res.render(url, opts);
    } catch (e) {
      next();
    }
  });

  app.use((req, res) => {
    let page404 = join(OPTS.root, 'error/404.' + OPTS.ext);
    if (fs.existsSync(page404)) {
      res.status(404).render('error/404', opts);
      return;
    }
    page404 = join(OPTS.root, '404.' + OPTS.ext);
    if (fs.existsSync(page404)) {
      res.status(404).render('404', opts);
      return;
    }
    res.status(404).send('<h1>Error 404</h1><h2>Page Not Found</h2>').end();
  });
}

function expressRateLimit(app, opts) {
  [app, opts, error] = sortAppArgs(app, opts);
  if(error){
    return;
  }

  const rateLimit = deviceRateLimit({
    err: function (req, res) {
      let page = join(OPTS.root, 'error/429.' + OPTS.ext);
      if (fs.existsSync(page)) {
        res.status(429).render('error/429', opts);
        return;
      }
      page = join(OPTS.root, err + '.' + OPTS.ext);
      if (fs.existsSync(page)) {
        res.status(429).render('429', opts);
        return;
      }
      res.status(429).send('<h1>Error 429</h1><h2>Too Many Requests</h2>').end();
    },
    ...opts,
  });

  if(!rateLimit){
    return false;
  }

  rateLimit.all(app);

  return true;
}


module.exports = (function(){
  const exports = function(opts = {
    views: 'views',
    ext: 'md',
    components: 'components',
    layout: 'layout',
    public: 'public',
    cache: '2h',
    opts: {},
    timeout: '30s',
    updateSpeed: defUpdateSpeed,
    before: undefined,
    after: undefined,
  }, app){
    if (typeof opts === 'function' || typeof app === 'object') {
      [opts, app] = [app, opts];
    }

    // set options
    if(typeof opts === 'object'){
      if(opts.root && typeof opts.root === 'string'){
        UserRoot = opts.root;
      }

      let root = (opts.views || opts.view || 'views');
      if(UserRoot){
        if(!root.startsWith(UserRoot)){
          root = join(UserRoot, root);
        }
      }else if(!root.startsWith(ROOT)){
        root = join(ROOT, root);
      }
      setOpt('root', root);

      if(typeof opts.ext === 'string'){
        setOpt('ext', opts.ext.replace(/[^\w_-]/g, ''))
      }else if(typeof opts.type === 'string'){
        setOpt('ext', opts.type.replace(/[^\w_-]/g, ''))
      }else{
        setOpt('ext', 'md');
      }

      setOpt('components', [opts.components, opts.component], 'components');
      setOpt('layout', [opts.layout, opts.template], 'components');
      setOpt('public', [opts.public, opts.static], 'public');
      setOpt('cache', opts.cache, '2h');

      if(typeof opts.opts === 'object'){
        setOpt('opts', '{}');
        ;(async function(){
          let json = await encrypt(JSON.stringify(opts.opts), EncKey);
          setOpt('opts', json);
        })();
      }else{
        setOpt('opts', '{}');
      }

      OPTS.timeout = toTimeMillis(opts.timeout) || toTimeMillis('30s');
      OPTS.updateSpeed = Number(opts.updateSpeed) || defUpdateSpeed;

      if(typeof opts.before === 'function'){
        OPTS.before = opts.before;
      }

      if(typeof opts.after === 'function'){
        OPTS.after = opts.after;
      }
    }

    // init express app
    if (typeof app === 'function') {
      setupExpress(app);
      ExpressApp = app;
    }

    // init compiler after a delay
    // this delay is to avoid overloading the system if nodemon is constantly turning the server off and on again
    setTimeout(initCompiler, 1000);

    return engine;
  };

  exports.compile = compile;
  exports.preCompile = preCompile;
  exports.inCache = preCompileHasCache;

  exports.renderPages = renderPages;
  exports.rateLimit = expressRateLimit;

  return exports;
})();
