// Smoke Test: Analytics Service (read-only reporting API).
// Service: internal ClusterIP analytics:8000 (no public ingress by default).
// Run inside the cluster or via port-forward:
//   k6 run -e BASE_URL=http://localhost:8000 tools/testing/k6/scenarios/analytics.js
//
// Expectations:
//   - /health, /ready      -> 200 (no dependencies)
//   - reporting endpoints   -> 200 with ClickHouse, 503 without it.
//     503 is EXPECTED degradation (never fake data), not a failure.
//   - invalid params        -> 422 (validation works)

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const unexpectedErrorRate = new Rate('analytics_unexpected_errors');
const reportingLatency = new Trend('analytics_reporting_latency');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '30s', target: 5 },  // Ramp up to 5 VUs (smoke)
    { duration: '2m', target: 5 },   // Sustain
    { duration: '30s', target: 0 },  // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    analytics_unexpected_errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://analytics.dev.svc.cluster.local:8000';
// Last 7 days in YYYY-MM-DD.
const TO = new Date().toISOString().slice(0, 10);
const FROM = new Date(Date.now() - 6 * 86400000).toISOString().slice(0, 10);
const AFFILIATE_ID = __ENV.AFFILIATE_ID || 'smoke-test-affiliate';

function isExpectedStatus(status) {
  // 200 = served, 503 = ClickHouse unreachable (graceful degradation),
  // 422 = validation (only for the invalid-params probe below).
  return status === 200 || status === 503;
}

export default function () {
  // --- 1. Health endpoints (must be 200, no auth, no dependencies) ---
  let res = http.get(`${BASE_URL}/health`);
  check(res, { 'health is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(res.status !== 200);

  res = http.get(`${BASE_URL}/ready`);
  check(res, { 'ready is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(res.status !== 200);

  sleep(0.5);

  // --- 2. Reporting endpoints (200 or graceful 503) ---
  const urls = [
    `/api/v1/analytics/affiliate/${AFFILIATE_ID}/summary?date_from=${FROM}&date_to=${TO}`,
    `/api/v1/analytics/affiliate/${AFFILIATE_ID}/funnel?days=7`,
    `/api/v1/analytics/affiliate/${AFFILIATE_ID}/ngr-trend?days=7`,
    `/api/v1/analytics/affiliate/${AFFILIATE_ID}/ltv?limit=10`,
    `/api/v1/analytics/affiliates/top?days=7&limit=10`,
    `/api/v1/analytics/payouts/aging?limit=10`,
    `/api/v1/analytics/reports?date_from=${FROM}&date_to=${TO}&limit=10`,
    `/api/v1/analytics/dashboard?date_from=${FROM}&date_to=${TO}`,
  ];
  for (const path of urls) {
    res = http.get(`${BASE_URL}${path}`);
    reportingLatency.add(res.timings.duration);
    check(res, { [`GET ${path} is 200-or-503`]: (r) => isExpectedStatus(r.status) });
    unexpectedErrorRate.add(!isExpectedStatus(res.status));
    if (res.status === 200) {
      check(res, {
        [`GET ${path} returns data envelope`]: (r) => {
          try {
            const body = JSON.parse(r.body);
            return body && body.data !== undefined;
          } catch (e) {
            return false;
          }
        },
      });
    }
    sleep(0.2);
  }

  // --- 3. Validation probe (reversed range must be 422, never 500) ---
  res = http.get(
    `${BASE_URL}/api/v1/analytics/affiliate/${AFFILIATE_ID}/summary?date_from=${TO}&date_to=${FROM}`
  );
  check(res, { 'reversed range is 422': (r) => r.status === 422 });
  unexpectedErrorRate.add(res.status !== 422);

  // --- 4. Export allowlist probe (non-analytics table must be 422) ---
  res = http.get(
    `${BASE_URL}/api/v1/analytics/export?table=users&date_from=${FROM}&date_to=${TO}`
  );
  check(res, { 'export of users table is 422': (r) => r.status === 422 });
  unexpectedErrorRate.add(res.status !== 422);

  sleep(0.5);
}
