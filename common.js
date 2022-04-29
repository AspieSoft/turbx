const crypto = require('crypto');
const validator = require('validator');

function escapeHTML(str){
  //todo: escape HTML
  return str;
}

function escapeHTMLArgs(str){
  //todo: escape HTML Args in quotes
  return str;
}

function compileMD(str){
  //todo: compile markdown
  return str;
}

function compileJS(str){
  //todo: minify js (and maybe compile typescript or something else)
  return str;
}

function compileCSS(str){
  //todo: compile less (and maybe scss)
  return str;
}

const sleep = (waitTimeInMs) => new Promise(resolve => setTimeout(resolve, waitTimeInMs));

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
    } else if(input === NaN) {
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


function getOpt(opts, arg, stringOutput = true){
  if(['number', 'boolean'].includes(typeof arg)){
    arg = arg.toString();
  }

  if(typeof arg !== 'string' || typeof opts !== 'object'){
    return '';
  }

  arg = arg.split('|');
  for(let i = 0; i < arg.length; i++){
    let value = arg[i].split('.').reduce((obj, key) => {
      if(typeof obj !== 'object' || obj instanceof RegExp){
        return obj;
      }

      if(key.match(/^\[.*?\]$/)){

        // get [var] object input
        key = key.replace(/^\[(.*)\]$/, '$1');
        if(key.match(/^(['"`])(.*)\1$/)){
          key = key.replace(/^(['"`])(.*)\1$/, '$1');
        }else{
          key = obj[key];
        }
      }

      return obj[key];
    }, opts);

    if(!stringOutput){
      if(value === undefined || value === null){
        if(i === arg.length-1){
          return value;
        }
        continue;
      }
      return value;
    }

    if(value === undefined || value === null || typeof value === 'object'){
      continue;
    }
    return value.toString();
  }

  return '';
}


module.exports = {
  escapeHTML,
  escapeHTMLArgs,
  compileMD,
  compileJS,
  compileCSS,
  sleep,
  randomToken,
  clean,
  varType,
  toTimeMillis,
  toNumber,
  asyncReplace,
  getOpt,
};
