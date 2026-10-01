import { test, expect } from '../fixtures';
import type { Page } from '@playwright/test';
import { AffiliatePage } from '../pages/affiliate-page';

/**
 * Affiliate cabinet journeys (authenticated project via storageState).
 * Affiliate backend is stubbed per-test with page.route() — deterministic
 * payloads, zero shared state, parallel-safe.
 */

const dashboardPayload = {
  data: {
    earnings_today: '124.80',
    earnings_this_month: '2481.50',
    pending_amount: '612.20',
    available_amount: '438.00',
    paid_amount: '250.00',
    clicks: 12481,
    registrations: 684,
    ftd_count: 179,
    active_players: 96,
  },
};

const emptyLists = { data: [] };

async function stubCabinetBackend(page: Page) {
  await page.route('**/api/v1/affiliate/profile', (route) =>
    route.fulfill({ status: 200, body: JSON.stringify({ data: { status: 'active' } }) }),
  );
  await page.route('**/api/v1/affiliate/dashboard', (route) =>
    route.fulfill({ status: 200, body: JSON.stringify(dashboardPayload) }),
  );
  await page.route('**/api/v1/affiliate/earnings*', (route) =>
    route.fulfill({ status: 200, body: JSON.stringify(emptyLists) }),
  );
  await page.route('**/api/v1/affiliate/links*', (route) =>
    route.fulfill({ status: 200, body: JSON.stringify(emptyLists) }),
  );
  await page.route('**/api/v1/affiliate/payouts', (route) =>
    route.fulfill({ status: 200, body: JSON.stringify(emptyLists) }),
  );
  await page.route('**/api/v1/affiliate/payout-methods', (route) =>
    route.fulfill({
      status: 200,
      body: JSON.stringify({
        data: [{ id: 'm-usdt', method_type: 'crypto', display_name: 'USDT TRC20', is_default: true }],
      }),
    }),
  );
}

test.describe('affiliate cabinet', () => {
  test('shows enrollment CTA when profile is missing', async ({ page }) => {
    await page.route('**/api/v1/affiliate/profile', (route) =>
      route.fulfill({ status: 404, body: JSON.stringify({ error: { code: 'NOT_FOUND' } }) }),
    );
    for (const path of ['dashboard', 'earnings', 'links', 'payouts', 'payout-methods']) {
      await page.route(`**/api/v1/affiliate/${path}*`, (route) =>
        route.fulfill({ status: 200, body: JSON.stringify(emptyLists) }),
      );
    }

    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();
    await expect(affiliatePage.enrollButton).toBeVisible();
    await expect(page.getByText('Not enrolled')).toBeVisible();
  });

  test('enrolls and renders live dashboard data', async ({ page }) => {
    // State machine rather than a call counter: React 18 StrictMode (and
    // the dev double-effect) can issue the initial GET more than once, so
    // "first call returns 404" is not a reliable switch.
    let enrolled = false;
    await page.route('**/api/v1/affiliate/enroll', (route) => {
      enrolled = true;
      return route.fulfill({
        status: 201,
        body: JSON.stringify({ data: { status: 'pending_review' } }),
      });
    });
    await page.route('**/api/v1/affiliate/profile', (route) => {
      if (!enrolled) {
        return route.fulfill({
          status: 404,
          body: JSON.stringify({ error: { code: 'NOT_FOUND' } }),
        });
      }
      return route.fulfill({
        status: 200,
        body: JSON.stringify({
          data: { status: 'pending_review', commission_rate: '0.2', hold_period_days: 14 },
        }),
      });
    });
    for (const path of ['dashboard', 'earnings', 'links', 'payouts', 'payout-methods']) {
      await page.route(`**/api/v1/affiliate/${path}*`, (route) =>
        route.fulfill({ status: 200, body: JSON.stringify(emptyLists) }),
      );
    }
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();
    await affiliatePage.enrollButton.click();
    // The page reloads its data after enroll; the profile status badge must
    // flip from "Not enrolled" to the submitted state.
    await expect(page.getByText('Not enrolled')).toBeHidden();
    await expect(page.getByText('pending_review')).toBeVisible();
  });

  test('renders dashboard figures from the API, not mocks', async ({ page }) => {
    await stubCabinetBackend(page);

    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();
    // Values come from dashboardPayload above — mock fallbacks would show
    // different numbers ($124.80 today matches both by design here, but
    // clicks=12481 with registrations=684 is the stubbed funnel shape).
    await expect(page.getByText('12481').first()).toBeVisible();
    await expect(page.getByText('684').first()).toBeVisible();
  });

  test('validates payout request client-side', async ({ page }) => {
    await stubCabinetBackend(page);

    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    // No method selected.
    await affiliatePage.payoutMethodSelect.selectOption('');
    await affiliatePage.payoutAmountInput.fill('150');
    await affiliatePage.payoutSubmitButton.click();
    await expect(page.getByText('Выберите payout method.')).toBeVisible();

    // Non-positive amount.
    await affiliatePage.payoutMethodSelect.selectOption('m-usdt');
    await affiliatePage.payoutAmountInput.fill('0');
    await affiliatePage.payoutSubmitButton.click();
    await expect(page.getByText('Введите корректную сумму больше нуля.')).toBeVisible();
  });

  test('creates a payout request', async ({ page }) => {
    await stubCabinetBackend(page);
    await page.route('**/api/v1/affiliate/payouts/request', (route) =>
      route.fulfill({ status: 201, body: JSON.stringify({ data: { id: 'p-1', status: 'requested' } }) }),
    );

    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();
    await affiliatePage.payoutMethodSelect.selectOption('m-usdt');
    await affiliatePage.payoutAmountInput.fill('150');
    await affiliatePage.payoutSubmitButton.click();
    await expect(page.getByText('Заявка на выплату создана и отправлена на review.')).toBeVisible();
  });
});
