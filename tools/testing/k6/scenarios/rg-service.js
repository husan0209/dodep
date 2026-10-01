// Load Test: RG Service enforcement gate + control plane
// Opus Casino - k6 scenario for services/go/rg
//
// Contract: docs/api/rg.md
// Ports:    HTTP 8091 (services/go/rg/internal/config/config.go), gRPC 50062 (not covered by k6)
//
// Run:
//   k6 run -e BASE_URL=http://rg.dev.svc.cluster.local:8091 tools/testing/k6/scenarios/rg-service.js
//   k6 run -e ACCESS_TOKEN=$(login) -e BASE_URL=... tools/testing/k6/scenarios/rg-service.js
//
// SAFETY: mutating RG endpoints that lock an account (self-exclusion,
// timeout, real limit increases) are NEVER exercised here. This scenario only
// reads plus the stateless gate. A denial (403) is a CORRECT decision, so it
// is tracked in rg_denials, not in rg_errors.
//
// Thresholds follow architecture budgets: the gate sits on the critical path
// of every bet/game launch/deposit, so p(95) < 500ms, errors < 1%.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const errorRate = new Rate('rg_errors'); // unexpected responses only
const denialRate = new Rate('rg_denials'); // correct "not allowed" decisions
const gateLatency = new Trend('rg_gate_latency', true);
const limitsLatency = new Trend('rg_limits_latency', true);
const statusLatency = new Trend('rg_status_latency', true);
const deniedCounter = new Counter('rg_denied_total');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '1m', target: 20 }, // warm up
    { duration: '3m', target: 20 }, // smoke at 20 VUs
    { duration: '2m', target: 100 }, // ramp (bet/deposit gate traffic)
    { duration: '3m', target: 100 }, // sustain
    { duration: '1m', target: 0 }, // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // arch budget: critical-path p95 < 500ms
    http_req_failed: ['rate<0.01'], // transport error rate < 1%
    rg_errors: ['rate<0.01'], // no unexpected responses
    rg_gate_latency: ['p(95)<200', 'p(99)<500'], // gate is the hot path
  },
  // A denied bet is expected under load; do not let it fail the run.
  insecureSkipTLSVerify: true,
};

const BASE_URL = __ENV.BASE_URL || 'http://rg.dev.svc.cluster.local:8091';
// Real access token via -e ACCESS_TOKEN=$(login). Empty exercises 401 paths.
const ACCESS_TOKEN = __ENV.ACCESS_TOKEN || '';
const AUTHED = ACCESS_TOKEN !== '';

const CHANNELS = ['bet', 'game_launch', 'deposit', 'withdrawal'];
// Amounts are decimal strings end-to-end (never float) - mirrors the API.
const AMOUNTS = ['10.00', '25.50', '100.00', '499.99'];

function authHeaders(extra) {
  const headers = Object.assign({ 'Content-Type': 'application/json' }, extra);
  if (AUTHED) {
    headers['Authorization'] = `Bearer ${ACCESS_TOKEN}`;
  }
  return { headers: headers };
}

// ===========================================================================
// Checks
// ===========================================================================

function checkHealth() {
  const res = http.get(`${BASE_URL}/health`);
  errorRate.add(!check(res, { 'health is 200': (r) => r.status === 200 }));
  sleep(1);
}

function checkReady() {
  const res = http.get(`${BASE_URL}/ready`);
  errorRate.add(!check(res, { 'ready is 200': (r) => r.status === 200 }));
  sleep(1);
}

