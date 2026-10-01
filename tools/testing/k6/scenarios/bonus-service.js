// Load Test: Bonus Service
// Opus Casino - k6 scenario for services/go/bonus
//
// Routes:    services/go/bonus/routes.go
//            All /api/v1/bonuses/* endpoints are behind AuthMiddleware, so the
//            scenario logs in against auth-service and sends a Bearer token
//            (user_id is never taken from the body — CONVENTIONS NEVER-7).
// Ports:     CONVENTIONS local HTTP 8088 (chart container 8080 is the Helm
//            default only). Override with -e BASE_URL=... .
// Auth:      provide ACCESS_TOKEN, or LOGIN_IDENTIFIER + LOGIN_PASSWORD so the
//            scenario can call POST {AUTH_BASE_URL}/api/v1/auth/login.
// Writes:    activate/cancel are opt-in via -e ALLOW_WRITES=true and need a
//            dedicated test user, because they mutate bonus state.
// Rate limit: the partner API is throttled at 300 reads/min and 30 writes/min
//            per user. The default profile stays well below that; the optional
//            rate-limit scenario (STRESS_RATE_LIMIT=true) verifies 429 handling.
//
// Run:
//   k6 run tools/testing/k6/scenarios/bonus-service.js
//   k6 run -e BASE_URL=http://bonus:8088 -e ACCESS_TOKEN=eyJ... tools/testing/k6/scenarios/bonus-service.js
//   k6 run -e BASE_URL=http://bonus:8088 -e AUTH_BASE_URL=http://auth:8083 \
//          -e LOGIN_IDENTIFIER=loadtest@example.com -e LOGIN_PASSWORD=secret \
//          -e ALLOW_WRITES=true tools/testing/k6/scenarios/bonus-service.js
// Thresholds follow architecture budgets: partner API p95 < 500ms, errors < 1%.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const unexpectedErrorRate = new Rate('bonus_unexpected_errors');
const authErrorRate = new Rate('bonus_auth_errors');
const rateLimited = new Counter('bonus_rate_limited_responses');
const healthLatency = new Trend('bonus_health_latency');
const listLatency = new Trend('bonus_list_latency');
const detailLatency = new Trend('bonus_detail_latency');
const wageringLatency = new Trend('bonus_wagering_latency');
const writeLatency = new Trend('bonus_write_latency');
const loginLatency = new Trend('bonus_login_latency');

// ===========================================================================
// Test Configuration
// ===========================================================================

const ALLOW_WRITES = __ENV.ALLOW_WRITES === 'true';
const STRESS_RATE_LIMIT = __ENV.STRESS_RATE_LIMIT === 'true';

const scenarios = {
  // Default: partner cabinet reads (the 99% traffic shape).
  partner_reads: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '1m', target: 5 }, // warm up
      { duration: '3m', target: 5 }, // smoke
      { duration: '2m', target: 25 }, // light load ramp
      { duration: '3m', target: 25 }, // sustain
      { duration: '1m', target: 0 }, // ramp down
    ],
    exec: 'partnerReads',
    tags: { flow: 'partner_reads' },
  },
};

if (ALLOW_WRITES) {
  // State-changing calls are rare in production; keep them far below the
  // 30 writes/min per-user budget.
  scenarios.partner_writes = {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 2 },
      { duration: '1m', target: 2 },
      { duration: '30s', target: 0 },
    ],
    exec: 'partnerWrites',
    tags: { flow: 'partner_writes' },
  };
}

if (STRESS_RATE_LIMIT) {
  // Deliberately exhausts the read budget to prove the limiter answers 429
  // with the documented headers instead of letting traffic through.
  scenarios.rate_limit_burst = {
    executor: 'constant-vus',
    vus: 1,
    duration: '30s',
    exec: 'rateLimitBurst',
    tags: { flow: 'rate_limit_burst' },
  };
}

export const options = {
  scenarios: scenarios,
  thresholds: {
    // NOTE: no `http_req_failed` threshold on purpose — 401/404/409/429 below
    // are EXPECTED responses (auth gate, unknown bonus, state conflict,
    // rate limiting), not transport failures.
    http_req_duration: ['p(95)<500'],
    bonus_unexpected_errors: ['rate<0.01'],
    bonus_rate_limited_responses: ['count>0'],
  },
};

// ===========================================================================
// Configuration
// ===========================================================================

