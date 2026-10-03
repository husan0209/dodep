// ESLint config for the admin panel (ESLint 8 flat-config-less "eslintrc" format,
// matching the pinned eslint ^8.56 in package.json).
//
// This file was missing: `npm run lint` runs `eslint . --ext ts,tsx`, and on a
// clean checkout ESLint aborted with "couldn't find a configuration file"
// before linting anything.
module.exports = {
  root: true,
  env: { browser: true, es2021: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  plugins: ['react', 'react-hooks', '@typescript-eslint'],
  settings: { react: { version: 'detect' } },
  rules: {
    // Money must never be computed in binary floats: the platform rule is
    // decimal strings end-to-end, so comparing a float literal here is a bug.
    eqeqeq: ['error', 'smart'],
    'no-console': ['warn', { allow: ['warn', 'error'] }],
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/no-unused-vars': [
      'warn',
      { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
    ],
    'react/react-in-jsx-scope': 'off',
  },
  ignorePatterns: ['dist', 'build', 'node_modules', 'coverage'],
};