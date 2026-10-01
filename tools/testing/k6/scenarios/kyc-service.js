// Load Test: KYC Service smoke + light load
// Opus Casino - k6 scenario for services/go/kyc
//
// Contract: docs/api/kyc.md
// Ports:    HTTP 8089 (CONVENTIONS.md), gRPC 50059 (not covered by k6)
//
// Run:
//   k6 run -e BASE_URL=http://kyc.dev.svc.cluster.local:8089 tools/testing/k6/scenarios/kyc-service.js
// Thresholds follow architecture budgets: KYC endpoints p95 < 1s, errors < 1%.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const errorRate = new Rate('kyc_errors');
const statusLatency = new Trend('kyc_status_latency');
const submitLatency = new Trend('kyc_submit_latency');

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
    http_req_duration: ['p(95)<1000'], // arch budget: KYC p95 < 1s
    http_req_failed: ['rate<0.01'], // error rate < 1%
    kyc_errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://kyc.dev.svc.cluster.local:8089';
// Test JWT: pass a real access token via -e ACCESS_TOKEN=$(login)
// for authenticated checks; empty value exercises 401 paths.
const ACCESS_TOKEN = __ENV.ACCESS_TOKEN || '';

function authHeaders(extra) {
  const headers = Object.assign({ 'Content-Type': 'application/json' }, extra);
  if (ACCESS_TOKEN !== '') {
    headers['Authorization'] = `Bearer ${ACCESS_TOKEN}`;
  }
  return { headers: headers };
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

function checkStatus() {
  const res = http.get(`${BASE_URL}/api/v1/kyc/status`, authHeaders());
  statusLatency.add(res.timings.duration);
  if (ACCESS_TOKEN === '') {
    // No token: must be rejected, never 5xx.
    const ok = check(res, {
      'status without token is 401': (r) => r.status === 401,
    });
    errorRate.add(!ok);
  } else {
    const ok = check(res, {
      'status with token is 200': (r) => r.status === 200,
      'status has level': (r) => {
        try {
          const body = JSON.parse(r.body);
          const data = body.data || body;
          return data.level !== undefined || data.status !== undefined;
        } catch (e) {
          return false;
        }
      },
    });
    errorRate.add(!ok);
  }
  sleep(1);
}

function checkListDocuments() {
  const res = http.get(
    `${BASE_URL}/api/v1/kyc/documents?page_size=20`,
    authHeaders(),
  );
  const expected = ACCESS_TOKEN === '' ? 401 : 200;
  const ok = check(res, {
    'list documents status as expected': (r) => r.status === expected,
  });
  errorRate.add(!ok);
  sleep(1);
}

function checkSubmitValidation() {
  if (ACCESS_TOKEN === '') {
    return; // mutating endpoint exercised only with a real token
  }
  // Empty body must fail validation (400/422), never 500.
  const idemKey = `00000000-0000-4000-8000-${String(__VU * 100000 + __ITER).padStart(12, '0')}`;
  const res = http.post(
    `${BASE_URL}/api/v1/kyc/documents`,
    JSON.stringify({}),
    authHeaders({ 'X-Idempotency-Key': idemKey }),
  );
  submitLatency.add(res.timings.duration);
  const ok = check(res, {
    'submit empty body is 4xx': (r) => r.status >= 400 && r.status < 500,
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
  checkStatus();
  checkListDocuments();
  checkSubmitValidation();
}

export function handleSummary(data) {
  return {
    'kyc-summary.json': JSON.stringify(data),
  };
}
