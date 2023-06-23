const crypto = require('crypto');
const validator = require('validator');

const sleep = (waitTimeInMs) => new Promise(resolve => setTimeout(resolve, waitTimeInMs));

// convert to megabytes
const formatMemoryUsage = (data) => Math.round(data / 1024 / 1024 * 100) / 100;

// @size: megabytes
async function waitForMemory(size = 1, timeout = 0){
  let mem = process.memoryUsage();
  let loops = 0;
  const startTime = Date.now();
  while(formatMemoryUsage(mem.heapTotal - mem.heapUsed) < size){
    if(timeout && loops++ > 1000){
      loops = 0;
      if(Date.now() - startTime > timeout){
        return false;
      }
    }
    await sleep(1);
    mem = process.memoryUsage();
  }
  return true;
}

function randomToken(size){
  return crypto.randomBytes(size).toString('hex');
}

function clean(input, allowControlChars = false) {
  // valid ascii characters: https://ascii.cl/htmlcodes.htm
  // more info: https://en.wikipedia.org/wiki/ASCII
  let allowList = [
    338,
    339,
    352,
    353,
    376,
    402,

    8211,
    8212,
    8216,
    8217,
    8218,
    8220,
    8221,
    8222,
    8224,
    8225,
    8226,
    8230,
    8240,
    8364,
    8482,
  ];

  function cleanStr(input) {
    input = validator.stripLow(input, {keep_new_lines: true});
    if(validator.isAscii(input)) {
      return input;
    }
    let output = '';
    for(let i = 0; i < input.length; i++) {
      let charCode = input.charCodeAt(i);
      if((allowControlChars && charCode >= 0 && charCode <= 31) || (charCode >= 32 && charCode <= 127) || (charCode >= 160 && charCode <= 255) || allowList.includes(charCode)) {
        output += input.charAt(i);
      }
    }
    if(validator.isAscii(output)) {
      return output;
    }
    return undefined;
  }

  function cleanArr(input) {
    let output = [];
    input.forEach(value => {
      output.push(cleanType(value));
    });
    return output;
  }

  function cleanObj(input) {
    let output = {};
    Object.keys(input).forEach(key => {
      key = cleanType(key);
      output[key] = cleanType(input[key]);
    });
    return output;
  }

  function cleanType(input) {
    if(input === null) {
      return null;
    } else if(input === undefined) {
      return undefined;
    } else if(Number.isNaN(input)) {
      return NaN;
    }

    let type = varType(input);

    switch(type) {
      case 'string':
        return cleanStr(input);
      case 'array':
        return cleanArr(input);
      case 'object':
        return cleanObj(input);
      case 'number':
        return Number(input);
      case 'boolean':
        return !!input;
      case 'regex':
        let flags = '';
        let re = input.toString().replace(/^\/(.*)\/(\w*)$/, function(str, r, f) {
          flags = cleanStr(f) || '';
          return cleanStr(r) || '';
        });
        if(!re || re === '') {return undefined;}
        return RegExp(re, flags);
      case 'symbol':
        input = cleanStr(input.toString());
        if(input !== undefined) {
          return Symbol(input);
        }
        return undefined;
      case 'bigint':
        return BigInt(input.toString().replace(/[^0-9\.\-\+enf_]/g, ''));
      default:
        return undefined;
    }
  }

  return cleanType(input);
}

function varType(value) {
  if(Array.isArray(value)) {
    return 'array';
  } else if(value === null) {
    return 'null';
  } else if(value instanceof RegExp) {
    return 'regex';
  }
  return typeof value;
}

