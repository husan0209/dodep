import http from "k6/http";
import { check } from "k6";
import { Rate } from "k6/metrics";

// ============================================================
// Settlement Load Test
// Tests the settlement pipeline under heavy load
// ============================================================

const errorRate = new Rate("errors");
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
  stages: [
    { duration: "30s", target: 50 },
    { duration: "2m", target: 200 },
    { duration: "1m", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(99)<200"],
    errors: ["rate<0.01"],
  },
};

function placeBet(userId) {
  const res = http.post(
    `${BASE_URL}/api/v1/users/${userId}/bets`,
    JSON.stringify({
      bet_type: "single",
      selections: [
        { event_id: 100, market_id: 1, outcome_id: 1, odds: "2.50" },
      ],
      stake: "10.00",
      currency_code: "USD",
      idempotency_key: `settle-test-${userId}-${Date.now()}`,
    }),
    { headers: { "Content-Type": "application/json" } },
  );

  if (res.status === 201) {
    return JSON.parse(res.body).bet_id;
  }
  return null;
}

export default function () {
  const userId = __VU * 1000 + __ITER;

  // Place bet
  const betId = placeBet(userId);
  if (!betId) {
    errorRate.add(true);
    return;
  }

  // Settle bet
  const result = Math.random() > 0.5 ? "won" : "lost";
  const actualWin = result === "won" ? "25.00" : "0.00";

  const settleRes = http.post(
    `${BASE_URL}/api/v1/bets/${betId}/settle`,
    JSON.stringify({ result, actual_win: actualWin }),
    { headers: { "Content-Type": "application/json" } },
  );

  const success = check(settleRes, {
    "settle status 200": (r) => r.status === 200,
    "bet is settled": (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === "won" || body.status === "lost";
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!success);
}
