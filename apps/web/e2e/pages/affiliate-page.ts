import { expect, type Locator, type Page } from "@playwright/test";
import { waitForHydration } from "../fixtures";

/**
 * AffiliatePage — /affiliate (partner cabinet).
 *
 * Scope note: this object covers the cabinet that exists on `main`. The
 * enrollment flow (`/profile` 404 -> CTA) and the payout-request form live
 * in `AffiliatePage` consumers that were still in flight when this suite was
 * written and are not on `main` yet, so they are not asserted here — a spec
 * bound to unmerged UI just stays red and trains people to ignore it.
 *
 * Backend contract consumed by the page (Go affiliate service). Note the
 * shapes: `api.get` returns the parsed body verbatim, so the dashboard
 * fields sit at the top level while the list endpoints nest them.
 * - GET /api/v1/affiliate/dashboard  -> { earnings_today, earnings_this_month,
 *        pending_amount, available_amount, clicks, registrations,
 *        ftd_count, active_players }
 * - GET /api/v1/affiliate/earnings   -> { earnings: [ { period_end,
 *        ggr_amount, ngr_amount, commission_amount, status } ] }
 * - GET /api/v1/affiliate/links      -> { links: [ { campaign_name,
 *        referral_code, referral_url, utm_source, utm_medium, utm_campaign } ] }
 * - GET /api/v1/affiliate/payouts    -> { data: [ { id, amount, status,
 *        requested_at, method_id } ] }
 */
export class AffiliatePage {
  readonly page: Page;
  readonly heading: Locator;
  readonly activeBadge: Locator;
  readonly payoutSettingsHeading: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole("heading", { name: "Партнерский кабинет" });
    this.activeBadge = page.getByText("Affiliate Active");
    this.payoutSettingsHeading = page.getByRole("heading", {
      name: "Payout Settings",
    });
  }

  async goto() {
    await this.page.goto("/affiliate");
    await expect(this.heading).toBeVisible();
    await waitForHydration(this.page);
  }

  /**
   * Value of a "label above, figure below" stat.
   *
   * Both the summary cards and the funnel cells render
   * `<p>{label}</p><p>{value}</p><p>{hint}</p>`, so the figure is the first
   * sibling paragraph after the label. Labels are unique across the page.
   */
  statValue(label: string): Locator {
    return this.page
      .getByText(label, { exact: true })
      .locator("xpath=following-sibling::p[1]");
  }

  summaryValue(label: string): Locator {
    return this.statValue(label);
  }

  funnelValue(label: string): Locator {
    return this.statValue(label);
  }

  /** A referral-link card identified by its campaign name. */
  linkCard(campaignName: string): Locator {
    return this.page
      .locator("div.rounded")
      .filter({ has: this.page.getByText(campaignName, { exact: true }) })
      .first();
  }

  /** A payout-history row identified by its amount. */
  payoutRow(amount: string): Locator {
    return this.page
      .locator("div.rounded")
      .filter({ has: this.page.getByText(amount, { exact: true }) })
      .first();
  }

  /** An earnings row identified by its period date. */
  earningRow(period: string): Locator {
    return this.page
      .locator("div.rounded")
      .filter({ has: this.page.getByText(period, { exact: true }) })
      .first();
  }
}
