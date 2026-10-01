import { expect, type Locator, type Page } from '@playwright/test';
import { waitForHydration } from '../fixtures';

/**
 * AffiliatePage — /affiliate (partner cabinet).
 *
 * Backend contract (Go affiliate service):
 * - GET /api/v1/affiliate/profile      → 404 when not enrolled
 * - POST /api/v1/affiliate/enroll      → 201/200 on application
 * - GET /api/v1/affiliate/dashboard|earnings|links|payouts|payout-methods
 * - POST /api/v1/affiliate/payouts/request
 */
export class AffiliatePage {
  readonly page: Page;
  readonly heading: Locator;
  readonly enrollButton: Locator;
  readonly payoutMethodSelect: Locator;
  readonly payoutAmountInput: Locator;
  readonly payoutSubmitButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: 'Партнерский кабинет' });
    this.enrollButton = page.getByRole('button', { name: 'Стать партнёром' });
    this.payoutMethodSelect = page.locator('select.input-field').first();
    this.payoutAmountInput = page.getByPlaceholder('Сумма');
    this.payoutSubmitButton = page.getByRole('button', { name: 'Запросить выплату' });
  }

  async goto() {
    await this.page.goto('/affiliate');
    await expect(this.heading).toBeVisible();
    await waitForHydration(this.page);
  }

  payoutFeedback() {
    return this.page.locator('p.text-red-400, p.text-green-400');
  }
}
