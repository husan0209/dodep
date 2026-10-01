import type { Page } from '@playwright/test';
import { test, expect, type TestUser } from '../fixtures';
import { RegisterPage } from '../pages/register-page';
import { LoginPage } from '../pages/login-page';

/** Register through the UI and land on the sportsbook. */
async function registerPageViaUi(page: Page, user: TestUser) {
  const registerPage = new RegisterPage(page);
  await registerPage.goto();
  await registerPage.register({
    username: user.username,
    email: user.email,
    password: user.password,
  });
  await expect(page).toHaveURL(/\/sportsbook/);
}

/**
 * Auth journeys (guest project — no storageState).
 * Backend: auth calls are proxied to mock-api-server by the fixture layer.
 */
test.describe('registration', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('registers a new user and lands on sportsbook', async ({ page, testUser }) => {
    const registerPage = new RegisterPage(page);
    await registerPage.goto();
    await registerPage.register({
      username: testUser.username,
      email: testUser.email,
      password: testUser.password,
    });
    await expect(page).toHaveURL(/\/sportsbook/);
  });

  test('rejects mismatched passwords client-side', async ({ page, testUser }) => {
    const registerPage = new RegisterPage(page);
    await registerPage.goto();
    await registerPage.register({
      username: testUser.username,
      email: testUser.email,
      password: testUser.password,
      confirmPassword: 'Different123!',
    });
    await expect(registerPage.errorBox()).toHaveText('Пароли не совпадают');
    await expect(page).toHaveURL(/\/register/);
  });

  test('rejects short passwords client-side', async ({ page, testUser }) => {
    const registerPage = new RegisterPage(page);
    await registerPage.goto();
    await registerPage.register({
      username: testUser.username,
      email: testUser.email,
      password: 'short',
      confirmPassword: 'short',
    });
    await expect(registerPage.errorBox()).toHaveText('Пароль должен содержать минимум 8 символов');
  });

  test('rejects duplicate email with a helpful message', async ({ page, testUser }) => {
    await registerPageViaUi(page, testUser);

    const registerPage = new RegisterPage(page);
    await registerPage.goto();
    await registerPage.register({
      username: `${testUser.username}x`,
      email: testUser.email,
      password: testUser.password,
    });
    // The mock answers 400 USER_ALREADY_EXISTS; the real auth service
    // answers 409 USER_ALREADY_EXISTS. Both map to the same UI copy.
    await expect(registerPage.errorBox()).toContainText('уже существует');
    await expect(page).toHaveURL(/\/register/);
  });
});

test.describe('login', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('logs in with valid credentials', async ({ page, testUser }) => {
    await registerPageViaUi(page, testUser);

    // Drop the session and log back in through the UI.
    await page.context().clearCookies();
    await page.evaluate(() => {
      localStorage.clear();
      sessionStorage.clear();
    });

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    const response = await loginPage.loginAs(testUser.email, testUser.password);
    expect(response.status()).toBe(200);
    await expect(page).toHaveURL(/\/sportsbook/);
  });

  test('shows an error on wrong password', async ({ page, testUser }) => {
    // Seed the account so the failure is "wrong password", not "no user".
    await registerPageViaUi(page, testUser);
    await page.evaluate(() => {
      localStorage.clear();
      sessionStorage.clear();
    });

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    const response = await loginPage.loginAs(testUser.email, 'WrongPass123!');
    // Mock backend returns 401 INVALID_CREDENTIALS (English); the real auth
    // service returns 401 AUTH_INVALID_CREDENTIALS and the page localizes it.
    expect(response.status()).toBe(401);
    await expect(loginPage.errorBox()).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });
});
