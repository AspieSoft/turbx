function main({ runFunc, getOpt }) {
  return {
    name: 'if',
    func: function (args, content, opts, level, file, def = false) {
      let isTrue = def;
      let lastArg = undefined;

      if (!args.length) {
        return content;
      }

      for (let i = 0; i < args.length; i++) {
        if (args[i] === '&') {
          if (isTrue) {
            continue;
          }
          break;
        } else if (args[i] === '|') {
          if (isTrue) {
            break;
          }
          continue;
        }

        let arg1, sign, arg2;
        if (args[i + 1] && args[i + 1].match(/^[!<>]?=|[<>]$/)) {
          arg1 = args[i];
          sign = args[i + 1];
          arg2 = args[i + 2];
          lastArg = arg1;
        } else if (args[i].match(/^[!<>]?=|[<>]$/) && args[i + 1]) {
          arg1 = lastArg;
          sign = args[i];
          arg2 = args[i + 1];
          i++;
        } else if (args.length === 1) {
          let pos = true;
          if (args[i].startsWith('!')) {
            pos = false;
          }
          if (args[i] === '!') {
            arg1 = args[i + 1];
            i++;
          } else if (args[i] === '!!') {
            arg1 = args[i + 1];
            i++;
            pos = true;
          } else {
            arg1 = args[i].replace('!', '');
          }

          if (arg1.startsWith('!')) {
            pos = !pos;
            arg1 = arg1.replace('!', '');
          }
          lastArg = arg1;

          if (arg1.match(/^["'`].*["'`]$/)) {
            arg1 = arg1.replace(/^["'`](.*)["'`]$/, '$1');
            if (!Number.isNaN(Number(arg1))) {
              arg1 = Number(arg1);
            }
          } else if (arg1.match(/^-?[0-9]+(\.[0-9]+|)$/)) {
            arg1 = Number(arg1);
          } else {
            arg1 = getOpt(opts, arg1, false);
          }

          isTrue = (!arg1 && arg1 !== 0) || arg1 === '' || (Array.isArray(arg1) && !arg1.length) || (typeof arg1 === 'object' && !Object.keys(arg1).length);
          if (pos) {
            isTrue = !isTrue;
          }

          continue;
        } else {
          continue;
        }

        if (arg1.match(/^["'`].*["'`]$/)) {
          arg1 = arg1.replace(/^["'`](.*)["'`]$/, '$1');
          if (!Number.isNaN(Number(arg1))) {
            arg1 = Number(arg1);
          }
        } else if (arg1.match(/^-?[0-9]+(\.[0-9]+|)$/)) {
          arg1 = Number(arg1);
        } else {
          arg1 = getOpt(opts, arg1, false);
          if (typeof arg1 === 'string' && arg1.match(/^-?[0-9]+(\.[0-9]+|)$/)) {
            arg1 = Number(arg1);
          }
        }

        if (!arg2) {
          arg2 = undefined;
        } else if (arg2.match(/^["'`].*["'`]$/)) {
          arg2 = arg2.replace(/^["'`](.*)["'`]$/, '$1');
          if (!Number.isNaN(Number(arg2))) {
            arg2 = Number(arg2);
          }
        } else if (arg2.match(/^-?[0-9]+(\.[0-9]+|)$/)) {
          arg2 = Number(arg2);
        } else {
          arg2 = getOpt(opts, arg2, false);
          if (typeof arg2 === 'string' && arg2.match(/^-?[0-9]+(\.[0-9]+|)$/)) {
            arg2 = Number(arg2);
          }
        }

        lastArg = arg1;

        switch (sign) {
          case '=':
            isTrue = arg1 === arg2;
            break;
          case '!=':
          case '!':
            isTrue = arg1 !== arg2;
            break;
          case '>=':
            isTrue = arg1 >= arg2;
            break;
          case '<=':
            isTrue = arg1 <= arg2;
            break;
          case '>':
            isTrue = arg1 > arg2;
            break;
          case '<':
            isTrue = arg1 < arg2;
            break;
          default:
            break;
        }

        i += 2;
      }

      let elseOpt = content.match(new RegExp(`<_el(if|se):${level}(\\s+[0-9]+|)/>`));
      if (elseOpt && isTrue) {
        return content.substring(0, elseOpt.index);
      } else if (elseOpt) {
        return runFunc('if', file.args[elseOpt[2].trim()], content.substring(elseOpt.index + elseOpt[0].length), opts, level, file, elseOpt[1] === 'se');
      } else if (isTrue) {
        return content;
      }

      return '';
    },
  };
}

module.exports = main;
