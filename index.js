const {join} = require('path');
const { spawn } = require('child_process');
const zlib = require('zlib');

const { sleep, waitForMemory, randomToken, clean, toTimeMillis, encrypt, decrypt } = require('./common');

const DebugMode = process.argv.includes('--turbx-debug');

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
    Compiler = spawn('go', ['run', '.', ...args], {cwd: join(__dirname, 'turbx')});
  }else{
    Compiler = spawn('./turbx/turbx', [...args], { cwd: __dirname });
  }

  Compiler.on('close', () => {
    if(!stoppingCompiler){
      initCompiler();
    }
  })
  Compiler.stderr.on('end', () => {
    if(!stoppingCompiler){
      initCompiler();
    }
  });

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
    const _render = res.render;
    res.render = function(view, options, fn){
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

      _render.call(this, view, options, fn);
    };
    
    next();
  });
}


async function compile(method, path, opts){
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

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    return {error: 'low memory'};
  }

  if(method === 'comp' && typeof OPTS.before === 'function'){
    OPTS.before(opts);
  }
  if (typeof opts.before === 'function') {
    opts.before(opts);
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || 10;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend('pre', token, path, JSON.stringify(compOpts))

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

  if(!opts._status){
    opts._status = function(){};
  }

  const compOpts = clean({...opts});
  const keys = Object.keys(compOpts);
  for(let i = 0; i < keys.length; i++){
    if(keys[i].startsWith('!') || keys[i] === 'settings'){
      delete compOpts[keys[i]];
    }
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

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');
  opts.settings.filename = path;

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    opts._status(503);
    return cb(null, '<h1>Error 503</h1><h2>Service Unavailable</h2><p>The server is currently overloaded and low on available memory (RAM). Please try again later.</p>');
  }

  // handle before functions
  if (OPTS.before) {
    OPTS.before(opts);
  }
  if (typeof opts.before === 'function') {
    opts.before(opts);
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || 10;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend('comp', token, path, JSON.stringify(compOpts))

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
  if (OPTS.after) {
    let newData = OPTS.after(opts, data);
    if (newData !== undefined) {
      data = newData;
    }
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
    zlib.gzip(html, (err, res) => {
      return cb(null, res);
    })
    // return cb(null, html.toString());
  });
}

async function preCompile(path, opts){
  path = clean(path);
  if(!opts){
    opts = {};
  }

  const compOpts = clean({...opts});
  const keys = Object.keys(compOpts);
  for(let i = 0; i < keys.length; i++){
    if(keys[i].startsWith('!')){
      delete compOpts[keys[i]];
    }
  }

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    return {error: 'low memory'};
  }

  if (typeof opts.before === 'function') {
    opts.before(opts);
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || 10;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend('pre', token, path, JSON.stringify(compOpts))

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

  const compOpts = clean({...opts});
  const keys = Object.keys(compOpts);
  for(let i = 0; i < keys.length; i++){
    if(keys[i].startsWith('!')){
      delete compOpts[keys[i]];
    }
  }

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  const timeout = Number(opts.timeout) || Number(OPTS.timeout) || toTimeMillis('30s');

  // ensure we have at least 1mb of memory available (or wait to reduce memory usage)
  const memAvailable = await waitForMemory(1, timeout);
  if(!memAvailable){
    return {error: 'low memory'};
  }

  if (typeof opts.before === 'function') {
    opts.before(opts);
  }

  const updateSpeed = Number(opts.updateSpeed) || Number(OPTS.updateSpeed) || 10;

  // request file from compiler
  const token = randomToken(64);
  if(CompilerOutput[token]){
    await sleep(Math.max(100, OPTS.updateSpeed + 100));
    delete CompilerOutput[token]
  }

  compilerSend('has', token, path, JSON.stringify(compOpts))

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

  if(!data){
    return {error: 'failed to complete request'};
  }

  if (data.res === 'error') {
    return {error: data.data};
  }

  return null;
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
    updateSpeed: 10,
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
          let json = await encrypt(JSON.stringify(opts.opts));
          setOpt('opts', json);
        })();
      }else{
        setOpt('opts', '{}');
      }

      OPTS.timeout = toTimeMillis(opts.timeout) || toTimeMillis('30s');
      OPTS.updateSpeed = Number(opts.updateSpeed) || 10;

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

  return exports;
})();
