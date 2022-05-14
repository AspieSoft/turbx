function main({runFunc, getOpt}){
  return {
    name: 'json',
    func: function(args, content, opts, level, file){

      let json = getOpt(opts, args[0], false);

      let spaces = Number(args[1]) || 0;

      if(spaces){
        json = JSON.stringify(json, null, spaces);
      }else{
        json = JSON.stringify(json);
      }

      if(typeof json !== 'string'){
        return '';
      }

      if(args[1] === 'true' || args[2] === 'true'){
        json = json.replace(/(,\s*|)\s?"([\w_-])":/g, '$1 $2:').replace(/^{\s/, '{');
      }

      return json;
    },
  }
}

module.exports = main;
