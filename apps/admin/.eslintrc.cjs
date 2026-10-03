module.exports = {
  root: true,
  env: { browser: true, es2021: true },
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
    // tsconfig.json sets "jsx": "react-jsx" and the app is built by
    // @vitejs/plugin-react, i.e. the automatic JSX runtime. jsx-runtime turns
    // off react/react-in-jsx-scope and react/react-in-jsx-no-undef, which only
    // apply to the legacy transform and otherwise fire on every component.
    'plugin:react/jsx-runtime',
    'plugin:react-hooks/recommended',
  ],
  settings: {
    react: { version: 'detect' },
  },
  rules: {
    // The admin SPA is a browser-only app; there is no Node global to rely on.
    'no-undef': 'off',
    // Advisory only: `any` still appears at API boundaries and in the report
    // builders. Treated as a warning so the gate blocks real defects instead
    // of forcing an app-wide type refactor.
    '@typescript-eslint/no-explicit-any': 'warn',
    // Matches tsconfig.json, which sets "noUnusedLocals": false and
    // "noUnusedParameters": false. Unused symbols are reported for visibility
    // but do not fail the build, exactly as tsc is configured not to.
    '@typescript-eslint/no-unused-vars': 'warn',
  },
  ignorePatterns: ['dist', 'build', 'node_modules', '*.cjs'],
}
