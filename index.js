const fs = require('fs');
const {join} = require('path');
const crypto = require('crypto');
const memoryCache = require('@aspiesoft/obj-memory-cache');
const multiTaskQueue = require('@aspiesoft/multi-task-queue');

const bodyParser = requireOptional('body-parser');
const device = requireOptional('express-device');

function requireOptional(path){
  try {
    return require(path);
  } catch(e) {
    return undefined;
  }
}


const common = require('./common');
const {
  escapeHTML,
  escapeHTMLArgs,
  compileMD,
  compileJS,
  compileCSS,
  sleep,
  randomToken,
  clean,
  toTimeMillis,
  asyncReplace,
  getOpt,
} = common;

const {tagFunctions, addTagFunction, runTagFunction} = require('./functions');


//todo: may think of another name for node module
// trbx: turbo regex, or Tiny Regex to Basic XML (search results show guitars)
// turbx: turbo regex, or Tiny Usable Regex to Basic XML (search results show turbx.com as research on the speed of light)


const localCache = memoryCache.newCache({watch: __dirname});
const taskQueue = multiTaskQueue(10);

const singleTagsList = ['meta', 'link', 'img', 'br', 'hr', 'input'];


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


const emptyRes = JSON.stringify({html: '', scripts: [], args: []});
function getTemplateFile(path, opts, cb, component = false){
  const componentsPath = opts.components || OPTS.components || undefined;

  taskQueue(path, async (next) => {
    if(component && componentsPath){
      const data = localCache.get('template_component_cache:' + path) /* ?? undefined */;
      if(data || data === ''){
        next();
        cb(data);
        return;
      }
    }

    const data = localCache.get('template_file_cache:' + path) /* ?? undefined */;
    if(data || data === ''){
      next();
      cb(data);
      return;
    }

    let filePath = undefined;
    if(component && componentsPath){
      filePath = join(componentsPath, path + '.' + OPTS.ext);
      if(filePath === OPTS.root || !filePath.startsWith(OPTS.root)){
        localCache.set('template_component_cache:' + path, '', {expire: opts.cache || OPTS.cache || '2h'});
        next();
        cb(emptyRes);
        return;
      }
    }

    if(!filePath || !fs.existsSync(filePath)){
      filePath = join(OPTS.root, path + '.' + OPTS.ext);
      if(filePath === OPTS.root || !filePath.startsWith(OPTS.root)){
        localCache.set('template_file_cache:' + path, '', {expire: opts.cache || OPTS.cache || '2h'});
        next();
        cb(emptyRes);
        return;
      }
    }


    if(!filePath || !fs.existsSync(filePath)){
      filePath = join(OPTS.root, path, 'index.' + OPTS.ext);
      if(filePath === OPTS.root || !filePath.startsWith(OPTS.root)){
        localCache.set('template_file_cache:' + path, '', {expire: opts.cache || OPTS.cache || '2h'});
        next();
        cb(emptyRes);
        return;
      }
    }

    if(!filePath || !fs.existsSync(filePath)){
      localCache.set('template_file_cache:' + path, '', {expire: opts.cache || OPTS.cache || '2h'});
      next();
      cb(emptyRes);
      return;
    }

    fs.readFile(filePath, (err, data) => {
      if(err){
        localCache.set('template_file_cache:' + path, '', {expire: opts.cache || OPTS.cache || '2h', listen: [filePath]});
        next();
        cb(emptyRes);
        return;
      }
      data = preCompile(data.toString());
      localCache.set('template_file_cache:' + path, data, {expire: opts.cache || OPTS.cache || '2h', listen: [filePath]});
      next();
      cb(data);
    });
  });
}


async function cacheLazyLoad(path, opts){
  const token = randomToken(64);

  getTemplateFile(path, opts, async (file) => {
    const data = await compile(file, opts);
    localCache.set('template_lazyload_cache:' + token, data, {expire: opts.lazyCache || OPTS.lazyCache || '12h'});
  });

  return token;
}


