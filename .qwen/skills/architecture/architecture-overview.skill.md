


# SKILL #1 — architecture-overview.skill.md

Это **фундаментальный skill** — его загружает КАЖДЫЙ агент перед началом работы.

---

```markdown
# architecture-overview.skill.md
# GAMBLING PLATFORM — ARCHITECTURE OVERVIEW
# Version: 1.0.0
# Updated: 2025
# Loaded by: ALL AGENTS

# ============================================================
# SECTION 1: ROLE AND CONTEXT
# ============================================================

## WHO YOU ARE

You are a Senior Software Engineer working on a high-scale online
gambling platform designed for 10M+ registered users and 500K+ DAU.

This platform handles REAL MONEY. Every bug is a financial loss.
Every security flaw is a regulatory violation. Every millisecond
of latency costs revenue.

You write code that is:
- Correct FIRST, then fast
- Secure by default
- Observable from day one
- Tested before merge

## WHAT THIS PLATFORM DOES

Online gambling platform with:
- Sports Betting (pre-match + live, 30+ sports)
- Online Casino (3000+ games via aggregator)
- Payment Processing (fiat + crypto, 20+ methods)
- Bonus System (welcome, reload, free spins, cashback)
- KYC/AML Compliance (identity verification, sanctions screening)
- Responsible Gambling (limits, self-exclusion, reality checks)
- Affiliate Program (revenue share, CPA, hybrid)
- Real-time odds delivery via WebSocket

## CRITICAL BUSINESS RULES

```text
RULE 1: MONEY MUST NEVER BE LOST OR DUPLICATED
  - Every financial operation is idempotent
  - Double-entry bookkeeping for all transactions
  - Reconciliation runs every hour
  - Discrepancy > $0.01 triggers P1 alert

RULE 2: BETS MUST NEVER BE SETTLED TWICE
  - Settlement is idempotent by bet_id + event_result
  - State machine enforces valid transitions only
  - Audit log records every state change

RULE 3: USER BALANCE MUST NEVER GO NEGATIVE
  - Database CHECK constraint: balance >= 0
  - Application-level validation before debit
  - Optimistic locking prevents race conditions

RULE 4: SELF-EXCLUDED USERS MUST NOT GAMBLE
  - Checked on every bet placement
  - Checked on every game launch
  - Checked on every deposit (they CAN withdraw)
  - Cannot be bypassed by any code path

RULE 5: REGULATORY COMPLIANCE IS NON-NEGOTIABLE
  - KYC levels enforce deposit/withdrawal limits
  - AML screening on all Level 2+ users
  - All user actions are audit-logged
  - Data retention per jurisdiction requirements
```

# ============================================================
# SECTION 2: TECHNOLOGY STACK
# ============================================================

## LANGUAGES AND THEIR DOMAINS

```text
┌─────────┬────────────────────────────┬──────────────────────┐
│ Language │ Domain                     │ Why                  │
├─────────┼────────────────────────────┼──────────────────────┤
│ Rust    │ Critical path:             │ No GC, p99 < 5ms,   │
│         │  - Betting Engine          │ memory safety,       │
│         │  - Wallet Core             │ io_uring,            │
│         │  - WebSocket Gateway       │ 150MB RAM per svc    │
│         │  - Odds Calculator         │                      │
│         │  - Risk Engine (real-time) │                      │
│         │  - Fraud Engine (real-time)│                      │
│         │  - Matching Engine         │                      │
│         │  - Audit Log Writer        │                      │
├─────────┼────────────────────────────┼──────────────────────┤
│ Go      │ Business logic:            │ Fast development,    │
│         │  - Auth Service            │ goroutines,          │
│         │  - User Service            │ 500MB RAM per svc,   │
│         │  - Payment Service         │ great stdlib         │
│         │  - Bonus Service           │                      │
│         │  - KYC/AML Service         │                      │
│         │  - Notification Service    │                      │
│         │  - Casino Orchestration    │                      │
│         │  - CMS Service             │                      │
│         │  - Affiliate Service       │                      │
│         │  - Responsible Gambling    │                      │
│         │  - Feature Flags           │                      │
│         │  - Scheduler               │                      │
├─────────┼────────────────────────────┼──────────────────────┤
│ Python  │ ML / Analytics:            │ ML libraries,        │
│         │  - Fraud ML Models         │ data processing,     │
│         │  - Analytics Pipelines     │ batch jobs           │
│         │  - Report Generation       │                      │
│         │  - Model Retraining        │                      │
├─────────┼────────────────────────────┼──────────────────────┤
│ TypeScript│ Frontend:                │ Type safety,         │
│         │  - Next.js 14 (Web)        │ ecosystem,           │
│         │  - React (Admin Panel)     │ SSR/SSG              │
├─────────┼────────────────────────────┼──────────────────────┤
│ Dart    │ Mobile:                    │ Single codebase,     │
│         │  - Flutter (iOS + Android) │ 60fps native perf    │
└─────────┴────────────────────────────┴──────────────────────┘
```

## DATA STORES

```text
┌─────────────────────┬───────────────┬────────────────────────┐
│ Store               │ Role          │ Key Details            │
├─────────────────────┼───────────────┼────────────────────────┤
│ PostgreSQL 16       │ Primary OLTP  │ + Citus for sharding   │
│ + Citus             │               │ Shard key: user_id     │
│                     │               │ ACID for money ops     │
│                     │               │ PgBouncer for pooling  │
├─────────────────────┼───────────────┼────────────────────────┤
│ DragonflyDB         │ Cache +       │ Redis API compatible   │
│                     │ Sessions      │ Multi-threaded         │
│                     │               │ 4M ops/sec/server      │
│                     │               │ < 1ms latency          │
├─────────────────────┼───────────────┼────────────────────────┤
│ ClickHouse          │ Analytics     │ Columnar storage       │
│                     │ OLAP          │ 1B rows/sec scan       │
│                     │               │ 10:1 compression       │
│                     │               │ Logs + events + reports│
├─────────────────────┼───────────────┼────────────────────────┤
│ Redpanda            │ Event         │ Kafka API compatible   │
│                     │ Streaming     │ No JVM/ZooKeeper       │
│                     │               │ < 5ms publish latency  │
│                     │               │ Schema Registry built-in│
├─────────────────────┼───────────────┼────────────────────────┤
│ S3 / MinIO          │ Object        │ KYC docs, backups,     │
│                     │ Storage       │ static assets, audit   │
└─────────────────────┴───────────────┴────────────────────────┘
```

## COMMUNICATION

```text
┌──────────────────┬────────────────────┬─────────────────────┐
│ Protocol         │ Where              │ Why                 │
├──────────────────┼────────────────────┼─────────────────────┤
│ gRPC + Protobuf  │ Service ↔ Service  │ 3-5x less CPU,     │
│                  │ (internal)         │ strict contracts,   │
│                  │                    │ codegen             │
├──────────────────┼────────────────────┼─────────────────────┤
│ REST + JSON      │ Client → API GW   │ Universal,          │
│                  │ (external)         │ frontend-friendly   │
├──────────────────┼────────────────────┼─────────────────────┤
│ WebSocket        │ Client ↔ Server    │ Real-time odds,     │
│                  │ (bidirectional)    │ live updates,       │
│                  │                    │ balance changes     │
├──────────────────┼────────────────────┼─────────────────────┤
│ Redpanda Events  │ Service → Service  │ Async decoupling,   │
│                  │ (async)            │ event sourcing,     │
│                  │                    │ analytics pipeline  │
└──────────────────┴────────────────────┴─────────────────────┘
```

# ============================================================
# SECTION 3: ARCHITECTURE PATTERNS
# ============================================================

## MICROSERVICE BOUNDARIES

```text
RULE: Each microservice owns its data. No shared databases.
RULE: Communication only via gRPC (sync) or Redpanda (async).
RULE: Each service has its own database schema/tables.
RULE: Cross-service joins are FORBIDDEN — use API calls or events.
```

```text
Service Ownership Map:

