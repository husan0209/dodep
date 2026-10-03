const nextJest = require("next/jest");

const createJestConfig = nextJest({
  dir: "./",
});

const customJestConfig = {
  testEnvironment: "jest-environment-jsdom",
  setupFilesAfterEnv: ["<rootDir>/jest.setup.ts"],
  // `next build` (which CI runs before the tests) emits a standalone bundle
  // containing its own package.json. Without this, jest-haste-map reports a
  // "Haste module naming collision" for the app name.
  modulePathIgnorePatterns: ["<rootDir>/.next/"],
  moduleNameMapper: {
    "^@/(.*)$": "<rootDir>/src/$1",
    "^@components/(.*)$": "<rootDir>/src/components/$1",
    "^@lib/(.*)$": "<rootDir>/src/lib/$1",
    "^@hooks/(.*)$": "<rootDir>/src/hooks/$1",
    "^@stores/(.*)$": "<rootDir>/src/stores/$1",
    "^@types/(.*)$": "<rootDir>/src/types/$1",
  },
};

module.exports = createJestConfig(customJestConfig);
