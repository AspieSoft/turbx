function main({runFunc, getOpt}){
  return {
    name: 'lazy',
    func: function(args, content, opts, level, file){

      //todo: setup lazy loading (use <lazy/> to seperate parts of the file for lazy loading within the content)
      // cache this tag seperately for faster requests of parts of the lazy loaded content
      // check when nothing is underneath to load

      return '';
    },
  }
}

module.exports = main;
