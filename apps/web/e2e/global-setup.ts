import type { FullConfig } from '@playwright/test';

/**
 * Global setup — warms the dev server before any test runs.
 *
 * Why this exists: with `next dev`, the first request to a route triggers
 * on-demand compilation (2-5s for ~700 modules). A cold first page load
 * therefore serves HTML whose client chunks are still being built, and the
 * hydration gate in waitForHydration() can miss the commit window on a
 * loaded CI machine. Warming every route up front moves that cost into
 * setup, where a failure is unambiguous.
 *
 * Also fails fast with a clear message if the app or the mock API is not
 * reachable, instead of surfacing 20 parallel "waiting for hydration"
 * timeouts.
 */
const WARM_ROUTES = [
  '/',
  '/login',
  '/register',
  '/sportsbook',
  '/casino',
  '/wallet',
  '/affiliate',
];

export default async function globalSetup(config: FullConfig) {
  const baseURL =
    process.env.E2E_BASE_URL ??
    (config.projects.find((p) => p.name !== 'setup')?.use?.baseURL as string | undefined) ??
    'http://localhost:3100';
  const apiURL = process.env.E2E_API_URL ?? 'http://localhost:18080';

  // 1. Mock API must be up (auth is proxied to it).
  const apiHealth = await fetch(`${apiURL}/health`).catch(() => null);
  if (!apiHealth?.ok) {
    throw new Error(
      `E2E global setup: mock API is not reachable at ${apiURL}. ` +
        'It is started by playwright.config webServer — check the [WebServer] logs above.',
    );
  }

  // 2. Warm every route so the dev server compiles them before the suite.
  for (const route of WARM_ROUTES) {
    const res = await fetch(`${baseURL}${route}`).catch(() => null);
    const status = res ? String(res.status) : 'no response';
    if (!res?.ok) {
      throw new Error(
        `E2E global setup: ${baseURL}${route} returned ${status}. ` +
          'The Next.js dev server did not start correctly.',
      );
    }
  }
}
