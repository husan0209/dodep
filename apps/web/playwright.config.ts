import { defineConfig, devices } from "@playwright/test";

/**
 * E2E suite for the Next.js web app (dod-web).
 *
 * Architecture (per Playwright docs + project conventions):
 * - specs/   — business journeys only (no selectors, no URLs except via POMs)
 * - pages/   — page objects, one selector per element, role-first locators
 * - fixtures — shared test data + API helpers (unique data per test)
 * - setup/   — one-time auth via API+UI, storageState reused by projects
 *
 * Backends:
 * - Mock API (apps/web/mock-api-server.js) serves auth. Default ports are
 *   E2E-isolated (:18080 API / :3100 web) so parallel agents running their
 *   own dev servers (:3000/:3002/:3005) never cross-talk with this suite.
 *   The mock listens on MOCK_PORT (default 8080, unchanged behaviour).
 * - The app's own API bases (NEXT_PUBLIC_*, default :8080/:8083) are
 *   never used: e2e/fixtures.ts intercepts every /api/v1/** call —
 *   auth is proxied to the mock API (real HTTP), the rest is stubbed.
 * - Wallet/affiliate backends are stubbed per-test with page.route()
 *   (deterministic, no shared state, parallel-safe).
 */
export default defineConfig({
  testDir: "./e2e",
  outputDir: "./e2e/test-results",
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 4 : undefined,
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: "./e2e/playwright-report" }],
    ["junit", { outputFile: "./e2e/test-results/junit.xml" }],
  ],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3100",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "off",
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
  },
  projects: [
    {
      name: "setup",
      testDir: "./e2e/setup",
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "./e2e/.auth/player.json",
      },
      dependencies: ["setup"],
    },
    {
      name: "mobile",
      use: {
        ...devices["Pixel 7"],
        storageState: "./e2e/.auth/player.json",
      },
      dependencies: ["setup"],
    },
  ],
  webServer: [
    {
      command: "node mock-api-server.js",
      cwd: __dirname,
      port: 18080,
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
      env: {
        MOCK_PORT: "18080",
      },
    },
    {
      // Plain `next dev` on an isolated port. The suite does NOT rely on
      // NEXT_PUBLIC_* env: every backend call is intercepted in
      // e2e/fixtures.ts (auth proxied to the mock API, everything else
      // stubbed), so the app's default API bases never matter.
      command: "npx next dev -p 3100",
      cwd: __dirname,
      port: 3100,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
    },
  ],
});