betting-engine (Rust):
  OWNS: bets, selections, settlements
  READS FROM (gRPC): wallet, user, odds-feed, risk-engine
  PUBLISHES TO (Redpanda): bets.placed, bets.settled, bets.cashout

wallet-service (Rust):
  OWNS: wallets, transactions, ledger_entries
  READS FROM (gRPC): user (for validation)
  PUBLISHES TO (Redpanda): wallet.credited, wallet.debited

auth-service (Go):
  OWNS: credentials, sessions, 2fa_secrets, roles, permissions
  READS FROM (gRPC): user
  PUBLISHES TO (Redpanda): auth.login, auth.failed_login

user-service (Go):
  OWNS: users, profiles, preferences, responsible_gambling_settings
  PUBLISHES TO (Redpanda): users.registered, users.updated, users.blocked

payment-service (Go):
  OWNS: payment_requests, payment_methods, psp_transactions
  READS FROM (gRPC): wallet, user, kyc
  PUBLISHES TO (Redpanda): payments.initiated, payments.completed

bonus-service (Go):
  OWNS: bonus_campaigns, user_bonuses, wagering_progress
  READS FROM (gRPC): wallet, user, betting-engine
  PUBLISHES TO (Redpanda): bonuses.claimed, bonuses.completed

kyc-service (Go):
  OWNS: kyc_records, verification_attempts, aml_screenings
  READS FROM (gRPC): user
  PUBLISHES TO (Redpanda): kyc.verified, kyc.rejected

casino-service (Go):
  OWNS: game_catalog, game_sessions, provider_configs
  READS FROM (gRPC): wallet, user
  PUBLISHES TO (Redpanda): casino.game_started, casino.game_ended

notification-service (Go):
  OWNS: notification_templates, notification_log, user_preferences
  SUBSCRIBES TO (Redpanda): multiple topics
  SENDS VIA: FCM, SES, Twilio

odds-feed-service (Rust):
  OWNS: events, markets, outcomes, odds_history
  RECEIVES FROM: Sportradar (external)
  PUBLISHES TO (Redpanda): events.odds_updated, events.resulted

risk-engine (Rust):
  OWNS: risk_scores, risk_rules, fraud_signals
  READS FROM (gRPC): user, wallet, betting-engine
  PUBLISHES TO (Redpanda): fraud.signals

websocket-gateway (Rust):
  OWNS: nothing (stateless except connections)
  SUBSCRIBES TO (Redpanda): odds updates, user notifications
  PUSHES TO: WebSocket clients
```

## LAYERED ARCHITECTURE (per service)

```text
Every microservice follows this layered architecture:

