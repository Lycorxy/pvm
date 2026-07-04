module.exports = {
  root: true,
  extends: ['unite/react'],
  settings: {
    react: {
      version: '19.0',
    },
  },
  parserOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
    ecmaFeatures: {
      jsx: true,
    },
  },
  ignorePatterns: ['*.mdx', '*.md'],
};
