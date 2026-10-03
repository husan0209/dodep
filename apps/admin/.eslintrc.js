// ESLint configuration for the Admin panel.
//
// This file was missing: `npm run lint` (`eslint . --ext ts,tsx`) aborted with
//   ESLint couldn't find a configuration file.
// even though eslint, eslint-plugin-react and eslint-plugin-react-hooks are all
// declared in devDependencies. ESLint 8 uses .eslintrc.* (flat config needs ESLint 9).
//
// The TypeScript parser/plugin were added at the same time: without them ESLint 8
// falls back to espree and reports "Parsing error: Unexpected token" on every
// `interface` / type annotation in src/.
//
// Only dependencies already required by the app (plus @typescript-eslint/*, which is
// mandatory to parse TypeScript at all) are used.
module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
  },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: {
      jsx: true,
    },
  },
  settings: {
    react: {
      version: 'detect',
    },
  },
  plugins: ['@typescript-eslint', 'react', 'react-hooks'],
  extends: [
    'eslint:recommended',
    // Includes the eslint-recommended overrides that disable core rules which
    // misfire on TypeScript (no-undef on types, no-unused-vars on type-only code, ...).
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:react/jsx-runtime',
    'plugin:react-hooks/recommended',
  ],
  rules: {
    'react/prop-types': 'off',
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/no-unused-vars': [
      'warn',
      { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
    ],
  },
  ignorePatterns: ['dist', 'build', 'coverage', 'node_modules', '*.cjs'],
}