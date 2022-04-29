if(process.argv.includes('--github')){
  // require('./index.min');
  require('./index');

  setTimeout(function(){
    process.exit(0);
  }, 5000);
}else{
  require('./index');
}
