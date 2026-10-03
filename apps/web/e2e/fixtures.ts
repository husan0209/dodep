import {
  test as base,
  expect,
  type APIRequestContext,
  type Page,
  type Route,
} from "@playwright/test";

/**
 * Shared E2E fixtures and helpers.
 *
 * Backend strategy (deterministic, parallel-safe, no env gymnastics):
 * - The app resolves its own API bases at build time (NEXT_PUBLIC_*), so the
 *   suite never depends on them. Every backend call is intercepted:
 *     * /api/v1/auth/**  → proxied to mock-api-server (real HTTP, real flows)
 *     * /api/v1/**       → stubbed empty envelope (wallet/affiliate specs
 *                          register their own routes, which win by being
 *                          registered later)
 * - Credentials are unique per test, so 8 parallel workers never collide.
 */

export const API_URL = process.env.E2E_API_URL ?? "http://localhost:18080";

export interface TestUser {
  email: string;
  username: string;
  password: string;
}

export function makeUniqueUser(tag = "e2e"): TestUser {
  const suffix = `${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;
  return {
    email: `${tag}-${suffix}@example.com`,
    username: `${tag}_${suffix}`.slice(0, 30),
    password: "TestPass123!",
  };
}

export async function registerViaApi(
  request: APIRequestContext,
  user: TestUser,
) {
  const res = await request.post(`${API_URL}/api/v1/auth/register`, {
    data: {
      email: user.email,
      password: user.password,
      username: user.username,
      country_code: "RU",
      currency_code: "RUB",
    },
  });
  if (!res.ok()) {
    throw new Error(
      `registerViaApi failed: ${res.status()} ${await res.text()}`,
    );
  }
  return (await res.json()) as {
    access_token: string;
    refresh_token: string;
    user: { id: number };
  };
}

const CORS_HEADERS = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
  "access-control-allow-headers":
    "Content-Type,Authorization,X-Idempotency-Key,X-Request-ID",
};

/**
 * Auth contract adapter.
 *
 * `mock-api-server.js` returns a FLAT token payload
 * (`{access_token, refresh_token, user}`), while the app's
 * `authApi.login/register` expects the AuthResult envelope
 * (`{user_id, tokens:{access_token,…}, session}`) — see
 * src/lib/api/auth.ts and handleAuthResult() in stores/auth-store.ts.
 * Without this translation the store dereferences `result.tokens.…` on
 * undefined, throws, and the page silently stays on /login.
 *
 * Real backends (auth service, services/go/auth) DO return AuthResult, so
 * this adapter is strictly a mock-shape fix. Recorded as finding #5 in
 * e2e/README.md.
 */
interface MockTokenPayload {
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  token_type?: string;
  user?: { id?: number; uuid?: string; email?: string; username?: string };
}

function toAuthResult(payload: MockTokenPayload) {
  const user = payload.user ?? {};
  return {
    user_id: String(user.id ?? 1),
    tokens: {
      access_token: payload.access_token ?? "e2e-access-token",
      refresh_token: payload.refresh_token ?? "e2e-refresh-token",
      expires_in: payload.expires_in ?? 3600,
      refresh_expires_in: 604800,
      token_type: payload.token_type ?? "Bearer",
    },
    session: {
      id: "e2e-session",
      user_id: String(user.id ?? 1),
      device_id: "e2e-device",
      ip_address: "127.0.0.1",
      country: "RU",
      created_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      is_active: true,
    },
    requires_2fa: false,
  };
}

function toAppUser(payload: MockTokenPayload) {
  const user = payload.user ?? {};
  return {
    id: String(user.id ?? 1),
    uuid: user.uuid ?? "e2e-uuid",
    email: user.email ?? "e2e@example.com",
    username: user.username ?? "e2e",
    country_code: "RU",
    currency_code: "RUB",
    kyc_level: 0,
    status: "active",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}

/** Forward an intercepted auth call to the mock API and fulfil it in-page. */
async function proxyAuthCall(route: Route, request: APIRequestContext) {
  const original = route.request();

  if (original.method() === "OPTIONS") {
    return route.fulfill({ status: 204, headers: CORS_HEADERS, body: "" });
  }

  const pathname = new URL(original.url()).pathname;
  const target = `${API_URL}${pathname}`;
  const method = original.method();
  const hasBody = method !== "GET" && method !== "HEAD";

  // headerValue is async in Playwright's Request API.
  const contentType =
    (await original.headerValue("content-type")) ?? "application/json";

  const fulfilJson = (status: number, body: unknown) =>
    route.fulfill({
      status,
      headers: { ...CORS_HEADERS, "content-type": "application/json" },
      body: JSON.stringify(body),
    });

  // Forward auth headers: the mock's /me requires `Authorization: Bearer …`
  // and rejects with 401 otherwise, which would break the store's
  // post-login `me()` call and send the journey back to /login.
  const authHeader = await original.headerValue("authorization");

  try {
    const postBody = hasBody ? original.postData() : null;

    const res = await request.fetch(target, {
      method,
      headers: {
        ...(postBody ? { "content-type": contentType } : {}),
        ...(authHeader ? { authorization: authHeader } : {}),
      },
      // Send NO body for empty POSTs (e.g. logout): the mock's
      // body-parser rejects an empty string with a JSON syntax error.
      data: postBody ?? undefined,
    });

    const text = await res.text();
    let parsed: unknown = text;
    try {
      parsed = JSON.parse(text);
    } catch {
      // Non-JSON passthrough (mock logs, plain errors).
      return route.fulfill({
        status: res.status(),
        headers: CORS_HEADERS,
        body: text,
      });
    }

    // Only reshape SUCCESS bodies. Wrapping a 4xx error payload into the
    // success envelope would drop its `code`/`message`, and the app would
    // then render the bare HTTP statusText ("Bad Request") instead of the
    // domain error copy — exactly the bug this adapter must not introduce.
    if (res.ok() && pathname.endsWith("/me")) {
      return fulfilJson(
        res.status(),
        toAppUser((parsed as MockTokenPayload) ?? {}),
      );
    }
    if (
      res.ok() &&
      (pathname.endsWith("/login") || pathname.endsWith("/register"))
    ) {
      return fulfilJson(
        res.status(),
        toAuthResult((parsed as MockTokenPayload) ?? {}),
      );
    }
    return fulfilJson(res.status(), parsed);
  } catch (err) {
    // Surface the real cause instead of letting the request hang (a dropped
    // request produces no `response` event, so specs time out with a
    // misleading "waiting for event response" message).
    return fulfilJson(502, {
      error: {
        code: "E2E_BACKEND_UNREACHABLE",
        message: `mock API unreachable at ${target}: ${(err as Error).message}`,
      },
    });
  }
}

/**
 * Wait until the client bundle has hydrated before interacting with a form.
 *
 * Race this suite hit in practice: a fast `fill` + `click` on /login can
 * land before React attaches `onSubmit`, so the browser performs a *native*
 * GET form submit and serialises the credentials into the URL
 * (/login?identifier=…&password=…). The journey then fails for a reason
 * that has nothing to do with the app logic under test.
 *
 * The app exposes no hydration marker (and the UI files are shared with
 * other agents), so we use the documented proxies: the network goes idle
 * and the client-only TanStack devtools toggle is attached to the DOM.
 */
export async function waitForHydration(page: Page) {
  await page.waitForLoadState("networkidle");

  // React attaches __reactProps$* to DOM nodes when it commits. Waiting for
  // the *form* to carry a real `onSubmit` handler is exactly the condition
  // this suite needs: without it the browser performs a native GET submit
  // and puts the password in the query string.
  //
  // Polling is interval-based, NOT the default requestAnimationFrame: a
  // backgrounded/occluded page in a parallel worker stops firing rAF, which
  // makes an rAF-polled wait hang until timeout.
  const isHydrated = async () =>
    page.evaluate(() => {
      // 1) React has committed at least one host node in this subtree.
      const root = document.getElementById("__next") ?? document.body;
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT);
      let committed = false;
      for (let el = walker.nextNode(); el; el = walker.nextNode()) {
        if (Object.keys(el).some((k) => k.startsWith("__reactFiber$"))) {
          committed = true;
          break;
        }
      }
      if (!committed) return false;

      // 2) If the page has a form, its submit handler must be bound —
      //    otherwise the browser does a native GET submit and leaks the
      //    credentials into the query string.
      const form = document.querySelector("form");
      if (!form) return true;
      for (const key of Object.keys(form)) {
        if (key.startsWith("__reactProps$")) {
          const props = (
            form as unknown as Record<string, { onSubmit?: unknown }>
          )[key];
          if (props && typeof props.onSubmit === "function") return true;
        }
      }
      return false;
    });

  // Retrying with a reload covers the cold `next dev` case: global-setup
  // warms the routes, but a first-ever compile can still serve HTML whose
  // chunks land after the poll window, and a reload then hits a warm cache.
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    if (await isHydrated()) return;
    if (attempt < 3) {
      await page.waitForTimeout(1_000);
      await page.reload({ waitUntil: "load" });
      await page.waitForLoadState("networkidle");
    }
  }
  throw new Error(
    `waitForHydration: React never committed a handler on ${page.url()} after 3 attempts.`,
  );
}

/** Install the default interception layer for one test. */
export async function installBackendStubs(
  page: Page,
  request: APIRequestContext,
) {
  // Registered FIRST → lower priority. Specs may add more specific routes.
  await page.route("**/api/v1/**", (route) =>
    route.fulfill({
      status: 200,
      headers: CORS_HEADERS,
      body: JSON.stringify({ data: [] }),
    }),
  );
  // Registered last → higher priority for the auth domain.
  await page.route("**/api/v1/auth/**", (route) =>
    proxyAuthCall(route, request),
  );
}

type Fixtures = {
  /** Fresh unique credentials for this test (no server side effects). */
  testUser: TestUser;
};

/**
 * `page` is overridden (not extended) so the interception layer is installed
 * for every test automatically. Specs that need richer payloads simply
 * register their own `page.route()` inside the test body — later
 * registrations take priority, so spec routes win over these defaults.
 */
export const test = base.extend<Fixtures>({
  testUser: async ({}, use) => {
    await use(makeUniqueUser());
  },
  page: async ({ page, request }, use) => {
    await installBackendStubs(page, request);
    await use(page);
  },
});

export { expect };
