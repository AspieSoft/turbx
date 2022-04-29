const {LoremIpsum} = require('lorem-ipsum');

const lorem = new LoremIpsum({
  sentencesPerParagraph: {
    max: 8,
    min: 4
  },
  wordsPerSentence: {
    max: 16,
    min: 4
  }
});

function main({runFunc, getOpt}) {
  return {
    name: ['lorem', 'ipsum', 'lorem-ipsum', 'text'],
    func: function (args, content, opts, level, file) {
      let arg1 = args[0]?.replace(/^(["'`])(.*)\1$/, '');
      let arg2 = args[1]?.replace(/^(["'`])(.*)\1$/, '');

      let mode = 0;
      if (typeof arg1 === 'string' && arg1.match(/^w(ords?|)$/i)) {
        mode = 1;
        arg1 = arg2;
      } else if (typeof arg2 === 'string' && arg2.match(/^w(ords?|)$/i)) {
        mode = 1;
      } else if (typeof arg1 === 'string' && arg1.match(/^s(ent(ences?|)|)$/i)) {
        mode = 2;
        arg1 = arg2;
      } else if (typeof arg2 === 'string' && arg2.match(/^s(ent(ences?|)|)$/i)) {
        mode = 2;
      }

      if (typeof arg1 === 'string' && arg1.match(/([0-9]+)/)) {
        arg1 = Number(arg1);
      } else if(arg1) {
        arg1 = Number(getOpt(opts, arg1));
      }

      if (arg1 === 0) {
        return '';
      } else if (!arg1) {
        arg1 = 1;
      }

      if (mode === 1) {
        return lorem.generateWords(arg1);
      } else if (mode === 2) {
        return lorem.generateSentences(arg1);
      }
      return lorem.generateParagraphs(arg1);
    },

    'lorem-p': function (args, content, opts, level, file) {
      let arg1 = args[0]?.replace(/^(["'`])(.*)\1$/, '');

      if (typeof arg1 === 'string' && arg1.match(/([0-9]+)/)) {
        arg1 = Number(arg1);
      } else if(arg1) {
        arg1 = Number(getOpt(opts, arg1));
      }

      if (arg1 === 0) {
        return '';
      } else if (!arg1) {
        arg1 = 1;
      }

      return lorem.generateParagraphs(arg1);
    },
    p: 'lorem-p',
    paragraph: 'lorem-p',

    'lorem-s': function (args, content, opts, level, file) {
      let arg1 = args[0]?.replace(/^(["'`])(.*)\1$/, '');

      if (typeof arg1 === 'string' && arg1.match(/([0-9]+)/)) {
        arg1 = Number(arg1);
      } else if(arg1) {
        arg1 = Number(getOpt(opts, arg1));
      }

      if (arg1 === 0) {
        return '';
      } else if (!arg1) {
        arg1 = 1;
      }

      return lorem.generateSentences(arg1);
    },
    s: 'lorem-s',
    sentence: 'lorem-s',

    'lorem-w': function (args, content, opts, level, file) {
      let arg1 = args[0]?.replace(/^(["'`])(.*)\1$/, '');

      if (typeof arg1 === 'string' && arg1.match(/([0-9]+)/)) {
        arg1 = Number(arg1);
      } else if(arg1) {
        arg1 = Number(getOpt(opts, arg1));
      }

      if (arg1 === 0) {
        return '';
      } else if (!arg1) {
        arg1 = 1;
      }

      return lorem.generateWords(arg1);
    },
    w: 'lorem-w',
    word: 'lorem-w',
  };
}

module.exports = main;
