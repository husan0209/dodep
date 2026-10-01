// Load Test: User Service smoke + light load
// Opus Casino - k6 scenario for services/go/user
//
// Routes:    services/go/user/routes.go (no auth middleware; user_id from path)
// Ports:     chart container 8080, CONVENTIONS local HTTP 8085, code default 8082.
//            Override with -e BASE_URL=... for the target environment.
// NOTE:      mutating checks run only with -e ALLOW_WRITES=true against a
//            dedicated TEST_USER_ID. Reads are safe by default.
//
// Run:
//   k6 run tools/testing/k6/scenarios/user-service.js
//   k6 run -e BASE_URL=http://user:8080 -e TEST_USER_ID=1 -e ALLOW_WRITES=true tools/testing/k6/scenarios/user-service.js
// Thresholds follow architecture budgets: user profile p95 < 500ms, errors < 1%.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const errorRate = new Rate('user_errors');
const profileLatency = new Trend('user_profile_latency');
const prefsLatency = new Trend('user_prefs_latency');
const limitsLatency = new Trend('user_limits_latency');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '1m', target: 10 }, // warm up
    { duration: '3m', target: 10 }, // smoke at 10 VUs
    { duration: '2m', target: 50 }, // light load ramp
    { duration: '3m', target: 50 }, // sustain light load
    { duration: '1m', target: 0 }, // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // arch budget: user profile p95 < 500ms in CI
    http_req_failed: ['rate<0.01'], // error rate < 1%
    user_errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://user.dev.svc.cluster.local:8080';
const TEST_USER_ID = __ENV.TEST_USER_ID || '1';
const ALLOW_WRITES = __ENV.ALLOW_WRITES === 'true';

function jsonHeaders() {
  return { headers: { 'Content-Type': 'application/json' } };
}

// ===========================================================================
// Checks
// ===========================================================================

function checkHealth() {
  const res = http.get(`${BASE_URL}/health`);
  const ok = check(res, {
    'health status is 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkReady() {
  const res = http.get(`${BASE_URL}/ready`);
  const ok = check(res, {
    'ready status is 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkGetUser() {
  const res = http.get(`${BASE_URL}/api/v1/users/${TEST_USER_ID}`);
  profileLatency.add(res.timings.duration);
  // 200 when the test user exists, 404 when it does not. Never 5xx.
  const ok = check(res, {
    'get user status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    'get user is not 5xx': (r) => r.status < 500,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkGetPreferences() {
  const res = http.get(`${BASE_URL}/api/v1/users/${TEST_USER_ID}/preferences`);
  prefsLatency.add(res.timings.duration);
  const ok = check(res, {
    'get preferences status is 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkGetLimits() {
  const res = http.get(`${BASE_URL}/api/v1/users/${TEST_USER_ID}/limits`);
  limitsLatency.add(res.timings.duration);
  const ok = check(res, {
    'get limits status is 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkUpdateValidation() {
  // Malformed JSON must fail with 400, never 500.
  const res = http.put(
    `${BASE_URL}/api/v1/users/${TEST_USER_ID}`,
    '{broken',
    jsonHeaders(),
  );
  const ok = check(res, {
    'update bad body is 400': (r) => r.status === 400,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkUpdatePreferencesWrite() {
  if (!ALLOW_WRITES) {
    return; // mutating endpoint exercised only on explicit opt-in
  }
  const res = http.put(
    `${BASE_URL}/api/v1/users/${TEST_USER_ID}/preferences`,
    JSON.stringify({ language: 'en', timezone: 'UTC' }),
    jsonHeaders(),
  );
  const ok = check(res, {
    'update preferences status is 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
  sleep(1);
}

// ===========================================================================
// VU entrypoint
// ===========================================================================

export default function () {
  checkHealth();
  checkReady();
  checkGetUser();
  checkGetPreferences();
  checkGetLimits();
  checkUpdateValidation();
  checkUpdatePreferencesWrite();
}

export function handleSummary(data) {
  return {
    'user-summary.json': JSON.stringify(data),
  };
}