const BASE_URL = __ENV.BASE_URL || 'http://bonus.dev.svc.cluster.local:8088';
const AUTH_BASE_URL = __ENV.AUTH_BASE_URL || 'http://auth.dev.svc.cluster.local:8083';
const ACCESS_TOKEN = __ENV.ACCESS_TOKEN || '';
const LOGIN_IDENTIFIER = __ENV.LOGIN_IDENTIFIER || '';
const LOGIN_PASSWORD = __ENV.LOGIN_PASSWORD || '';
const TEST_BONUS_ID = __ENV.TEST_BONUS_ID || '';

// ===========================================================================
// Helpers
// ===========================================================================

let uuidCounter = 0;

// uuid4 builds a parseable v4-shaped UUID. The service validates
// X-Idempotency-Key with uuid.Parse, so a real UUID format is mandatory;
// a counter + timestamp is enough to make retries distinct per iteration.
function uuid4() {
  uuidCounter += 1;
  const hex = (n, len) => {
    let s = n.toString(16);
    while (s.length < len) {
      s = '0' + s;
    }
    return s.slice(-len);
  };
  // 8-4-4-4-12 with the v4 version nibble (4) and a valid variant nibble
  // (8/9/a/b) in the 4th group, so google/uuid accepts the value.
  const t = Date.now() * 1000 + uuidCounter;
  const variant = (8 + (uuidCounter % 4)).toString(16);
  return (
    hex(t, 8) + '-' + hex(uuidCounter, 4) + '-4' + hex(uuidCounter, 3) + '-' +
    variant + hex(uuidCounter, 3) + '-' + hex(t * 7, 12)
  );
}

function authHeaders(extra) {
  const headers = Object.assign(
    { 'Content-Type': 'application/json', Accept: 'application/json' },
    extra || {},
  );
  if (ACCESS_TOKEN) {
    headers.Authorization = `Bearer ${ACCESS_TOKEN}`;
  }
  return { headers: headers };
}

