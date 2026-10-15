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

## Sports Feed (read-only)

> **STATUS: `501 Not Implemented`.**
> The betting-owned `events` / `markets` / `outcomes` tables do not exist
> yet (the only sports tables in `libs/migrations` are the admin-owned
> `sports_events` / `sports_markets` from `013_admin_bff.sql`: UUID keys,
> no outcome odds, cross-service reads forbidden). Every endpoint below
> therefore returns `501` with a machine-readable WebSocket fallback
> contract instead of an empty route, so mobile can leave its error-state
> behind and subscribe to live odds today.

These routes are **public** (no auth): the sports catalog contains no
user data and is browsed pre-login, exactly like the web sportsbook.
Placing bets stays behind auth.

### List Sports Events

**GET** `/api/v1/sports/events?sport=football&live=true&limit=20&cursor=...`

Query params:

| Param  | Type   | Default | Description                              |
|--------|--------|---------|------------------------------------------|
| sport  | string | null    | Sport slug, e.g. `football`, `tennis`    |
| live   | bool   | null    | `true` → live only, `false` → prematch only |
| limit  | int    | 20      | Page size, clamped to 1–100              |
| cursor | string | null    | Opaque next-page token from prior response |

### Get Sports Event

**GET** `/api/v1/sports/events/{id}`

`{id}` is an opaque string (future feed tables may use BIGINT or UUID).
Returns the event with its markets.

### Get Event Odds

**GET** `/api/v1/sports/events/{id}/odds`

Live / pre-match odds resync (see also FRONTEND_SPEC canonical API).
Target cache TTL: **5s live / 30s prematch** (DragonflyDB key
`odds:{event_id}:{market_id}:{outcome_id}`), p99 target **< 50ms**.

### 501 Response (current behavior)

```json
{
  "error": {
    "code": "SPORTS_FEED_UNAVAILABLE",
    "message": "Odds for event evt-9 are not available via REST yet",
    "details": {
      "resource": "event_odds",
      "event_id": "evt-9",
      "retryable": false,
      "docs": "docs/api/betting-engine.md#sports-feed",
      "ws_alternative": {
        "transport": "websocket",
        "redpanda_topic": "events.odds_updated",
        "topic_key": "event_id",
        "fields": ["event_id", "market_id", "outcome_id", "odds", "status", "updated_at"],
        "odds_format": "decimal string, e.g. \"2.50\"",
        "cache_key_pattern": "odds:{event_id}:{market_id}:{outcome_id}",
        "ttl_live_secs": 5,
        "ttl_prematch_secs": 30
      }
    }
  }
}
```

### Future `200 OK` contract (for mobile, when feed tables land)

Odds and money are **decimal strings, never JSON numbers** (no `f64`).

```json
// GET /api/v1/sports/events?sport=football&live=true
{
  "data": [
    {
      "event_id": "evt-9",
      "sport": "football",
      "league": "Premier League",
      "home_team": "Arsenal",
      "away_team": "Chelsea",
      "status": "live",
      "is_live": true,
      "start_time": "2026-10-15T18:00:00Z",
      "market_count": 42
    }
  ],
  "page_size": 20,
  "cursor": "evt-9"
}
```

```json
// GET /api/v1/sports/events/evt-9/odds
{
  "event_id": "evt-9",
  "is_live": true,
  "ttl_secs": 5,
  "markets": [
    {
      "market_id": "m1",
      "name": "Match Result",
      "status": "open",
      "outcomes": [
        { "outcome_id": "o1", "name": "Home", "odds": "2.10", "status": "active" },
        { "outcome_id": "o2", "name": "Draw", "odds": "3.40", "status": "active" },
        { "outcome_id": "o3", "name": "Away", "odds": "3.20", "status": "active" }
      ],
      "margin": "0.0828"
    }
  ]
}
```

### WebSocket alternative (live today)

Subscribe via **websocket-gateway** (`:8082`) instead of polling REST:

| Item | Value |
|------|-------|
| Subscribe message | `{ "action": "subscribe", "channel": "odds:event:{event_id}" }` |
| Redpanda topic | `events.odds_updated` (key: `event_id`, at-least-once, idempotent consumer) |
| Message fields | `event_id`, `market_id`, `outcome_id`, `odds` (decimal string), `status` (`active`/`suspended`/`settled`), `updated_at` (RFC 3339) |
| Suspended market | `status: "suspended"` — mobile must block bet placement on that market |
| Cache (DragonflyDB) | `odds:{event_id}:{market_id}:{outcome_id}`, TTL 5s live / 30s prematch |
| Reconnect | exponential backoff, resync via `GET /sports/events/{id}/odds` once it returns 200 |

REST vs WS guidance for mobile:

| Need | Use |
|------|-----|
| Initial catalog / SEO / pre-login browse | REST `GET /sports/events*` (200 once feed tables land) |
| Live odds ticks (< 50ms) | WS subscription (always) |
| Resync after reconnect | WS replay, then REST odds endpoint when available |
| Feed outage (> 60s no ticks) | show stale-odds banner, block placement on affected markets |

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
