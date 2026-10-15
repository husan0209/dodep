# SKILL #2 — microservices-patterns.skill.md

```markdown
# microservices-patterns.skill.md
# GAMBLING PLATFORM — MICROSERVICES PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: All Backend Agents, DevOps Agent

# ============================================================
# SECTION 1: SERVICE BOUNDARIES
# ============================================================

```text
RULE: Each service owns its data. No shared databases.
RULE: Communication via gRPC (sync) or Redpanda (async).
RULE: Each service is independently deployable.
RULE: Each service has its own CI/CD pipeline.
RULE: Each service can be scaled independently.

SERVICE SIZE GUIDELINE:
  Too small: "user-email-service" (just one CRUD endpoint)
  Too large: "user-everything-service" (auth + profile + kyc + settings)
  Right size: "auth-service" (login, register, tokens, sessions, 2FA)
              "user-service" (profile, preferences, status)
              "kyc-service" (verification, AML, documents)
SERVICE CATALOG
text

RUST SERVICES (critical path, p99 < 5ms):
  betting-engine       — bet placement, settlement, cashout
  wallet-service       — balance, transactions, ledger
  websocket-gateway    — real-time push to clients
  odds-feed-service    — receive + distribute odds
  risk-engine          — real-time fraud scoring

GO SERVICES (business logic, p99 < 100ms):
  auth-service         — login, register, tokens, sessions
  user-service         — profile, preferences, status
  payment-service      — deposits, withdrawals, PSP integration
  bonus-service        — campaigns, wagering, loyalty
  kyc-service          — identity verification, AML
  casino-service       — game catalog, aggregator wallet API
  notification-service — email, SMS, push, in-app
  affiliate-service    — tracking, commissions, reporting
  cms-service          — pages, banners, promotions
  scheduler-service    — cron jobs, delayed tasks

PYTHON SERVICES (ML/analytics):
  fraud-ml-service     — model training, batch scoring
============================================================
SECTION 2: SYNC COMMUNICATION (gRPC)
============================================================
text

USE gRPC WHEN:
  ✅ Caller NEEDS the response to continue
  ✅ Latency matters (< 10ms for critical path)
  ✅ Strong contract needed between services
  ✅ Request-response pattern

EXAMPLES:
  betting-engine → wallet-service.Lock()      ← needs lock_id to proceed
  betting-engine → risk-engine.CheckBet()     ← needs approval to proceed
  payment-service → wallet-service.Credit()    ← needs confirmation
  auth-service → user-service.GetUser()        ← needs user data

RULES:
  1. ALWAYS set deadline/timeout (default: 5s)
  2. ALWAYS implement retry with exponential backoff (max 3)
  3. ALWAYS use circuit breaker (open after 5 consecutive failures)
  4. ALWAYS propagate trace context via metadata
  5. NEVER call more than 3 services in sequence (fan-out, not chain)
  6. NEVER make gRPC calls in a loop (use batch endpoints)
CIRCUIT BREAKER
text

States: CLOSED → OPEN → HALF-OPEN → CLOSED

CLOSED (normal):
  Requests pass through normally.
  Track error count.
  If errors >= 5 in 30 seconds → switch to OPEN.

OPEN (blocking):
  All requests fail immediately (no actual call).
  After 30 seconds → switch to HALF-OPEN.

HALF-OPEN (probing):
  Allow 1 request through.
  If success → switch to CLOSED.
  If failure → switch back to OPEN.

IMPLEMENTATION:
  Rust: tower::retry + custom circuit breaker middleware
  Go: sony/gobreaker or custom middleware
Go

// Go circuit breaker example
import "github.com/sony/gobreaker"

func newWalletClient(conn *grpc.ClientConn) *WalletClient {
    cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        "wallet-service",
        MaxRequests: 1,                   // half-open allows 1 request
        Interval:    30 * time.Second,    // error counting window
        Timeout:     30 * time.Second,    // open → half-open timeout
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            return counts.ConsecutiveFailures >= 5
        },
        OnStateChange: func(name string, from, to gobreaker.State) {
            log.Warn().Str("service", name).
                Str("from", from.String()).
                Str("to", to.String()).
                Msg("Circuit breaker state change")
            // Emit metric
            metrics.CircuitBreakerState.WithLabelValues(name, to.String()).Set(1)
        },
    })
    
    return &WalletClient{client: walletpb.NewWalletServiceClient(conn), cb: cb}
}
============================================================
SECTION 3: ASYNC COMMUNICATION (Redpanda/Events)
============================================================
text

USE EVENTS WHEN:
  ✅ Caller does NOT need response to continue
  ✅ Multiple consumers interested in the same event
  ✅ Eventual consistency is acceptable
  ✅ Decoupling is more important than latency