// resolveToken returns a usable access token, logging in on demand.
// Returns an empty string when no credentials are configured; read scenarios
// then assert 401 instead of silently passing.
function resolveToken() {
  if (ACCESS_TOKEN) {
    return ACCESS_TOKEN;
  }
  if (!LOGIN_IDENTIFIER || !LOGIN_PASSWORD) {
    return '';
  }
  const res = http.post(
    `${AUTH_BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ identifier: LOGIN_IDENTIFIER, password: LOGIN_PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  loginLatency.add(res.timings.duration);
  if (res.status !== 200) {
    return '';
  }
  let body = {};
  try {
    body = res.json();
  } catch (e) {
    return '';
  }
  return (body.tokens && body.tokens.access_token) || body.access_token || '';
}

function tokenOrSkip() {
  const token = resolveToken();
  return token;
}

// ===========================================================================
// Checks: auth gate
// ===========================================================================

function checkAuthGate() {
  // The partner API must reject anonymous traffic outright (NEVER-7: identity
  // comes from the token, never from the body).
  const res = http.get(`${BASE_URL}/api/v1/bonuses/`);
  authErrorRate.add(res.status === 200);
  const ok = check(res, {
    'anonymous request is 401': (r) => r.status === 401,
    'anonymous request is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
}

// ===========================================================================
// Checks: health
// ===========================================================================

function checkHealth() {
  const res = http.get(`${BASE_URL}/health`);
  healthLatency.add(res.timings.duration);
  const ok = check(res, { 'health is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(!ok);
}

function checkReady() {
  const res = http.get(`${BASE_URL}/ready`);
  const ok = check(res, { 'ready is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(!ok);
}

// ===========================================================================
// Checks: partner reads
// ===========================================================================

function checkList(token) {
  const res = http.get(`${BASE_URL}/api/v1/bonuses/?limit=20&offset=0`, authHeaders({ Authorization: `Bearer ${token}` }));
  listLatency.add(res.timings.duration);
  const ok = check(res, {
    'list is 200': (r) => r.status === 200,
    'list is not 5xx': (r) => r.status < 500,
    'list carries rate-limit headers': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
  });
  unexpectedErrorRate.add(!ok);
  sleep(1);
}

function checkActive(token) {
  const res = http.get(`${BASE_URL}/api/v1/bonuses/active`, authHeaders({ Authorization: `Bearer ${token}` }));
  const ok = check(res, {
    'active is 200': (r) => r.status === 200,
    'active is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
  sleep(0.5);
}

function checkWagering(token, bonusId) {
  const res = http.get(
    `${BASE_URL}/api/v1/bonuses/${bonusId}/wagering`,
    authHeaders({ Authorization: `Bearer ${token}` }),
  );
  wageringLatency.add(res.timings.duration);
  // 200 for an owned bonus, 404 for a foreign/unknown one — never 5xx.
  const ok = check(res, {
    'wagering is 200 or 404': (r) => r.status === 200 || r.status === 404,
    'wagering is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
  sleep(0.5);
}

function checkDetail(token, bonusId) {
  const res = http.get(
    `${BASE_URL}/api/v1/bonuses/${bonusId}`,
    authHeaders({ Authorization: `Bearer ${token}` }),
  );
  detailLatency.add(res.timings.duration);
  const ok = check(res, {
    'detail is 200 or 404': (r) => r.status === 200 || r.status === 404,
    'detail is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
  sleep(0.5);
}

// ===========================================================================
// Checks: writes (opt-in)
// ===========================================================================

function checkActivateWithoutIdempotencyKey(token, bonusId) {
  // Mutating endpoints must require X-Idempotency-Key (400 without it).
  const res = http.post(
    `${BASE_URL}/api/v1/bonuses/${bonusId}/activate`,
    null,
    { headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' } },
  );
  const ok = check(res, {
    'activate without idempotency key is 400': (r) => r.status === 400,
  });
  unexpectedErrorRate.add(!ok);
  sleep(0.5);
}

function checkActivate(token, bonusId) {
  const res = http.post(
    `${BASE_URL}/api/v1/bonuses/${bonusId}/activate`,
    null,
    authHeaders({
      Authorization: `Bearer ${token}`,
      'X-Idempotency-Key': uuid4(),
    }),
  );
  writeLatency.add(res.timings.duration);
  // 200 on success; 409 when the bonus is not pending; 404 unknown bonus.
  const ok = check(res, {
    'activate is 200/404/409': (r) => r.status === 200 || r.status === 404 || r.status === 409,
    'activate is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
  sleep(1);
}

function checkCancel(token, bonusId) {
  const res = http.post(
    `${BASE_URL}/api/v1/bonuses/${bonusId}/cancel`,
    null,
    authHeaders({
      Authorization: `Bearer ${token}`,
      'X-Idempotency-Key': uuid4(),
    }),
  );
  writeLatency.add(res.timings.duration);
  const ok = check(res, {
    'cancel is 200/404/409': (r) => r.status === 200 || r.status === 404 || r.status === 409,
    'cancel is not 5xx': (r) => r.status < 500,
  });
  unexpectedErrorRate.add(!ok);
  sleep(1);
}

// ===========================================================================
// VU entrypoints
// ===========================================================================

export function partnerReads() {
  checkHealth();
  checkReady();
  checkAuthGate();

  const token = tokenOrSkip();
  if (!token) {
    sleep(1);
    return;
  }
  checkList(token);
  checkActive(token);
  if (TEST_BONUS_ID) {
    checkDetail(token, TEST_BONUS_ID);
    checkWagering(token, TEST_BONUS_ID);
  }
}

export function partnerWrites() {
  if (!ALLOW_WRITES || !TEST_BONUS_ID) {
    return;
  }
  const token = tokenOrSkip();
  if (!token) {
    return;
  }
  checkActivateWithoutIdempotencyKey(token, TEST_BONUS_ID);
  checkActivate(token, TEST_BONUS_ID);
  checkCancel(token, TEST_BONUS_ID);
}

export function rateLimitBurst() {
  // No sleeps: exhaust the per-user read budget (300/min) on purpose.
  const token = tokenOrSkip();
  if (!token) {
    return;
  }
  const res = http.get(
    `${BASE_URL}/api/v1/bonuses/active`,
    authHeaders({ Authorization: `Bearer ${token}` }),
  );
  if (res.status === 429) {
    rateLimited.add(1);
  }
  const ok = check(res, {
    'burst response is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'throttled response carries Retry-After': (r) =>
      r.status !== 429 || r.headers['Retry-After'] !== undefined,
  });
  unexpectedErrorRate.add(!ok);
}

export function handleSummary(data) {
  return {
    'bonus-summary.json': JSON.stringify(data),
  };
}
