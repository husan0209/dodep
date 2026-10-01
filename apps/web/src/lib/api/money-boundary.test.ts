/**
 * Contract tests for the money boundary of the API layer.
 *
 * These assert the platform rule (CONVENTIONS NEVER-6) at the only place
 * where it can leak: the request payload that `fetch` actually receives.
 * No float may ever be sent for a monetary field.
 */

import { betsApi, type PlaceBetRequest } from "./bets";
import { walletApi } from "./wallet";
import { MoneyError } from "./money";

jest.mock("@/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({
      accessToken: "test-token",
      refreshTokens: jest.fn(),
      logout: jest.fn(),
    }),
  },
}));

type FetchArgs = [string, RequestInit];

// jsdom in this environment exposes neither `fetch` nor `crypto.randomUUID`
// (the API client uses both). Provide minimal implementations so the suite
// does not depend on the runtime version.
beforeAll(() => {
  if (typeof globalThis.crypto === "undefined") {
    (globalThis as unknown as { crypto: unknown }).crypto = {};
  }
  if (typeof globalThis.crypto.randomUUID !== "function") {
    globalThis.crypto.randomUUID = (() =>
      "00000000-0000-4000-8000-000000000000") as () => `${string}-${string}-${string}-${string}-${string}`;
  }
});

function mockFetchOk() {
  // jsdom in this environment has no global fetch — define it explicitly
  // instead of spying, so the suite is independent of the runtime.
  const calls: FetchArgs[] = [];
  const impl = jest.fn(async (url: string, init: RequestInit) => {
    calls.push([url, init]);
    return {
      ok: true,
      status: 200,
      json: async () => ({ ok: true }),
    } as Response;
  });
  (globalThis as unknown as { fetch: unknown }).fetch = impl;
  return { calls, impl };
}

function sentBody(calls: FetchArgs[], index = 0): Record<string, unknown> {
  const [, init] = calls[index];
  return JSON.parse(String(init.body)) as Record<string, unknown>;
}

describe("wallet API money contract", () => {
  it("sends a decimal string amount for a deposit", async () => {
    const { calls } = mockFetchOk();

    await walletApi.deposit({ amount: "1500.75", method: "card", currency: "USD" });

    const body = sentBody(calls);
    expect(typeof body.amount).toBe("string");
    expect(body.amount).toBe("1500.75");
  });

  it("refuses a float deposit instead of silently rounding it", async () => {
    const { impl } = mockFetchOk();

    await expect(
      walletApi.deposit({
        amount: 1500.75 as unknown as string,
        method: "card",
      }),
    ).rejects.toBeInstanceOf(MoneyError);
    expect(impl).not.toHaveBeenCalled();
  });

  it("refuses zero and negative withdrawals", async () => {
    mockFetchOk();

    await expect(
      walletApi.withdraw({ amount: "0", method: "crypto" }),
    ).rejects.toBeInstanceOf(MoneyError);
    await expect(
      walletApi.withdraw({ amount: "-10", method: "crypto" }),
    ).rejects.toBeInstanceOf(MoneyError);
  });

  it("keeps sub-cent precision that a float would destroy", async () => {
    const { calls } = mockFetchOk();

    await walletApi.deposit({
      amount: "0.00000001",
      method: "crypto",
      currency: "BTC",
    });

    expect(sentBody(calls).amount).toBe("0.00000001");
  });
});

describe("betting API money contract", () => {
  const baseBet: PlaceBetRequest = {
    bet_type: "single",
    selections: [
      {
        event_id: 1,
        market_id: 2,
        outcome_id: 3,
        odds: "1.85",
        outcome_name: "Home win",
      },
    ],
    stake: "50.00",
    currency: "USD",
    accept_odds_changes: "none",
    idempotency_key: "11111111-1111-1111-1111-111111111111",
  };

  it("sends stake and odds as decimal strings", async () => {
    const { calls } = mockFetchOk();

    await betsApi.placeBet(baseBet);

    const body = sentBody(calls);
    expect(typeof body.stake).toBe("string");
    expect(body.stake).toBe("50.00");
    const selections = body.selections as Array<{ odds: unknown }>;
    expect(typeof selections[0].odds).toBe("string");
  });

  it("refuses a float stake", async () => {
    const { impl } = mockFetchOk();

    await expect(
      betsApi.placeBet({ ...baseBet, stake: 50 as unknown as string }),
    ).rejects.toBeInstanceOf(MoneyError);
    expect(impl).not.toHaveBeenCalled();
  });

  it("refuses float odds and reports the offending field", async () => {
    mockFetchOk();

    await expect(
      betsApi.placeBet({
        ...baseBet,
        selections: [{ ...baseBet.selections[0], odds: 1.85 as unknown as string }],
      }),
    ).rejects.toThrow(/bet\.selections\[0\]\.odds/);
  });
});