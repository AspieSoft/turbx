const fs = require('fs');
const {join} = require('path');
const {minify} = require('terser');
const {exec} = require('child_process');

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

const sleep = (waitTimeInMs) => new Promise(resolve => setTimeout(resolve, waitTimeInMs));


let finished = 0;

;(async function(){
  const funcDir = join(__dirname, 'functions');

  const funcList = [];

  const funcs = fs.readdirSync(funcDir).map(file => {
    html = fs.readFileSync(join(funcDir, file)).toString();
    let fName = file.replace(/\.js$/, '').replace(/[^\w_]/g, '');
    html = html.replace(/require\s*\(\s*(['"`])\.\.\/(common|functions)(?:\.js|)\1\s*\)/gs, (_, _1, type) => {
      if(type === 'common'){
        return 'eCommonFunc';
      }
      return 'eRunFunc';
    });
    html = `;eFunc['${fName}']=(function(){${html.replace(/module\.exports\s*=\s*/gs, 'return ')}})();`;
    funcList.push(fName);
    return html;
  });

  let runFuncs = fs.readFileSync(join(__dirname, 'functions.js')).toString();
  runFuncs = runFuncs.replace(/fs\.readdir\s*\(\s*join\s*\(\s*__dirname\s*,\s*(['"`])functions\1\s*\)\s*,\s*\(\s*([\w_]+),\s*([\w_]+)\s*\)\s*=>\s*\{\s*if\s*\(\s*\2\s*\)\s*\{\s*return\s*;?\s*\}/gs, ';(($3) => {').replace(/\}\);?\s*module\.exports/s, '})(eListFunc);module.exports').replace(/require\s*\(\s*join\s*\(\s*__dirname\s*,\s*(['"`])functions\1\s*,\s*([\w_\.\[\]]+)\s*\)\s*\)/gs, 'eFunc[$2]');
  runFuncs = runFuncs.replace(/require\s*\(\s*(['"`])\.\/(common|functions)(?:\.js|)\1\s*\)/gs, (_, _1, type) => {
    if(type === 'common'){
      return 'eCommonFunc';
    }
    return 'eRunFunc';
  });
  runFuncs = `;const eRunFunc=(function(){${runFuncs.replace(/module\.exports\s*=\s*/gs, 'return ')}})();`;

  let commonFuncs = fs.readFileSync(join(__dirname, 'common.js')).toString();
  commonFuncs = commonFuncs.replace(/require\s*\(\s*(['"`])\.\/(common|functions)(?:\.js|)\1\s*\)/gs, (_, _1, type) => {
    if(type === 'common'){
      return 'eCommonFunc';
    }
    return 'eRunFunc';
  });
  commonFuncs = `;const eCommonFunc=(function(){${commonFuncs.replace(/module\.exports\s*=\s*/gs, 'return ')}})();`;

  let index = fs.readFileSync(join(__dirname, 'index.js')).toString();
  index = index.replace(/require\s*\(\s*(['"`])\.\/(common|functions)(?:\.js|)\1\s*\)/gs, (_, _1, type) => {
    if(type === 'common'){
      return 'eCommonFunc';
    }
    return 'eRunFunc';
  });

  let res = `
  ${commonFuncs}
  const eListFunc=${JSON.stringify(funcList)};
  const eFunc = {};
  ${funcs.join('\n')}
  ${runFuncs}
  ${index}
  `;

  res = await minifyFile(res);

  fs.writeFileSync(join(__dirname, 'index.min.js'), res);

  finished++;
})();


;(async function(){
  let gitIgnore = fs.readFileSync(join(__dirname, '.gitignore')).toString().split('\n').filter(line => line.trim() !== '').map(line => line.replace(/[^\w_\-$\.]/g, '').trim());

  let npmIgnore = fs.readFileSync(join(__dirname, '.npmignore')).toString().split('\n').filter(line => line.trim() !== '').map(line => line.replace(/[^\w_\-$\.]/g, '').trim());

  const ignoreFiles = [
    ...gitIgnore,
    ...npmIgnore,
    'node_modules',
    'build.js',
    'functions',
    'functions.js',
    'common.js',
    'index.js',
  ];

  async function minifyDir(dir){
    const files = fs.readdirSync(dir);
    for(let i = 0; i < files.length; i++){
      if(fs.lstatSync(join(dir, files[i])).isDirectory() && !ignoreFiles.includes(files[i])){
        await minifyDir(join(dir, files[i]));
      }else if(files[i].endsWith('.js') && !files[i].endsWith('.min.js') && files[i] !== 'build.js' && !ignoreFiles.includes(files[i])){
        let file = fs.readFileSync(join(dir, files[i]));
        let js = await minifyFile(file.toString());

        js = js.replace(/require\((['"`])((?:\.\.?\/)+)([\w_\-\/\\$\s]+)(?:\.js|)\1\)/gs, (str, q, level, file) => {
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

  finished++;
})();


;(async function(){
  while(finished < 2){
    await sleep(100);
  }

  console.log('Build Finished!');

  exec('node test/run.js --github');
})();
