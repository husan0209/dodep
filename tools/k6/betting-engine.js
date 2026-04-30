import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// ============================================================
// Custom Metrics
// ============================================================
const errorRate = new Rate("errors");
const placeBetDuration = new Trend("place_bet_duration", true);
const getBetDuration = new Trend("get_bet_duration", true);

// ============================================================
// Configuration
// ============================================================
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
  stages: [
    { duration: "30s", target: 100 }, // Ramp up to 100 users
    { duration: "1m", target: 500 }, // Ramp up to 500 users
    { duration: "2m", target: 1000 }, // Sustain 1000 users
    { duration: "1m", target: 500 }, // Ramp down
    { duration: "30s", target: 0 }, // Cool down
  ],
  thresholds: {
    http_req_duration: ["p(95)<50", "p(99)<100"], // p99 < 100ms
    errors: ["rate<0.01"], // < 1% error rate
    place_bet_duration: ["p(99)<100"], // p99 < 100ms
  },
};

// ============================================================
// Helper: generate unique user ID per VU
// ============================================================
function getUserId() {
  return __VU * 10000 + __ITER;
}

function generateIdempotencyKey() {
  return `${__VU}-${__ITER}-${Date.now()}`;
}

// ============================================================
// Scenario: Place Bet (heavy load)
// ============================================================
export function placeBet() {
  const userId = getUserId();
  const key = generateIdempotencyKey();

  const payload = JSON.stringify({
    bet_type: "single",
    selections: [
      {
        event_id: 100,
        market_id: 1,
        outcome_id: 1,
        odds: "2.50",
      },
    ],
    stake: "10.00",
    currency_code: "USD",
    idempotency_key: key,
  });

  const params = {
    headers: { "Content-Type": "application/json" },
  };

  const res = http.post(
    `${BASE_URL}/api/v1/users/${userId}/bets`,
    payload,
    params,
  );

  placeBetDuration.add(res.timings.duration);

  const success = check(res, {
    "place bet status 201": (r) => r.status === 201,
    "bet has id": (r) => {
      try {
        return JSON.parse(r.body).bet_id > 0;
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!success);
}

// ============================================================
// Scenario: Get Bet
// ============================================================
export function getBet() {
  const userId = getUserId();

  // First place a bet
  const placeRes = http.post(
    `${BASE_URL}/api/v1/users/${userId}/bets`,
    JSON.stringify({
      bet_type: "single",
      selections: [
        { event_id: 100, market_id: 1, outcome_id: 1, odds: "2.50" },
      ],
      stake: "5.00",
      currency_code: "USD",
      idempotency_key: generateIdempotencyKey(),
    }),
    { headers: { "Content-Type": "application/json" } },
  );

  if (placeRes.status !== 201) {
    errorRate.add(true);
    return;
  }

  const betId = JSON.parse(placeRes.body).bet_id;

  // Get the bet
  const getRes = http.get(`${BASE_URL}/api/v1/users/${userId}/bets/${betId}`);

  getBetDuration.add(getRes.timings.duration);

  const success = check(getRes, {
    "get bet status 200": (r) => r.status === 200,
    "correct bet id": (r) => {
      try {
        return JSON.parse(r.body).bet_id === betId;
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!success);
}

// ============================================================
// Scenario: Bet History
// ============================================================
export function getHistory() {
  const userId = getUserId();

  const res = http.get(`${BASE_URL}/api/v1/users/${userId}/bets?limit=20`);

  const success = check(res, {
    "history status 200": (r) => r.status === 200,
    "has data array": (r) => {
      try {
        return Array.isArray(JSON.parse(r.body).data);
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!success);
}

// ============================================================
// Scenario: Health Check
// ============================================================
export function healthCheck() {
  const res = http.get(`${BASE_URL}/healthz`);

  check(res, {
    "health 200": (r) => r.status === 200,
  });
}

// ============================================================
// Default scenario (mixed load)
// ============================================================
export default function () {
  const scenario = Math.random();

  if (scenario < 0.5) {
    placeBet();
  } else if (scenario < 0.8) {
    getHistory();
  } else {
    getBet();
  }

  sleep(0.1); // 100ms between iterations
}
