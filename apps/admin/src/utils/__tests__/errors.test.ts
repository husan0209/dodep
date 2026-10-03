import { describe, it, expect } from "vitest";
import { AxiosError, type InternalAxiosRequestConfig } from "axios";
import { getErrorMessage } from "@/utils/errors";

const STUB_CONFIG = { headers: {} } as unknown as InternalAxiosRequestConfig;

function axiosErrorWithBody(
  body: unknown,
  { message = "Request failed with status code 400", status = 400 } = {},
): AxiosError {
  return new AxiosError(message, "ERR_BAD_REQUEST", STUB_CONFIG, undefined, {
    data: body,
    status,
    statusText: "",
    headers: {},
    config: STUB_CONFIG,
  });
}

describe("getErrorMessage", () => {
  it("maps a known API error code to a human-readable message", () => {
    const error = axiosErrorWithBody({
      error: {
        code: "WALLET_INSUFFICIENT_BALANCE",
        message: "wallet balance is 0",
      },
    });
    expect(getErrorMessage(error)).toBe("Insufficient balance");
  });

  it("maps auth and rate-limit codes too", () => {
    expect(
      getErrorMessage(
        axiosErrorWithBody({
          error: { code: "AUTH_INVALID_CREDENTIALS", message: "x" },
        }),
      ),
    ).toBe("Invalid email or password");

    expect(
      getErrorMessage(
        axiosErrorWithBody({
          error: { code: "RATE_LIMITED", message: "slow down" },
        }),
      ),
    ).toBe("Too many requests, please try again later");

    expect(
      getErrorMessage(
        axiosErrorWithBody({ error: { code: "KYC_REQUIRED", message: "x" } }),
      ),
    ).toBe("KYC verification required");
  });

  it("prefers the mapping over the raw backend message", () => {
    const error = axiosErrorWithBody({
      error: { code: "BET_ODDS_CHANGED", message: "odds 1.90 -> 2.10" },
    });
    expect(getErrorMessage(error)).toBe("Odds have changed");
  });

  it("falls back to the backend message for an unmapped code", () => {
    const error = axiosErrorWithBody({
      error: { code: "SOMETHING_NEW", message: "upstream is degraded" },
    });
    expect(getErrorMessage(error)).toBe("upstream is degraded");
  });

  it("falls back to the transport message when the response has no body", () => {
    const error = axiosErrorWithBody(undefined, {
      message: "Network Error",
      status: 0,
    });
    expect(getErrorMessage(error)).toBe("Network Error");
  });

  it("falls back to the transport message when the error envelope has no message", () => {
    const error = axiosErrorWithBody({ error: { code: "SOMETHING_NEW" } });
    expect(getErrorMessage(error)).toBe(
      "Request failed with status code 400",
    );
  });

  it("returns the message of a plain Error", () => {
    expect(getErrorMessage(new Error("boom"))).toBe("boom");
    expect(getErrorMessage(new TypeError("bad type"))).toBe("bad type");
  });

  it("returns a generic message for non-Error throwables", () => {
    const generic = "An unexpected error occurred";
    expect(getErrorMessage("a string")).toBe(generic);
    expect(getErrorMessage(null)).toBe(generic);
    expect(getErrorMessage(undefined)).toBe(generic);
    expect(getErrorMessage(42)).toBe(generic);
    expect(getErrorMessage({ nope: true })).toBe(generic);
  });

  it("does not treat a lookalike object with isAxiosError=false as an axios error", () => {
    expect(getErrorMessage({ isAxiosError: false })).toBe(
      "An unexpected error occurred",
    );
  });
});
