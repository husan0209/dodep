import { expect, type Locator, type Page } from '@playwright/test';
import { waitForHydration } from '../fixtures';

/**
 * RegisterPage — /register.
 *
 * NOTE (a11y finding, not fixed here): labels use htmlFor correctly on this
 * page, so getByLabel works. The login page has a mismatched
 * label(htmlFor=email)/input(id=identifier) pair — see LoginPage.
 */
export class RegisterPage {
  readonly page: Page;
  readonly usernameInput: Locator;
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly confirmPasswordInput: Locator;
  readonly countrySelect: Locator;
  readonly currencySelect: Locator;
  readonly termsCheckbox: Locator;
  readonly submitButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.usernameInput = page.getByLabel('Имя пользователя');
    this.emailInput = page.getByLabel('Email', { exact: true });
    this.passwordInput = page.getByLabel('Пароль', { exact: true });
    this.confirmPasswordInput = page.getByLabel('Подтвердите пароль');
    this.countrySelect = page.getByLabel('Страна');
    this.currencySelect = page.getByLabel('Валюта');
    this.termsCheckbox = page.locator('#terms');
    this.submitButton = page.getByRole('button', { name: 'Создать аккаунт' });
  }

  async goto() {
    await this.page.goto('/register');
    await expect(this.getHeading()).toBeVisible();
    await waitForHydration(this.page);
  }

  getHeading() {
    return this.page.getByRole('heading', { name: 'Регистрация' });
  }

  errorBox() {
    return this.page.locator('form div.rounded-xl p.text-red-200');
  }

  async register(input: {
    username: string;
    email: string;
    password: string;
    confirmPassword?: string;
  }) {
    await this.usernameInput.fill(input.username);
    await this.emailInput.fill(input.email);
    await this.passwordInput.fill(input.password);
    await this.confirmPasswordInput.fill(input.confirmPassword ?? input.password);
    await this.termsCheckbox.check();
    await this.submitButton.click();
  }
}