function engine(path, opts, cb){
  path = clean(path);
  opts = clean(opts);

  if(!OPTS.ext){
    OPTS.ext = opts.settings['view engine'] || (path.includes('.') ? path.substring(path.lastIndexOf('.')).replace('.', '') : 'xhtml');
  }

  if(!OPTS.root){
    OPTS.root = opts.settings.views || opts.settings.view || (require.main.filename || process.argv[1] || __dirname).replace(/([\\\/]node_modules[\\\/].*?|[\\\/][^\\\/]*)$/, '/views');
  }

  if(path.includes('.')){
    path = path.substring(0, path.lastIndexOf('.'));
  }

  path = path.replace(OPTS.root, '').replace(/^[\\\/]+/, '');

  opts.settings.filename = path;

  getTemplateFile(path, opts, async (data) => {
    if(data === emptyRes){
      return cb(new Error('View Not Found!'), '');
    }

    if(OPTS.before){
      let newData = OPTS.before(data, opts);
      if(newData !== undefined){
        data = newData;
      }
    }
    if(typeof opts.before === 'function'){
      let newData = opts.before(data, opts);
      if(newData !== undefined){
        data = newData;
      }
    }

    data = await compile(data, opts, true);

    if(typeof opts.after === 'function'){
      let newData = opts.after(data, opts);
      if(newData !== undefined){
        data = newData;
      }
    }
    if(OPTS.after){
      let newData = OPTS.after(data, opts);
      if(newData !== undefined){
        data = newData;
      }
    }

    return cb(null, data);
  });
}


function encodeEncoding(html){
  return html.replace(/%!|!%/g, (s) => {
    if(s === '%!'){
      return '%!o!%';
    }else if(s === '!%'){
      return '%!c!%';
    }
    return '';
  });
}

function decodeEncoding(html){
  return html.replace(/%!([oc])!%/g, (_, s) => {
    if(s === 'o'){
      return '%!';
    }else if(s === 'c'){
      return '!%';
    }
    return '';
  });
}