┌─────────────────────────────────────┐
│         HANDLER / CONTROLLER        │  ← HTTP/gRPC entry point
│         (thin, no business logic)   │     Extract request → call service
├─────────────────────────────────────┤     Return response
│         SERVICE / USE CASE          │  ← Business logic lives HERE
│         (orchestrates operations)   │     Validation, rules, flows
├─────────────────────────────────────┤
│         REPOSITORY / STORE          │  ← Data access abstraction
│         (database operations)       │     SQL queries, cache ops
├─────────────────────────────────────┤
│         DOMAIN / ENTITY             │  ← Core types, value objects
│         (pure data structures)      │     No dependencies
├─────────────────────────────────────┤
│         INFRASTRUCTURE              │  ← External integrations
│         (clients, adapters)         │     PSP clients, Sportradar, etc.
└─────────────────────────────────────┘

RULES:
  - Dependencies flow DOWNWARD only
  - Handler NEVER imports Repository directly
  - Service NEVER constructs SQL queries
  - Domain has ZERO external dependencies
  - Infrastructure implements interfaces defined in Service layer
```

## STATE MACHINES

```text
Critical entities use explicit state machines.
State transitions are validated and logged.

BET STATE MACHINE:
  pending → active → won
  pending → active → lost
  pending → active → void
  pending → active → cashout
  pending → rejected
  
  INVALID TRANSITIONS (must be blocked):
  active → pending  (NEVER go backwards)
  won → lost        (NEVER change result)
  settled → active  (NEVER resettle without void first)

USER STATE MACHINE:
  pending → active → suspended → active
  pending → active → self_excluded
  pending → active → closed
  active → blocked → active (admin unblock)
  self_excluded → active (only after period, if not permanent)

PAYMENT STATE MACHINE:
  initiated → processing → completed
  initiated → processing → failed
  initiated → cancelled
  processing → requires_review → completed
  processing → requires_review → rejected
  
  RULE: completed is FINAL — never reverse in DB
        (create a new "refund" transaction instead)

IMPLEMENTATION:
  - State field is an ENUM in database
  - Transition function validates old_state → new_state
  - Every transition creates an audit log entry
  - Invalid transitions return error (never silently skip)
```

## IDEMPOTENCY

```text
EVERY write operation that involves money MUST be idempotent.

Pattern:
  1. Client generates UUID idempotency_key
  2. Server checks DragonflyDB: GET idempotency:{key}
  3. If exists → return cached response (no re-execution)
  4. If not exists → execute operation
  5. Store result: SET idempotency:{key} {response} EX 86400
  6. Also: UNIQUE constraint on idempotency_key in PostgreSQL
     (catches race conditions that cache misses)

Apply to:
  ✅ Bet placement
  ✅ Wallet credit/debit
  ✅ Payment initiation
  ✅ Bonus claim
  ✅ Bet settlement
  ✅ Any state change on financial entity

DO NOT apply to:
  ❌ Read operations (GET requests)
  ❌ Stateless calculations (odds calculation)
  ❌ Logging/analytics events (at-least-once is fine)
```

# ============================================================
# SECTION 4: API DESIGN STANDARDS
# ============================================================

## REST API (External — Client-facing)

```text
BASE URL: https://api.platform.com/api/v1

VERSIONING: URL path (/api/v1, /api/v2)
  - v1 supported minimum 12 months after v2 release
  - Breaking changes ONLY in new version

NAMING:
  - URLs: kebab-case, plural nouns
    ✅ GET /api/v1/sports-events
    ✅ POST /api/v1/bets
    ❌ GET /api/v1/getSportsEvents
    ❌ POST /api/v1/bet/place

  - Query params: snake_case
    ✅ ?page_size=20&sort_by=created_at
    ❌ ?pageSize=20&sortBy=createdAt

  - Request/Response body: snake_case
    ✅ { "user_id": 123, "created_at": "..." }
    ❌ { "userId": 123, "createdAt": "..." }

HTTP METHODS:
  GET     — read (NEVER modifies data)
  POST    — create / action
  PUT     — full replace
  PATCH   — partial update
  DELETE  — soft delete (we rarely hard-delete)

STATUS CODES:
  200 — Success (with body)
  201 — Created (with Location header)
  204 — Success (no body, for DELETE)
  400 — Validation error (client mistake)
  401 — Unauthorized (no/invalid token)
  403 — Forbidden (valid token, no permission)
  404 — Not found
  409 — Conflict (duplicate, state conflict)
  422 — Unprocessable (valid format, invalid business logic)
  429 — Rate limited (include Retry-After header)
  500 — Internal error (our bug, never expose details)
  503 — Service unavailable (maintenance, overload)
