function main({runFunc, getOpt}){
  return {
    name: 'each',
    func: function(args, content, opts, level, file){

      const argKeys = Object.keys(args);

      //todo: setup each statements
      if(!argKeys.length){
        return '';
      }

      let argAs = undefined;
      let argOf = undefined;
      let argIn = undefined;
      let argType = 0;
      for(let i = 1; i < argKeys.length; i++){
        if(args[argKeys[i]] === 'as'){
          argType = 1;
        }else if(args[argKeys[i]] === 'of'){
          argType = 2;
        }else if(args[argKeys[i]] === 'in'){
          argType = 3;
        }else if(argType === 1){
          argAs = args[argKeys[i]];
          argType = 0;
        }else if(argType === 2){
          argOf = args[argKeys[i]];
          argType = 0;
        }else if(argType === 3){
          argIn = args[argKeys[i]];
          argType = 0;
        }else if(!argAs){
          argAs = args[argKeys[i]];
        }else if(!argOf){
          argOf = args[argKeys[i]];
        }else if(!argIn){
          argIn = args[argKeys[i]];
        }
      }

      if(argType === 1){
        argAs = args[argKeys[argKeys.length-1]];
      }else if(argType === 2){
        argOf = args[argKeys[argKeys.length-1]];
      }else if(argType === 3){
        argIn = args[argKeys[argKeys.length-1]];
      }

      let obj = getOpt(opts, args[0], false);
      const res = [];

      if(Array.isArray(obj)){
        for(let i = 0; i < obj.length; i++){
          let opt = {...opts};
          if(argAs){
            opt[argAs] = obj[i];
          }else{
            opt[args[0]] = obj[i];
          }
          if(argOf){
            opt[argOf] = i;
          }
          if(argIn){
            opt[argIn] = i;
          }
          res.push({html: content, opts: opt});
        }
      }else if(typeof obj === 'object'){
        const keys = Object.keys(obj);
        for(let i = 0; i < keys.length; i++){
          let opt = {...opts};
          if(argAs){
            opt[argAs] = obj[keys[i]];
          }else{
            opt[args[0]] = obj[keys[i]];
          }
          if(argOf){
            opt[argOf] = keys[i];
          }
          if(argIn){
            opt[argIn] = i;
          }
          res.push({html: content, opts: opt});
        }
      }

      return res;
    },
  }
}

module.exports = main;