EXAMPLES:
  bets.placed      → analytics, fraud engine, notification
  payments.completed → analytics, bonus service, affiliate service
  users.registered  → notification (welcome email), analytics
  kyc.verified     → payment service (update limits)

RULES:
  1. Events are FACTS (past tense): bet_placed, not place_bet
  2. Events are IMMUTABLE: never update a published event
  3. Consumers MUST be idempotent (process same event twice = same result)
  4. Events carry enough data to be useful without callback
  5. Use Protobuf for event schema (with Schema Registry)
  6. Key = entity_id (for partition ordering guarantees)
  7. Dead letter topic for events that fail after 3 retries
EVENT VS COMMAND
text

EVENT (notification pattern):
  "This happened" — producer doesn't care who listens
  Topic: bets.placed
  Multiple consumers, fan-out
  Producer succeeds regardless of consumers

COMMAND (request pattern):
  "Do this" — producer wants specific action
  Topic: notifications.send
  Usually one consumer
  Consider gRPC instead if response needed

OUR APPROACH:
  Events for cross-domain notifications (preferred)
  Commands only for fire-and-forget tasks (notifications, audit)
  gRPC for synchronous needs
============================================================
SECTION 4: SAGA PATTERN
============================================================
text

PROBLEM: Bet placement spans multiple services.
  1. Risk Engine: approve bet
  2. Wallet: lock funds
  3. Database: store bet
  If step 3 fails, must undo step 2.

SOLUTION: Choreography-based saga with compensating actions.

BET PLACEMENT SAGA:
  Step 1: risk-engine.CheckBet()
    → Success: continue
    → Failure: reject bet (no compensation needed)

  Step 2: wallet-service.Lock(amount)
    → Success: continue, save lock_id
    → Failure: reject bet (no compensation needed)

  Step 3: database.InsertBet()
    → Success: publish bets.placed, return bet
    → Failure: COMPENSATE → wallet-service.Unlock(lock_id)

PAYMENT SAGA:
  Step 1: wallet-service.Lock(amount)
  Step 2: psp.InitiateTransfer()
    → Success: wallet-service.Debit(lock_id)
    → Failure: wallet-service.Unlock(lock_id)
    → Timeout: wallet-service.Unlock(lock_id) + alert for manual check

RULES:
  1. Every step has a compensating action
  2. Compensating actions are idempotent
  3. Log saga state for debugging
  4. Set overall saga timeout (30s for bets, 5min for payments)
  5. Dead letter / alert if saga stuck
============================================================
SECTION 5: SERVICE RESILIENCE
============================================================
text

TIMEOUT HIERARCHY:
  Client → API Gateway:    30s
  API Gateway → Service:   25s
  Service → Service (gRPC): 5s (default), 30s (payment/KYC)
  Service → Database:       5s
  Service → Cache:          1s
  Service → Redpanda:       5s

RETRY POLICY:
  Retryable errors: timeout, 503, 429, connection refused
  Non-retryable: 400, 401, 403, 404, 409, 422
  Max retries: 3
  Backoff: exponential (100ms, 400ms, 1600ms)
  Jitter: ±20% (prevent thundering herd)

GRACEFUL DEGRADATION:
  Cache down → serve from DB (slower but functional)
  Risk engine down → use default policy (allow small bets, block large)
  Notification service down → queue for later (don't fail main operation)
  Analytics down → drop events (non-critical)
  
  NEVER degrade: wallet, auth, responsible gambling checks
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER share database between services (coupling)
❌ NEVER chain more than 3 sync calls (latency explosion)
❌ NEVER call service without timeout (unbounded wait)
❌ NEVER retry non-idempotent operations without idempotency key
❌ NEVER ignore circuit breaker open state (cascade failure)
❌ NEVER use sync call where async event works (unnecessary coupling)
❌ NEVER skip compensation in saga (data inconsistency)
❌ NEVER create circular dependencies (A → B → A)
❌ NEVER fan-out gRPC calls without concurrency limit
❌ NEVER put business logic in API gateway (it's routing only)
============================================================
SECTION 7: TESTING
============================================================
text

MUST TEST:
  ✅ Circuit breaker opens after 5 failures, closes after recovery
  ✅ Retry with backoff works for transient errors
  ✅ Saga compensation executes on partial failure
  ✅ Timeout propagation: inner timeout < outer timeout
  ✅ Graceful degradation: service works when dependency is down
  ✅ Event consumer idempotency: same event processed twice = same state
  ✅ Dead letter: failed events routed to DLQ after 3 retries
  ✅ No circular dependencies in service call graph