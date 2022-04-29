require('./index');

if(process.argv.includes('--github')){
  setTimeout(function(){
    process.exit(0);
  }, 5000);
}
