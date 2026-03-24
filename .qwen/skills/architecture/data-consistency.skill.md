SKILL #4 — data-consistency.skill.md
Markdown

# data-consistency.skill.md
# GAMBLING PLATFORM — DATA CONSISTENCY PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: All Backend Agents, Data Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

This platform handles REAL MONEY across distributed services.
Strong consistency for financial operations is NON-NEGOTIABLE.
Eventual consistency is acceptable only for analytics and display.

# ============================================================
# SECTION 2: CONSISTENCY LEVELS
# ============================================================

```text
STRONG CONSISTENCY (ACID):
  ✅ Wallet balance changes
  ✅ Bet placement + fund locking
  ✅ Bet settlement + payouts
  ✅ Payment processing
  ✅ Bonus credit/debit
  ✅ User status changes (block, self-exclude)
  
  HOW: PostgreSQL transactions, optimistic locking, idempotency

EVENTUAL CONSISTENCY (acceptable delay):
  ✅ Analytics dashboards (< 5 min delay OK)
  ✅ Leaderboards (< 30 sec delay OK)
  ✅ User activity history display (< 10 sec delay OK)
  ✅ Search indexes (< 1 min delay OK)
  ✅ Notification delivery (< 30 sec delay OK)
  ✅ Affiliate commission calculations (hourly OK)
  
  HOW: Redpanda events, async consumers, materialized views
============================================================
SECTION 3: IDEMPOTENCY
============================================================
text

EVERY write operation that changes financial state MUST be idempotent.

PATTERN:
  1. Client generates UUID idempotency_key before request
  2. Server checks DragonflyDB: GET idempotency:{key}
  3. If exists → return cached response (no re-execution)
  4. If not → execute operation within DB transaction
  5. Store response: SET idempotency:{key} {response} EX 86400
  6. UNIQUE constraint on idempotency_key in PostgreSQL (safety net)
Rust

/// Idempotency middleware for critical operations
pub async fn with_idempotency<T, F, Fut>(
    cache: &CacheClient,
    key: &Uuid,
    operation: F,
) -> Result<T, AppError>
where
    T: Serialize + DeserializeOwned,
    F: FnOnce() -> Fut,
    Fut: Future<Output = Result<T, AppError>>,
{
    // Check cache
    let cache_key = format!("idempotency:{key}");
    if let Some(cached) = cache.get::<T>(&cache_key).await? {
        return Ok(cached);
    }
    
    // Execute operation
    let result = operation().await?;
    
    // Cache result (24 hours)
    cache.set(&cache_key, &result, 86400).await?;
    
    Ok(result)
}
Go

// Go idempotency helper
func WithIdempotency[T any](
    ctx context.Context,
    cache *redis.Client,
    key string,
    operation func() (*T, error),
) (*T, error) {
    cacheKey := "idempotency:" + key
    
    // Check cache
    cached, err := cache.Get(ctx, cacheKey).Result()
    if err == nil {
        var result T
        json.Unmarshal([]byte(cached), &result)
        return &result, nil
    }
    
    // Execute
    result, err := operation()
    if err != nil {
        return nil, err
    }
    
    // Cache 24h
    data, _ := json.Marshal(result)
    cache.Set(ctx, cacheKey, data, 24*time.Hour)
    
    return result, nil
}
============================================================
SECTION 4: OPTIMISTIC LOCKING
============================================================
text

PROBLEM: Two requests read balance=$100, both debit $80.
  Without locking: both succeed → balance = -$60 (WRONG)

SOLUTION: Version column on mutable rows.
  1. Read:   SELECT balance, version FROM wallets WHERE user_id = $1
  2. Check:  balance >= amount
  3. Write:  UPDATE wallets SET balance = balance - $amount, 
             version = version + 1
             WHERE user_id = $1 AND version = $expected_version
  4. Verify: rows_affected = 1 (if 0 → someone else modified, retry)
Rust

/// Retry loop for optimistic locking
pub async fn with_optimistic_retry<T, F, Fut>(
    max_retries: u32,
    operation: F,
) -> Result<T, AppError>
where
    F: Fn() -> Fut,
    Fut: Future<Output = Result<T, AppError>>,
{
    for attempt in 0..max_retries {
        match operation().await {
            Ok(result) => return Ok(result),
            Err(AppError::ConcurrencyConflict) if attempt < max_retries - 1 => {
                let jitter = rand::thread_rng().gen_range(0..50);
                let delay = Duration::from_millis(50 * 2u64.pow(attempt) + jitter);
                tokio::time::sleep(delay).await;
                tracing::debug!(attempt = attempt + 1, "Retrying after concurrency conflict");
                continue;
            }
            Err(e) => return Err(e),
        }
    }
    Err(AppError::ConcurrencyConflict)
}
text

WHERE TO USE:
  ✅ Wallet balance updates (high concurrency, short duration)
  ✅ Bet status transitions (prevent double settlement)
  ✅ Bonus wagering progress updates

WHERE TO USE PESSIMISTIC LOCKING (SELECT FOR UPDATE) INSTEAD:
  ✅ Settlement batch processing (low concurrency, long duration)
  ✅ Reconciliation (needs stable read)
  ✅ Admin manual adjustments (rare, needs guarantee)
============================================================
SECTION 5: DISTRIBUTED TRANSACTIONS (SAGA)
============================================================
text

PROBLEM: Bet placement spans wallet + betting-engine databases.
  Cannot use single DB transaction across services.

SOLUTION: Saga with compensating actions.

BET PLACEMENT SAGA:
  ┌─────────┐     ┌──────────┐     ┌───────────┐
  │  Check   │────▶│  Lock    │────▶│  Store    │
  │  Risk    │     │  Wallet  │     │  Bet      │
  └─────────┘     └──────────┘     └───────────┘
       │               │                 │
       │ fail          │ fail            │ fail
       ▼               ▼                 ▼
    [reject]     [no compensation   [COMPENSATE:
                  needed - nothing    unlock wallet]
                  was locked]

WITHDRAWAL SAGA:
  Lock wallet → Risk check → Call PSP → Debit wallet
  Compensation chain:
    PSP fails → unlock wallet
    Risk fails → unlock wallet
    
RULES:
  1. Steps execute in order
  2. Each step has a compensating action
  3. Compensating actions are idempotent
  4. If compensation fails → alert + manual intervention
  5. Log every step for audit trail
  6. Set overall saga timeout
Go

// Saga executor pattern
type SagaStep struct {
    Name       string
    Execute    func(ctx context.Context) error
    Compensate func(ctx context.Context) error
}

func ExecuteSaga(ctx context.Context, steps []SagaStep) error {
    var completedSteps []SagaStep
    
    for _, step := range steps {
        log.Info().Str("step", step.Name).Msg("Executing saga step")
        
        if err := step.Execute(ctx); err != nil {
            log.Error().Err(err).Str("step", step.Name).Msg("Saga step failed")
            
            // Compensate in reverse order
            for i := len(completedSteps) - 1; i >= 0; i-- {
                compStep := completedSteps[i]
                log.Info().Str("step", compStep.Name).Msg("Compensating")
                
                if compErr := compStep.Compensate(ctx); compErr != nil {
                    log.Error().Err(compErr).Str("step", compStep.Name).
                        Msg("CRITICAL: Compensation failed — manual intervention required")
                    // Alert P1
                }
            }
            return fmt.Errorf("saga failed at step %s: %w", step.Name, err)
        }
        
        completedSteps = append(completedSteps, step)
    }
    
    return nil
}
============================================================
SECTION 6: OUTBOX PATTERN
============================================================
text

PROBLEM: DB commit succeeds but event publish fails → lost event.

SOLUTION: Write event to outbox table in same DB transaction.
  Separate worker reads outbox → publishes to Redpanda → marks sent.

OUTBOX TABLE:
  CREATE TABLE outbox (
    id         BIGSERIAL PRIMARY KEY,
    topic      VARCHAR(100) NOT NULL,
    key        VARCHAR(255) NOT NULL,
    payload    BYTEA NOT NULL,
    headers    JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMPTZ,
    retries    INT NOT NULL DEFAULT 0
  );

WORKER:
  Every 100ms:
    SELECT * FROM outbox WHERE sent_at IS NULL ORDER BY id LIMIT 100;
    For each: publish to Redpanda
    On success: UPDATE outbox SET sent_at = NOW() WHERE id = $1;
    On failure: UPDATE outbox SET retries = retries + 1 WHERE id = $1;
    If retries > 10: alert, move to dead letter

USE FOR:
  ✅ bets.placed, bets.settled (critical financial events)
  ✅ payments.completed (must trigger downstream)
  
SKIP FOR:
  ❌ analytics.events (loss acceptable, simpler to publish directly)
  ❌ notifications.send (can be resent on next trigger)
============================================================
SECTION 7: RECONCILIATION
============================================================
text

LAST LINE OF DEFENSE. If everything above fails, reconciliation catches it.

WALLET RECONCILIATION (every hour):
  Compare: wallet.balance vs SUM(ledger_entries)
  Alert if: difference > $0.01

BET RECONCILIATION (every hour):
  Check: all bets with settled events have status != 'active'
  Alert if: active bet with resulted event older than 1 hour

PAYMENT RECONCILIATION (daily):
  Compare: our records vs PSP settlement reports
  Alert if: any mismatch in amounts or statuses

CROSS-SERVICE RECONCILIATION (daily):
  Total deposits (payment service) = total credits (wallet service)
  Total bets placed (betting engine) = total locks (wallet service)

IMPLEMENTATION:
  Reconciliation jobs run as scheduled tasks (scheduler-service)
  Results stored in ClickHouse for trend analysis
  Alerts → PagerDuty for discrepancies
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
text

❌ NEVER assume network call succeeds (always handle failure)
❌ NEVER modify two services' databases in one "logical transaction" without saga
❌ NEVER skip idempotency key on financial writes
❌ NEVER use auto-increment ID as idempotency key (not client-generated)
❌ NEVER retry without exponential backoff (thundering herd)
❌ NEVER ignore reconciliation alerts (they indicate real money issues)
❌ NEVER delete records for "cleanup" (soft delete or archive)
❌ NEVER trust client-submitted balance (always server-authoritative)
❌ NEVER publish event before DB commit (event without data)
❌ NEVER skip version check on wallet/bet status updates