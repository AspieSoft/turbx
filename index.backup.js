const fs = require('fs');
const { join } = require('path');
const zlib = require('zlib');
const multiTaskQueue = require('@aspiesoft/multi-task-queue');
const { spawn } = require('child_process');

const deviceRateLimit = requireOptional('express-device-rate-limit');

//todo: will likely rebuild for the new compiler

//todo: make sure DebugMode is set to "false" before publishing to github and npm
const DebugMode = false;

function requireOptional(path) {
  try {
    return require(path);
  } catch (e) {
    return undefined;
  }
}

const common = require('./common');
const { sleep, waitForMemory, randomToken, clean, toTimeMillis, encrypt, decrypt } = common;

const taskQueue = multiTaskQueue(10);

let ExpressApp = undefined;

const ROOT = (function () {
  if (require.main.filename) {
    return clean(require.main.filename.toString()).replace(/[\\\/][^\\\/]+[\\\/]?$/, '');
  }
  if (require.main.path) {
    return clean(require.main.path.toString());
  }
  if (process.cwd && process.cwd()) {
    return clean(process.cwd());
  }
  return clean(
    join(__dirname)
      .toString()
      .replace(/[\/\\]node_modules[\/\\][^\\\/]+[\\\/]?$/, '')
  );
})();

const OPTS = {};

const goCompiledResults = {};
let goCompiler;
let goCompilerLastInit = 0;

const EncKey = randomToken(32);

const goRecentId = {};
setInterval(function () {
  const now = Date.now();
  const id = Object.keys(goRecentId);
  for (let i = 0; i < id.length; i++) {
    if (now - goRecentId[id] > 20000) {
      delete goRecentId[id];
    }
  }
}, 20000);

let pingRes = false;
const golangOpts = {};

function initGoCompiler() {
  if(Date.now() - goCompilerLastInit < 100){
    return;
  }
  goCompilerLastInit = Date.now();

  const args = [
    `--enc="${EncKey}"`,
    `--root="${golangOpts.root}"`,
    `--ext="${golangOpts.ext}"`,
    `--components="${golangOpts.components}"`,
    `--layout="${golangOpts.layout}"`,
    `--public="${golangOpts.public}"`,
    `--opts="${JSON.stringify(golangOpts.opts)}"`,
    `--cache="${golangOpts.cache}"`,
  ]

  if(DebugMode){
    goCompiler = spawn('go', ['run', '.', ...args], {cwd: join(__dirname, 'turbx')});
  }else{
    goCompiler = spawn('./turbx/turbx', [...args], { cwd: __dirname });
  }

  goCompiler.on('close', () => {
    initGoCompiler();
  })
  goCompiler.stderr.on('end', () => {
    initGoCompiler();
  });

  goCompiler.stdout.on('data', async (data) => {
    data = data.toString().trim();

    if(data === 'pong'){
      pingRes = true;
      return;
    }

    data = await decrypt(data, EncKey);
    if(!data){
      console.log(data);
      return;
    }

    let resToken = undefined;
    let resType = undefined;
    data = data.replace(/^([\w_-]+):([\w_-]+):/, (_, id, type) => {
      resToken = id;
      resType = type;
      return '';
    });
    if (!idToken || !resType) {
      return;
    }

    const now = Date.now();
    if (now - goRecentId[resToken] < 20000) {
      return;
    }

    goRecentId[resToken] = now;
    goCompiledResults[resToken] = {res: resType, data};
  });
}
initGoCompiler();

setInterval(async function(){
  pingRes = false;
  goCompiler.stdin.write('ping\n');
  await sleep(1000);
  if(!pingRes){
    initGoCompiler();
  }
}, 10000);

async function goCompilerSendRes(res){
  const enc = await encrypt(res, EncKey);
  if(enc){
    goCompiler.stdin.write(enc + '\n');
  }
}

