SKILL #9 — rust-sqlx-database.skill.md
Markdown

# rust-sqlx-database.skill.md
# GAMBLING PLATFORM — RUST SQLx DATABASE PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

All Rust services use SQLx with PostgreSQL 16 + Citus.
SQLx provides compile-time checked queries — if it compiles,
the SQL is valid against your schema.

Connection pooling via SQLx built-in pool + PgBouncer in front.

# ============================================================
# SECTION 2: CONNECTION POOL SETUP
# ============================================================

```rust
use sqlx::postgres::{PgPoolOptions, PgConnectOptions, PgSslMode};
use std::time::Duration;

pub async fn create_pool(config: &DatabaseConfig) -> Result<PgPool, sqlx::Error> {
    let options = PgConnectOptions::new()
        .host(&config.host)
        .port(config.port)
        .database(&config.database)
        .username(&config.username)
        .password(&config.password)
        .ssl_mode(PgSslMode::Require)
        .application_name("betting-engine")
        .statement_cache_capacity(256);

    PgPoolOptions::new()
        .max_connections(config.max_connections)    // 20-50 per instance
        .min_connections(config.min_connections)    // 5
        .acquire_timeout(Duration::from_secs(5))
        .idle_timeout(Duration::from_secs(300))
        .max_lifetime(Duration::from_secs(1800))   // 30 min
        .test_before_acquire(true)
        .connect_with(options)
        .await
}

// Configuration
pub struct DatabaseConfig {
    pub host: String,            // PgBouncer address
    pub port: u16,               // 6432 (PgBouncer) not 5432
    pub database: String,
    pub username: String,
    pub password: String,        // from Vault
    pub max_connections: u32,    // 30
    pub min_connections: u32,    // 5
}
============================================================
SECTION 3: COMPILE-TIME CHECKED QUERIES
============================================================
Rust

// ── query! macro — validates SQL at compile time ──
// Requires DATABASE_URL env var at build time

// Single row fetch
let user = sqlx::query_as!(
    UserRow,
    r#"
    SELECT id, email, status as "status: UserStatus",
           kyc_level, country_code, created_at
    FROM users
    WHERE id = $1
    "#,
    user_id
)
.fetch_one(&pool)
.await?;

// Optional row (returns Option)
let bet = sqlx::query_as!(
    BetRow,
    r#"
    SELECT id, user_id, stake, odds,
           status as "status: BetStatus"
    FROM bets
    WHERE id = $1 AND user_id = $2
    "#,
    bet_id,
    user_id
)
.fetch_optional(&pool)
.await?;

// Multiple rows
let bets = sqlx::query_as!(
    BetRow,
    r#"
    SELECT id, user_id, stake, odds,
           status as "status: BetStatus"
    FROM bets
    WHERE user_id = $1
    ORDER BY placed_at DESC
    LIMIT $2 OFFSET $3
    "#,
    user_id,
    limit,
    offset
)
.fetch_all(&pool)
.await?;

// Scalar value
let count = sqlx::query_scalar!(
    "SELECT COUNT(*) FROM bets WHERE user_id = $1 AND status = 'active'",
    user_id
)
.fetch_one(&pool)
.await?
.unwrap_or(0);

// Execute (INSERT/UPDATE/DELETE — no return)
let rows_affected = sqlx::query!(
    "UPDATE wallets SET balance = balance - $1, version = version + 1
     WHERE user_id = $2 AND version = $3 AND balance >= $1",
    amount, user_id, expected_version
)
.execute(&pool)
.await?
.rows_affected();
TYPE OVERRIDES
Rust

// PostgreSQL custom types need explicit casting in query
// Use "column: Type" syntax in the SELECT

// Enum type override
let bet = sqlx::query_as!(
    BetRow,
    r#"
    SELECT
        id,
        status as "status: BetStatus",
        bet_type as "bet_type: BetType"
    FROM bets WHERE id = $1
    "#,
    bet_id
)
.fetch_one(&pool)
.await?;

// Nullable column
let user = sqlx::query_as!(
    UserRow,
    r#"
    SELECT
        id,
        phone as "phone: Option<String>",
        last_login_at as "last_login_at: Option<DateTime<Utc>>"
    FROM users WHERE id = $1
    "#,
    user_id
)
.fetch_one(&pool)
.await?;

// Decimal (NUMERIC in PostgreSQL)
// SQLx maps NUMERIC → rust_decimal::Decimal automatically
// when feature "rust_decimal" is enabled

// Array types
let ids = sqlx::query_scalar!(
    "SELECT id FROM bets WHERE user_id = ANY($1)",
    &bet_ids[..] as &[i64]  // pass slice as PostgreSQL array
)
.fetch_all(&pool)
.await?;
============================================================
SECTION 4: TRANSACTIONS
============================================================
Rust

// ── Basic transaction ──
pub async fn create_bet_with_selections(
    pool: &PgPool,
    bet: &NewBet,
    selections: &[NewSelection],
) -> Result<Bet, sqlx::Error> {
    let mut tx = pool.begin().await?;

    // Insert bet
    let row = sqlx::query_as!(
        BetRow,
        r#"INSERT INTO bets (user_id, stake, odds, status, idempotency_key)
           VALUES ($1, $2, $3, 'pending', $4)
           RETURNING id, user_id, stake, odds, status as "status: BetStatus""#,
        bet.user_id, bet.stake, bet.odds, bet.idempotency_key
    )
    .fetch_one(&mut *tx)
    .await?;

    // Insert selections
    for sel in selections {
        sqlx::query!(
            "INSERT INTO bet_selections (bet_id, event_id, market_id, outcome_id, odds)
             VALUES ($1, $2, $3, $4, $5)",
            row.id, sel.event_id, sel.market_id, sel.outcome_id, sel.odds
        )
        .execute(&mut *tx)
        .await?;
    }

    // Commit
    tx.commit().await?;

    Ok(Bet::from(row))
}

// ── Transaction passed as parameter (for service orchestration) ──
pub async fn update_bet_status(
    tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    bet_id: i64,
    from: BetStatus,
    to: BetStatus,
    payout: Option<Decimal>,
) -> Result<BetRow, sqlx::Error> {
    sqlx::query_as!(
        BetRow,
        r#"UPDATE bets
           SET status = $3, actual_win = COALESCE($4, actual_win),
               settled_at = CASE WHEN $3 IN ('won','lost','void','cashout')
                            THEN NOW() ELSE settled_at END,
               updated_at = NOW()
           WHERE id = $1 AND status = $2
           RETURNING id, user_id, stake, odds, status as "status: BetStatus",
                     actual_win, settled_at"#,
        bet_id,
        from as BetStatus,
        to as BetStatus,
        payout
    )
    .fetch_optional(&mut **tx)
    .await?
    .ok_or(sqlx::Error::RowNotFound) // state conflict
}

// ── Rollback on error (automatic with drop) ──
// If tx.commit() is not called, transaction rolls back on drop.
// But explicit error handling is clearer:

pub async fn transfer_funds(pool: &PgPool, from: i64, to: i64, amount: Decimal) -> Result<(), AppError> {
    let mut tx = pool.begin().await?;

    match execute_transfer(&mut tx, from, to, amount).await {
        Ok(_) => {
            tx.commit().await?;
            Ok(())
        }
        Err(e) => {
            // tx.rollback() happens automatically on drop
            // but explicit rollback gives clearer logs
            tx.rollback().await?;
            Err(e)
        }
    }
}
============================================================
SECTION 5: DYNAMIC QUERIES
============================================================
Rust

// When filters are optional, use QueryBuilder for safe dynamic SQL

use sqlx::QueryBuilder;

pub async fn search_bets(
    pool: &PgPool,
    user_id: i64,
    filters: &BetFilters,
) -> Result<Vec<BetRow>, sqlx::Error> {
    let mut qb = QueryBuilder::new(
        "SELECT id, user_id, stake, odds, status, placed_at FROM bets WHERE user_id = "
    );
    qb.push_bind(user_id);

    if let Some(status) = &filters.status {
        qb.push(" AND status = ");
        qb.push_bind(status);
    }

    if let Some(sport_id) = filters.sport_id {
        qb.push(" AND sport_id = ");
        qb.push_bind(sport_id);
    }

    if let Some(date_from) = filters.date_from {
        qb.push(" AND placed_at >= ");
        qb.push_bind(date_from);
    }

    if let Some(date_to) = filters.date_to {
        qb.push(" AND placed_at <= ");
        qb.push_bind(date_to);
    }

    // Sorting (whitelist allowed columns!)
    let sort_col = match filters.sort_by.as_deref() {
        Some("stake") => "stake",
        Some("odds") => "odds",
        Some("placed_at") | None => "placed_at",
        Some(_) => "placed_at", // ignore invalid, use default
    };
    let sort_dir = if filters.sort_desc.unwrap_or(true) { "DESC" } else { "ASC" };
    qb.push(format!(" ORDER BY {sort_col} {sort_dir}"));

    qb.push(" LIMIT ");
    qb.push_bind(filters.page_size.unwrap_or(20).min(100));

    if let Some(offset) = filters.offset {
        qb.push(" OFFSET ");
        qb.push_bind(offset);
    }

    qb.build_query_as::<BetRow>()
        .fetch_all(pool)
        .await
}
============================================================
SECTION 6: BATCH OPERATIONS
============================================================
Rust

// ── Batch insert (for settlement, analytics) ──

use sqlx::QueryBuilder;

pub async fn batch_insert_ledger_entries(
    tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    entries: &[LedgerEntry],
) -> Result<(), sqlx::Error> {
    if entries.is_empty() {
        return Ok(());
    }

    // SQLx QueryBuilder supports push_values for multi-row INSERT
    let mut qb = QueryBuilder::new(
        "INSERT INTO ledger_entries (transaction_id, account_type, account_id, entry_type, amount, balance_after, reference_type, reference_id, idempotency_key)"
    );

    qb.push_values(entries, |mut b, entry| {
        b.push_bind(entry.transaction_id)
         .push_bind(&entry.account_type)
         .push_bind(&entry.account_id)
         .push_bind(&entry.entry_type)
         .push_bind(entry.amount)
         .push_bind(entry.balance_after)
         .push_bind(&entry.reference_type)
         .push_bind(entry.reference_id)
         .push_bind(entry.idempotency_key);
    });

    qb.build().execute(&mut **tx).await?;
    Ok(())
}

// ── Batch fetch with IN clause ──
pub async fn get_bets_by_ids(
    pool: &PgPool,
    bet_ids: &[i64],
) -> Result<Vec<BetRow>, sqlx::Error> {
    sqlx::query_as!(
        BetRow,
        r#"SELECT id, user_id, stake, odds, status as "status: BetStatus"
           FROM bets WHERE id = ANY($1)"#,
        bet_ids
    )
    .fetch_all(pool)
    .await
}
============================================================
SECTION 7: MIGRATIONS
============================================================
Rust

// Run migrations at startup
pub async fn run_migrations(pool: &PgPool) -> Result<(), sqlx::migrate::MigrateError> {
    sqlx::migrate!("./migrations")
        .run(pool)
        .await
}

// Migration file naming:
// migrations/
//   20250101000001_create_bets.sql
//   20250101000002_create_selections.sql
//   20250101000003_add_cashout_column.sql
SQL

-- migrations/20250101000001_create_bets.sql
CREATE TYPE bet_status AS ENUM (
    'pending', 'active', 'won', 'lost', 'void', 'cashout', 'rejected'
);

CREATE TYPE bet_type AS ENUM ('single', 'accumulator', 'system');

CREATE TABLE bets (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,
    bet_type        bet_type NOT NULL,
    status          bet_status NOT NULL DEFAULT 'pending',
    stake           NUMERIC(18,8) NOT NULL,
    combined_odds   NUMERIC(12,6) NOT NULL,
    potential_win   NUMERIC(18,8) NOT NULL,
    actual_win      NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency_code   CHAR(3) NOT NULL,
    idempotency_key UUID UNIQUE NOT NULL,
    lock_id         BIGINT,
    ip_address      INET,
    device_fp       VARCHAR(64),
    placed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_stake_positive CHECK (stake > 0),
    CONSTRAINT chk_odds_valid CHECK (combined_odds >= 1.01)
);

CREATE INDEX idx_bets_user_status ON bets(user_id, status);
CREATE INDEX idx_bets_user_placed ON bets(user_id, placed_at DESC);
CREATE INDEX idx_bets_status_placed ON bets(status, placed_at) WHERE status = 'active';

-- Citus distribution
SELECT create_distributed_table('bets', 'user_id');
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
text

❌ NEVER use format!() to build SQL strings (SQL injection)
   ✅ Use query! with $1 params or QueryBuilder::push_bind()

❌ NEVER use fetch_one() when row might not exist
   ✅ Use fetch_optional() and handle None

❌ NEVER ignore rows_affected after UPDATE/DELETE
   ✅ Check rows_affected to detect concurrent modifications

❌ NEVER SELECT * in production code
   ✅ List explicit columns (schema changes won't break silently)

❌ NEVER do N+1 queries (loop with individual queries)
   ✅ Use IN clause, JOIN, or batch queries

❌ NEVER hold transaction open across external calls (gRPC, HTTP)
   ✅ Minimize transaction scope: read → external call → write in tx

❌ NEVER skip index on frequently filtered columns
   ✅ Add index for every WHERE clause pattern

❌ NEVER use OFFSET for deep pagination (slow on large tables)
   ✅ Use cursor-based pagination (WHERE id > $cursor)
============================================================
SECTION 9: TESTING
============================================================
Rust

// Use testcontainers for integration tests
use testcontainers::{clients::Cli, GenericImage};

async fn setup_test_db() -> PgPool {
    let docker = Cli::default();
    let pg = docker.run(
        GenericImage::new("postgres", "16")
            .with_env_var("POSTGRES_DB", "test")
            .with_env_var("POSTGRES_USER", "test")
            .with_env_var("POSTGRES_PASSWORD", "test")
    );
    
    let port = pg.get_host_port_ipv4(5432);
    let url = format!("postgres://test:test@localhost:{port}/test");
    
    let pool = PgPoolOptions::new()
        .max_connections(5)
        .connect(&url)
        .await
        .expect("Failed to connect");
    
    sqlx::migrate!("./migrations").run(&pool).await.expect("Migrations failed");
    pool
}

#[tokio::test]
async fn test_create_bet_returns_id() {
    let pool = setup_test_db().await;
    let repo = BetRepository::new(pool);
    
    let bet = repo.create_bet(NewBet {
        user_id: 1,
        stake: dec!(100),
        odds: dec!(2.50),
        idempotency_key: Uuid::new_v4(),
    }).await.unwrap();
    
    assert!(bet.id > 0);
    assert_eq!(bet.status, BetStatus::Pending);
}