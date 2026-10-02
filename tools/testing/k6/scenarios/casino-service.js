// Load Test: Casino Service — catalog reads and provider callbacks
// Opus Casino - k6 scenario for services/go/casino
//
// Contract: services/go/casino/main.go, internal/handlers/http_handler.go,
//           internal/handlers/provider_handler.go
// Ports:    HTTP 8086 (CONVENTIONS.md), gRPC 50057 (not covered).
//           The Prometheus listener (separate mux) is intentionally not probed
//           here - it is scraped by the platform, not by load tests.
//
// Run:
//   k6 run -e BASE_URL=http://casino.dev.svc.cluster.local:8086 tools/testing/k6/scenarios/casino-service.js
//   # catalog assertions need a seeded catalog:
//   k6 run -e BASE_URL=... -e CATALOG_SEEDED=true tools/testing/k6/scenarios/casino-service.js
//
// Design notes:
// - The lobby (GET /games, /providers) is the hottest public path: every
//   casino page load hits it, so it gets the tighter threshold.
// - Provider callbacks are unauthenticated by design (nginx IP allowlist +
//   HMAC). This scenario asserts they never 5xx and never leak on unsigned
//   input; it does NOT forge valid HMAC signatures, so it cannot credit
//   money. Mutating callbacks are never replayed with real amounts.
// - Launching a game creates a real session upstream, so it is opt-in.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const errorRate = new Rate('casino_errors');
const gamesLatency = new Trend('casino_games_latency');
const providersLatency = new Trend('casino_providers_latency');
const callbackLatency = new Trend('casino_callback_latency');
const launchCounter = new Counter('casino_launches_total');
const callbackCounter = new Counter('casino_callbacks_total');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '1m', target: 20 }, // warm up
    { duration: '2m', target: 20 }, // smoke
    { duration: '2m', target: 75 }, // ramp
    { duration: '3m', target: 75 }, // sustain
    { duration: '1m', target: 0 }, // ramp down
  ],
  thresholds: {
    // Lobby reads are cacheable and on the page-load path: p95 < 500ms.
    casino_games_latency: ['p(95)<500'],
    casino_providers_latency: ['p(95)<500'],
    // Provider callbacks are latency-sensitive for the provider side too.
    casino_callback_latency: ['p(95)<1000'],
    http_req_failed: ['rate<0.01'],
    casino_errors: ['rate<0.01'],
  },
  insecureSkipTLSVerify: true,
};

const BASE_URL = __ENV.BASE_URL || 'http://casino.dev.svc.cluster.local:8086';
const ACCESS_TOKEN = __ENV.ACCESS_TOKEN || '';
// Catalog assertions need seeded data; default off so the run does not
// depend on seed state.
const CATALOG_SEEDED = (__ENV.CATALOG_SEEDED || 'false') === 'true';
// Game launch creates a real upstream session: opt-in only.
const LAUNCH_ENABLED = (__ENV.LAUNCH_ENABLED || 'false') === 'true';
const AUTHED = ACCESS_TOKEN !== '';

function jsonHeaders() {
  return { headers: { 'Content-Type': 'application/json' } };
}

function authedHeaders(extra) {
  const headers = Object.assign({ 'Content-Type': 'application/json' }, extra);
  if (AUTHED) headers['Authorization'] = `Bearer ${ACCESS_TOKEN}`;
  return { headers: headers };
}

// ===========================================================================
// Checks
// ===========================================================================

function checkHealth() {
  const res = http.get(`${BASE_URL}/health`);
  errorRate.add(
    !check(res, {
      'health is 200': (r) => r.status === 200,
      'health names the service': (r) => {
        try {
          return JSON.parse(r.body).service === 'casino';
        } catch (e) {
          return false;
        }
      },
    }),
  );
  sleep(0.5);
}

function checkReady() {
  const res = http.get(`${BASE_URL}/ready`);
  errorRate.add(!check(res, { 'ready is 200': (r) => r.status === 200 }));
  sleep(0.5);
}

// GET /api/v1/casino/games — the lobby listing.
function checkGameList() {
  const res = http.get(
    `${BASE_URL}/api/v1/casino/games?limit=20&offset=0`,
    jsonHeaders(),
  );
  gamesLatency.add(res.timings.duration);
  const ok = check(res, {
    'games never 5xx': (r) => r.status < 500,
    'games returns 200': (r) => r.status === 200,
    'games payload has games and total': (r) => {
      try {
        const b = JSON.parse(r.body);
        return Array.isArray(b.games) && typeof b.total === 'number';
      } catch (e) {
        return false;
      }
    },
  });
  errorRate.add(!ok);
  if (ok && CATALOG_SEEDED) {
    const res2 = http.get(
      `${BASE_URL}/api/v1/casino/games?limit=20&offset=0`,
      jsonHeaders(),
    );
    check(res2, {
      'seeded catalog is not empty': (r) => {
        try {
          return JSON.parse(r.body).games.length > 0;
        } catch (e) {
          return false;
        }
      },
    });
  }
  sleep(1);
}