function goCompilerSetOpt(key, value, setLiveOpts = false) {
  key = key.toString().replace(/[^\w_-]/g, '');

  if(setLiveOpts){
    golangOpts[key] = value;
    goCompilerSendRes(`opts:${JSON.stringify(value)}`);
    return;
  }

  value = value.toString().replace(/[\r\n\v]/g, '');
  golangOpts[key] = value;

  let oldCompiler = goCompiler;
  initGoCompiler();

  setTimeout(async function(){
    const enc = await encrypt(res, EncKey);
    if(enc){
      oldCompiler.stdin.write(enc + '\n');
    }
  }, 1000);
}

async function goCompilerPreCompile(file, opts) {
  const token = randomToken(64);
  goCompilerSendRes(`pre:${token}:${file.toString().replace(/[\r\n\v]/g, '')}:${JSON.stringify(opts)}`)

  const updateSpeed = Number(OPTS.updateSpeed) || 1;

  let loops = (toTimeMillis(OPTS.timeout) || 30000) / updateSpeed;
  while (loops-- > 0) {
    if (goCompiledResults[token]) {
      break;
    }
    await sleep(updateSpeed);
  }

  const res = goCompiledResults[token];
  delete goCompiledResults[token];
  return res;
}

async function goCompilerHasCache(file) {
  const token = randomToken(64);
  goCompilerSendRes(`has:${token}:${file.toString().replace(/[\r\n\v]/g, '')}`)

  const updateSpeed = Number(OPTS.updateSpeed) || 1;

  let loops = (toTimeMillis(OPTS.timeout) || 30000) / updateSpeed;
  while (loops-- > 0) {
    if (goCompiledResults[token]) {
      break;
    }
    await sleep(updateSpeed);
  }

  const res = goCompiledResults[token];
  delete goCompiledResults[token];
  if(res === 'true' || res === true){
    return true;
  }
  return false;
}

async function goCompilerCompile(file, opts) {
  const token = randomToken(64);

  const updateSpeed = Number(OPTS.updateSpeed) || 1;

  let reqStarted = Date.now();
  goCompilerSendRes(`comp:${token}:${file.toString().replace(/[\r\n\v]/g, '')}:${JSON.stringify(opts)}`)

  while (loops-- > 0) {
    if (goCompiledResults[token]) {
      break;
    }

    if (reqStarted < goCompilerLastInit) {
      reqStarted = Date.now();
      goCompilerSendRes(token + ':' + zippedOpts + ':' + file.toString().replace(/[\r\n\v]/g, ''));
    }

    await sleep(updateSpeed);
  }

  const res = goCompiledResults[token];
  delete goCompiledResults[token];
  return res;
}

