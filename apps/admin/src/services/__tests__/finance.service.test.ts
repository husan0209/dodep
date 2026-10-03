import { describe, it, expect, beforeEach } from "vitest";
import type {
  AxiosResponse,
  InternalAxiosRequestConfig,
} from "axios";
import apiClient from "@/services/api";
import { financeService } from "@/services/finance.service";
import { useAuthStore } from "@/stores/authStore";
import { API_BASE_URL } from "@/utils/constants";

/**
 * The real axios instance (with its real request interceptor) is used; only the
 * transport is replaced, so the assertions below cover the URL/method/body the
 * service actually puts on the wire plus the headers the interceptor injects.
 */
let sent: InternalAxiosRequestConfig[] = [];
let nextResponseBody: unknown = {};

/** Fully-qualified URL that the browser would request. */
function requestUrl(config: InternalAxiosRequestConfig): string {
  const base = config.baseURL ?? "";
  return `${base}${config.url ?? ""}`;
}

function bodyOf(config: InternalAxiosRequestConfig): unknown {
  if (typeof config.data === "string") return JSON.parse(config.data);
  return config.data;
}

function header(config: InternalAxiosRequestConfig, name: string): unknown {
  const headers = config.headers as unknown as Record<string, unknown>;
  return headers[name] ?? headers[name.toLowerCase()];
}

beforeEach(() => {
  sent = [];
  nextResponseBody = { data: null };
  apiClient.defaults.adapter = async (
    config: InternalAxiosRequestConfig,
  ): Promise<AxiosResponse> => {
    sent.push(config);
    return {
      data: nextResponseBody,
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    };
  };
  useAuthStore.setState({ accessToken: null });
});

describe("financeService.request shaping", () => {
  it("targets the admin deposits endpoint with the search params attached", async () => {
    await financeService.getDeposits({
      status: "completed",
      page: 2,
      page_size: 50,
    });

    expect(sent).toHaveLength(1);
    const [config] = sent;
    expect(config.method).toBe("get");
    expect(requestUrl(config)).toBe(
      `${API_BASE_URL}/admin/finance/deposits`,
    );
    expect(config.params).toEqual({ status: "completed", page: 2, page_size: 50 });
  });

  it("returns the paginated envelope itself, not the inner item", async () => {
    const page = { data: [{ id: "dep-1" }], total: 1, page: 1, page_size: 20 };
    nextResponseBody = page;

    const result = await financeService.getDeposits({ page: 1 });

    expect(result).toEqual(page);
  });

  it("interpolates the id into the single-deposit path and unwraps .data", async () => {
    const deposit = { id: "dep-42", amount: "10.00", currency_code: "USD" };
    nextResponseBody = { data: deposit };

    const result = await financeService.getDeposit("dep-42");

    expect(requestUrl(sent[0])).toBe(
      `${API_BASE_URL}/admin/finance/deposits/dep-42`,
    );
    expect(result).toEqual(deposit);
  });

  it("POSTs a withdrawal approval to the id-scoped action endpoint with no body", async () => {
    await financeService.approveWithdrawal("wd-7");

    const [config] = sent;
    expect(config.method).toBe("post");
    expect(requestUrl(config)).toBe(
      `${API_BASE_URL}/admin/finance/withdrawals/wd-7/approve`,
    );
    expect(config.data).toBeUndefined();
  });

  it("POSTs the rejection reason in the request body", async () => {
    await financeService.rejectWithdrawal("wd-7", "KYC documents expired");

    const [config] = sent;
    expect(config.method).toBe("post");
    expect(requestUrl(config)).toBe(
      `${API_BASE_URL}/admin/finance/withdrawals/wd-7/reject`,
    );
    expect(bodyOf(config)).toEqual({ reason: "KYC documents expired" });
  });

  it("keeps decimal money as an exact string in the adjust-balance body", async () => {
    await financeService.adjustBalance("user-9", {
      amount: "-1250.55",
      currency: "USD",
      reason: "chargeback",
      type: "debit",
    });

    const [config] = sent;
    expect(config.method).toBe("post");
    expect(requestUrl(config)).toBe(
      `${API_BASE_URL}/admin/finance/users/user-9/adjust-balance`,
    );
    expect(bodyOf(config)).toEqual({
      amount: "-1250.55",
      currency: "USD",
      reason: "chargeback",
      type: "debit",
    });
  });

  it("returns Promise<void> for mutations (no response body leaks through)", async () => {
    nextResponseBody = { data: "should be ignored" };

    await expect(financeService.approveWithdrawal("wd-1")).resolves.toBeUndefined();
  });

  it("passes the optional summary date window and unwraps .data", async () => {
    const summary = {
      total_deposits: "100.00",
      total_withdrawals: "40.00",
      net_revenue: "60.00",
      ggr: "60.00",
      pending_withdrawals_count: 3,
      pending_withdrawals_amount: "30.00",
    };
    nextResponseBody = { data: summary };

    const result = await financeService.getFinancialSummary({
      date_from: "2026-01-01",
      date_to: "2026-01-31",
    });

    expect(requestUrl(sent[0])).toBe(`${API_BASE_URL}/admin/finance/summary`);
    expect(sent[0].params).toEqual({
      date_from: "2026-01-01",
      date_to: "2026-01-31",
    });
    expect(result).toEqual(summary);
  });
});

describe("apiClient request interceptor", () => {
  it("attaches a Bearer token from the auth store on every method", async () => {
    useAuthStore.setState({ accessToken: "jwt-abc" });

    await financeService.getWithdrawals({ page: 1 });
    await financeService.approveWithdrawal("wd-3");

    expect(sent[0].headers.Authorization).toBe("Bearer jwt-abc");
    expect(sent[1].headers.Authorization).toBe("Bearer jwt-abc");
  });

  it("omits the Authorization header when there is no session", async () => {
    await financeService.getWithdrawals({ page: 1 });

    expect(header(sent[0], "Authorization")).toBeUndefined();
  });

  it("adds an X-Idempotency-Key to mutating requests only", async () => {
    await financeService.getWithdrawals({ page: 1 });
    await financeService.approveWithdrawal("wd-3");
    await financeService.rejectWithdrawal("wd-3", "nope");

    expect(header(sent[0], "X-Idempotency-Key")).toBeUndefined();
    expect(header(sent[1], "X-Idempotency-Key")).toMatch(/^\d+-[a-z0-9]+$/);
    expect(header(sent[2], "X-Idempotency-Key")).toMatch(/^\d+-[a-z0-9]+$/);
    // Two mutations must not collide on the same key.
    expect(header(sent[1], "X-Idempotency-Key")).not.toBe(
      header(sent[2], "X-Idempotency-Key"),
    );
  });

  it("sends the refresh cookie cross-origin", () => {
    expect(apiClient.defaults.withCredentials).toBe(true);
    expect(apiClient.defaults.baseURL).toBe(API_BASE_URL);
    expect(apiClient.defaults.timeout).toBe(30000);
  });
});