function toTimeMillis(str){
  if(typeof str === 'number'){return Number(str);}
  if(!str || typeof str !== 'string' || str.trim() === ''){return NaN;}
  if(str.endsWith('h')){
    return toNumber(str)*3600000;
  }else if(str.endsWith('m')){
    return toNumber(str)*60000;
  }else if(str.endsWith('s')){
    return toNumber(str)*1000;
  }else if(str.endsWith('D')){
    return toNumber(str)*86400000;
  }else if(str.endsWith('M')){
    return toNumber(str)*2628000000;
  }else if(str.endsWith('Y')){
    return toNumber(str)*31536000000;
  }else if(str.endsWith('DE')){
    return toNumber(str)*315360000000;
  }else if(str.endsWith('C') || this.endsWith('CE')){
    return toNumber(str)*3153600000000;
  }else if(str.endsWith('ms')){
    return toNumber(str);
  }else if(str.endsWith('us') || this.endsWith('mic')){
    return toNumber(str)*0.001;
  }else if(str.endsWith('ns')){
    return toNumber(str)*0.000001;
  }
  return toNumber(str);
}

function toNumber(str){
  if(typeof str === 'number'){return str;}
  return Number(str.replace(/[^0-9.]/g, '').split('.', 2).join('.'));
}

async function asyncReplace(str, re, cb, global = undefined){
  if(global === undefined){
    global = !!re.toString().match(/\/[\w]*g[\w]*$/);
  }

  if(!global){
    const match = str.match(re);
    if(!match){
      return str;
    }

    return str.substring(0, match.index) + await cb(...match, match.index) + str.substring(match.index + match[0].length);
  }

  const matches = [...str.matchAll(re)];
  if(matches && matches.length){
    const replace = [];
    for(let i = 0; i < matches.length; i++){
      replace.push({start: matches[i].index, end: matches[i].index + matches[i][0].length, res: await cb(...matches[i], matches[i].index)});
    }

    for(let i = replace.length-1; i >= 0; i--){
      str = str.substring(0, replace[i].start) + replace[i].res + str.substring(replace[i].end);
    }
  }
  return str;
}

function loadedMiddleware(app, search){
  let stack = [];
  if(app.stack){
    stack = stack.concat(app.stack)
  }
  if(app._router && app._router.stack){
    stack = stack.concat(app._router.stack)
  }

  const using = [];
  for(let i = 0; i < stack.length; i++){
    let ind = search.indexOf(stack[i].name);
    if(ind !== -1){
      using.push(search.splice(ind, 1)[0]);
    }
  }

  return using;
}


async function encrypt(text, key){
  // await waitForMemory(1000000);
  await waitForMemory(1);

  const hash = crypto.createHash('sha256');
  hash.update(key);
  const keyBytes = hash.digest();

  const iv = crypto.randomBytes(16);
  const cipher = crypto.createCipheriv('aes-256-cfb', keyBytes, iv);
  let enc = [iv, cipher.update(text, 'utf8')];
  enc.push(cipher.final());
  return Buffer.concat(enc).toString('base64');
}

async function decrypt(text, key){
  // await waitForMemory(1000000);
  await waitForMemory(1);

  const hash = crypto.createHash('sha256');
  hash.update(key);
  const keyBytes = hash.digest();

  const contents = Buffer.from(text, 'base64');
  // const iv = contents.slice(0, 16);
  const iv = contents.subarray(0, 16);
  // const textBytes = contents.slice(16);
  const textBytes = contents.subarray(16);
  if(!textBytes.length){
    return undefined;
  }
  const decipher = crypto.createDecipheriv('aes-256-cfb', keyBytes, iv);
  let res = decipher.update(textBytes, '', 'utf8');
  res += decipher.final('utf8');
  return res;
}


function requireOptional(path) {
  try {
    return require(path);
  } catch (e) {
    return function(){
      console.error(`\x1b[31mThe module \x1b[34m${path}\x1b[31m is not installed!\nYou can install it by running \x1b[34mnpm i ${path}\x1b[0m`)
    };
  }
}


module.exports = {
  sleep,
  waitForMemory,
  randomToken,
  clean,
  varType,
  toTimeMillis,
  toNumber,
  asyncReplace,
  loadedMiddleware,
  encrypt,
  decrypt,
  requireOptional
};
