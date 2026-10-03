const nextJest = require("next/jest");

const createJestConfig = nextJest({
  dir: "./",
});

const customJestConfig = {
  testEnvironment: "jest-environment-jsdom",
  setupFilesAfterEnv: ["<rootDir>/jest.setup.ts"],
  // e2e/ is Playwright territory (see playwright.config.ts). Without this,
  // `jest --listTests` picks up the .spec.ts files there and `npm test`
  // fails trying to run them in jsdom. Keep the runners disjoint.
  testPathIgnorePatterns: ["<rootDir>/e2e/", "<rootDir>/node_modules/"],
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
