# Web E2E Suite (Playwright + TypeScript)

Critical user journeys for `dod-web`, runnable locally and in CI.
Follows Playwright official best practices (POM + fixtures + setup
project with `storageState`, role-first locators, web-first assertions,
no `waitForTimeout`, trace on first retry).

## Layout

```
apps/web/
  playwright.config.ts      # projects, reporters, webServer (mock API + next dev)
  tsconfig.e2e.json         # scoped type-check (independent of the app's red baseline)
  e2e/
    global-setup.ts         # warm routes + fail fast if app/mock are down
    specs/                  # journeys only: auth, wallet, affiliate, smoke
    pages/                  # page objects (one selector per element)
    fixtures.ts             # extended test: backend stubs, auth proxy, unique users
    setup/auth.setup.ts     # one-time login → e2e/.auth/player.json
    README.md               # this file
```

## Backends in tests

All backend traffic is intercepted in `e2e/fixtures.ts` — the suite never
depends on the app's own `NEXT_PUBLIC_*` API bases:

- **Auth** — real HTTP, proxied through `page.route()` to
  `mock-api-server.js` (isolated port 18080 via `MOCK_PORT`, so dev servers
  on :3000/:3002/:3005 and any :8080 mock stay untouched). Real
  register/login/refresh/logout/me semantics, CORS headers injected.
- **Wallet / affiliate** — stubbed per-test with `page.route()`.
  Deterministic payloads, zero shared state. Spec routes are registered
  after the default layer, so they take priority.

### Assert against the base branch, not against a worktree

Specs must cover the UI that exists on the branch they are merged into.
An earlier version of `affiliate.spec.ts` asserted an enrollment CTA and a
payout-request form that only existed in someone's uncommitted working
copy. It passed locally and failed on CI, because CI checks out `main`.

The symptom is worth recognising: a spec that fails only in CI, on the
same commit, usually means it was written against unmerged code rather
than a flaky selector. Check what the base branch actually renders before
debugging the locator.

Payloads use sentinel values (`777.11`, `SENTINEL-ESP-01`, …) that appear
nowhere in the component's hardcoded fallbacks. Asserting the sentinels is
what proves the panel is API-driven rather than still showing its static
defaults.

## Run locally

```bash
cd apps/web
npm ci
npx playwright install chromium
npx playwright test            # full suite (starts isolated mock API :18080 + next dev :3100)
npx playwright test --project=chromium --headed   # debug one project
npx playwright show-report e2e/playwright-report   # open HTML report
```

Traces for failed tests: `trace: on-first-retry` (open in trace viewer).

## CI

`.github/workflows/e2e-web.yml`: `npm ci` → `tsc -p tsconfig.e2e.json`
(scoped E2E type-check, independent of the app's red baseline) →
chromium install → `playwright test` → HTML + JUnit artifacts.
Triggers only on `apps/web/**` changes.

## Hydration gate (why `waitForHydration` exists)

`next dev` serves HTML before the client chunks commit, and a fast
`fill` + `click` on a form can land before React binds `onSubmit`. The
browser then performs a **native GET submit** and serialises the
credentials into the URL (`/login?identifier=…&password=…`) — the journey
fails for a reason unrelated to the app logic.

`waitForHydration()` (in `fixtures.ts`) waits until React has committed
nodes under `__next` and, when the page has a form, until that form
carries a real `onSubmit` handler. It polls on an interval (not the
default `requestAnimationFrame`, which stalls in a backgrounded parallel
worker) and retries once with a reload. `e2e/global-setup.ts` warms every
route first so the cold-compile cost is paid before the suite, and fails
fast with a clear message if the app or the mock API is not up.

## Auth shape adapter (why responses are translated)

`mock-api-server.js` returns a flat token payload
(`{access_token, refresh_token, user}`), while `authApi.login/register`
expects `AuthResult` (`{user_id, tokens:{…}, session}`). The proxy layer
in `fixtures.ts` translates success bodies and leaves 4xx bodies
untouched — wrapping an error in the success envelope would strip its
`code`/`message` and the UI would fall back to the bare HTTP statusText.
Real backends (`services/go/auth`) already return `AuthResult`, so this is
strictly a mock-shape fix.

## Known findings (documented, out of scope)

1. `login/page.tsx`: `<label htmlFor="email">` is paired with
   `<input id="identifier">` — `getByLabel()` cannot resolve the field
   (spec uses `#identifier`). One-line a11y fix, чужой файл — не трогал.
2. `wallet.tsx` tab labels look windows-1251-encoded served as UTF-8
   (mojibake in UI). Wallet specs assert structure + the deposit form
   (clean strings) and skip tab names until the encoding is fixed.
3. `deposit-form.tsx` `handleSubmit` never calls any API (analytics event
   only) — locked by a spec so the future real deposit journey must
   extend it instead of silently passing.
4. Mock login error code is `INVALID_CREDENTIALS`, real backend uses
   `AUTH_INVALID_CREDENTIALS` — specs assert the status code (401), not
   the copy, so both backends pass.
5. `mock-api-server.js` answers duplicate registration with
   `400 USER_ALREADY_EXISTS`; the auth service uses `409`. The page
   checks the code, not the status, so the copy is identical — but a
   status-based assertion in a spec would be mock-specific.