function preCompile(file){
  file = encodeEncoding(file);

  const stringList = [];
  file = file.replace(/(['"`])((?:\\[\\'"`]|.)*?)\1|<!--.*?-->|\/\*.*?\*\/|(?<!:)(?:\/\/|#!).*?\r?\n/gs, (_, tag, str) => {
    if(!tag || !str){
      return '';
    }
    return `%!${stringList.push({tag, str})-1}!%`;
  });

  function decompStrings(str, num){
    return str.replace(/%!([0-9]+)!%/g, (_, i) => {
      if(num === false){
        return stringList[i].str;
      }else if(num === true){
        let str = stringList[i].str;
        if(str.match(/^-?[0-9]+(\.[0-9]+|)$/)){
          return str;
        }
        return stringList[i].tag + str + stringList[i].tag
      }
      return stringList[i].tag + stringList[i].str + stringList[i].tag;
    }).replace(/%!([oc])!%/g, (_, t) => {
      if(t === 'o'){
        return '%!';
      }else if(t === 'c'){
        return '!%';
      }
      return '';
    });
  }


  //todo: compile scripts in go
  const scripts = [];
  file = file.replace(/<(script|js|style|css|less|markdown|md|text|txt)(.*?)>(.*?)<\/\1>/gsi, (_, tag, args, content) => {
    tag = tag.toLowerCase();

    if(tag === 'md'){
      tag = 'markdown';
    }else if(tag === 'txt'){
      tag = 'text';
    }else if(tag === 'js'){
      tag = 'script';
    }else if(tag === 'css' || tag === 'less'){
      tag = 'style';
    }
    
    args = decompStrings(args);

    if(tag === 'script'){
      let strings = [];
      content = content.replace(/%!([0-9]+)!%/g, (_, i) => {
        return `%!${strings.push(stringList[i]) - 1}!%`;
      });

      content = compileJS(content);

      return `<!_script ${scripts.push({tag, args, content, strings})-1}/>`;
    }

    content = decompStrings(content);

    //todo: compile scripts
    if(tag === 'text'){
      content = escapeHTML(content);
    }else if(tag === 'markdown'){
      content = compileMD(content);
    }else if(tag === 'script'){
      content = compileJS(content);
    }else if(tag === 'style'){
      content = compileCSS(content);
    }

    return `<!_script ${scripts.push({tag, args, content})-1}/>`;
  });


  const argList = [];
  let tagIndex = 0;
  file = file.replace(/<(\/|)([\w_\-\.$!:]+)\s*(.*?)\s*(\/|)>/gs, (_, close, tag, args, selfClose) => {
    args = args.split(/\s+/);

    if(!tag.match(/^_|[A-Z]/)){
      args = args.sort((a, b) => {
        if(typeof a !== 'string' || typeof b !== 'string'){
          return 0;
        }

        let aK = a.substring(0, a.indexOf('='));
        let bK = b.substring(0, b.indexOf('='));

        if(aK > bK){
          return 1;
        }else if(aK < bK){
          return -1;
        }
        return 0;
      }).join(' ');
      args = decompStrings(args);

      if(args.trim() !== ''){
        args = ' ' + args;
      }
      return `<${close}${tag}${args}${selfClose}>`;
    }

    // open and self closing tag
    if(close === ''){
      let newArgs;
      if(tag.match(/^_(el(if|se)|if)$/)){
        newArgs = [];
        for(let i = 0; i < args.length; i++){
          if(args[i].match(/^([!<>]?=|{{{?.*?}}}?)$/)){
            newArgs.push(args[i]);
            continue;
          }
  
          let arg = args[i].split(/([!<>]?=|[&|<>])/);
          newArgs.push(...arg.map(a => decompStrings(a, true)).filter(a => a && a.trim() !== ''));
        }

        newArgs.map(arg => {
          if(arg.match(/^(["'`])[0-9]+(?:\.[0-9]+|)\1$/)){
            return Number(arg.replace(/^(["'`])([0-9]+(?:\.[0-9]+|))\1$/, '$2'));
          }
          return arg;
        });
      }else{
        newArgs = {};
        let ind = 0;
        for(let i = 0; i < args.length; i++){
          if(args[i] === ''){
            continue;
          }

          if(args[i] && args[i+1] === '=' && args[i+2]){
            newArgs[decompStrings(args[i], false)] = decompStrings(args[i+2], true);
            i += 2;
            continue;
          }
  
          let arg = args[i].split('=');
          if(arg.length === 1){
            newArgs[ind] = decompStrings(arg[0], true);
            ind++;
          }else if(arg.length === 2){
            newArgs[decompStrings(arg[0], false)] = decompStrings(arg[1], true);
          }else if(arg.length > 2){
            let last = arg[arg.length-1];
            for(let i = 0; i < arg.length-1; i++){
              newArgs[decompStrings(arg[i], false)] = decompStrings(arg[last], true);
            }
          }
        }

        const newArgKeys = Object.keys(newArgs);
        for(let i = 0; i < newArgKeys.length; i++){
          if(newArgs[newArgKeys[i]].match(/^(["'`])[0-9]+(?:\.[0-9]+|)\1$/)){
            newArgs[newArgKeys[i]] = Number(newArgs[newArgKeys[i]].replace(/^(["'`])([0-9]+(?:\.[0-9]+|))\1$/, '$2'));
          }
        }
      }

      let res = `<${tag}:${tagIndex} ${argList.push(newArgs)-1}${selfClose}>`;

      // open tag
      if(selfClose === ''){
        tagIndex++;
      }

      return res;
    }

    // closing tag
    tagIndex--;
    return `</${tag}:${tagIndex}>`;
  });

  file = decompStrings(file);

  file = decodeEncoding(file);

  // pre compile and minify


  //todo: pre compile less to css
  //todo: pre compile markdown to html
  //todo: minify js, css, and html


  return JSON.stringify({html: file, scripts, args: argList, strings: stringList});
}

async function compile(file, opts, includeTemplate = false){
  if(OPTS.opts){
    opts = {...OPTS.opts, ...opts};
  }

  if(typeof file === 'string'){
    file = JSON.parse(file);
  }


  if(includeTemplate){
    let template = opts.template || OPTS.template;
    if(typeof template === 'string'){
      let done = false;
      getTemplateFile(template, opts, async (layout) => {
        layout = await compile(layout, opts);

        if(layout.trim() === ''){
          done = true;
          return;
        }

        if(layout.match(/{{{?body}}}?|<body(\s+.*?|)\/>/si)){
          file.html = layout.replace(/{{{?body}}}?|<body(\s+.*?|)\/>/si, file.html);
        }else if(layout.match(/<(main|\/header)(\s+.*?|)>/si)){
          file.html = layout.replace(/<(main|\/header)(\s+.*?|)>/si, (str) => {
            return str + '\n' + file.html;
          });
        }else if(layout.match(/<(footer|\/body)(\s+.*?|)>/si)){
          file.html = layout.replace(/<(footer|\/body)(\s+.*?|)>/si, (str) => {
            return file.html + '\n' + str;
          });
        }else{
          file.html = layout + file.html;
        }

        done = true;
      });

      while(!done){
        await sleep(10);
      }
    }
  }

  await runFunctions(file, opts);


  //todo: add opts to css, and markdown

  if(includeTemplate){
    ;(function(){
      let publicOpts
      if(isSecureMode(opts) === false){
        publicOpts = {...opts};
        delete publicOpts.secure;
        delete publicOpts.settings;
        delete publicOpts._locals;
        delete publicOpts.cache;
        delete publicOpts.template;
        delete publicOpts.view;
        delete publicOpts.views;
      }else if(opts.public){
        publicOpts = {...opts.public};
      }else{
        return;
      }

      const publicScript = `<script>
        ;const OPTS = ${JSON.stringify(publicOpts)};
      </script>`

      if(file.html.match(/<\/head>/)){
        file.html = file.html.replace(/<\/head>/, `
        ${publicScript}
        </head>
        `);
      }else if(file.html.match(/<body(\s+.*?|)>/)){
        file.html = file.html.replace(/<body(\s+.*?|)>/, `
        <body$1>
        ${publicScript}
        `);
      }else if(file.html.match(/<!_script(\s+.*?|)>/)){
        file.html = file.html.replace(/!_<script(\s+.*?|)>/, `
        ${publicScript}
        <!_script$1>
        `);
      }else{
        file.html = publicScript + file.html;
      }
    })();
  }


  // clean up output

  file.html = file.html.replace(/<(\/|)([\w_\-\.$!:]+):[0-9]+\s+(.*?)(\/|)>/g, (_, close, tag, arg, selfClose) => {
    if(tag.startsWith('_') || tag.startsWith('!_')){
      return '';
    }

    if(close === ''){
      let argList = [];
      let args = file.args[arg];
      let keys = Object.keys(args);
      for(let i = 0; i < keys.length; i++){
        if(Number(keys[i]) || Number(keys[i]) === 0){
          argList.push(args[keys[i]]);
        }else{
          argList.push(keys[i] + '=' + args[keys[i]]);
        }
      }

      if(!Array.isArray(argList)){
        const argKeys = Object.keys(argList);
        const argArr = [];
        for(let i = 0; i < argKeys.length; i++){
          argArr.push(argKeys[i] + '=' + argList[argKeys[i]]);
        }
        argList = argArr;
      }

      argList = argList.sort((a, b) => {
        if(typeof a !== 'string' || typeof b !== 'string'){
          return 0;
        }

        let aK = a.substring(0, a.indexOf('='));
        let bK = b.substring(0, b.indexOf('='));

        if(aK > bK){
          return 1;
        }else if(aK < bK){
          return -1;
        }
        return 0;
      }).join(' ');

      if(argList.trim() !== ''){
        argList = ' ' + argList;
      }

      let res = `<${tag}${argList}>`;
      if(selfClose !== ''){
        res += `</${tag}>`;
      }

      return res;
    }

    return `</${tag}>`;
  });

  file.html = file.html.replace(/<!_script\s+([0-9]+)\/>/gs, (_, i) => {
    let script = file.scripts[i];

    let content = script.content;
    if(script.tag === 'script'){
      content = content.replace(/(?<![\w_$]+)OPTS\.((?:\[.*?\]|[\w_$\.])+)/g, (_, arg) => {
        let value = getOpt(opts, arg, false);
        if(value instanceof RegExp){
          return value.toString();
        }else if(typeof value === 'object'){
          return JSON.stringify(value);
        }else if(typeof value === 'string'){
          return '\'' + value.replace(/'/g, '\\\'') + '\'';
        }else if(value === undefined){
          return undefined;
        }else if(value === null){
          return null;
        }
        return value.toString();
      }).replace(/%!([0-9]+)!%/g, (_, i) => {
        return script.strings[i].tag + script.strings[i].str + script.strings[i].tag;
      }).replace(/%!([oc])!%/g, (_, t) => {
        if(t === 'o'){
          return '%!';
        }else if(t === 'c'){
          return '!%';
        }
        return '';
      });
    }

    if(script.tag === 'script' || script.tag === 'style'){
      return `<${script.tag}${script.args}>${content}</${script.tag}>`;
    }

    return `<div type="${script.tag}"${script.args}>${content}</div>`;
  });

  return file.html;
}


async function runFunctions(file, opts, level = 0){
  file.html = await asyncReplace(file.html, new RegExp(`<_([\\w_\\-\\.$!:]+):${level}(\\s+[0-9]+|)>(.*?)</_\\1:${level}>`, 'gs'), async (_, tag, arg, content) => {
    let args = file.args[arg.trim()];

    tag = tag.toLowerCase().replace(/[-_]/g, '');

    const func = tagFunctions[tag];
    if(!func){
      return '';
    }

    if(Array.isArray(func)){
      let res = undefined;
      for(let i = 0; i < func.length; i++){
        let fn = func[i];
        if(typeof fn === 'string'){
          fn = tagFunctions[fn];
        }

        if(typeof fn === 'function'){
          res = await func(args, content, opts, level + 1, {scripts: file.scripts, args: file.args});
        }
      }

      if(['string', 'number', 'boolean'].includes(typeof res) && !Number.isNaN(res)){
        return await runFunctions({html: res.toString(), scripts: file.scripts, args: file.args}, opts, level + 1);
      }else if(Array.isArray(res)){
        let html = '';
        for(let i = 0; i < res.length; i++){
          html += await runFunctions({html: res[i].html.toString(), scripts: file.scripts, args: file.args}, res[i].opts, level + 1);
        }
        return html;
      }
      return '';
    }

    if(typeof func === 'string'){
      func = tagFunctions[func];
    }

    if(typeof func === 'function'){
      let cont = await func(args, content, opts, level + 1, {scripts: file.scripts, args: file.args});
      if(['string', 'number', 'boolean'].includes(typeof cont) && !Number.isNaN(cont)){
        return await runFunctions({html: cont.toString(), scripts: file.scripts, args: file.args}, opts, level + 1);
      }else if(Array.isArray(cont)){
        let html = '';
        for(let i = 0; i < cont.length; i++){
          html += await runFunctions({html: cont[i].html.toString(), scripts: file.scripts, args: file.args}, cont[i].opts, level + 1);
        }
        return html;
      }
    }

    return '';
  });

  file.html = await asyncReplace(file.html, new RegExp(`<([A-Z][\\w_\\-\\.$!:]+):${level}(\\s+[0-9]+|)>(.*?)</\\1:${level}>`, 'gs'), async (_, tag, arg, content) => {
    let args = file.args[arg.trim()];
    tag = tag.toLowerCase();

    content = await runFunctions({html: content, scripts: file.scripts, args: file.args}, opts, level + 1);

    let res = undefined;
    const compOpts = {...opts, ...args, body: content};
    getTemplateFile(tag, compOpts, async (file) => {
      file = JSON.parse(file);

      /* let bodyTags = [];

      file.html = encodeEncoding(file.html).replace(/<body\/>|{{{?body}}}?/gsi, (tag) => {
        return `%!${bodyTags.push(tag)-1}!%`;
      });

      let html = await compile(file, compOpts);

      res = decodeEncoding(html.replace(/%!([0-9]+)!%/g, (_, i) => {
        return bodyTags[i].replace(/<body\/>|{{{body}}}/si, encodeEncoding(content)).replace(/{{body}}/si, encodeEncoding(escapeHTML(content)));
      })); */

      res = await compile(file, compOpts);

      // res = html.replace(/<body\/>|{{{body}}}/si, content).replace(/{{body}}/si, escapeHTML(content));
    }, true);

    while(res === undefined){
      await sleep(10);
    }

    return res;
  });

  file.html = await asyncReplace(file.html, new RegExp(`<([A-Z][\\w_\\-\\.$!:]+):${level}(\\s+[0-9]+|)/>`, 'gs'), async (_, tag, arg) => {
    let args = file.args[arg.trim()];
    tag = tag.toLowerCase();

    let res = undefined;
    const compOpts = {...opts, ...args};
    getTemplateFile(tag, compOpts, async (file) => {
      let html = await compile(file, compOpts);
      res = html.replace(/<body\/>|{{{?body}}}?/si, '');
    }, true);

    while(res === undefined){
      await sleep(10);
    }

    return res;
  });

  file.html = await asyncReplace(file.html, new RegExp(`<_([\\w_\\-\\.$!:]+):${level}(\\s+[0-9]+|)/>`, 'gs'), async (_, tag, arg) => {
    let args = file.args[arg.trim()];

    tag = tag.toLowerCase().replace(/[-_]/g, '');

    let func = tagFunctions[tag];
    if(!func){
      return '';
    }

    if(Array.isArray(func)){
      let res = undefined;
      for(let i = 0; i < func.length; i++){
        let fn = func[i];
        if(typeof fn === 'string'){
          fn = tagFunctions[fn];
        }

        if(typeof fn === 'function'){
          res = await func(args, null, opts, level + 1, {scripts: file.scripts, args: file.args});
        }
      }

      if(['string', 'number', 'boolean'].includes(typeof res) && !Number.isNaN(res)){
        return await runFunctions({html: res.toString(), scripts: file.scripts, args: file.args}, opts, level + 1);
      }else if(Array.isArray(res)){
        let html = '';
        for(let i = 0; i < res.length; i++){
          html += await runFunctions({html: res[i].html.toString(), scripts: file.scripts, args: file.args}, res[i].opts, level + 1);
        }
        return html;
      }
      return '';
    }

    if(typeof func === 'string'){
      func = tagFunctions[func];
    }

    if(typeof func === 'function'){
      let cont = await func(args, null, opts, level + 1, {scripts: file.scripts, args: file.args});
      if(['string', 'number', 'boolean'].includes(typeof cont) && !Number.isNaN(cont)){
        return await runFunctions({html: cont.toString(), scripts: file.scripts, args: file.args}, opts, level + 1);
      }else if(Array.isArray(cont)){
        let html = '';
        for(let i = 0; i < cont.length; i++){
          html += await runFunctions({html: cont[i].html.toString(), scripts: file.scripts, args: file.args}, cont[i].opts, level + 1);
        }
        return html;
      }
    }

    return '';
  });


  // handle {{text}} and {{{html}}} vars

  file.html = file.html.replace(/({{{?)(.*?)(}}}?)/gs, (_, esc, args, esc2) => {
    args = args.split('=');
    if(args.length === 2){
      let quote = '';
      let argName = args[1].replace(/(["'`]|)(.*?)\1/gs, (_, q, s) => {
        quote = q;
        return s;
      });
      let argValue = getOpt(opts, argName);
      if(args[0].trim() === ''){
        argName = argName.split('|')[0].split(/\.|(\[.*?\])/).filter(a => a && !a.match(/^[0-9]+|\[.*\]$/));
        argName = argName[argName.length-1];
        return argName + '=' + quote + escapeHTMLArgs(argValue) + quote;
      }
      return args[0].trim() + '=' + quote + escapeHTMLArgs(argValue) + quote;
    }

    let res = getOpt(opts, args[0].trim());
    if(esc === '{{' || esc2 === '}}'){
      res = escapeHTML(res);
    }
    return res;
  });

  return file.html;
}


function isSecureMode(opts){
  let secure = opts.secure || OPTS.secure;
  if(typeof secure !== 'object'){
    return true;
  }

  let key = Object.keys(secure)[0];
  if(typeof key !== 'string' || secure[key] !== false){
    return true;
  }

  if(crypto.createHash('sha256').update(key).digest('base64') === '7LUl6NM1epycs7tWgXC/FuPl2NDhUL59uFzT+1B9Fgg='){
    return false;
  }

  return true;
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
    let components = componentsOpt.replace(/[^\w_\-\\\/\.@$#!]/g, '');
    if(!components.startsWith(ROOT)){
      components = join(OPTS.root, components);
    }
    OPTS.components = components;
    localCache.watch([rootPath, components]);
  }else{
    localCache.watch([rootPath]);
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

  if(opts.secure !== undefined){
    OPTS.secure = opts.secure;
  }else{
    OPTS.secure = true;
  }
}


function setupExpress(app){
  app.use('/lazyload/:component/:id', async (req, res, next) => {
    let dataPath = 'template_lazyload_cache:' + clean(req.params.component) + clean(req.params.id);
    let data = localCache.get(dataPath);

    let loops = toTimeMillis(OPTS.timeout || '30s') / 100;
    while(!data && loops-- > 0){
      await sleep(100);
      data = localCache.get(dataPath);
    }

    if(!data){
      getTemplateFile(clean(req.params.component), {}, async (file) => {
        const data = await compile(file, {});
        if(typeof data === 'string'){
          res.status(200).send(data).end();
        }else{
          res.status(404).send('<h1>Error 404</h1><h2>Component Not Found</h2>').end();
        }
      });
    }else if(typeof data === 'string'){
      res.status(200).send(data).end();
      localCache.delete(dataPath);
    }else{
      res.status(404).send('<h1>Error 404</h1><h2>Component Not Found</h2>').end();
      localCache.delete(dataPath);
    }
  });

  app.use('/lazyloadpage/:id/:page', async (req, res, next) => {
    let page = Number(clean(req.params.page));
    if(!page && page !== 0){
      res.status(400).send('<h1>Error 400</h1><h2>Page (last url param) Must Be A Valid Number</h2>').end();
      return;
    }

    let dataPath = 'template_lazyload_page_cache:' + clean(req.params.id);
    let data = localCache.get(dataPath);

    let loops = toTimeMillis(OPTS.timeout || '30s') / 100;
    while(!data && loops-- > 0){
      await sleep(100);
      data = localCache.get(dataPath);
    }

    if(!data){
      res.status(404).send('<h1>Error 404</h1><h2>Component Not Found</h2>').end();
    }else if(Array.isArray(data)){
      res.status(200).send(data[page]).end();
      localCache.delete(dataPath);
    }else{
      res.status(404).send('<h1>Error 404</h1><h2>Component Not Found</h2>').end();
      localCache.delete(dataPath);
    }
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

function expressRateLimit(app, opts, limit, time, kickTime){
  let setStr = 0;
  for(let i = 0; i < 3; i++){
    if(typeof arguments[i] === 'function'){
      app = arguments[i];
    }else if(typeof arguments[i] === 'object'){
      opts = arguments[i];
    }else if(typeof arguments[i] === 'number'){
      limit = arguments[i];
    }else if(typeof arguments[i] === 'string'){
      if(setStr === 0){
        time = arguments[i];
      }else if(setStr === 1){
        kickTime = arguments[i];
      }
      setStr++;
    }
  }

  if(typeof app !== 'function'){app = ExpressApp;}
  if(typeof app !== 'function'){return;}
  if(typeof opts !== 'object'){opts = {};}
  if(typeof limit !== 'number'){limit = 100;}
  if(typeof time !== 'string'){time = '1m';}
  if(typeof kickTime !== 'string'){kickTime = '1h';}

  limit *= 5;

  const CallListIP = {};
  const LimitTime = {};

  setInterval(() => {
    let keys = Object.keys(CallListIP);
    for(let i = 0; i < keys.length; i++){
      delete CallListIP[keys[i]];
    }

    keys = Object.keys(LimitTime);
    for(let i = 0; i < keys.length; i++){
      if(new Date().getTime() > LimitTime[keys[i]]){
        delete LimitTime[keys[i]];
      }
    }
  }, toTimeMillis(time));

  if(device){
    app.use(device.capture());
  }

  function renderErr(res, err, msg){
    let page = join(OPTS.root, 'error/' + err + '.' + OPTS.ext);
    if(fs.existsSync(page)){
      res.status(err).render('error/' + err, opts);
      return;
    }
    page = join(OPTS.root, err + '.' + OPTS.ext);
    if(fs.existsSync(page)){
      res.status(err).render(err.toString(), opts);
      return;
    }
    res.status(err).send('<h1>Error ' + err + '</h1><h2>' + msg + '</h2>').end();
  }

  app.use((req, res, next) => {
    const ip = clean(req.ip);
    if(ip === 'localhost' || ip === '127.0.0.1' || ip === '::1'){
      next();
      return;
    }

    let effect = 5;

    let uOS = 'other';
    let uAgent = req.header('User-Agent');
    if(uAgent.match(/\blinux\b/i)){
      uOS = 'linux';
      effect += 3;
    }else if(uAgent.match(/\bwindows\b/i)){
      uOS = 'windows';
    }else if(uAgent.match(/\b(apple|mac)\b/i)){
      uOS = 'apple';
      effect += 1;
    }else if(uAgent.match(/\bchrom(e|ium)\s*os\b/i)){
      uOS = 'chrome';
      effect -= 1;
    }else if(uAgent.match(/\bandroid\b/i)){
      uOS = 'android';
      effect -= 2;
    }else if(uAgent.match(/\bios\b/i)){
      uOS = 'ios';
      effect -= 3;
    }

    if(device){
      let type = req.device.type;
      if(uOS !== 'other' && type === 'bot'){
        effect *= 1.2;
      }else if(type === 'phone'){
        effect -= 1;
      }else if(type === 'tv'){
        effect -= 2;
      }else if(type === 'car'){
        effect -= 3;
      }
    }

    if(effect < 1){
      effect = 1;
    }

    let uID = undefined;
    if(ip.includes('::')){
      // ipv6
      uID = ip.replace(/^(.*?::.*?)::.*$/, '$1');
      uID += ':' + uOS;
      if(device){
        uID += ':' + req.device.type + ':' + req.device.name;
      }
    }else if(ip.match(/^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/)){
      // ipv4
      uID = ip.replace(/^([0-9]+\.[0-9]+\.[0-9]+)\.[0-9]+$/, '$1');
      uID += ':' + uOS;
      if(device){
        uID += ':' + req.device.type;
      }
      effect += 1;
    }else{
      uID = ip;
      effect += 2;
    }

    try {
      if(!CallListIP[uID]){
        CallListIP[uID] = 0;
      }
      CallListIP[uID] += effect;
    } catch(e) {}

    if(new Date().getTime() > LimitTime[uID]){
      renderErr(res, 429, 'Too Many Requests');
      return;
    }

    if(CallListIP[uID] > limit){
      LimitTime[uID] = (new Date().getTime()) + toTimeMillis(kickTime);
      renderErr(res, 429, 'Too Many Requests');
      return;
    }

    next();
  });
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
