# Betting Engine — API Documentation

## Base URL

```
http://betting-engine.platform:8080
gRPC: betting-engine.platform:9090
```

---

## REST API

### Place Bet

**POST** `/api/v1/users/{user_id}/bets`

```json
{
  "bet_type": "single",
  "selections": [
    {
      "event_id": 100,
      "market_id": 1,
      "outcome_id": 1,
      "odds": "2.50"
    }
  ],
  "stake": "100.00",
  "currency_code": "USD",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "accept_odds_changes": "none"
}
```

**Response** `201 Created`

```json
{
  "bet_id": 1,
  "user_id": 42,
  "bet_type": "single",
  "status": "pending",
  "stake": "100.00",
  "odds": "2.50",
  "potential_win": "250.00",
  "actual_win": "0",
  "currency_code": "USD",
  "placed_at": "2026-03-24T10:30:00Z",
  "settled_at": null,
  "selections": [
    {
      "event_id": 100,
      "market_id": 1,
      "outcome_id": 1,
      "odds": "2.50",
      "result": null
    }
  ]
}
```

---

### Get Bet

**GET** `/api/v1/users/{user_id}/bets/{bet_id}`

Returns a single bet. User can only see own bets.

---

### Get Bet History

**GET** `/api/v1/users/{user_id}/bets?limit=20&cursor=123&status=active`

Query params:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| limit | int | 20 | Page size (1-100) |
| cursor | int | null | Cursor-based pagination (last bet id) |
| status | string | null | Filter: pending, active, won, lost, void, cashout |

**Response** `200 OK`

```json
{
  "data": [ ... ],
  "total": 150,
  "page_size": 20,
  "cursor": "80"
}
```

---

### Settle Bet

**POST** `/api/v1/bets/{bet_id}/settle`

```json
{
  "result": "won",
  "actual_win": "250.00"
}
```

Result values: `won`, `lost`, `void`

---

### Void Bet

**POST** `/api/v1/bets/{bet_id}/void`

No body required. Returns bet with status `void`.

---

### Cashout Bet

**POST** `/api/v1/users/{user_id}/bets/{bet_id}/cashout`

No body required. Returns:

```json
{
  "bet_id": 1,
  "cashout_value": "47.50",
  "original_stake": "100.00",
  "status": "cashout"
}
```

---

### Health Checks

| Endpoint       | Description                 |
| -------------- | --------------------------- |
| `GET /healthz` | Liveness probe              |
| `GET /readyz`  | Readiness probe (checks DB) |

---

## Error Response Format

```json
{
  "error": {
    "code": "BET_STAKE_TOO_LOW",
    "message": "Min stake 0.10",
    "details": { "min": "0.10", "actual": "0.01" }
  }
}
```

### Error Codes

| Code                        | HTTP | Description                |
| --------------------------- | ---- | -------------------------- |
| VALIDATION_ERROR            | 400  | Invalid request format     |
| AUTH_UNAUTHORIZED           | 401  | Missing/invalid token      |
| AUTH_FORBIDDEN              | 403  | No permission              |
| NOT_FOUND                   | 404  | Bet/event not found        |
| CONFLICT                    | 409  | State conflict             |
| BET_EVENT_SUSPENDED         | 422  | Event suspended            |
| BET_MARKET_CLOSED           | 422  | Market closed              |
| BET_ODDS_CHANGED            | 409  | Odds changed since request |
| BET_STAKE_TOO_LOW           | 422  | Below min stake            |
| BET_STAKE_TOO_HIGH          | 422  | Above max stake            |
| BET_MAX_PAYOUT_EXCEEDED     | 422  | Potential win exceeds cap  |
| BET_REJECTED                | 422  | Risk check failed          |
| BET_ALREADY_SETTLED         | 409  | Already settled            |
| BET_CASHOUT_UNAVAILABLE     | 422  | Cashout not available      |
| WALLET_INSUFFICIENT_BALANCE | 422  | Not enough funds           |
| RATE_LIMITED                | 429  | Too many requests          |
| SERVICE_UNAVAILABLE         | 503  | Upstream service down      |
| INTERNAL_ERROR              | 500  | Server error               |

---

## gRPC API

**Proto:** `libs/proto/betting.proto`

```protobuf
service BettingEngine {
  rpc PlaceBet(PlaceBetRequest) returns (PlaceBetResponse);
  rpc CancelBet(CancelBetRequest) returns (CancelBetResponse);
  rpc GetBet(GetBetRequest) returns (GetBetResponse);
  rpc GetUserBets(GetUserBetsRequest) returns (GetUserBetsResponse);
  rpc SettleBet(SettleBetRequest) returns (SettleBetResponse);
  rpc StreamOdds(OddsStreamRequest) returns (stream OddsUpdate);
}
```

---

## Performance Targets

| Metric        | Target                    |
| ------------- | ------------------------- |
| Place bet p99 | < 5ms                     |
| Get bet p99   | < 5ms                     |
| History p99   | < 20ms                    |
| Throughput    | 10K+ req/sec per instance |
| Startup       | < 2 seconds               |
| Memory        | < 500MB per instance      |