```

## STANDARD RESPONSE FORMAT

```json
// Success response
{
  "data": { ... },
  "meta": {
    "request_id": "req_abc123def456",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}

// Success response (list)
{
  "data": [ ... ],
  "pagination": {
    "cursor": "eyJpZCI6MTAwfQ==",
    "has_more": true,
    "total_count": 1500
  },
  "meta": {
    "request_id": "req_abc123def456",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}

// Error response
{
  "error": {
    "code": "WALLET_INSUFFICIENT_BALANCE",
    "message": "Insufficient balance for this operation",
    "details": {
      "required": "100.00",
      "available": "50.00",
      "currency": "USD"
    }
  },
  "meta": {
    "request_id": "req_abc123def456",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}
```

## ERROR CODES

```text
Error codes are namespaced by service:

AUTH_*:     1000-1999
  AUTH_INVALID_CREDENTIALS     = 1001
  AUTH_TOKEN_EXPIRED           = 1002
  AUTH_TOKEN_INVALID           = 1003
  AUTH_2FA_REQUIRED            = 1004
  AUTH_2FA_INVALID             = 1005
  AUTH_ACCOUNT_LOCKED          = 1006
  AUTH_ACCOUNT_SUSPENDED       = 1007
  AUTH_SESSION_LIMIT_REACHED   = 1008

USER_*:     2000-2999
  USER_NOT_FOUND               = 2001
  USER_EMAIL_EXISTS            = 2002
  USER_PHONE_EXISTS            = 2003
  USER_COUNTRY_BLOCKED         = 2004
  USER_SELF_EXCLUDED           = 2005
  USER_UNDERAGE                = 2006

WALLET_*:   3000-3999
  WALLET_INSUFFICIENT_BALANCE  = 3001
  WALLET_CURRENCY_MISMATCH     = 3002
  WALLET_LIMIT_EXCEEDED        = 3003
  WALLET_LOCKED                = 3004

BET_*:      4000-4999
  BET_EVENT_SUSPENDED          = 4001
  BET_MARKET_CLOSED            = 4002
  BET_ODDS_CHANGED             = 4003
  BET_STAKE_TOO_LOW            = 4004
  BET_STAKE_TOO_HIGH           = 4005
  BET_MAX_PAYOUT_EXCEEDED      = 4006
  BET_DUPLICATE                = 4007
  BET_NOT_FOUND                = 4008
  BET_CASHOUT_UNAVAILABLE      = 4009
  BET_ALREADY_SETTLED          = 4010

PAYMENT_*:  5000-5999
  PAYMENT_METHOD_UNAVAILABLE   = 5001
  PAYMENT_AMOUNT_TOO_LOW       = 5002
  PAYMENT_AMOUNT_TOO_HIGH      = 5003
  PAYMENT_DAILY_LIMIT_EXCEEDED = 5004
  PAYMENT_KYC_REQUIRED         = 5005
  PAYMENT_PROVIDER_ERROR       = 5006
  PAYMENT_DECLINED             = 5007
  PAYMENT_WAGERING_INCOMPLETE  = 5008

CASINO_*:   6000-6999
KYC_*:      7000-7999
BONUS_*:    8000-8999
SYSTEM_*:   9000-9999
```

## gRPC (Internal — Service-to-Service)

```text
RULES:
  1. All internal communication uses gRPC + Protobuf
  2. Proto files are in a SHARED REPOSITORY (single source of truth)
  3. Use buf.build for linting and breaking change detection
  4. Package naming: platform.{service}.v1
  5. Service methods: VerbNoun (GetUser, PlaceBet, CreditWallet)
  6. Field naming: snake_case
  7. Use wrapper types for optional fields
  8. Use google.protobuf.Timestamp for timestamps
  9. Deadline/timeout: ALWAYS set (default 5 seconds)
  10. Retry policy: max 3 retries with exponential backoff

DEADLINE GUIDELINES:
  Wallet operations:   2 seconds  (critical path)
  User lookups:        1 second   (cached usually)
  Risk checks:         3 seconds  (may involve ML)
  Payment operations:  30 seconds (external PSP calls)
  KYC operations:      30 seconds (external API)
  Notification send:   5 seconds  (fire and forget OK)
```

# ============================================================
# SECTION 5: DATA PATTERNS
# ============================================================

## POSTGRESQL CONVENTIONS

```sql
-- TABLE NAMING: snake_case, plural
CREATE TABLE users (...);
CREATE TABLE bets (...);
CREATE TABLE transactions (...);

-- COLUMN NAMING: snake_case
-- EVERY table has these columns:
  id          BIGSERIAL PRIMARY KEY
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- MONEY: Always NUMERIC(18,8), NEVER float/double
  amount      NUMERIC(18,8) NOT NULL

-- STATUS: Always ENUM type
  CREATE TYPE bet_status AS ENUM (
    'pending', 'active', 'won', 'lost', 'void', 'cashout'
  );
  status bet_status NOT NULL DEFAULT 'pending'

-- SOFT DELETE: Use deleted_at, NEVER hard delete user data
  deleted_at  TIMESTAMPTZ  -- NULL = not deleted

-- INDEXES: Named explicitly
  CREATE INDEX idx_bets_user_id_created ON bets(user_id, created_at);
  -- Pattern: idx_{table}_{columns}

-- FOREIGN KEYS: Named explicitly
  CONSTRAINT fk_bets_user FOREIGN KEY (user_id) REFERENCES users(id)

-- PARTITIONING: For high-volume tables
  -- bets: RANGE partition by placed_at (daily)
  -- transactions: RANGE partition by created_at (monthly)

-- SHARDING (Citus):
  -- Shard key: user_id (co-locate user-related data)
  SELECT create_distributed_table('users', 'id');
  SELECT create_distributed_table('wallets', 'user_id');
  SELECT create_distributed_table('bets', 'user_id');
  SELECT create_distributed_table('transactions', 'user_id');
  -- Reference tables (replicated to all nodes):
  SELECT create_reference_table('currencies');
  SELECT create_reference_table('countries');
  SELECT create_reference_table('game_configs');
```

## CACHING PATTERNS (DragonflyDB)

```text
KEY NAMING: {namespace}:{entity}:{identifier}
  session:user:12345
  cache:odds:event:789:market:456
  rl:login:ip:1.2.3.4
  lock:wallet:user:12345
  idempotency:abc-def-123

TTL GUIDELINES:
  Sessions:         7 days
  Odds (live):      3-5 seconds
  Odds (pre-match): 30 seconds
  User profile:     5 minutes
  Game catalog:     10 minutes
  Rate limit:       window duration
  Idempotency key:  24 hours
  Feature flags:    1 minute
  Distributed lock: 30 seconds (with renewal)

PATTERNS:
  Cache-Aside:      Read: check cache → if miss → query DB → set cache
                    Write: update DB → invalidate cache
                    USE FOR: user profiles, game catalog, configs

  Write-Through:    Write: update cache AND DB simultaneously
                    USE FOR: odds (cache is source of truth for reads)

  NEVER cache:      Financial balances for display
                    (always read from wallet service)
                    Exception: balance can be cached for < 1s 
                    in WebSocket gateway for push updates
```

## EVENT PATTERNS (Redpanda)

```text
TOPIC NAMING: {domain}.{event_name}
  bets.placed
  bets.settled
  payments.completed
  users.registered

MESSAGE FORMAT: Protobuf (with schema registry)

KEY: entity_id (for partitioning)
  bets.placed → key: user_id (all bets from same user → same partition)
  events.odds_updated → key: event_id

HEADERS:
  trace_id:       OpenTelemetry trace ID
  correlation_id: Business correlation ID
  produced_at:    ISO 8601 timestamp
  producer:       service name
  event_version:  "1.0"

CONSUMER RULES:
  1. Consumers MUST be idempotent
     (same message processed twice = same result)
  2. Use consumer group per service
  3. Commit offset AFTER successful processing
  4. Dead letter topic for failed messages (after 3 retries)
  5. Monitor consumer lag — alert if > 1000 messages

RETENTION:
  Operational topics: 7 days
  Audit topics:       90 days
  Analytics topics:   30 days

GUARANTEED DELIVERY:
  Producer: acks=all, retries=3
  Consumer: at-least-once (idempotent processing)
  NO exactly-once (too expensive, idempotency is cheaper)
```

# ============================================================
# SECTION 6: SECURITY FUNDAMENTALS
# ============================================================

```text
PRINCIPLE: Defense in depth. Every layer validates.

AUTHENTICATION:
  - Passwords: Argon2id (memory=64MB, iterations=3, parallelism=4)
  - Access tokens: JWT signed with Ed25519, TTL=15 minutes
  - Refresh tokens: opaque random, stored in DB, TTL=7 days
  - Refresh rotation: each use issues new pair, old invalidated
  - 2FA: TOTP (RFC 6238) + WebAuthn + SMS fallback

AUTHORIZATION:
  - RBAC (Role-Based Access Control)
  - Permission check on EVERY request (middleware)
  - Resource-level authorization (user can only see OWN data)
  - Admin actions require additional auth (re-enter password)

INPUT VALIDATION:
  - Validate at API Gateway (Kong) — basic
  - Validate in Handler — format and types
  - Validate in Service — business rules
  - Validate in Database — constraints
  - NEVER trust client input, even from mobile app
  - ALWAYS sanitize strings for SQL, XSS, command injection

ENCRYPTION:
  - In transit: TLS 1.3 minimum (no TLS 1.0, 1.1, 1.2)
  - At rest: AES-256-GCM for sensitive data
  - Key management: HashiCorp Vault + HSM
  - PII fields: encrypted at application level before DB storage
    (email can be hashed for lookup, encrypted for display)

LOGGING SECURITY:
  - NEVER log: passwords, tokens, card numbers, CVV
  - MASK in logs: email (j***@example.com), phone (***1234)
  - OK to log: user_id, request_id, action, status, IP
  - Audit log: immutable, append-only, stored in ClickHouse

RATE LIMITING:
  - Per IP: login=10/min, register=5/hour, API=100/min
  - Per User: bets=60/min, deposits=10/hour
  - Per Action: password_reset=3/hour, 2fa_verify=5/5min
  - Algorithm: Token Bucket (with DragonflyDB storage)
  - Response: 429 with Retry-After header

CORS:
  - Allow only: platform domains (web.platform.com)
  - Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
  - Headers: Authorization, Content-Type, X-Request-ID
  - Credentials: true
  - Max-Age: 3600
```

# ============================================================
# SECTION 7: OBSERVABILITY STANDARDS
# ============================================================

## STRUCTURED LOGGING

```text
FORMAT: JSON, one line per log entry

REQUIRED FIELDS:
  timestamp:    ISO 8601 with milliseconds
  level:        debug, info, warn, error, fatal
  message:      human-readable description
  service:      service name
  trace_id:     OpenTelemetry trace ID
  span_id:      OpenTelemetry span ID
  request_id:   unique per HTTP request

OPTIONAL FIELDS (add when relevant):
  user_id:      authenticated user
  method:       HTTP method
  path:         URL path
  status_code:  HTTP response code
  duration_ms:  request duration
  error:        error details (for level=error)

EXAMPLE:
{
  "timestamp": "2025-01-15T10:30:00.123Z",
  "level": "info",
  "message": "Bet placed successfully",
  "service": "betting-engine",
  "trace_id": "abc123",
  "span_id": "def456",
  "request_id": "req_789",
  "user_id": 12345,
  "bet_id": 67890,
  "stake": "50.00",
  "odds": "2.50",
  "duration_ms": 12
}

LOG LEVELS:
  debug:  Development only, verbose (DB queries, cache hits)
  info:   Normal operations (request handled, bet placed)
  warn:   Recoverable issues (retry succeeded, cache miss spike)
  error:  Failures requiring attention (DB error, PSP failure)
  fatal:  Service cannot continue (config missing, DB unreachable)

RULES:
  - Production default level: info
  - Can be changed per-service at runtime (feature flag)
  - NEVER use fmt.Println / println! / print() in production
  - Use structured logger: tracing (Rust), zerolog (Go), structlog (Python)
```

## METRICS

```text
FORMAT: Prometheus / OpenMetrics (compatible with VictoriaMetrics)

NAMING: {service}_{subsystem}_{metric}_{unit}
  betting_engine_bets_placed_total          (counter)
  betting_engine_bet_placement_duration_seconds  (histogram)
  wallet_service_balance_operations_total    (counter)
  payment_service_deposit_amount_usd_total  (counter)

REQUIRED METRICS FOR EVERY SERVICE:
  # RED metrics (Rate, Errors, Duration)
  {service}_http_requests_total{method, path, status}
  {service}_http_request_duration_seconds{method, path}
  {service}_grpc_requests_total{method, status}
  {service}_grpc_request_duration_seconds{method}

  # Resource metrics
  {service}_db_connections_active
  {service}_db_connections_idle
  {service}_db_query_duration_seconds{query}
  {service}_cache_hits_total
  {service}_cache_misses_total

BUSINESS METRICS (domain-specific):
  platform_active_users_gauge
  platform_bets_placed_total{sport, bet_type}
  platform_bets_settled_total{sport, result}
  platform_deposits_total{method, currency}
  platform_withdrawals_total{method, currency}
  platform_ggr_total{product}  (Gross Gaming Revenue)
  platform_active_websocket_connections

HISTOGRAM BUCKETS:
  HTTP/gRPC latency: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0]
  DB query latency:  [0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5]
```

## TRACING

```text
OpenTelemetry SDK in every service.

SPAN NAMING: {operation_type} {description}
  HTTP GET /api/v1/bets
  gRPC betting.v1.BetService/PlaceBet
  DB SELECT bets
  CACHE GET odds:event:123
  REDPANDA PUBLISH bets.placed
  EXTERNAL Sportradar.GetOdds
  EXTERNAL Stripe.CreateCharge

REQUIRED SPAN ATTRIBUTES:
  service.name:       betting-engine
  service.version:    1.2.3
  user.id:            12345 (if authenticated)
  http.method:        GET
  http.url:           /api/v1/bets
  http.status_code:   200

CONTEXT PROPAGATION:
  - HTTP: via traceparent header (W3C Trace Context)
  - gRPC: via grpc-metadata
  - Redpanda: via message headers
  - ALL services MUST propagate trace context
  - NEVER start a new trace mid-flow
```

# ============================================================
# SECTION 8: TESTING STANDARDS
# ============================================================

```text
TEST PYRAMID:

         ┌─────┐
         │ E2E │         5%  — Critical user journeys only
        ┌┴─────┴┐
        │ Integ │        25% — Service + DB + cache
       ┌┴───────┴┐
       │  Unit   │       70% — Pure logic, no I/O
      └──────────┘

COVERAGE TARGETS:
  Critical path (Rust):    > 85% line coverage
  Business logic (Go):     > 80% line coverage
  Frontend:                > 70% line coverage
  ML code:                 > 60% + model validation tests

NAMING:
  Rust:   #[test] fn test_{function}_{scenario}_{expected}()
          test_place_bet_insufficient_balance_returns_error
  Go:     func Test{Function}_{Scenario}_{Expected}(t *testing.T)
          TestPlaceBet_InsufficientBalance_ReturnsError
  TS:     describe('{Component}') → it('should {behavior}')

TEST DATA:
  - Use factories/builders (NOT raw SQL inserts in tests)
  - Use testcontainers for DB/cache in integration tests
  - NEVER use production data in tests
  - Seed data: deterministic, version-controlled

WHAT TO TEST:
  ✅ Happy path (basic success scenario)
  ✅ Validation errors (invalid input)
  ✅ Business rule violations (insufficient balance, limits)
  ✅ Edge cases (zero amounts, max values, Unicode)
  ✅ Concurrency (race conditions on wallet operations)
  ✅ Idempotency (duplicate requests)
  ✅ State machine transitions (valid and invalid)
  ✅ Error handling (DB down, cache down, external service down)

WHAT NOT TO TEST:
  ❌ Framework internals (Axum routing, Fiber middleware)
  ❌ Generated code (protobuf stubs)
  ❌ Third-party library internals
  ❌ Trivial getters/setters
```

# ============================================================
# SECTION 9: DEPLOYMENT AND OPERATIONS
# ============================================================

```text
ENVIRONMENTS:
  dev:      Auto-deploy on push to main
  staging:  Manual trigger, mirrors production config
  production: Canary deployment via Argo Rollouts

CANARY STRATEGY:
  1. Deploy new version to 5% of traffic
  2. Monitor for 10 minutes (error rate, latency, business metrics)
  3. If healthy: promote to 25% → 50% → 100%
  4. If unhealthy: auto-rollback to previous version
  5. Rollback time: < 2 minutes

FEATURE FLAGS:
  - All new features behind feature flags
  - Flags stored in DragonflyDB (fast reads)
  - Managed via admin panel
  - Types: boolean, percentage rollout, user segment
  - Kill switch for every critical feature

ZERO-DOWNTIME DEPLOYMENTS:
  - Rolling updates (K8s default)
  - Database migrations: backward-compatible only
    ✅ ADD column with default
    ✅ ADD index CONCURRENTLY
    ❌ DROP column (deprecate first, drop in next release)
    ❌ RENAME column (add new, migrate data, drop old)
    ❌ Change column type (add new column, migrate)
  - gRPC: backward-compatible proto changes only
    ✅ Add new field (with field number)
    ✅ Add new RPC method
    ❌ Remove field
    ❌ Change field type
    ❌ Change field number

HEALTH CHECKS:
  Every service exposes:
    GET /healthz        — liveness (is process running?)
    GET /readyz         — readiness (can accept traffic?)
    GET /metrics        — Prometheus metrics

  Readiness checks verify:
    - Database connection alive
    - Cache connection alive
    - Required config loaded
    - Dependent services reachable (with timeout)
```

# ============================================================
# SECTION 10: PROJECT STRUCTURE CONVENTIONS
# ============================================================

## MONOREPO STRUCTURE

```text
platform/
├── proto/                          # Protobuf definitions (shared)
│   ├── buf.yaml
│   ├── common/v1/
│   ├── auth/v1/
│   ├── user/v1/
│   ├── wallet/v1/
│   ├── betting/v1/
│   └── ...
│
├── services/                       # All microservices
│   ├── betting-engine/             # Rust
│   ├── wallet-service/             # Rust
│   ├── websocket-gateway/          # Rust
│   ├── odds-feed-service/          # Rust
│   ├── risk-engine/                # Rust
│   ├── auth-service/               # Go
│   ├── user-service/               # Go
│   ├── payment-service/            # Go
│   ├── bonus-service/              # Go
│   ├── kyc-service/                # Go
│   ├── notification-service/       # Go
│   ├── casino-service/             # Go
│   ├── affiliate-service/          # Go
│   ├── cms-service/                # Go
│   └── fraud-ml-service/           # Python
│
├── libs/                           # Shared libraries
│   ├── rust-platform/              # Rust shared crates
│   │   ├── Cargo.toml (workspace)
│   │   ├── crates/
│   │   │   ├── platform-common/
│   │   │   ├── platform-auth/
│   │   │   ├── platform-db/
│   │   │   ├── platform-cache/
│   │   │   ├── platform-events/
│   │   │   ├── platform-crypto/
│   │   │   └── platform-testing/
│   │   └── ...
│   ├── go-platform/                # Go shared packages
│   │   ├── go.mod
│   │   └── pkg/
│   │       ├── config/
│   │       ├── errors/
│   │       ├── middleware/
│   │       ├── database/
│   │       ├── cache/
│   │       ├── events/
│   │       ├── validator/
│   │       └── testing/
│   └── ts-platform/                # TypeScript shared
│       ├── api-client/
│       ├── types/
│       └── utils/
│
├── frontend/
│   ├── web/                        # Next.js 14
│   ├── mobile/                     # Flutter
│   └── admin/                      # React admin panel
│
├── infra/
│   ├── terraform/                  # IaC
│   │   ├── modules/
│   │   ├── environments/
│   │   └── ...
│   ├── kubernetes/                 # K8s manifests
│   │   ├── base/
│   │   ├── overlays/
│   │   └── charts/
│   ├── docker/                     # Dockerfiles
│   └── scripts/                    # Operational scripts
│
├── docs/                           # Documentation
│   ├── architecture/
│   ├── adr/                        # Architecture Decision Records
│   ├── runbooks/
│   ├── api/
│   └── onboarding/
│
├── skills/                         # AI Agent skills
│   ├── architecture-overview.skill.md
│   ├── rust-general.skill.md
│   ├── go-general.skill.md
│   └── ...
│
├── tests/
│   ├── e2e/                        # End-to-end tests
│   ├── load/                       # k6 load tests
│   └── chaos/                      # Litmus chaos experiments
│
├── .github/
│   └── workflows/                  # CI/CD pipelines
│
├── Makefile                        # Top-level commands
├── docker-compose.yml              # Local development
└── README.md
```

## SERVICE INTERNAL STRUCTURE

### Rust Service

```text
services/betting-engine/
├── Cargo.toml
├── Dockerfile
├── src/
│   ├── main.rs                     # Bootstrap, start server
│   ├── config.rs                   # Configuration struct
│   ├── router.rs                   # Route definitions
│   ├── state.rs                    # AppState (shared deps)
│   ├── handlers/                   # Request handlers
│   │   ├── mod.rs
│   │   ├── bet_handler.rs
│   │   ├── cashout_handler.rs
│   │   └── settlement_handler.rs
│   ├── services/                   # Business logic
│   │   ├── mod.rs
│   │   ├── bet_service.rs
│   │   ├── cashout_service.rs
│   │   ├── settlement_service.rs
│   │   └── odds_service.rs
│   ├── repositories/              # Data access
│   │   ├── mod.rs
│   │   ├── bet_repo.rs
│   │   └── event_repo.rs
│   ├── domain/                     # Core types
│   │   ├── mod.rs
│   │   ├── bet.rs
│   │   ├── selection.rs
│   │   ├── odds.rs
│   │   └── settlement.rs
│   ├── events/                     # Redpanda producers/consumers
│   │   ├── mod.rs
│   │   ├── producer.rs
│   │   └── consumer.rs
│   ├── grpc/                       # gRPC server/client
│   │   ├── mod.rs
│   │   ├── server.rs
│   │   └── clients.rs
│   ├── middleware/                  # Custom middleware
│   │   ├── mod.rs
│   │   ├── auth.rs
│   │   └── tracing.rs
│   └── errors.rs                   # Error types
├── tests/
│   ├── integration/
│   │   ├── bet_placement_test.rs
│   │   └── settlement_test.rs
│   └── fixtures/
│       └── mod.rs
├── migrations/                     # SQL migrations
│   ├── 001_create_bets.sql
│   └── 002_create_selections.sql
└── README.md
```

### Go Service

```text
services/auth-service/
├── go.mod
├── go.sum
├── Dockerfile
├── cmd/
│   └── server/
│       └── main.go                 # Bootstrap, start server
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration
│   ├── handler/                    # HTTP/gRPC handlers
│   │   ├── auth_handler.go
│   │   └── session_handler.go
│   ├── service/                    # Business logic
│   │   ├── auth_service.go
│   │   ├── token_service.go
│   │   └── session_service.go
│   ├── repository/                 # Data access
│   │   ├── user_repo.go
│   │   └── session_repo.go
│   ├── domain/                     # Core types
│   │   ├── user.go
│   │   ├── session.go
│   │   └── token.go
│   ├── middleware/                  # Custom middleware
│   │   ├── auth.go
│   │   └── logging.go
│   └── errors/                     # Error types
│       └── errors.go
├── tests/
│   ├── integration/
│   │   └── auth_test.go
│   └── fixtures/
│       └── fixtures.go
├── migrations/
│   ├── 001_create_credentials.sql
│   └── 002_create_sessions.sql
└── README.md
```

# ============================================================
# SECTION 11: GIT CONVENTIONS
# ============================================================

```text
BRANCH NAMING:
  feature/{ticket}-{short-description}     feature/PLAT-123-bet-placement
  fix/{ticket}-{short-description}         fix/PLAT-456-settlement-race-condition
  hotfix/{ticket}-{short-description}      hotfix/PLAT-789-negative-balance
  chore/{ticket}-{short-description}       chore/PLAT-012-update-dependencies

COMMIT MESSAGES (Conventional Commits):
  feat(betting): add accumulator bet placement
  fix(wallet): prevent race condition on concurrent debits
  perf(odds): optimize odds cache lookup to < 1ms
  refactor(auth): extract token validation to shared lib
  test(payment): add integration tests for Stripe webhook
  docs(api): update betting API documentation
  chore(deps): update Axum to 0.8.0
  ci(deploy): add canary deployment for betting-engine
  security(auth): fix JWT validation bypass vulnerability

PR RULES:
  - Title: conventional commit format
  - Description: what, why, how, testing done
  - Minimum 1 reviewer approval
  - All CI checks must pass (lint, test, security scan)
  - No force push to main
  - Squash merge to main (clean history)
  - Delete branch after merge

PROTECTED BRANCHES:
  main:     production code, deploy to staging
  release/*: release candidates, deploy to production
```

# ============================================================
# SECTION 12: PERFORMANCE BUDGETS
# ============================================================

```text
API LATENCY TARGETS:
  ┌──────────────────────────┬──────┬──────┬───────┐
  │ Operation                │ p50  │ p95  │ p99   │
  ├──────────────────────────┼──────┼──────┼───────┤
  │ Bet placement            │  5ms │ 20ms │  50ms │
  │ Odds query               │  2ms │  5ms │  10ms │
  │ Balance query             │  3ms │  8ms │  15ms │
  │ Wallet debit/credit      │  5ms │ 15ms │  30ms │
  │ User profile             │  3ms │ 10ms │  20ms │
  │ Auth token validation    │ <1ms │  1ms │   2ms │
  │ Login                    │ 50ms │100ms │ 200ms │
  │ Casino game launch       │100ms │300ms │ 500ms │
  │ Payment initiation       │200ms │500ms │   1s  │
  │ KYC verification start   │500ms │  1s  │   2s  │
  └──────────────────────────┴──────┴──────┴───────┘

WEBSOCKET:
  Odds update delivery:     < 50ms from Sportradar to client
  Connection setup:         < 200ms
  Reconnect:                < 2 seconds
  Concurrent connections:   100K per instance, 500K total

FRONTEND:
  First Contentful Paint:   < 1.5s
  Largest Contentful Paint: < 2.5s
  Time to Interactive:      < 3.5s
  Cumulative Layout Shift:  < 0.1
  Bundle size (gzipped):    < 200KB initial JS

DATABASE:
  Simple query (by PK):     < 1ms
  Complex query (joins):    < 10ms
  Batch insert (1000 rows): < 100ms
  Full-text search:         < 50ms

CACHE:
  DragonflyDB GET/SET:      < 0.5ms
  Cache hit rate:           > 95%
```

# ============================================================
# SECTION 13: CHECKLIST BEFORE EVERY PR
# ============================================================

```text
CODE QUALITY:
  [ ] Follows layered architecture (handler → service → repo)
  [ ] No business logic in handlers
  [ ] No SQL in service layer
  [ ] Error handling is explicit (no silenced errors)
  [ ] Structured logging (no println)
  [ ] All public functions documented

SECURITY:
  [ ] Input validated and sanitized
  [ ] Auth/permission check on every endpoint
  [ ] No secrets in code or logs
  [ ] SQL parameterized (no string concatenation)
  [ ] Rate limiting considered

TESTING:
  [ ] Unit tests for new logic
  [ ] Integration tests for new endpoints
  [ ] Edge cases covered
  [ ] Idempotency tested (if write operation)

OBSERVABILITY:
  [ ] Metrics added for new operations
  [ ] Tracing spans for external calls
  [ ] Structured log messages at appropriate levels
  [ ] Dashboard updated (if new metric)

DATA:
  [ ] Migration is backward-compatible
  [ ] Indexes for new query patterns
  [ ] Appropriate TTL for cache entries
  [ ] Events published for state changes

PERFORMANCE:
  [ ] No N+1 queries
  [ ] Batch operations where possible
  [ ] Cache used for hot paths
  [ ] Appropriate timeouts on external calls
```
```

---

Это **фундаментальный skill #1** — все остальные skills ссылаются на него. 
