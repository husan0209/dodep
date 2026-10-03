// ESLint 8 (eslintrc) configuration for the admin panel.
//
// `npm run lint` runs `eslint . --ext ts,tsx`. ESLint needs an eslintrc-style
// config to do anything at all: without one it aborts with
// "ESLint couldn't find a configuration file" (exit 2), so the frontend-build
// job failed at the Lint step before it ever reached build or test.
//
// admin is a Vite + React 18 + TypeScript app, so the default espree parser is
// not enough - TS type syntax has to go through @typescript-eslint. That parser
// was not in devDependencies, which is why it is added here rather than relying
// on a transitive copy.
module.exports = {
  root: true,
  env: { browser: true, es2022: true, node: true },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  plugins: ['@typescript-eslint', 'react', 'react-hooks'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:react/jsx-runtime',
  ],
  settings: {
    react: { version: 'detect' },
  },
  rules: {
    // The app uses the automatic JSX runtime, so React need not be imported.
    'react/react-in-jsx-scope': 'off',
    // Unused bindings are prefixed with `_` by convention in this codebase.
    '@typescript-eslint/no-unused-vars': [
      'error',
      { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
    ],
    // The admin panel predates this config and uses `any` in ~50 places:
    // `values as any` when handing Ant Design form values to a service, and
    // `(value, row: any)` in table column renderers. Typing all of it properly
    // means inventing row and form types across ~25 files of code that moves
    // money, which is not something to do as a side effect of turning on a
    // linter. The rule stays on as a warning so every remaining `any` is still
    // listed on each run and can be retired file by file; promote it back to
    // 'error' once the count reaches zero.
    '@typescript-eslint/no-explicit-any': 'warn',
  },
  ignorePatterns: ['dist', 'node_modules', 'coverage', '*.config.ts', '*.config.js'],
};