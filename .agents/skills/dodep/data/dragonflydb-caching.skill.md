## #36 dragonflydb-caching.skill.md

```markdown
# dragonflydb-caching.skill.md

## РОЛЬ
Ты — Backend Developer, работающий с DragonflyDB (Redis-совместимый кэш)
для гемблинг-платформы.

## КОНТЕКСТ
- DragonflyDB: multi-threaded, 25x быстрее Redis
- API совместим с Redis (те же команды)
- 3 ноды: master + 2 replicas
- 64GB RAM per node
- Клиенты: fred (Rust), go-redis (Go)
- Latency: p99 < 1ms

## NAMESPACE CONVENTION
Все ключи имеют prefix = namespace:entity:id[:field]

Namespaces:
session: — user sessions, tokens
cache: — cached data (odds, profiles)
rl: — rate limiting counters
lock: — distributed locks
lb: — leaderboards
idem: — idempotency keys
ff: — feature flags
ws: — websocket subscriptions
counter: — atomic counters

text


## KEY PATTERNS

### Sessions
session:{user_id} → Hash {token, device, ip, created_at, last_activity}
session:{user_id}:devices → Set {device_fingerprint_1, device_fingerprint_2}
session:refresh:{refresh_token} → String user_id
TTL: 7 days

text


```rust
// Rust (fred)
pub async fn create_session(
    cache: &RedisPool,
    user_id: i64,
    session: &SessionData,
) -> Result<()> {
    let key = format!("session:{user_id}");

    cache.hset(&key, vec![
        ("token", &session.access_token),
        ("device", &session.device_fingerprint),
        ("ip", &session.ip_address),
        ("created_at", &Utc::now().to_rfc3339()),
    ]).await?;

    cache.expire(&key, 7 * 24 * 3600).await?;  // 7 days

    // Refresh token → user_id mapping
    let refresh_key = format!("session:refresh:{}", session.refresh_token);
    cache.set(&refresh_key, user_id.to_string(), Some(7 * 24 * 3600), None, false).await?;

    // Track device
    let devices_key = format!("session:{user_id}:devices");
    cache.sadd(&devices_key, &session.device_fingerprint).await?;

    Ok(())
}
Odds Cache
text

cache:odds:{event_id}:{market_id}   → String (Protobuf bytes)
TTL: live=5s, prematch=30s
Rust

// Rust — кэширование odds
pub async fn cache_odds(
    cache: &RedisPool,
    event_id: i64,
    market_id: i64,
    odds: &OddsData,
    is_live: bool,
) -> Result<()> {
    let key = format!("cache:odds:{event_id}:{market_id}");
    let ttl = if is_live { 5 } else { 30 };
    let bytes = odds.encode_to_vec();  // Protobuf

    cache.set(&key, bytes, Some(ttl), None, false).await?;
    Ok(())
}

pub async fn get_odds(
    cache: &RedisPool,
    event_id: i64,
    market_id: i64,
) -> Result<Option<OddsData>> {
    let key = format!("cache:odds:{event_id}:{market_id}");

    match cache.get::<Option<Vec<u8>>, _>(&key).await? {
        Some(bytes) => Ok(Some(OddsData::decode(&bytes[..])?)),
        None => Ok(None),
    }
}
Rate Limiting
text

rl:{type}:{identifier}:{window}  → String (counter)
TTL: = window duration
Rust

// Rust — sliding window rate limiter
pub async fn check_rate_limit(
    cache: &RedisPool,
    action: &str,
    identifier: &str,
    max_requests: u64,
    window_secs: i64,
) -> Result<RateLimitResult> {
    let key = format!("rl:{action}:{identifier}:{}", 
        Utc::now().timestamp() / window_secs);

    let count: u64 = cache.incr(&key, 1).await?;

    if count == 1 {
        cache.expire(&key, window_secs).await?;
    }

    if count > max_requests {
        return Ok(RateLimitResult::Exceeded {
            retry_after: window_secs - (Utc::now().timestamp() % window_secs),
        });
    }

    Ok(RateLimitResult::Allowed {
        remaining: max_requests - count,
    })
}

pub enum RateLimitResult {
    Allowed { remaining: u64 },
    Exceeded { retry_after: i64 },
}

// Использование:
let result = check_rate_limit(&cache, "place_bet", &user_id.to_string(), 10, 60).await?;
match result {
    RateLimitResult::Exceeded { retry_after } => {
        return Err(Error::RateLimited { retry_after });
    }
    RateLimitResult::Allowed { .. } => { /* continue */ }
}
Idempotency Keys
text

