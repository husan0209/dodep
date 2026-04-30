# WebSocket Gateway API Documentation

**Service:** `websocket-gateway`
**Port:** 8080
**Protocol:** WebSocket (WSS in production via Istio/CloudFlare)

## Endpoints

### WebSocket Connection

```
GET /ws?token={jwt_token}
```

**Parameters:**

- `token` (required): JWT access token from Auth Service

**Response:**

- 101 Switching Protocols (WebSocket upgrade)
- 401 Unauthorized (invalid/missing token)
- 503 Service Unavailable (connection limit reached)

### Health Checks

```
GET /healthz    — Liveness probe
GET /readyz     — Readiness probe
```

## Client Protocol

All client messages are JSON. Server sends JSON for control messages and binary Protobuf for data.

### Subscribe to topics

```json
{
  "action": "subscribe",
  "topics": [
    { "type": "event_odds", "id": 12345 },
    { "type": "event_stats", "id": 12345 },
    { "type": "user_bets", "id": 67890 },
    { "type": "user_balance", "id": 67890 }
  ]
}
```

**Response:**

```json
{
  "action": "subscribed",
  "topics": [{ "type": "event_odds", "id": 12345 }]
}
```

### Unsubscribe from topics

```json
{
  "action": "unsubscribe",
  "topics": [{ "type": "event_odds", "id": 12345 }]
}
```

**Response:**

```json
{
  "action": "unsubscribed",
  "topics": [{ "type": "event_odds", "id": 12345 }]
}
```

### Ping/Pong (heartbeat)

```json
{ "action": "ping" }
```

**Response:**

```json
{ "action": "pong" }
```

## Topic Types

| Type           | Description                                   | Key      | Source                          |
| -------------- | --------------------------------------------- | -------- | ------------------------------- |
| `event_odds`   | Real-time odds updates for a sports event     | event_id | Redpanda: `events.odds_updated` |
| `event_stats`  | Live match statistics                         | event_id | Redpanda: `analytics.events`    |
| `user_bets`    | Bet status updates (placed, settled, cashout) | user_id  | Redpanda: `bets.bet.*`          |
| `user_balance` | Balance changes                               | user_id  | Redpanda: payment events        |
| `sport_scores` | Live scores for a sport                       | sport_id | Redpanda: `analytics.events`    |

## Auto-subscriptions

On connect, the gateway automatically subscribes to:

- `user_bets:{user_id}` — your bet updates
- `user_balance:{user_id}` — your balance changes

These cannot be unsubscribed during the session.

## Server Messages

### Data (binary)

Odds updates and event data are sent as binary Protobuf payloads.
Decode using `OddsUpdate` message from `betting.proto`.

### Error

```json
{
  "action": "error",
  "code": "WS_TOO_MANY_SUBSCRIPTIONS",
  "message": "Max 50 subscriptions per connection"
}
```

**Error codes:**
| Code | Description |
|------|-------------|
| `WS_UNAUTHORIZED` | Invalid or expired token |
| `WS_TOO_MANY_SUBSCRIPTIONS` | Exceeded 50 topic limit |
| `WS_RATE_LIMITED` | Too many messages |
| `WS_CONNECTION_LIMIT` | Server at capacity |
| `WS_INVALID_TOPIC` | Unknown topic type |
| `WS_INVALID_MESSAGE` | Malformed JSON |
| `WS_KAFKA_ERROR` | Event stream unavailable |
| `INTERNAL_ERROR` | Internal server error |

## Limits

| Limit                            | Value                     |
| -------------------------------- | ------------------------- |
| Max connections per instance     | 100,000                   |
| Max subscriptions per connection | 50                        |
| Channel buffer size              | 256 messages              |
| Message rate                     | 60 msg/min per connection |

## Client Libraries

### JavaScript/TypeScript

```typescript
const ws = new WebSocket(`wss://api.opus.casino/ws?token=${accessToken}`);

ws.onopen = () => {
  ws.send(
    JSON.stringify({
      action: "subscribe",
      topics: [{ type: "event_odds", id: 12345 }],
    }),
  );
};

ws.onmessage = (event) => {
  if (event.data instanceof Blob) {
    // Binary data — decode Protobuf OddsUpdate
    const buffer = await event.data.arrayBuffer();
    const odds = OddsUpdate.decode(new Uint8Array(buffer));
    console.log("Odds update:", odds);
  } else {
    const msg = JSON.parse(event.data);
    if (msg.action === "pong") {
      // heartbeat acknowledged
    }
  }
};

// Heartbeat every 30 seconds
setInterval(() => {
  ws.send(JSON.stringify({ action: "ping" }));
}, 30000);
```

### Flutter/Dart

```dart
final channel = WebSocketChannel.connect(
  Uri.parse('wss://api.opus.casino/ws?token=$accessToken'),
);

channel.sink.add(jsonEncode({
  'action': 'subscribe',
  'topics': [{'type': 'event_odds', 'id': 12345}]
}));

channel.stream.listen((message) {
  if (message is String) {
    final msg = jsonDecode(message);
    if (msg['action'] == 'pong') {
      // heartbeat acknowledged
    }
  } else if (message is List<int>) {
    // Binary Protobuf — decode OddsUpdate
    final odds = OddsUpdate.fromBuffer(message);
  }
});
```

## Deployment

- **Replicas:** 3-20 (HPA by CPU/memory)
- **PDB:** minAvailable: 2
- **Resources:** 4 CPU / 8GB per pod
- **Network Policy:** Ingress from Istio, Egress to Redpanda only
- **Scaling:** Horizontal Pod Autoscaler, target 70% CPU
