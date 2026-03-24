# k6 load testing script template

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 100 },   // Ramp up to 100 users
    { duration: '1m', target: 100 },    // Stay at 100 users
    { duration: '30s', target: 500 },   // Ramp up to 500 users
    { duration: '2m', target: 500 },    // Stay at 500 users
    { duration: '30s', target: 1000 },  // Ramp up to 1000 users
    { duration: '3m', target: 1000 },   // Stay at 1000 users
    { duration: '30s', target: 0 },     // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],   // 95% of requests should be below 100ms
    http_req_failed: ['rate<0.01'],     // Error rate should be below 1%
    errors: ['rate<0.01'],              // Custom error rate below 1%
  },
};

const BASE_URL = 'http://localhost:8080';

export default function () {
  // Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health status is 200': (r) => r.status === 200,
  });
  errorRate.add(healthRes.status !== 200);

  sleep(1);

  // Login (with test credentials)
  const loginPayload = JSON.stringify({
    email: 'test@example.com',
    password: 'testpassword',
  });

  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(loginRes, {
    'login status is 200': (r) => r.status === 200,
  });
  errorRate.add(loginRes.status !== 200);

  sleep(1);
}