idem:{key}  → String (JSON response)
TTL: 24 hours
Rust

// Rust — idempotency check
pub async fn check_idempotency<T: Serialize + DeserializeOwned>(
    cache: &RedisPool,
    idempotency_key: &Uuid,
) -> Result<Option<T>> {
    let key = format!("idem:{idempotency_key}");
    match cache.get::<Option<String>, _>(&key).await? {
        Some(json) => Ok(Some(serde_json::from_str(&json)?)),
        None => Ok(None),
    }
}

pub async fn store_idempotency<T: Serialize>(
    cache: &RedisPool,
    idempotency_key: &Uuid,
    response: &T,
) -> Result<()> {
    let key = format!("idem:{idempotency_key}");
    let json = serde_json::to_string(response)?;
    cache.set(&key, json, Some(86400), None, false).await?;  // 24h
    Ok(())
}
Distributed Locks
text

lock:{resource}:{id}  → String (lock_owner UUID)
TTL: lock duration
Rust

// Rust — distributed lock (Redlock simplified)
pub async fn acquire_lock(
    cache: &RedisPool,
    resource: &str,
    id: &str,
    ttl_secs: i64,
) -> Result<Option<LockGuard>> {
    let key = format!("lock:{resource}:{id}");
    let owner = Uuid::new_v4().to_string();

    // SET NX EX — атомарная операция
    let acquired: bool = cache
        .set(&key, &owner, Some(ttl_secs), Some(SetCondition::NX), false)
        .await?;

    if acquired {
        Ok(Some(LockGuard {
            cache: cache.clone(),
            key,
            owner,
        }))
    } else {
        Ok(None)
    }
}

pub struct LockGuard {
    cache: RedisPool,
    key: String,
    owner: String,
}

impl Drop for LockGuard {
    fn drop(&mut self) {
        // Release lock только если мы owner (Lua script)
        let script = r#"
            if redis.call("get", KEYS[1]) == ARGV[1] then
                return redis.call("del", KEYS[1])
            else
                return 0
            end
        "#;
        // fire-and-forget в Drop
        let _ = tokio::spawn({
            let cache = self.cache.clone();
            let key = self.key.clone();
            let owner = self.owner.clone();
            async move {
                let _ = cache.eval::<i64, _, _>(script, vec![&key], vec![&owner]).await;
            }
        });
    }
}
Leaderboards
text

lb:{type}:{period}  → Sorted Set (user_id → score)
Rust

// Rust — leaderboard
pub async fn update_leaderboard(
    cache: &RedisPool,
    leaderboard_type: &str,
    period: &str,
    user_id: i64,
    score_delta: f64,
) -> Result<()> {
    let key = format!("lb:{leaderboard_type}:{period}");
    cache.zincrby(&key, score_delta, user_id.to_string()).await?;
    Ok(())
}

pub async fn get_leaderboard(
    cache: &RedisPool,
    leaderboard_type: &str,
    period: &str,
    top_n: usize,
) -> Result<Vec<LeaderboardEntry>> {
    let key = format!("lb:{leaderboard_type}:{period}");
    let results: Vec<(String, f64)> = cache
        .zrevrange(&key, 0, (top_n - 1) as i64, true)
        .await?;

    Ok(results.into_iter().enumerate().map(|(rank, (user_id, score))| {
        LeaderboardEntry {
            rank: rank + 1,
            user_id: user_id.parse().unwrap_or(0),
            score,
        }
    }).collect())
}
Feature Flags
text

ff:{flag_name}  → String (JSON config)
TTL: none (обновляется через pub/sub)
Go

// Go — feature flags
func IsFeatureEnabled(ctx context.Context, rdb *redis.Client, 
    flag string, userID int64) (bool, error) {

    key := fmt.Sprintf("ff:%s", flag)
    val, err := rdb.Get(ctx, key).Result()
    if err == redis.Nil {
        return false, nil  // flag не существует = отключен
    }
    if err != nil {
        return false, err
    }

    var config FeatureFlagConfig
    if err := json.Unmarshal([]byte(val), &config); err != nil {
        return false, err
    }

    switch config.Strategy {
    case "all":
        return config.Enabled, nil
    case "percentage":
        return userID % 100 < int64(config.Percentage), nil
    case "user_list":
        for _, id := range config.UserIDs {
            if id == userID { return true, nil }
        }
        return false, nil
    default:
        return false, nil
    }
}
CACHE PATTERNS
Cache-Aside (основной паттерн)
Rust

