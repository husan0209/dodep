import { expect, type Locator, type Page } from "@playwright/test";
import { waitForHydration } from "../fixtures";

/**
 * LoginPage — /login.
 *
 * KNOWN ISSUE (documented, not fixed — чужой файл):
 * <label htmlFor="email"> is paired with <input id="identifier">, so
 * getByLabel() cannot resolve the field. Locators below use the stable
 * input ids / placeholders instead.
 */
export class LoginPage {
  readonly page: Page;
  readonly identifierInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.identifierInput = page.locator("#identifier");
    this.passwordInput = page.locator("#password");
    this.submitButton = page.getByRole("button", {
      name: "Войти",
      exact: true,
    });
  }

  async goto() {
    await this.page.goto("/login");
    await expect(this.getHeading()).toBeVisible();
    await waitForHydration(this.page);
  }

  getHeading() {
    return this.page.getByRole("heading", { name: "Вход в аккаунт" });
  }

  errorBox() {
    return this.page.locator("form div.rounded-xl p.text-red-200");
  }

  /**
   * Submit the form and wait for the auth request.
   *
   * Waiting on the response (instead of only the URL) both prevents the
   * pre-hydration native-GET race and makes a real 401/422 failure obvious
   * instead of timing out on a URL assertion.
   */
  async loginAs(identifier: string, password: string) {
    await this.identifierInput.fill(identifier);
    await this.passwordInput.fill(password);
    const authResponse = this.page.waitForResponse((res) =>
      res.url().includes("/api/v1/auth/login"),
    );
    await this.submitButton.click();
    return authResponse;
  }
}
