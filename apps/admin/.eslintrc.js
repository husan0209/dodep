module.exports = {
  root: true,
  env: { browser: true, es2020: true },
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
  settings: {
    react: { version: 'detect' },
  },
  rules: {
    // The new JSX transform makes the React import unnecessary, but the
    // lint rule that flags it is not worth failing a build over.
    'react/react-in-jsx-scope': 'off',
    // Prop types are expressed as TypeScript types; the runtime rule only adds
    // noise on top of tsc, which already type-checks every component.
    'react/prop-types': 'off',
    // Matches the project's own tsconfig, which sets noUnusedLocals and
    // noUnusedParameters to false. Unused locals are not an error here.
    '@typescript-eslint/no-unused-vars': 'off',
    // `any` is used at the API boundary where responses are not modelled yet.
    // Tightening it is a separate change, not a reason to fail lint.
    '@typescript-eslint/no-explicit-any': 'off',
  },
  ignorePatterns: ['dist', 'node_modules', '*.config.ts', '*.config.js'],
}