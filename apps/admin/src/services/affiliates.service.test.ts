import { describe, it, expect, vi, beforeEach } from "vitest";

const { getMock, putMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  putMock: vi.fn(),
  postMock: vi.fn(),
}));

vi.mock("./api", () => ({
  default: {
    get: getMock,
    put: putMock,
    post: postMock,
  },
}));

import {
  affiliatesService,
  type PostbackConfig,
} from "./affiliates.service";

const AFFILIATE_ID = "123e4567-e89b-42d3-a456-426614174000";

const SAMPLE_CONFIGS: PostbackConfig[] = [
  {
    event: "registration",
    url: "https://partner.example/postback?click_id={click_id}",
    method: "GET",
    variables: { click_id: "{click_id}" },
    retry_count: 3,
    retry_backoff: "exponential",
  },
  {
    event: "ftd",
    url: "https://partner.example/postback",
    method: "POST",
    retry_count: 3,
    retry_backoff: "exponential",
  },
];

beforeEach(() => {
  getMock.mockReset();
  putMock.mockReset();
  postMock.mockReset();
});

describe("affiliatesService.getPostbackConfigs", () => {
  it("requests the affiliate postback-config endpoint", async () => {
    getMock.mockResolvedValue({ data: { data: SAMPLE_CONFIGS } });

    const result =
      await affiliatesService.getPostbackConfigs(AFFILIATE_ID);

    expect(getMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledWith(
      `/admin/affiliates/${AFFILIATE_ID}/postback-config`,
    );
    expect(result).toEqual(SAMPLE_CONFIGS);
  });

  it("returns an empty array when backend data is missing", async () => {
    getMock.mockResolvedValue({ data: {} });

    const result =
      await affiliatesService.getPostbackConfigs(AFFILIATE_ID);

    expect(result).toEqual([]);
  });
});

describe("affiliatesService.updatePostbackConfigs", () => {
  it("PUTs the full config array to the endpoint", async () => {
    putMock.mockResolvedValue({ data: { success: true } });

    await affiliatesService.updatePostbackConfigs(
      AFFILIATE_ID,
      SAMPLE_CONFIGS,
    );

    expect(putMock).toHaveBeenCalledTimes(1);
    expect(putMock).toHaveBeenCalledWith(
      `/admin/affiliates/${AFFILIATE_ID}/postback-config`,
      SAMPLE_CONFIGS,
    );
  });

  it("propagates backend errors", async () => {
    putMock.mockRejectedValue(new Error("database error"));

    await expect(
      affiliatesService.updatePostbackConfigs(AFFILIATE_ID, []),
    ).rejects.toThrow("database error");
  });
});

describe("affiliatesService fraud flag review", () => {
  it("resolves a flag with mandatory notes", async () => {
    postMock.mockResolvedValue({ data: { success: true } });

    await affiliatesService.resolveFraudFlag("flag-1", "self-referral confirmed");

    expect(postMock).toHaveBeenCalledWith(
      "/admin/affiliates/fraud-flags/flag-1/resolve",
      { notes: "self-referral confirmed" },
    );
  });

  it("dismisses a flag with optional notes sent as empty string", async () => {
    postMock.mockResolvedValue({ data: { success: true } });

    await affiliatesService.dismissFraudFlag("flag-2");

    expect(postMock).toHaveBeenCalledWith(
      "/admin/affiliates/fraud-flags/flag-2/dismiss",
      { notes: "" },
    );
  });
});

describe("affiliatesService stats and players", () => {
  it("fetches stats without a day window by default", async () => {
    const stats = { ngr: "100.00000000", players: 7 };
    getMock.mockResolvedValue({ data: { data: stats } });

    const result = await affiliatesService.getAffiliateStats(AFFILIATE_ID);

    expect(getMock).toHaveBeenCalledWith(
      `/admin/affiliates/${AFFILIATE_ID}/stats`,
      { params: undefined },
    );
    expect(result).toEqual(stats);
  });

  it("passes the day window when provided", async () => {
    getMock.mockResolvedValue({ data: { data: { ngr: "0" } } });

    await affiliatesService.getAffiliateStats(AFFILIATE_ID, 30);

    expect(getMock).toHaveBeenCalledWith(
      `/admin/affiliates/${AFFILIATE_ID}/stats`,
      { params: { days: 30 } },
    );
  });

  it("fetches referred players with pagination", async () => {
    const players = {
      data: [{ player_ref: "plr_0123456789ab", attributed_at: "2026-01-01" }],
      pagination: { total: 1 },
    };
    getMock.mockResolvedValue({ data: players });

    const result = await affiliatesService.getAffiliatePlayers(AFFILIATE_ID, {
      page: 1,
      page_size: 20,
    });

    expect(getMock).toHaveBeenCalledWith(
      `/admin/affiliates/${AFFILIATE_ID}/players`,
      { params: { page: 1, page_size: 20 } },
    );
    expect(result.data[0].player_ref).toBe("plr_0123456789ab");
  });
});

describe("affiliatesService fraud flag conflicts", () => {
  it("marks an approved payout as paid with a provider reference", async () => {
    postMock.mockResolvedValue({ data: { success: true } });

    await affiliatesService.markPayoutPaid("pay-9", "txn_abc");

    expect(postMock).toHaveBeenCalledWith(
      "/admin/affiliates/payouts/pay-9/paid",
      { provider_reference: "txn_abc" },
    );
  });

  it("sends an empty provider reference when not supplied", async () => {
    postMock.mockResolvedValue({ data: { success: true } });

    await affiliatesService.markPayoutPaid("pay-10");

    expect(postMock).toHaveBeenCalledWith(
      "/admin/affiliates/payouts/pay-10/paid",
      { provider_reference: "" },
    );
  });

  it("surfaces the 409 when a payout was already transitioned", async () => {
    postMock.mockRejectedValue(new Error("Request failed with status code 409"));

    await expect(affiliatesService.markPayoutPaid("pay-11")).rejects.toThrow(
      "409",
    );
  });

  it("surfaces the 409 conflict when a flag was already closed", async () => {
    postMock.mockRejectedValue(new Error("Request failed with status code 409"));

    await expect(
      affiliatesService.resolveFraudFlag("flag-3", "late review"),
    ).rejects.toThrow("409");
  });
});
