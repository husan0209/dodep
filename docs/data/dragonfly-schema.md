# DragonflyDB Cache Documentation

**Author:** DATA_ENGINEER
**Updated:** 2026-03-24
**Cluster:** 3 nodes (master + 2 replicas), 72GB RAM/node

## Кластер

| Параметр    | Значение                  |
| ----------- | ------------------------- |
| Ноды        | 3 (StatefulSet)           |
| Память      | 72GB/node, 64GB maxmemory |
| Режим       | Cache mode (LRU eviction) |
| Snapshots   | Каждые 5 минут            |
| P99 latency | < 1ms                     |
| Throughput  | 1M+ ops/sec               |

## Namespace Convention

Все ключи имеют префикс: `{namespace}:{entity}:{identifier}[:field]`

| Namespace  | Описание             | Пример                                      |
| ---------- | -------------------- | ------------------------------------------- |
| `session:` | Сессии пользователей | `session:user:12345`                        |
| `cache:`   | Кэш данных           | `cache:odds:event:789`                      |
| `rl:`      | Rate limiting        | `rl:login:ip:192.168.1.1:20260324`          |
| `idem:`    | Idempotency keys     | `idem:550e8400-e29b-41d4-a716-446655440000` |
| `lock:`    | Distributed locks    | `lock:bet:settle:98765`                     |
| `lb:`      | Leaderboards         | `lb:daily:20260324`                         |
| `ff:`      | Feature flags        | `ff:new_bonus_system`                       |
| `ws:`      | WebSocket sessions   | `ws:conn:user:12345`                        |
| `counter:` | Счётчики             | `counter:login:attempts:user:12345`         |

## Key Patterns

### Sessions

```
Key:    session:{user_id}
Type:   Hash
TTL:    7 дней
Fields:
  - token: string (JWT refresh token hash)
  - device: string (device fingerprint)
  - ip: string
  - user_agent: string
  - created_at: timestamp
  - last_activity: timestamp
```

**Max sessions per user:** 5 (при создании нового — удаляется самый старый)

### Odds Cache

```
Key:    cache:odds:{event_id}:{market_id}
Type:   String (Protobuf bytes)
TTL:    5 секунд (live) / 30 секунд (prematch)
Value:  Serialized BettingOdds proto message
```

**Invalidation:** При обновлении коэффициентов в Betting Engine → delete cache key

### Rate Limiting

```
Key:    rl:{action}:{identifier}:{window}
Type:   String (counter, INCR)
TTL:    window duration
```

| Action         | Limit | Window | Identifier |
| -------------- | ----- | ------ | ---------- |
| login          | 10    | 1 min  | IP         |
| register       | 5     | 1 hour | IP         |
| bet.place      | 60    | 1 min  | user_id    |
| deposit        | 10    | 1 hour | user_id    |
| api.general    | 100   | 1 min  | user_id    |
| password_reset | 3     | 1 hour | email      |

### Idempotency

```
Key:    idem:{uuid_key}
Type:   String (JSON)
TTL:    24 часа
Value:  {"status": "completed", "response": {...}, "created_at": "..."}
```

**Flow:**

1. Check key exists → return cached response
2. Execute operation in DB transaction
3. Cache response with 24h TTL
4. DB UNIQUE constraint on `idempotency_key` as safety net

### Distributed Locks

```
Key:    lock:{resource}:{id}
Type:   String (owner UUID)
TTL:    lock duration (typically 30s)
```

**Release:** Lua script проверяет owner перед удалением:

```lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
```

### Leaderboards

```
Key:    lb:{type}:{period}
Type:   Sorted Set
Members: user_id
Scores:  metric value (balance, wins, bets_count)
```

| Type         | Period   | Score          | Description              |
| ------------ | -------- | -------------- | ------------------------ |
| top_winners  | daily    | total_wins     | Топ выигравших за день   |
| top_winners  | weekly   | total_wins     | Топ выигравших за неделю |
| top_bettors  | daily    | bets_count     | Топ ставящих за день     |
| high_rollers | all_time | total_deposits | Топ депозиторов          |

### Feature Flags

```
Key:    ff:{flag_name}
Type:   String (JSON)
TTL:    Нет (обновляется через pub/sub)
Value:  {"enabled": true, "rollout_percent": 50, "config": {...}}
```

## Cache Patterns

### Cache-Aside (primary pattern)

```
Read:
  1. GET from cache
  2. If hit → return
  3. If miss → read from DB
  4. SET in cache with TTL
  5. Return value

Write:
  1. Write to DB
  2. DELETE cache key (never update directly)
  3. Return success
```

### Negative Caching

При отсутствии данных в БД → кэшируем "NOT_FOUND" с TTL 1 минута.

```
Key:    cache:{entity}:{id}
Value:  "NOT_FOUND"
TTL:    60 seconds
```

### Cache Stampede Prevention

При cache miss → acquire lock → single DB query → release lock.

```
1. GET cache → miss
2. SETNX lock:{key} → acquired?
   Yes: query DB, SET cache, DEL lock
   No:  wait 100ms, retry GET cache
```

## Anti-patterns

- ❌ Значения > 5MB → ссылка на S3
- ❌ Без TTL → memory leak
- ❌ `KEYS` команда → `SCAN`
- ❌ Обновление кэша напрямую → race condition → delete + re-read
- ❌ Общие ключи без namespace → коллизии
