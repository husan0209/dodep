import { expect, type Locator, type Page } from '@playwright/test';
import { waitForHydration } from '../fixtures';

/**
 * WalletPage — /wallet (deposit tab is default).
 *
 * SCOPE NOTE: tab buttons in wallet.tsx carry mojibake labels (file looks
 * windows-1251-encoded served as UTF-8), so specs assert structure and the
 * deposit form (clean strings) instead of tab names. History rows are
 * hardcoded mocks in the page — asserted as render-only.
 */
export class WalletPage {
  readonly page: Page;
  readonly depositHeading: Locator;
  readonly amountInput: Locator;
  readonly limitsHint: Locator;

  constructor(page: Page) {
    this.page = page;
    this.depositHeading = page.getByRole('heading', { name: 'Выберите способ оплаты' });
    this.amountInput = page.locator('input[placeholder="0"]');
    this.limitsHint = page.getByText(/Мин: .*₽ \| Макс: .*₽/);
  }

  async goto() {
    await this.page.goto('/wallet');
    await expect(this.depositHeading).toBeVisible();
    await waitForHydration(this.page);
  }

  methodButton(name: string): Locator {
    return this.page.getByRole('button', { name: new RegExp(name) });
  }
}
