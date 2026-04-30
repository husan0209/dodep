// Load Test: 10 Million Users Simulation
// Opus Casino - Ultimate Load Test Scenario
// k6 script for extreme scale testing

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

// ===========================================================================
// Custom Metrics
// ===========================================================================

// Error rates
const loginErrorRate = new Rate('login_errors');
const betPlacementErrorRate = new Rate('bet_placement_errors');
const paymentErrorRate = new Rate('payment_errors');

// Latency metrics
const loginLatency = new Trend('login_latency');
const betPlacementLatency = new Trend('bet_placement_latency');
const oddsUpdateLatency = new Trend('odds_update_latency');

// Business metrics
const successfulBets = new Counter('successful_bets');
const successfulPayments = new Counter('successful_payments');
const activeUsers = new Counter('active_users');

// ===========================================================================
// Test Configuration - 10M Users Simulation
// ===========================================================================

export const options = {
  // Scenarios for different user behaviors
  scenarios: {
    // ========================================================================
    // Scenario 1: Browse Only (70% of users)
    // ========================================================================
    browse_only: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 70000 },   // Ramp to 70K users
        { duration: '30m', target: 70000 },  // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'browseOnly',
      tags: { user_type: 'browse' },
    },
    
    // ========================================================================
    // Scenario 2: Registered Users - Login + Browse (20% of users)
    // ========================================================================
    registered_browse: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 20000 },   // Ramp to 20K users
        { duration: '30m', target: 20000 },  // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'registeredBrowse',
      tags: { user_type: 'registered' },
    },
    
    // ========================================================================
    // Scenario 3: Active Bettors (8% of users)
    // ========================================================================
    active_bettors: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 8000 },    // Ramp to 8K users
        { duration: '30m', target: 8000 },   // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'activeBettors',
      tags: { user_type: 'bettor' },
    },
    
    // ========================================================================
    // Scenario 4: High Rollers - Heavy Betting (1.9% of users)
    // ========================================================================
    high_rollers: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 1900 },    // Ramp to 1.9K users
        { duration: '30m', target: 1900 },   // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'highRollers',
      tags: { user_type: 'high_roller' },
    },
    
    // ========================================================================
    // Scenario 5: Payment Processing (0.1% of users)
    // ========================================================================
    payment_processing: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 100 },     // Ramp to 100 concurrent payments
        { duration: '30m', target: 100 },    // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'paymentProcessing',
      tags: { user_type: 'payment' },
    },
    
    // ========================================================================
    // Scenario 6: WebSocket Connections (Real-time odds)
    // ========================================================================
    websocket_connections: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 50000 },   // 50K concurrent WS connections
        { duration: '30m', target: 50000 },  // Sustain
        { duration: '5m', target: 0 },       // Ramp down
      ],
      gracefulRampDown: '30s',
      exec: 'websocketConnections',
      tags: { user_type: 'websocket' },
    },
  },
  
  // Thresholds for pass/fail criteria
  thresholds: {
    // Overall error rates
    http_req_failed: ['rate<0.01'],  // < 1% errors
    loginErrorRate: ['rate<0.01'],
    betPlacementErrorRate: ['rate<0.01'],
    paymentErrorRate: ['rate<0.001'],  // < 0.1% for payments
    
    // Latency thresholds
    http_req_duration: [
      'p(50)<100',   // 50% under 100ms
      'p(90)<300',   // 90% under 300ms
      'p(95)<500',   // 95% under 500ms
      'p(99)<1000',  // 99% under 1000ms
    ],
    loginLatency: ['p(95)<500'],
    betPlacementLatency: ['p(95)<800'],
    oddsUpdateLatency: ['p(95)<100'],  // Real-time odds must be fast
    
    // Business metrics
    successfulBets: ['count>100000'],     // At least 100K bets
    successfulPayments: ['count>10000'],  // At least 10K payments
  },
  
  // Global settings
  maxRedirects: 3,
  userDefinedVariables: {
    BASE_URL: __ENV.BASE_URL || 'https://api.opus-casino.com',
    WS_URL: __ENV.WS_URL || 'wss://ws.opus-casino.com',
  },
  noConnectionReuse: false,
  insecureSkipTLSVerify: false,
};

// ===========================================================================
// Test Data
// ===========================================================================