// ✅ ПРАВИЛЬНО: Cache-Aside
pub async fn get_user_profile(
    cache: &RedisPool,
    db: &PgPool,
    user_id: i64,
) -> Result<UserProfile> {
    let cache_key = format!("cache:user:{user_id}");

    // 1. Попытка из кэша
    if let Some(cached) = cache.get::<Option<String>, _>(&cache_key).await? {
        return Ok(serde_json::from_str(&cached)?);
    }

    // 2. Из БД
    let profile = sqlx::query_as!(UserProfile,
        "SELECT id, email, status, kyc_level FROM users WHERE id = $1",
        user_id
    )
    .fetch_one(db)
    .await?;

    // 3. Записать в кэш
    let json = serde_json::to_string(&profile)?;
    cache.set(&cache_key, json, Some(300), None, false).await?;  // 5 мин

    Ok(profile)
}
Cache Invalidation
Rust

// ✅ ПРАВИЛЬНО: инвалидация при обновлении
pub async fn update_user_profile(
    cache: &RedisPool,
    db: &PgPool,
    user_id: i64,
    update: &ProfileUpdate,
) -> Result<UserProfile> {
    // 1. Обновить в БД
    let profile = update_in_db(db, user_id, update).await?;

    // 2. Удалить из кэша (НЕ обновлять — удалять!)
    let cache_key = format!("cache:user:{user_id}");
    cache.del(&cache_key).await?;

    // 3. Следующий запрос прочитает из БД и закэширует
    Ok(profile)
}

// ❌ ПЛОХО: обновлять кэш напрямую (race condition)
// cache.set(key, new_value) ← может перезаписать более новое значение
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: огромные значения в кэше
cache.set("cache:all_games", huge_json_5mb, ...).await?;

// ✅ ПРАВИЛЬНО: кэшировать по частям
cache.set("cache:games:page:1", page_json, ...).await?;

// ❌ ПЛОХО: без TTL
cache.set(key, value, None, None, false).await?;  // живёт вечно

// ✅ ПРАВИЛЬНО: всегда TTL
cache.set(key, value, Some(300), None, false).await?;  // 5 мин

// ❌ ПЛОХО: использовать KEYS команду
cache.keys("cache:user:*").await?;  // блокирует, O(N)

// ✅ ПРАВИЛЬНО: SCAN для итерации
cache.scan("cache:user:*", 100).await?;

// ❌ ПЛОХО: кэшировать null/пустые результаты без защиты
// Запрос несуществующего user → каждый раз идёт в БД

// ✅ ПРАВИЛЬНО: negative caching
if user.is_none() {
    cache.set(key, "NOT_FOUND", Some(60), None, false).await?;  // 1 мин
}

// ❌ ПЛОХО: Race condition при cache stampede
// 1000 запросов одновременно → все идут в БД

// ✅ ПРАВИЛЬНО: lock при cache miss
let lock = acquire_lock(&cache, "cache_fill", &user_id.to_string(), 5).await?;
if lock.is_some() {
    // Только один запрос идёт в БД, остальные ждут
    let data = fetch_from_db(db, user_id).await?;
    cache.set(key, data, ttl).await?;
}
GO КЛИЕНТ
Go

// Go — go-redis клиент
import "github.com/redis/go-redis/v9"

func NewCacheClient() *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         "dragonflydb:6379",
        Password:     os.Getenv("DRAGONFLY_PASSWORD"),
        DB:           0,
        PoolSize:     100,
        MinIdleConns: 20,
        ReadTimeout:  2 * time.Second,
        WriteTimeout: 2 * time.Second,
        DialTimeout:  5 * time.Second,
    })
}

// Cache-Aside pattern в Go
func GetUserProfile(ctx context.Context, rdb *redis.Client, 
    db *gorm.DB, userID int64) (*User, error) {

    key := fmt.Sprintf("cache:user:%d", userID)

    // 1. Try cache
    cached, err := rdb.Get(ctx, key).Result()
    if err == nil {
        var user User
        if err := json.Unmarshal([]byte(cached), &user); err == nil {
            return &user, nil
        }
    }

    // 2. From DB
    var user User
    if err := db.First(&user, userID).Error; err != nil {
        return nil, err
    }

    // 3. Cache it
    data, _ := json.Marshal(&user)
    rdb.Set(ctx, key, data, 5*time.Minute)

    return &user, nil
}