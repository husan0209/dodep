/* eslint-env node */
module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
  },
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react/recommended",
    "plugin:react-hooks/recommended",
  ],
  parser: "@typescript-eslint/parser",
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    ecmaFeatures: {
      jsx: true,
    },
  },
  plugins: ["react", "react-hooks"],
  settings: {
    react: {
      version: "detect",
    },
  },
  rules: {
    // The app uses the automatic JSX runtime (jsx: "react-jsx"), so the
    // legacy `React` import is neither needed nor present.
    "react/react-in-jsx-scope": "off",
    // Underscore-prefixed identifiers are the established convention in this
    // codebase for deliberately unused parameters/values.
    "@typescript-eslint/no-unused-vars": [
      "error",
      { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrors: "none" },
    ],
    // ~55 `any` annotations remain in the antd Table render callbacks and
    // service helpers. They are pre-existing and `tsc --strict` passes, so
    // they are reported as warnings until they are typed properly instead of
    // being silenced file by file.
    "@typescript-eslint/no-explicit-any": "warn",
  },
  ignorePatterns: ["dist", "coverage", "node_modules", "*.cjs"],
};