// Simulated user credentials (in production, use proper test data management)
const testUsers = {
  browse: [],
  registered: Array.from({ length: 1000 }, (_, i) => ({
    email: `user${i}@test.com`,
    password: 'TestPassword123!',
  })),
  bettors: Array.from({ length: 500 }, (_, i) => ({
    email: `bettor${i}@test.com`,
    password: 'TestPassword123!',
    token: null,
  })),
  highRollers: Array.from({ length: 100 }, (_, i) => ({
    email: `highroller${i}@test.com`,
    password: 'TestPassword123!',
    token: null,
  })),
};

// Sample bet data
const sampleBets = [
  { stake: 10, odds: 1.5, selections: [{ eventId: 1, selectionId: 1 }] },
  { stake: 50, odds: 2.0, selections: [{ eventId: 2, selectionId: 2 }] },
  { stake: 100, odds: 3.5, selections: [{ eventId: 3, selectionId: 3 }] },
];

// ===========================================================================
// Helper Functions
// ===========================================================================

function getRandomElement(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function generateIdempotencyKey() {
  return `idemp_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

// ===========================================================================
// Scenario 1: Browse Only
// ===========================================================================

export function browseOnly() {
  const baseUrl = __ENV.BASE_URL;
  
  // View sports list
  const sportsResponse = http.get(`${baseUrl}/api/v1/sports`, {
    tags: { name: 'GetSports' },
  });
  
  check(sportsResponse, {
    'sports list loaded': (r) => r.status === 200,
  });
  
  sleep(1);
  
  // View events
  const eventsResponse = http.get(`${baseUrl}/api/v1/events?sport_id=1`, {
    tags: { name: 'GetEvents' },
  });
  
  check(eventsResponse, {
    'events loaded': (r) => r.status === 200,
  });
  
  sleep(1);
  
  // View odds
  const oddsResponse = http.get(`${baseUrl}/api/v1/odds?event_id=1`, {
    tags: { name: 'GetOdds' },
  });
  
  check(oddsResponse, {
    'odds loaded': (r) => r.status === 200,
  });
  
  sleep(2);
  
  activeUsers.add(1);
}

// ===========================================================================
// Scenario 2: Registered Users - Login + Browse
// ===========================================================================

export function registeredBrowse() {
  const baseUrl = __ENV.BASE_URL;
  const user = getRandomElement(testUsers.registered);
  
  // Login
  const loginStart = Date.now();
  const loginResponse = http.post(
    `${baseUrl}/api/v1/auth/login`,
    JSON.stringify({
      email: user.email,
      password: user.password,
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'Login' },
    }
  );
  
  loginLatency.add(Date.now() - loginStart);
  
  const loginSuccess = check(loginResponse, {
    'login successful': (r) => r.status === 200,
  });
  
  loginErrorRate.add(!loginSuccess);
  
  if (!loginSuccess) {
    return;
  }
  
  const token = loginResponse.json('access_token');
  
  // Browse with auth
  const profileResponse = http.get(
    `${baseUrl}/api/v1/user/profile`,
    {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
      tags: { name: 'GetProfile' },
    }
  );
  
  check(profileResponse, {
    'profile loaded': (r) => r.status === 200,
  });
  
  sleep(2);
  
  activeUsers.add(1);
}

// ===========================================================================
// Scenario 3: Active Bettors
// ===========================================================================

export function activeBettors() {
  const baseUrl = __ENV.BASE_URL;
  const user = getRandomElement(testUsers.bettors);
  
  // Login if no token
  if (!user.token) {
    const loginResponse = http.post(
      `${baseUrl}/api/v1/auth/login`,
      JSON.stringify({
        email: user.email,
        password: user.password,
      }),
      {
        headers: { 'Content-Type': 'application/json' },
      }
    );
    
    if (loginResponse.status === 200) {
      user.token = loginResponse.json('access_token');
    }
  }
  
  if (!user.token) {
    return;
  }
  
  // Get balance
  const balanceResponse = http.get(
    `${baseUrl}/api/v1/wallet/balance`,
    {
      headers: {
        'Authorization': `Bearer ${user.token}`,
      },
      tags: { name: 'GetBalance' },
    }
  );
  
  check(balanceResponse, {
    'balance loaded': (r) => r.status === 200,
  });
  
  sleep(1);
  
  // Place bet
  const betData = getRandomElement(sampleBets);
  const betStart = Date.now();
  
  const betResponse = http.post(
    `${baseUrl}/api/v1/bets/place`,
    JSON.stringify({
      ...betData,
      idempotency_key: generateIdempotencyKey(),
    }),
    {
      headers: {
        'Authorization': `Bearer ${user.token}`,
        'Content-Type': 'application/json',
      },
      tags: { name: 'PlaceBet' },
    }
  );
  
  betPlacementLatency.add(Date.now() - betStart);
  
  const betSuccess = check(betResponse, {
    'bet placed': (r) => r.status === 201,
  });
  
  betPlacementErrorRate.add(!betSuccess);
  
  if (betSuccess) {
    successfulBets.add(1);
  }
  
  sleep(3);
  
  activeUsers.add(1);
}

// ===========================================================================
// Scenario 4: High Rollers - Heavy Betting
// ===========================================================================

export function highRollers() {
  const baseUrl = __ENV.BASE_URL;
  const user = getRandomElement(testUsers.highRollers);
  
  // Login
  if (!user.token) {
    const loginResponse = http.post(
      `${baseUrl}/api/v1/auth/login`,
      JSON.stringify({
        email: user.email,
        password: user.password,
      }),
      {
        headers: { 'Content-Type': 'application/json' },
      }
    );
    
    if (loginResponse.status === 200) {
      user.token = loginResponse.json('access_token');
    }
  }
  
  if (!user.token) {
    return;
  }
  
  // Place multiple bets rapidly
  for (let i = 0; i < 5; i++) {
    const betData = {
      stake: 500 + Math.random() * 5000,  // High stakes
      odds: 1.5 + Math.random() * 3,
      selections: [{ eventId: Math.floor(Math.random() * 100), selectionId: Math.floor(Math.random() * 10) }],
      idempotency_key: generateIdempotencyKey(),
    };
    
    const betStart = Date.now();
    
    const betResponse = http.post(
      `${baseUrl}/api/v1/bets/place`,
      JSON.stringify(betData),
      {
        headers: {
          'Authorization': `Bearer ${user.token}`,
          'Content-Type': 'application/json',
        },
        tags: { name: 'HighRollerBet' },
      }
    );
    
    betPlacementLatency.add(Date.now() - betStart);
    
    const betSuccess = check(betResponse, {
      'high roller bet placed': (r) => r.status === 201,
    });
    
    betPlacementErrorRate.add(!betSuccess);
    
    if (betSuccess) {
      successfulBets.add(1);
    }
    
    sleep(0.5);  // Rapid betting
  }
  
  sleep(5);
  
  activeUsers.add(1);
}

// ===========================================================================
// Scenario 5: Payment Processing
// ===========================================================================

export function paymentProcessing() {
  const baseUrl = __ENV.BASE_URL;
  
  // Simulate deposit
  const depositAmounts = [10, 20, 50, 100, 200, 500];
  const amount = getRandomElement(depositAmounts);
  
  const paymentStart = Date.now();
  
  const paymentResponse = http.post(
    `${baseUrl}/api/v1/payment/deposit`,
    JSON.stringify({
      amount: amount,
      currency: 'USD',
      method: 'card',
      // In real test, use proper payment gateway test credentials
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer test_token`,
      },
      tags: { name: 'Deposit' },
    }
  );
  
  const paymentLatency = Date.now() - paymentStart;
  
  const paymentSuccess = check(paymentResponse, {
    'payment processed': (r) => r.status === 200 || r.status === 201,
  });
  
  paymentErrorRate.add(!paymentSuccess);
  
  if (paymentSuccess) {
    successfulPayments.add(1);
  }
  
  sleep(5);
  
  activeUsers.add(1);
}

// ===========================================================================
// Scenario 6: WebSocket Connections
// ===========================================================================

export function websocketConnections() {
  const wsUrl = __ENV.WS_URL;
  
  // Note: k6 has limited WebSocket support in open source version
  // For full WebSocket testing, use k6 extension or Grafana k6 Cloud
  
  // Simulate WS connection overhead
  const wsResponse = http.get(`${wsUrl.replace('wss://', 'https://')}/health`, {
    tags: { name: 'WebSocketHealth' },
  });
  
  check(wsResponse, {
    'ws endpoint reachable': (r) => r.status === 200,
  });
  
  sleep(1);
  
  activeUsers.add(1);
}

// ===========================================================================
// Handle Summary
// ===========================================================================

export function handleSummary(data) {
  return {
    'summary.json': JSON.stringify(data),
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}
