const fs = require('fs');
const {join} = require('path');

const common = require('./common');

const tagFunctions = {};

function addTagFunction(name, func){
  if(!Array.isArray(name)){
    name = [name];
  }

  name = name.filter(n => (typeof n === 'string' && !tagFunctions[n])).map(n => n.toLowerCase().replace(/[-_]/g, ''));
  if(!name.length){return;}

  if(Array.isArray(func)){
    func = func.filter(fn => (['string', 'function'].includes(typeof fn))).map(fn => {
      if(typeof fn === 'string'){
        return fn.toLowerCase().replace(/[-_]/g, '');
      }
      return fn;
    });
  }else if(typeof func === 'string'){
    func = func.toLowerCase().replace(/[-_]/g, '')
  }

  tagFunctions[name[0]] = func;

  for(let i = 1; i < name.length; i++){
    if(typeof name[i] === 'string' && !tagFunctions[name[i]]){
      tagFunctions[name[i]] = name[0];
    }
  }
}

function runTagFunction(name){
  name = name.toLowerCase().replace(/[-_]/g, '')

  let func = tagFunctions[name];
  if(typeof func === 'string'){
    func = tagFunctions[func];
  }
  
  if(typeof func === 'function'){
    let args = [...arguments];
    args.shift();
    return func(...args);
  }

  return '';
}


const commonFunc = {
  ...common,
  runFunc: runTagFunction,
};


fs.readdir(join(__dirname, 'functions'), (err, files) => {
  if(err){return;}
  for(let i = 0; i < files.length; i++){
    let func = require(join(__dirname, 'functions', files[i]));
    if(typeof func === 'function'){
      func = func(commonFunc);
    }
    if(typeof func !== 'object'){
      return;
    }

    let names = Object.keys(func);
    for(let i = 0; i < names.length; i++){
      if(names[i] !== 'name' && names[i] !== 'func' && names[i] !== 'function'){
        addTagFunction(names[i], func[names[i]]);
      }
    }

    if(!func.name){
      func.name = files[i].replace(/\.[^\.]+$/, '');
    }
    if(!func.func && func.function){
      func.func = func.function;
    }else if(!func.func){
      return;
    }

    addTagFunction(func.name, func.func);
  }
});


module.exports = {
  tagFunctions,
  addTagFunction,
  runTagFunction,
};
