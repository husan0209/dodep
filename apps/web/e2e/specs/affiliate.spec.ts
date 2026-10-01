import { test, expect } from '../fixtures';
import type { Page, Route } from '@playwright/test';
import { AffiliatePage } from '../pages/affiliate-page';

/**
 * Affiliate cabinet journeys (authenticated project via storageState).
 * Affiliate backend is stubbed per-test with page.route() — deterministic
 * payloads, zero shared state, parallel-safe.
 *
 * Payloads use sentinel figures that appear nowhere in the page's hardcoded
 * fallbacks (124.80 / 12 481 / "Main Campaign" / ...). Asserting the
 * sentinels is what proves the panel is driven by the API and not by the
 * static defaults the component falls back to.
 */

const SENTINEL = {
  earnings_today: '777.11',
  earnings_this_month: '8888.22',
  pending_amount: '333.44',
  available_amount: '555.66',
  clicks: '31337',
  registrations: '4242',
  ftd_count: '1379',
  active_players: '691',
};

const dashboardPayload = { ...SENTINEL };

const earningsPayload = {
  earnings: [
    {
      period_end: '2099-01-31',
      ggr_amount: '910.10',
      ngr_amount: '505.55',
      commission_amount: '101.11',
      status: 'available',
    },
    {
      period_end: '2099-01-30',
      ggr_amount: '111.11',
      ngr_amount: '222.22',
      commission_amount: '44.44',
      status: 'pending',
    },
  ],
};

const linksPayload = {
  links: [
    {
      campaign_name: 'Sentinel Esports',
      referral_code: 'SENTINEL-ESP-01',
      referral_url: 'https://dod.example/r/SENTINEL-ESP-01/main',
      utm_source: 'sentinelsource',
      utm_medium: 'sentinelmedium',
      utm_campaign: 'sentinelcampaign',
    },
  ],
};

const payoutsPayload = {
  data: [
    {
      id: 'pay_sentinel_1',
      amount: '901.99',
      status: 'reviewing',
      requested_at: '2099-01-29',
      method_id: 'Sentinel USDT',
    },
  ],
};

type StubOverrides = {
  dashboard?: unknown;
  earnings?: unknown;
  links?: unknown;
  payouts?: unknown;
  /** Paths that should answer 500 instead of a payload. */
  failing?: string[];
};

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

async function stubCabinetBackend(page: Page, overrides: StubOverrides = {}) {
  const failing = new Set(overrides.failing ?? []);
  const routes: Array<[string, unknown]> = [
    ['dashboard', overrides.dashboard ?? dashboardPayload],
    ['earnings', overrides.earnings ?? earningsPayload],
    ['links', overrides.links ?? linksPayload],
    ['payouts', overrides.payouts ?? payoutsPayload],
  ];

  for (const [path, body] of routes) {
    await page.route(`**/api/v1/affiliate/${path}`, (route) =>
      failing.has(path) ? json(route, { error: 'boom' }, 500) : json(route, body),
    );
  }
}

test.describe('affiliate cabinet', () => {
  test('renders summary cards from the dashboard API', async ({ page }) => {
    await stubCabinetBackend(page);
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    await expect(affiliatePage.activeBadge).toBeVisible();
    await expect(affiliatePage.summaryValue('Сегодня')).toHaveText(`$${SENTINEL.earnings_today}`);
    await expect(affiliatePage.summaryValue('За месяц')).toHaveText(
      `$${SENTINEL.earnings_this_month}`,
    );
    await expect(affiliatePage.summaryValue('В ожидании')).toHaveText(`$${SENTINEL.pending_amount}`);
    await expect(affiliatePage.summaryValue('Доступно')).toHaveText(`$${SENTINEL.available_amount}`);
  });

  test('renders funnel figures from the dashboard API', async ({ page }) => {
    await stubCabinetBackend(page);
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    await expect(affiliatePage.funnelValue('Клики')).toHaveText(SENTINEL.clicks);
    await expect(affiliatePage.funnelValue('Регистрации')).toHaveText(SENTINEL.registrations);
    await expect(affiliatePage.funnelValue('FTD')).toHaveText(SENTINEL.ftd_count);
    await expect(affiliatePage.funnelValue('Активные игроки')).toHaveText(SENTINEL.active_players);
  });

  test('renders referral links returned by the API', async ({ page }) => {
    await stubCabinetBackend(page);
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    const card = affiliatePage.linkCard('Sentinel Esports');
    await expect(card).toBeVisible();
    await expect(card.getByText('SENTINEL-ESP-01', { exact: true })).toBeVisible();
    await expect(
      card.getByText('https://dod.example/r/SENTINEL-ESP-01/main', { exact: true }),
    ).toBeVisible();
    await expect(card.getByText(/utm_source=sentinelsource/)).toBeVisible();
    // Copy button exists per link card.
    await expect(card.getByRole('button', { name: 'Скопировать' })).toBeVisible();
  });

  test('renders payout history returned by the API', async ({ page }) => {
    await stubCabinetBackend(page);
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    const row = affiliatePage.payoutRow(`$${payoutsPayload.data[0].amount}`);
    await expect(row).toBeVisible();
    await expect(row.getByText('reviewing', { exact: true })).toBeVisible();
    await expect(row.getByText('Sentinel USDT', { exact: true })).toBeVisible();
    await expect(row.getByText('2099-01-29', { exact: true })).toBeVisible();
  });

  test('renders earnings rows returned by the API', async ({ page }) => {
    await stubCabinetBackend(page);
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    const row = affiliatePage.earningRow('2099-01-31');
    await expect(row).toBeVisible();
    await expect(row.getByText('$910.10 / $505.55')).toBeVisible();
    await expect(row.getByText('$101.11', { exact: true })).toBeVisible();
    await expect(row.getByText('available', { exact: true })).toBeVisible();
  });

  test('keeps static fallbacks when list endpoints return nothing', async ({ page }) => {
    // Empty arrays are treated as "no data": the component leaves its
    // hardcoded rows in place rather than blanking the panel.
    await stubCabinetBackend(page, {
      earnings: { earnings: [] },
      links: { links: [] },
      payouts: { data: [] },
    });
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    await expect(affiliatePage.earningRow('2026-04-15')).toBeVisible();
    await expect(affiliatePage.linkCard('Main Campaign')).toBeVisible();
    await expect(affiliatePage.payoutRow('$250.00')).toBeVisible();
    // The dashboard has no such guard — it always renders, zeros included.
    await expect(affiliatePage.summaryValue('Сегодня')).toHaveText(`$${SENTINEL.earnings_today}`);
  });

  test('survives a failing affiliate endpoint', async ({ page }) => {
    await stubCabinetBackend(page, { failing: ['dashboard', 'earnings'] });
    const affiliatePage = new AffiliatePage(page);
    await affiliatePage.goto();

    // The four requests go out through one Promise.all, so a single 500
    // rejects the batch and none of the setters run. The cabinet therefore
    // keeps its hardcoded figures — not zeros. Asserting the defaults is
    // what pins that down; a future refactor to per-request handling would
    // legitimately change it to $0.00 and this test should catch that.
    await expect(affiliatePage.heading).toBeVisible();
    await expect(affiliatePage.summaryValue('Сегодня')).toHaveText('$124.80');
    await expect(affiliatePage.funnelValue('Клики')).toHaveText('12 481');
    await expect(affiliatePage.earningRow('2026-04-15')).toBeVisible();
  });
});