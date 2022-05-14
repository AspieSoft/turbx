const fs = require('fs');
const {join} = require('path');
const {minify} = require('terser');

const minifyOptions = {
	ecma: 2020,
	keep_classnames: true,
	parse: {shebang: true},
	compress: {
		ecma: 2020,
		keep_infinity: true,
		passes: 5,
		top_retain: ['module', 'global', 'return', 'process'],
		typeofs: false,
    drop_console: true,
	},
	mangle: {
		keep_classnames: true,
		reserved: ['module', 'global', 'return', 'process'],
	},
};

async function minifyFile(file){
	let result = await minify(file, minifyOptions);
	if(result && !result.error && result.code){
		return result.code;
	}
	return file;
}

(async function(){

  let gitIgnore = fs.readFileSync(join(__dirname, '.gitignore')).toString().split('\n').filter(line => line.trim() !== '').map(line => line.replace(/[^\w_\-$\.]/g, '').trim());

  let npmIgnore = fs.readFileSync(join(__dirname, '.npmignore')).toString().split('\n').filter(line => line.trim() !== '').map(line => line.replace(/[^\w_\-$\.]/g, '').trim());
  
  const ignoreFiles = [
    ...gitIgnore,
    ...npmIgnore,
  ];

  async function minifyDir(dir){
    const files = fs.readdirSync(dir);
    for(let i = 0; i < files.length; i++){
      if(fs.lstatSync(join(dir, files[i])).isDirectory() && !ignoreFiles.includes(files[i])){
        await minifyDir(join(dir, files[i]));
      }else if(files[i].endsWith('.js') && !files[i].endsWith('.min.js') && files[i] !== 'build.js'){
        let file = fs.readFileSync(join(dir, files[i]));
        let js = await minifyFile(file.toString());

        js = js.replace(/require\((['"`])((?:\.\.?\/)+)([\w_\-\/\\\.$\s]+)\1\)/gs, (str, q, level, file) => {
          if(files.includes(file + '.js')){
            return `require(${q}${level}${file + '.min'}${q})`;
          }
          return str;
        });

        fs.writeFileSync(join(dir, files[i].replace(/\.js$/, '.min.js')), js);
      }
    }
  }
  await minifyDir(__dirname);

  let testFile = fs.readFileSync(join(__dirname, 'test/index.js')).toString();
  testFile = testFile.replace(/require\((['"`])(\.\.\/index)\1\)/gs, (_, q, file) => {
    return `require(${q}${file + '.min'}${q})`;
  });
  fs.writeFileSync(join(__dirname, 'test/index.min.js'), testFile);

  // require('./test/index.min');

  /* setTimeout(function(){
    process.exit(0);
  }, 3000); */

})();
