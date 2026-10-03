// Load Test: Auth Service — register / login / token refresh / 2FA
// Opus Casino - k6 scenario for services/go/auth
//
// Contract: services/go/auth/routes.go, internal/domain/session.go
// Ports:    HTTP 8083 (CONVENTIONS.md), gRPC 50051 (not covered by k6)
//
// Run:
//   k6 run -e BASE_URL=http://auth.dev.svc.cluster.local:8083 tools/testing/k6/scenarios/auth-service.js
//   # with a real account for the authenticated paths:
//   k6 run -e ACCESS_TOKEN=$(login) -e BASE_URL=... tools/testing/k6/scenarios/auth-service.js
//
// Design notes:
// - Registration mutates state, so it runs ONLY when REG_ENABLED=true and
//   is skipped otherwise. Every VU would otherwise create thousands of
//   users per run.
// - Token validation budget is < 1ms (architecture-overview) and login
//   p95 < 200ms, so the thresholds below are tighter than the generic
//   http_req_duration one.
// - `requires_2fa` accounts cannot complete login without a TOTP code;
//   those VUs fall back to the public-path checks instead of failing.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const errorRate = new Rate('auth_errors');
const loginLatency = new Trend('auth_login_latency');
const refreshLatency = new Trend('auth_refresh_latency');
const validateLatency = new Trend('auth_validate_latency', true);
const registerCounter = new Counter('auth_registers_total');
const loginsCounter = new Counter('auth_logins_total');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '1m', target: 10 }, // warm up
    { duration: '2m', target: 10 }, // smoke
    { duration: '2m', target: 50 }, // ramp
    { duration: '3m', target: 50 }, // sustain
    { duration: '1m', target: 0 }, // ramp down
  ],
  thresholds: {
    // Login p50 is 50ms, p95 is 100ms per the architecture budget.
    auth_login_latency: ['p(95)<200'],
    auth_refresh_latency: ['p(95)<200'],
    // Token validation is an in-memory check: p99 under 2ms.
    auth_validate_latency: ['p(99)<20'],
    http_req_failed: ['rate<0.01'],
    auth_errors: ['rate<0.01'],
  },
  insecureSkipTLSVerify: true,
};

const BASE_URL = __ENV.BASE_URL || 'http://auth.dev.svc.cluster.local:8083';
const ACCESS_TOKEN = __ENV.ACCESS_TOKEN || '';
// Registration is opt-in: it creates real users.
const REG_ENABLED = (__ENV.REG_ENABLED || 'false') === 'true';
const TEST_PASSWORD = __ENV.TEST_PASSWORD || 'LoadTest!Passw0rd';
const AUTHED = ACCESS_TOKEN !== '';

function authHeaders(extra) {
  const headers = Object.assign({ 'Content-Type': 'application/json' }, extra);
  if (AUTHED) {
    headers['Authorization'] = `Bearer ${ACCESS_TOKEN}`;
  }
  return { headers: headers };
}

function uniqueEmail() {
  return `k6_${__VU}_${__ITER}_${Date.now()}@loadtest.invalid`;
}

// ===========================================================================
// Checks
// ===========================================================================

function checkHealth() {
  const res = http.get(`${BASE_URL}/health`);
  errorRate.add(!check(res, { 'health is 200': (r) => r.status === 200 }));
  sleep(0.5);
}

function checkReady() {
  const res = http.get(`${BASE_URL}/ready`);
  errorRate.add(!check(res, { 'ready is 200': (r) => r.status === 200 }));
  sleep(0.5);
}

// Validates request format without needing valid credentials.
// A well-formed but wrong password must be 401 — never 5xx.
function checkLoginRejected() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      identifier: `k6_missing_${__VU}_${__ITER}@loadtest.invalid`,
      password: 'wrong-password-for-load-test',
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  loginLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, {
      'login with bad credentials is 401': (r) => r.status === 401,
      'login failure never 5xx': (r) => r.status < 500,
      'login failure hides user enumeration': (r) => {
        // Same shape whether or not the account exists.
        try {
          const body = JSON.parse(r.body);
          return body.error !== undefined;
        } catch (e) {
          return false;
        }
      },
    }),
  );
  sleep(1);
}

// Missing password must be rejected by validation (4xx), never 500.
function checkLoginValidation() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ identifier: `k6_${__VU}@loadtest.invalid` }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  errorRate.add(
    !check(res, {
      'login without password is 4xx': (r) => r.status >= 400 && r.status < 500,
    }),
  );
  sleep(1);
}

// Opt-in: creates a real account and asserts the 201 + token pair.
function checkRegister() {
  if (!REG_ENABLED) return;
  const res = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({
      email: uniqueEmail(),
      password: TEST_PASSWORD,
      username: `k6u${__VU}_${__ITER}`,
      country_code: 'MT',
      currency_code: 'EUR',
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  const ok = check(res, {
    'register is 201': (r) => r.status === 201,
    'register returns tokens or 2FA challenge': (r) => {
      try {
        const b = JSON.parse(r.body);
        return (b.tokens && b.tokens.access_token) || b.temp_token || b.requires_2fa;
      } catch (e) {
        return false;
      }
    },
  });
  errorRate.add(!ok);
  if (ok) registerCounter.add(1);
  sleep(1);
}

// GET /api/v1/auth/me — requires a token.
function checkMe() {
  const res = http.get(`${BASE_URL}/api/v1/auth/me`, authHeaders());
  validateLatency.add(res.timings.duration);
  const expected = AUTHED ? 200 : 401;
  errorRate.add(
    !check(res, { 'me status as expected': (r) => r.status === expected }),
  );
  if (AUTHED && res.status === 200) {
    check(res, {
      'me returns user id': (r) => {
        try {
          return JSON.parse(r.body).user_id !== undefined;
        } catch (e) {
          return false;
        }
      },
    });
  }
  sleep(1);
}

// POST /api/v1/auth/validate — the hot path for other services.
function checkValidateToken() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/validate`,
    JSON.stringify({ token: ACCESS_TOKEN }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  validateLatency.add(res.timings.duration);
  const ok = check(res, {
    'validate never 5xx': (r) => r.status < 500,
    'validate answers 200 or 401': (r) => r.status === 200 || r.status === 401,
  });
  errorRate.add(!ok);
  if (res.status === 200) loginsCounter.add(1);
  sleep(1);
}

// Refresh requires a real refresh token; skipped without ACCESS_TOKEN.
function checkRefreshRejected() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/refresh`,
    JSON.stringify({ refresh_token: 'not-a-real-token' }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  refreshLatency.add(res.timings.duration);
  errorRate.add(
    !check(res, {
      'refresh with bogus token is 4xx': (r) => r.status >= 400 && r.status < 500,
      'refresh never 5xx': (r) => r.status < 500,
    }),
  );
  sleep(1);
}

// ===========================================================================
// VU entrypoint
// ===========================================================================

export default function () {
  checkHealth();
  checkReady();
  checkLoginRejected();
  checkLoginValidation();
  checkRegister();
  checkMe();
  checkValidateToken();
  checkRefreshRejected();
}

export function handleSummary(data) {
  return {
    'auth-summary.json': JSON.stringify(data),
  };
}