// The enforcement gate. RULE 4: must decide, never 5xx, never crash.
function checkGate() {
  const channel = CHANNELS[__ITER % CHANNELS.length];
  const payload = JSON.stringify({
    channel: channel,
    amount: AMOUNTS[__ITER % AMOUNTS.length],
    currency: 'USD',
  });
  const res = http.post(`${BASE_URL}/api/v1/rg/check`, payload, authHeaders());
  gateLatency.add(res.timings.duration);

  if (!AUTHED) {
    errorRate.add(
      !check(res, { 'gate without token is 401': (r) => r.status === 401 }),
    );
    sleep(1);
    return;
  }

  const allowed = res.status === 200;
  const denied = res.status === 403;
  const ok = check(res, {
    'gate decides without 5xx': (r) => r.status < 500,
    'gate returns 200 or 403': () => allowed || denied,
    'denial carries reason code': (r) => {
      if (!denied) return true;
      try {
        const body = JSON.parse(r.body);
        const reason = (body.data && body.data.reason) || body.reason;
        return typeof reason === 'string' && reason.length > 0;
      } catch (e) {
        return false;
      }
    },
  });
  errorRate.add(!ok);
  if (denied) {
    denialRate.add(true);
    deniedCounter.add(1);
  } else {
    denialRate.add(false);
  }
  sleep(1);
}

// Withdrawal must NEVER be blocked by RG (regulatory requirement).
function checkWithdrawalNeverBlocked() {
  const payload = JSON.stringify({ channel: 'withdrawal', amount: '500.00', currency: 'USD' });
  const res = http.post(`${BASE_URL}/api/v1/rg/check`, payload, authHeaders());
  gateLatency.add(res.timings.duration);
  if (!AUTHED) {
    errorRate.add(!check(res, { 'withdrawal gate without token is 401': (r) => r.status === 401 }));
    sleep(1);
    return;
  }
  const blocked = res.status === 403;
  errorRate.add(
    !check(res, {
      'withdrawal gate decides without 5xx': (r) => r.status < 500,
      'withdrawal is never RG-blocked': () => !blocked,
    }),
  );
  sleep(1);
}

function checkInvalidAmountRejected() {
  if (!AUTHED) return;
  const res = http.post(
    `${BASE_URL}/api/v1/rg/check`,
    JSON.stringify({ channel: 'deposit', amount: 'not-a-number', currency: 'USD' }),
    authHeaders(),
  );
  errorRate.add(
    !check(res, {
      'invalid amount is 4xx': (r) => r.status >= 400 && r.status < 500,
    }),
  );
  sleep(1);
}

function checkInvalidChannelRejected() {
  if (!AUTHED) return;
  const res = http.post(
    `${BASE_URL}/api/v1/rg/check`,
    JSON.stringify({ channel: 'withdraw_via_crypto' }),
    authHeaders(),
  );
  errorRate.add(
    !check(res, { 'unknown channel is 4xx': (r) => r.status >= 400 && r.status < 500 }),
  );
  sleep(1);
}

function checkGetLimits() {
  const res = http.get(`${BASE_URL}/api/v1/rg/limits`, authHeaders());
  limitsLatency.add(res.timings.duration);
  const expected = AUTHED ? 200 : 401;
  errorRate.add(
    !check(res, { 'limits status as expected': (r) => r.status === expected }),
  );
  if (AUTHED && res.status === 200) {
    check(res, {
      'limits payload has limits and pending': (r) => {
        try {
          const body = JSON.parse(r.body);
          const data = body.data || body;
          return data.limits !== undefined && Array.isArray(data.pending);
        } catch (e) {
          return false;
        }
      },
    });
  }
  sleep(1);
}

function checkGetStatus() {
  const res = http.get(`${BASE_URL}/api/v1/rg/status`, authHeaders());
  statusLatency.add(res.timings.duration);
  const expected = AUTHED ? 200 : 401;
  errorRate.add(
    !check(res, { 'status status as expected': (r) => r.status === expected }),
  );
  if (AUTHED && res.status === 200) {
    check(res, {
      'status reports gambling_allowed': (r) => {
        try {
          const body = JSON.parse(r.body);
          const data = body.data || body;
          return typeof data.gambling_allowed === 'boolean';
        } catch (e) {
          return false;
        }
      },
    });
  }
  sleep(1);
}

// ===========================================================================
// VU entrypoint
// ===========================================================================

export default function () {
  checkHealth();
  checkReady();
  checkGate();
  checkWithdrawalNeverBlocked();
  checkInvalidAmountRejected();
  checkInvalidChannelRejected();
  checkGetLimits();
  checkGetStatus();
}

export function handleSummary(data) {
  return {
    'rg-summary.json': JSON.stringify(data),
  };
}