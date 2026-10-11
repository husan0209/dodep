// Load Test: Affiliate Service
// Covers public tracking + health + auth-gated cabinet endpoints.
// Service ports per CONVENTIONS.md: 8090 (HTTP) / 50061 (gRPC, not covered here).

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ===========================================================================
// Custom Metrics
// ===========================================================================

const unexpectedErrorRate = new Rate('affiliate_unexpected_errors');
const healthLatency = new Trend('affiliate_health_latency');
const trackingLatency = new Trend('affiliate_tracking_latency');

// ===========================================================================
// Test Configuration
// ===========================================================================

export const options = {
  stages: [
    { duration: '1m', target: 10 },   // Ramp up to 10 VUs
    { duration: '3m', target: 10 },   // Sustain
    { duration: '1m', target: 50 },   // Ramp up to 50 VUs (stress)
    { duration: '3m', target: 50 },   // Sustain
    { duration: '1m', target: 0 },    // Ramp down
  ],
  thresholds: {
    // NOTE: no `http_req_failed` threshold here on purpose — 401/302 below
    // are EXPECTED responses (auth gate / redirect tracking), not failures.
    http_req_duration: ['p(95)<500'],
    affiliate_unexpected_errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://affiliate.dev.svc.cluster.local:8090';
const TEST_CODE = __ENV.AFFILIATE_CODE || 'LOADTEST';

export default function () {
  // --- 1. Health endpoints (must be 200, no auth) ---
  let res = http.get(`${BASE_URL}/health`);
  healthLatency.add(res.timings.duration);
  check(res, { 'health is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(res.status !== 200);

  res = http.get(`${BASE_URL}/ready`);
  check(res, { 'ready is 200': (r) => r.status === 200 });
  unexpectedErrorRate.add(res.status !== 200);

  sleep(0.5);

  // --- 2. Click tracking (public, 302 redirect + aff_click cookie) ---
  res = http.get(`${BASE_URL}/r/${TEST_CODE}`, { redirects: 0 });
  trackingLatency.add(res.timings.duration);
  check(res, {
    'tracking is 302': (r) => r.status === 302,
    'tracking sets aff_click cookie': (r) =>
      (r.cookies['aff_click'] || []).length > 0,
  });
  unexpectedErrorRate.add(res.status !== 302);

  res = http.get(`${BASE_URL}/r/${TEST_CODE}/summer-promo`, { redirects: 0 });
  check(res, { 'campaign tracking is 302': (r) => r.status === 302 });
  unexpectedErrorRate.add(res.status !== 302);

  sleep(0.5);

  // --- 3. Cabinet endpoints without JWT (must be 401, fail-secure) ---
  const protectedPaths = [
    '/api/v1/affiliate/profile',
    '/api/v1/affiliate/dashboard',
    '/api/v1/affiliate/links',
    '/api/v1/affiliate/earnings',
    '/api/v1/affiliate/payouts',
  ];
  for (const path of protectedPaths) {
    res = http.get(`${BASE_URL}${path}`);
    check(res, { [`${path} requires auth (401)`]: (r) => r.status === 401 });
    unexpectedErrorRate.add(res.status !== 401);
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    'summary.json': JSON.stringify(data),
  };
}
