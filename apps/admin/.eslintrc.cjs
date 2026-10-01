// ESLint configuration for the Admin Panel (React + TypeScript + Vite).
//
// This file used to be missing entirely: `npm run lint` invoked
// `eslint . --ext ts,tsx`, ESLint 8 walked up from apps/admin/src, found no
// configuration file and exited with code 2 ("ESLint couldn't find a
// configuration file"), so the `frontend-build (admin)` job could never pass.
//
// The parser has to be @typescript-eslint/parser: the default espree parser
// cannot parse TypeScript type annotations, and this app is 100% .ts/.tsx.
module.exports = {
  root: true,
  env: { browser: true, es2020: true, node: true },
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
    'plugin:react-hooks/recommended',
  ],
  settings: { react: { version: 'detect' } },
  rules: {
    // TypeScript already reports unresolved variables/regexps with types, and
    // the base rules produce false positives on type-only constructs.
    'no-undef': 'off',
    'no-unused-vars': 'off',
    // `React` is not in scope with the modern JSX transform
    // ("jsx": "react-jsx" in tsconfig.json).
    'react/react-in-jsx-scope': 'off',
    'react/prop-types': 'off',

    // Introducing ESLint to an existing codebase: every correctness rule
    // (including react-hooks/rules-of-hooks and exhaustive-deps) stays an
    // error, while the two style/opinion rules that produce 110 findings
    // across untouched application code start as warnings. They stay visible
    // in the log and can be ratcheted to "error" as the code is cleaned up.
    '@typescript-eslint/no-unused-vars': [
      'warn',
      { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
    ],
    '@typescript-eslint/no-explicit-any': 'warn',
  },
  ignorePatterns: ['dist', 'node_modules', '*.config.ts', '*.config.js'],
}
