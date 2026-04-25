module.exports = {
  content: [
    {
      files: ['./templates/**/*.{html,templ}', '/Users/figge/Developer/webhooktester/templates/**/*.{html,templ}', 'templates/**/*.{html,templ}'],
      extract: (content) => content.match(/[^<>()"'`\s]*[^<>()"'`\s:]/g) || [],
    },
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};
