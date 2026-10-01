import { test as setup, expect, makeUniqueUser, registerViaApi } from '../fixtures';
import { LoginPage } from '../pages/login-page';

/**
 * One-time authentication. Runs before the chromium/mobile projects and
 * writes a reusable storageState — no test logs in through the UI itself.
 * Re-runs on every invocation (never cached between CI runs) so tokens
 * cannot expire mid-suite.
 *
 * The auth calls are proxied to mock-api-server by the fixture layer in
 * ../fixtures.ts (installBackendStubs) — no NEXT_PUBLIC_* env required.
 */
const AUTH_FILE = './e2e/.auth/player.json';

setup('authenticate as player', async ({ page, request }) => {
  const user = makeUniqueUser('setup');
  // Seed the account through the mock API so the UI login has real
  // credentials to verify against.
  await registerViaApi(request, user);

  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.loginAs(user.email, user.password);

  // Login lands on /sportsbook for authenticated users.
  await expect(page).toHaveURL(/\/sportsbook/);
  await page.context().storageState({ path: AUTH_FILE });
});
