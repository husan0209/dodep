/**
 * Jest configuration.
 *
 * The `test` script existed in package.json but there was no configuration and
 * no test files, so `npm test` failed with "no tests found". ts-jest is wired
 * in with the tsconfig the package actually builds with, so tests type-check
 * under the same strictness as `npm run build`.
 */
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  roots: ['<rootDir>/src'],
  testMatch: ['**/*.test.ts'],
  clearMocks: true,
  restoreMocks: true,
  collectCoverageFrom: ['src/**/*.ts', '!src/**/*.test.ts', '!src/index.ts'],
  coverageThreshold: {
    // A shared package is consumed by apps/admin via a file: dependency, so a
    // silent regression here surfaces as wrong money in another service.
    global: {
      statements: 85,
      branches: 80,
      functions: 90,
      lines: 85,
    },
  },
  transform: {
    '^.+\\.ts$': ['ts-jest', { tsconfig: '<rootDir>/tsconfig.test.json' }],
  },
};