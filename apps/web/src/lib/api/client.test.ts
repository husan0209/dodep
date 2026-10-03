jest.mock("@/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({
      accessToken: null,
      refreshTokens: jest.fn(),
      logout: jest.fn(),
    }),
  },
}));

import { ApiClientError, createApiClient, resolveApiBaseUrl, resolveAuthApiBaseUrl } from "./client";

describe("api client configuration", () => {
  test("prefers explicit auth API url when provided", () => {
    const base = resolveAuthApiBaseUrl({
      NEXT_PUBLIC_AUTH_API_URL: "http://localhost:18083",
      NEXT_PUBLIC_API_URL: "http://localhost:8080",
    });
    expect(base).toBe("http://localhost:18083");
  });

  test("falls back to generic API url for auth when auth-specific url is missing", () => {
    const base = resolveAuthApiBaseUrl({
      NEXT_PUBLIC_API_URL: "http://localhost:8080",
    });
    expect(base).toBe("http://localhost:8080");
  });

  test("uses default API url when env is empty", () => {
    expect(resolveApiBaseUrl({})).toBe("http://localhost:8080");
  });
});

describe("network diagnostics", () => {
  test("converts fetch connection failure to structured ApiClientError", async () => {
    const client = createApiClient("http://localhost:8080");

    // `jest.spyOn(global, "fetch")` throws "Property `fetch` does not exist in the
    // provided object" whenever the test environment does not expose a global fetch,
    // which depends on the Node version jest is running under. Defining the property
    // directly works in every environment and is still reverted afterwards.
    const originalFetch = globalThis.fetch;
    const fetchMock = jest.fn().mockRejectedValue(new TypeError("fetch failed"));
    Object.defineProperty(globalThis, "fetch", {
      value: fetchMock,
      configurable: true,
      writable: true,
    });

    let error: unknown;
    try {
      await client.post("/api/v1/auth/register", { email: "broken@example.com" });
    } catch (caught) {
      error = caught;
    } finally {
      if (originalFetch) {
        Object.defineProperty(globalThis, "fetch", {
          value: originalFetch,
          configurable: true,
          writable: true,
        });
      } else {
        delete (globalThis as { fetch?: unknown }).fetch;
      }
    }

    expect(error).toBeInstanceOf(ApiClientError);
    const apiError = error as ApiClientError;
    expect(apiError.status).toBe(0);
    expect(apiError.error.code).toBe("NETWORK_CONNECTION_REFUSED");
    expect(apiError.error.message).toContain("http://localhost:8080/api/v1/auth/register");
  });
});
