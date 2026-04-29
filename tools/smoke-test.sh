#!/usr/bin/env bash
# smoke-test.sh — Quick end-to-end validation of key service endpoints
# Usage: bash tools/smoke-test.sh [BASE_URL]
set -euo pipefail

BASE="${1:-http://localhost}"
PASS=0
FAIL=0

check() {
  local name="$1"
  local expected_status="$2"
  local url="$3"
  local method="${4:-GET}"
  local body="${5:-}"

  if [ -n "$body" ]; then
    status=$(curl -sf -o /dev/null -w "%{http_code}" -X "$method" \
      -H "Content-Type: application/json" -d "$body" "$url" 2>/dev/null || echo "000")
  else
    status=$(curl -sf -o /dev/null -w "%{http_code}" -X "$method" "$url" 2>/dev/null || echo "000")
  fi

  if [ "$status" = "$expected_status" ]; then
    echo "  ✅  $name ($status)"
    PASS=$((PASS + 1))
  else
    echo "  ❌  $name — expected $expected_status, got $status"
    FAIL=$((FAIL + 1))
  fi
}

echo ""
echo "═══════════════════════════════════════════"
echo "  Opus Casino — Smoke Tests"
echo "  Base URL: $BASE"
echo "═══════════════════════════════════════════"

echo ""
echo "── Infrastructure health ────────────────"
check "nginx"             "200" "$BASE/"
check "auth service"      "200" "$BASE/api/v1/auth/health"
check "user service"      "200" "$BASE/api/v1/users/health"
check "payment service"   "200" "$BASE/api/v1/payments/healthz"
check "casino service"    "200" "$BASE/api/v1/casino/health"
check "bonus service"     "200" "$BASE/api/v1/bonuses/health"
check "wallet-core"       "200" "$BASE/api/v1/wallet/health"
check "betting-engine"    "200" "$BASE/api/v1/bets/health"

echo ""
echo "── Auth flow ────────────────────────────"
REGISTER_RESP=$(curl -sf -X POST "$BASE/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"smoke_'$(date +%s)'@test.com","password":"Smoke1234!","username":"smoketest"}' 2>/dev/null || echo '{}')

if echo "$REGISTER_RESP" | grep -q '"access_token"'; then
  echo "  ✅  register → JWT returned"
  PASS=$((PASS + 1))
  TOKEN=$(echo "$REGISTER_RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
else
  echo "  ❌  register → no access_token in response"
  FAIL=$((FAIL + 1))
  TOKEN=""
fi

echo ""
echo "── Casino catalog ───────────────────────"
check "casino games list" "200" "$BASE/api/v1/casino/games"
check "casino providers"  "200" "$BASE/api/v1/casino/providers"

echo ""
echo "── NOWPayments connectivity ─────────────"
NP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  -H "x-api-key: ${NOWPAYMENTS_API_KEY:-test}" \
  "https://api.nowpayments.io/v1/status" 2>/dev/null || echo "000")
if [ "$NP_STATUS" = "200" ]; then
  echo "  ✅  NOWPayments API reachable"
  PASS=$((PASS + 1))
else
  echo "  ⚠️   NOWPayments API ($NP_STATUS) — check NOWPAYMENTS_API_KEY"
fi

echo ""
echo "═══════════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "═══════════════════════════════════════════"
echo ""

if [ $FAIL -gt 0 ]; then
  exit 1
fi
