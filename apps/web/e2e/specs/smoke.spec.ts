import { test, expect } from '../fixtures';

/**
 * Smoke suite: every critical route responds, and the saved session
 * from the setup project is actually reused (guard against silent
 * storageState failures — a broken session would redirect to /login
 * and fail confusingly elsewhere).
 */
test.describe('routes respond', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  for (const path of ['/', '/login', '/register', '/sportsbook', '/casino', '/wallet', '/affiliate']) {
    test(`GET ${path} responds 200`, async ({ page }) => {
      const response = await page.goto(path);
      expect(response?.status()).toBe(200);
    });
  }
});

test.describe('session reuse guard', () => {
  test('authenticated session survives navigation (no login redirect)', async ({ page }) => {
    await page.goto('/wallet');
    await expect(page).toHaveURL(/\/wallet/);
    await page.goto('/affiliate');
    await expect(page).toHaveURL(/\/affiliate/);
  });
});
