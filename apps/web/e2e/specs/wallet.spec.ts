import { test, expect } from "../fixtures";
import { WalletPage } from "../pages/wallet-page";

/**
 * Wallet journeys (authenticated project via storageState).
 * No wallet backend exists locally: specs assert structure, client-side
 * validation and mock history rendering — never outgoing requests.
 */
test.describe("wallet", () => {
  test("renders deposit methods with limits", async ({ page }) => {
    const walletPage = new WalletPage(page);
    await walletPage.goto();

    await expect(walletPage.methodButton("Банковская карта")).toBeVisible();
    await expect(walletPage.methodButton("СБП")).toBeVisible();
    await expect(walletPage.methodButton("Cryptocurrency")).toBeVisible();
    // Default method (card) limits hint.
    await expect(walletPage.limitsHint).toContainText(
      "Мин: 100₽ | Макс: 100000₽",
    );
  });

  test("switching method updates limits hint", async ({ page }) => {
    const walletPage = new WalletPage(page);
    await walletPage.goto();

    await walletPage.methodButton("Cryptocurrency").click();
    await expect(walletPage.limitsHint).toContainText(
      "Мин: 500₽ | Макс: 500000₽",
    );
    await expect(walletPage.amountInput).toHaveAttribute("min", "500");
  });

  test("submit fires no payment request yet (stub behaviour)", async ({
    page,
  }) => {
    const walletPage = new WalletPage(page);
    await walletPage.goto();

    let requested = false;
    await page.route("**/api/v1/payments/**", (route) => {
      requested = true;
      return route.fulfill({ status: 200, body: "{}" });
    });

    // handleSubmit currently only tracks analytics and never calls the API.
    // This test locks that behaviour: the day it starts requesting,
    // the flag flips and the spec must grow into a real deposit journey.
    await page.getByRole("button", { name: "+1000₽" }).click();
    const submit = page.getByRole("button", { name: "Пополнить на 1000₽" });
    await expect(submit).toBeVisible();
    await submit.click();
    await expect(walletPage.limitsHint).toBeVisible();
    expect(requested).toBe(false);
  });
});