function exitHandler(options, exitCode) {
  if (options.cleanup) {
    goCompilerSendRes('stop');
  }
  if (options.exit) {
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

function engine(path, opts, cb) {
  path = clean(path);
  opts = clean(opts);

  if(!opts){
    opts = {};
  }

  if (!OPTS.ext) {
    OPTS.ext = opts.settings['view engine'] || (path.includes('.') ? path.substring(path.lastIndexOf('.')).replace('.', '') : 'xhtml');
    goCompilerSetOpt('ext', OPTS.ext);
  }

  if (!OPTS.root) {
    OPTS.root = opts.settings.views || opts.settings.view || (require.main.filename || process.argv[1] || __dirname).replace(/([\\\/]node_modules[\\\/].*?|[\\\/][^\\\/]*)$/, '/views');
    goCompilerSetOpt('root', OPTS.root);
  }

  if (path.includes('.')) {
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  opts.settings.filename = path;

  taskQueue(path, async () => {
    if (OPTS.before) {
      OPTS.before(opts);
    }
    if (typeof opts.before === 'function') {
      opts.before(opts);
    }

    let data = await goCompilerCompile(path, opts);

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

    if (!data || data === 'error') {
      return cb(null, '<h1>Error 500</h1><h2>Internal Server Error</h2>');
    }

    return cb(null, data);
  });
}

function setOpts(opts) {
  let before = opts.before;
  let after = opts.after;
  opts = clean(opts);
  let root = opts.views || opts.view || 'views';

  OPTS.ext = (opts.ext || 'md').replace(/[^\w_\-]/g, '');
  let rootPath = root.replace(/[^\w_\-\\\/\.@$#!]/g, '');
  if (!rootPath.startsWith(ROOT)) {
    rootPath = join(ROOT, rootPath);
  }
  OPTS.root = rootPath;

  let componentsOpt = opts.components || opts.component || 'components';
  if (componentsOpt) {
    OPTS.components = componentsOpt.replace(/[^\w_\-\\\/\.@$#!]/g, '');
  }

  let template = (opts.template ?? opts.layout ?? 'layout').replace(/[^\w_\-\\\/\.@$#!]/g, '');
  if (template && template.trim() !== '') {
    OPTS.template = template;
  }

  OPTS.cache = opts.cache || '2h';
  OPTS.timeout = opts.timeout || '30s';

  if (typeof before === 'function') {
    OPTS.before = before;
  }

  if (typeof after === 'function') {
    OPTS.after = after;
  }

  if (typeof opts.static === 'string') {
    OPTS.static = opts.static;
  } else {
    OPTS.static = '/';
  }

  if (typeof opts.public === 'string') {
    OPTS.public = opts.public;
  }

  if (typeof opts.opts === 'object') {
    OPTS.opts = opts.opts;
  } else {
    OPTS.opts = {};
  }

  if (['number', 'string'].includes(typeof opts.updateSpeed)) {
    updateSpeed = toTimeMillis(opts.updateSpeed);
    if (updateSpeed && updateSpeed > 0) {
      OPTS.updateSpeed = updateSpeed;
    }
  } else {
    // OPTS.updateSpeed = 10;
    OPTS.updateSpeed = 1;
  }

  goCompilerSetOpt('root', OPTS.root);
  goCompilerSetOpt('ext', OPTS.ext);
  goCompilerSetOpt('components', OPTS.components);
  goCompilerSetOpt('cache', OPTS.cache);
  goCompilerSetOpt('opts', OPTS.opts);

  if (!OPTS.template || OPTS.template === '') {
    goCompilerSetOpt('layout', '!');
  }else{
    goCompilerSetOpt('layout', OPTS.template);
  }
}

function setupExpress(app) {
  app.use((req, res, next) => {
    res.preRender = goCompilerPreCompile;
    res.preCompiled = goCompilerHasCache;
    next();
  });

  return;

  app.use('/lazyload/:token/:component', async (req, res, next) => {
    //todo: add lazy loading option
  });
}

function expressFallbackPages(app, opts) {
  if (typeof app === 'object') {
    [app, opts] = [opts, app];
  }
  if (typeof app !== 'function') {
    app = ExpressApp;
  }
  if (typeof app !== 'function') {
    return;
  }
  if (typeof opts !== 'object') {
    opts = {};
  }

  app.use((req, res, next) => {
    const url = clean(req.url)
      .replace(/^[\\\/]+/, '')
      .replace(/\?.*/, '');
    if (url === OPTS.template || url.match(/^(errors?\/|)[0-9]{3}$/)) {
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
  if (typeof app === 'object') {
    [app, opts] = [opts, app];
  }
  if (typeof app !== 'function') {
    app = ExpressApp;
  }
  if (typeof app !== 'function') {
    return;
  }
  if (typeof opts !== 'object') {
    opts = {};
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

  rateLimit.all(app);
}

module.exports = (function () {
  const exports = function (
    opts = {
      views: 'views',
      components: 'components',
      ext: 'xhtml',
      template: undefined,
      cache: '2h',
      timeout: '30s',
      before: undefined,
      after: undefined,
      static: '/',
      opts: {},
      updateSpeed: 10,
    },
    app
  ) {
    if (typeof opts === 'function' || typeof app === 'object') {
      [opts, app] = [app, opts];
    }

    if (typeof opts === 'object') {
      setOpts(opts);
    } else {
      setOpts({});
    }

    if (typeof app === 'function') {
      setupExpress(app);
      ExpressApp = app;
    }

    return engine;
  };

  exports.preRender = goCompilerPreCompile;
  exports.preCompiled = goCompilerHasCache;
  exports.render = goCompilerCompile;

  exports.renderPages = expressFallbackPages;
  exports.rateLimit = expressRateLimit;

  return exports;
})();