// Filter combinations must not break the query builder.
function checkGameFilters() {
  const filters = [
    'provider=pragmatic',
    'category=slots',
    'search=1000',
  ];
  const filter = filters[__ITER % filters.length];
  const res = http.get(
    `${BASE_URL}/api/v1/casino/games?limit=10&${filter}`,
    jsonHeaders(),
  );
  gamesLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, {
      'filtered games is 200': (r) => r.status === 200,
      'filtered games never 5xx': (r) => r.status < 500,
    }),
  );
  sleep(1);
}

// GET /api/v1/casino/games/:id — unknown id must be 404, not 500.
function checkUnknownGame() {
  const res = http.get(
    `${BASE_URL}/api/v1/casino/games/does-not-exist-${__VU}-${__ITER}`,
    jsonHeaders(),
  );
  gamesLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, {
      'unknown game is 404': (r) => r.status === 404,
      'unknown game never 5xx': (r) => r.status < 500,
    }),
  );
  sleep(1);
}

function checkProviders() {
  const res = http.get(`${BASE_URL}/api/v1/casino/providers`, jsonHeaders());
  providersLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, {
      'providers is 200': (r) => r.status === 200,
      'providers never 5xx': (r) => r.status < 500,
    }),
  );
  sleep(1);
}

// POST /api/v1/casino/games/launch — requires JWT, so 401 without a token.
function checkLaunchUnauthenticated() {
  const res = http.post(
    `${BASE_URL}/api/v1/casino/games/launch`,
    JSON.stringify({ game_id: 'k6-nonexistent', device_type: 'desktop' }),
    jsonHeaders(),
  );
  gamesLatency.add(res.timings.duration);
  const expected = AUTHED ? 422 : 401; // 422 for a rejected game_id
  const ok = check(res, {
    'launch rejects unauthenticated or unknown game': (r) => r.status === expected,
    'launch never 5xx': (r) => r.status < 500,
  });
  errorRate.add(!ok);
  if (AUTHED && LAUNCH_ENABLED && res.status === 200) {
    launchCounter.add(1);
  }
  sleep(1);
}

// GET /api/v1/casino/history
function checkHistory() {
  const res = http.get(
    `${BASE_URL}/api/v1/casino/history?limit=20&offset=0`,
    authedHeaders(),
  );
  gamesLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, { 'history never 5xx': (r) => r.status < 500 }),
  );
  sleep(1);
}

// Provider callbacks: no JWT, secured by IP allowlist + HMAC.
// Unsigned traffic must be rejected without a 5xx and without side effects.
function checkProviderCallbackUnsigned() {
  const paths = [
    ['pragmatic', '/pragmatic/balance', 'post'],
    ['pgsoft', '/pgsoft/balance', 'get'],
    ['amatic', '/amatic/getBalance', 'post'],
    ['amusnet', '/amusnet/balance', 'post'],
  ];
  const [provider, path, method] = paths[__ITER % paths.length];
  const res =
    method === 'get'
      ? http.get(`${BASE_URL}/api/v1/casino/providers${path}`, jsonHeaders())
      : http.post(
          `${BASE_URL}/api/v1/casino/providers${path}`,
          JSON.stringify({ amount: '0.00', currency: 'USD' }),
          jsonHeaders(),
        );
  callbackLatency.add(res.timings.duration);
  callbackCounter.add(1);
  const ok = check(res, {
    'unsigned callback never 5xx': (r) => r.status < 500,
    'unsigned callback is rejected': (r) => r.status === 401 || r.status === 400 || r.status === 403,
    'unsigned callback does not succeed': (r) => r.status !== 200,
  });
  errorRate.add(!ok);
  void provider;
  sleep(0.5);
}

// ===========================================================================
// VU entrypoint
// ===========================================================================

export default function () {
  checkHealth();
  checkReady();
  checkGameList();
  checkGameFilters();
  checkUnknownGame();
  checkProviders();
  checkLaunchUnauthenticated();
  checkHistory();
  checkProviderCallbackUnsigned();
}

export function handleSummary(data) {
  return {
    'casino-summary.json': JSON.stringify(data),
  };
}