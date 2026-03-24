SKILL #13 — rust-testing.skill.md
Markdown

# rust-testing.skill.md
# GAMBLING PLATFORM — RUST TESTING PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Rust critical path services handle real money at 10K+ req/sec.
Tests are your safety net. Coverage target: > 85%.

Test pyramid: 70% unit, 25% integration, 5% e2e.
CI blocks merge if any test fails or clippy warns.

# ============================================================
# SECTION 2: TEST ORGANIZATION
# ============================================================

```text
services/betting-engine/
├── src/
│   ├── services/
│   │   ├── bet_service.rs          ← unit tests at bottom of file
│   │   └── settlement_service.rs
│   └── domain/
│       ├── bet.rs                  ← unit tests for domain logic
│       └── odds.rs
├── tests/
│   ├── integration/
│   │   ├── bet_placement_test.rs   ← integration tests (DB, cache)
│   │   ├── settlement_test.rs
│   │   └── common/
│   │       ├── mod.rs
│   │       ├── setup.rs            ← test infrastructure
│   │       └── fixtures.rs         ← test data factories
│   └── benchmarks/
│       └── bet_placement_bench.rs  ← criterion benchmarks
============================================================
SECTION 3: UNIT TESTS
============================================================
Rust

// Unit tests live at the bottom of the source file in #[cfg(test)] mod

// src/domain/bet.rs
#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use rust_decimal_macros::dec;

    // ── Naming: test_{function}_{scenario}_{expected} ──

    #[test]
    fn test_status_transition_valid() {
        assert!(BetStatus::Pending.can_transition_to(BetStatus::Active));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Won));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Lost));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Void));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Cashout));
    }

    #[test]
    fn test_status_transition_invalid_backwards() {
        assert!(!BetStatus::Won.can_transition_to(BetStatus::Active));
        assert!(!BetStatus::Lost.can_transition_to(BetStatus::Active));
        assert!(!BetStatus::Active.can_transition_to(BetStatus::Pending));
    }

    #[test]
    fn test_accumulator_odds_multiply() {
        let odds = vec![dec!(1.80), dec!(2.20), dec!(1.50)];
        let combined = BetCalculator::accumulator_odds(&odds);
        assert_eq!(combined, dec!(5.940));
    }

    #[test]
    fn test_accumulator_with_void_uses_odds_one() {
        let odds = vec![dec!(2.00), dec!(3.00)];
        let results = vec![SelectionOutcome::Won, SelectionOutcome::Void];
        let payout = BetCalculator::accumulator_with_voids(dec!(10), &odds, &results);
        // 2.00 × 1.00 = 2.00, payout = 10 × 2.00 = 20
        assert_eq!(payout, dec!(20.00));
    }

    #[test]
    fn test_accumulator_with_loss_returns_zero() {
        let odds = vec![dec!(2.00), dec!(3.00)];
        let results = vec![SelectionOutcome::Won, SelectionOutcome::Lost];
        let payout = BetCalculator::accumulator_with_voids(dec!(10), &odds, &results);
        assert_eq!(payout, Decimal::ZERO);
    }

    #[test]
    fn test_validate_stake_rejects_zero() {
        let result = validate_positive_decimal(&Decimal::ZERO);
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_stake_rejects_negative() {
        let result = validate_positive_decimal(&dec!(-5));
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_stake_accepts_minimum() {
        let result = validate_positive_decimal(&dec!(0.01));
        assert!(result.is_ok());
    }
}
ASYNC UNIT TESTS
Rust

// For async functions, use #[tokio::test]

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_idempotency_returns_cached() {
        let cache = MockCacheClient::new();
        let bet = Bet { id: BetId(1), stake: dec!(100), ..Default::default() };
        
        // Pre-populate cache
        cache.set_idempotent(&Uuid::new_v4(), &bet).await.unwrap();
        
        // Second call should return cached result
        let result = cache.get_idempotent::<Bet>(&key).await.unwrap();
        assert!(result.is_some());
        assert_eq!(result.unwrap().id, BetId(1));
    }
}
============================================================
SECTION 4: INTEGRATION TESTS
============================================================
Rust

// tests/integration/bet_placement_test.rs
// Uses testcontainers for real PostgreSQL and DragonflyDB

use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;
use testcontainers_modules::redis::Redis;

mod common;
use common::{setup::TestApp, fixtures::*};

#[tokio::test]
async fn test_place_single_bet_success() {
    let app = TestApp::start().await;
    
    // Seed data
    let user = app.create_user(UserFixture::active()).await;
    app.credit_balance(user.id, dec!(1000)).await;
    app.create_event(EventFixture::football_open()).await;
    
    // Execute
    let req = PlaceBetRequest {
        bet_type: BetType::Single,
        selections: vec![SelectionRequest {
            event_id: 1, market_id: 1, outcome_id: 1,
            odds: dec!(2.50),
        }],
        stake: dec!(100),
        currency_code: "USD".into(),
        idempotency_key: Uuid::new_v4(),
        ..Default::default()
    };
    
    let result = app.bet_service().place_bet(user.id, req).await;
    
    // Assert
    assert!(result.is_ok());
    let bet = result.unwrap();
    assert_eq!(bet.status, BetStatus::Pending);
    assert_eq!(bet.stake, dec!(100));
    assert_eq!(bet.potential_win, dec!(250));
    
    // Verify side effects
    let balance = app.get_balance(user.id).await;
    assert_eq!(balance.available, dec!(900));
    assert_eq!(balance.locked, dec!(100));
}

#[tokio::test]
async fn test_place_bet_idempotent() {
    let app = TestApp::start().await;
    let user = app.create_user(UserFixture::active()).await;
    app.credit_balance(user.id, dec!(1000)).await;
    app.create_event(EventFixture::football_open()).await;
    
    let key = Uuid::new_v4();
    let req = || PlaceBetRequest {
        idempotency_key: key,
        stake: dec!(100),
        ..default_bet_request()
    };
    
    let bet1 = app.bet_service().place_bet(user.id, req()).await.unwrap();
    let bet2 = app.bet_service().place_bet(user.id, req()).await.unwrap();
    
    assert_eq!(bet1.id, bet2.id); // same bet returned
    
    let balance = app.get_balance(user.id).await;
    assert_eq!(balance.available, dec!(900)); // only deducted once
}

#[tokio::test]
async fn test_concurrent_bets_no_overdraft() {
    let app = TestApp::start().await;
    let user = app.create_user(UserFixture::active()).await;
    app.credit_balance(user.id, dec!(100)).await;
    app.create_event(EventFixture::football_open()).await;
    
    // 10 concurrent bets of $20 each, but only $100 available
    let handles: Vec<_> = (0..10).map(|_| {
        let svc = app.bet_service().clone();
        let uid = user.id;
        tokio::spawn(async move {
            svc.place_bet(uid, PlaceBetRequest {
                stake: dec!(20),
                idempotency_key: Uuid::new_v4(),
                ..default_bet_request()
            }).await
        })
    }).collect();
    
    let results: Vec<_> = futures::future::join_all(handles).await;
    let successes = results.iter().filter(|r| r.as_ref().unwrap().is_ok()).count();
    let failures = results.iter().filter(|r| r.as_ref().unwrap().is_err()).count();
    
    assert_eq!(successes, 5);
    assert_eq!(failures, 5);
    
    let balance = app.get_balance(user.id).await;
    assert_eq!(balance.available, dec!(0));
    assert_eq!(balance.locked, dec!(100));
}

#[tokio::test]
async fn test_insufficient_balance_returns_error() {
    let app = TestApp::start().await;
    let user = app.create_user(UserFixture::active()).await;
    app.credit_balance(user.id, dec!(50)).await;
    app.create_event(EventFixture::football_open()).await;
    
    let result = app.bet_service().place_bet(user.id, PlaceBetRequest {
        stake: dec!(100),
        ..default_bet_request()
    }).await;
    
    assert!(matches!(result, Err(AppError::InsufficientBalance { .. })));
}
============================================================
SECTION 5: TEST INFRASTRUCTURE
============================================================
Rust

// tests/integration/common/setup.rs

pub struct TestApp {
    pub pool: PgPool,
    pub cache: CacheClient,
    _pg_container: ContainerAsync<Postgres>,
    _redis_container: ContainerAsync<Redis>,
    state: AppState,
}

impl TestApp {
    pub async fn start() -> Self {
        // Start containers
        let pg = Postgres::default().start().await.unwrap();
        let redis = Redis::default().start().await.unwrap();
        
        let pg_url = format!(
            "postgres://postgres:postgres@127.0.0.1:{}/postgres",
            pg.get_host_port_ipv4(5432).await.unwrap()
        );
        let redis_url = format!(
            "redis://127.0.0.1:{}",
            redis.get_host_port_ipv4(6379).await.unwrap()
        );
        
        let pool = PgPoolOptions::new()
            .max_connections(5)
            .connect(&pg_url).await.unwrap();
        
        sqlx::migrate!("./migrations").run(&pool).await.unwrap();
        
        let cache = CacheClient::new(&redis_url).await.unwrap();
        let producer = MockProducer::new(); // no real Redpanda in tests
        
        let state = AppState::new(test_config(), pool.clone(), cache.clone(), producer);
        
        Self { pool, cache, _pg_container: pg, _redis_container: redis, state }
    }
    
    pub fn bet_service(&self) -> &BetService { self.state.bet_service() }
    
    pub async fn create_user(&self, fixture: UserFixture) -> User {
        fixture.insert(&self.pool).await
    }
    
    pub async fn credit_balance(&self, user_id: UserId, amount: Decimal) {
        sqlx::query!(
            "INSERT INTO wallets (user_id, currency_code, balance, version) 
             VALUES ($1, 'USD', $2, 0) ON CONFLICT (user_id, currency_code) 
             DO UPDATE SET balance = wallets.balance + $2",
            user_id.0, amount
        ).execute(&self.pool).await.unwrap();
    }
    
    pub async fn get_balance(&self, user_id: UserId) -> WalletBalance {
        sqlx::query_as!(WalletBalance,
            "SELECT balance as available, locked_balance as locked FROM wallets WHERE user_id = $1",
            user_id.0
        ).fetch_one(&self.pool).await.unwrap()
    }
}
============================================================
SECTION 6: FIXTURES (test data factories)
============================================================
Rust

// tests/integration/common/fixtures.rs

use fake::{Fake, faker::internet::en::SafeEmail};

pub struct UserFixture {
    pub email: String,
    pub status: UserStatus,
    pub kyc_level: i32,
    pub country: String,
}

impl UserFixture {
    pub fn active() -> Self {
        Self {
            email: SafeEmail().fake(),
            status: UserStatus::Active,
            kyc_level: 2,
            country: "DE".into(),
        }
    }
    
    pub fn self_excluded() -> Self {
        Self { status: UserStatus::SelfExcluded, ..Self::active() }
    }
    
    pub async fn insert(self, pool: &PgPool) -> User {
        sqlx::query_as!(User,
            "INSERT INTO users (email, status, kyc_level, country_code, currency_code, password_hash)
             VALUES ($1, $2, $3, $4, 'USD', 'not_real_hash')
             RETURNING *",
            self.email, self.status as _, self.kyc_level, self.country
        ).fetch_one(pool).await.unwrap()
    }
}

pub fn default_bet_request() -> PlaceBetRequest {
    PlaceBetRequest {
        bet_type: BetType::Single,
        selections: vec![SelectionRequest {
            event_id: 1, market_id: 1, outcome_id: 1, odds: dec!(2.50),
        }],
        stake: dec!(10),
        currency_code: "USD".into(),
        idempotency_key: Uuid::new_v4(),
        accept_odds_changes: AcceptOddsChanges::Any,
        ip_address: None,
        device_fingerprint: None,
    }
}
============================================================
SECTION 7: BENCHMARKS
============================================================
Rust

// benches/bet_placement_bench.rs
use criterion::{criterion_group, criterion_main, Criterion, BenchmarkId};

fn bench_accumulator_odds(c: &mut Criterion) {
    let odds_sets: Vec<Vec<Decimal>> = vec![
        vec![dec!(1.50), dec!(2.00)],                              // 2 selections
        vec![dec!(1.50), dec!(2.00), dec!(1.80)],                  // 3
        vec![dec!(1.50), dec!(2.00), dec!(1.80), dec!(2.50)],      // 4
        (0..10).map(|_| dec!(1.50)).collect(),                     // 10
        (0..20).map(|_| dec!(1.50)).collect(),                     // 20
    ];
    
    let mut group = c.benchmark_group("accumulator_odds");
    for odds in &odds_sets {
        group.bench_with_input(
            BenchmarkId::new("selections", odds.len()),
            odds,
            |b, odds| b.iter(|| BetCalculator::accumulator_odds(odds)),
        );
    }
    group.finish();
}

fn bench_margin_calculation(c: &mut Criterion) {
    let odds = vec![dec!(2.10), dec!(3.40), dec!(3.20)];
    c.bench_function("margin_3way", |b| {
        b.iter(|| calculate_margin(&odds))
    });
}

criterion_group!(benches, bench_accumulator_odds, bench_margin_calculation);
criterion_main!(benches);
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
text

❌ NEVER test implementation details (private functions, internal state)
   ✅ Test public API behavior and outcomes

❌ NEVER use sleep() in tests to wait for async operations
   ✅ Use channels, semaphores, or poll with timeout

❌ NEVER share mutable state between tests (tests run in parallel)
   ✅ Each test creates its own TestApp with fresh containers

❌ NEVER write tests that depend on execution order
   ✅ Each test is independent and self-contained

❌ NEVER skip testing error paths (only happy path)
   ✅ Test every Err variant and edge case

❌ NEVER use production database for tests
   ✅ Use testcontainers (fresh DB per test or test suite)

❌ NEVER test with hardcoded IDs that might conflict
   ✅ Use UUID or auto-generated IDs

❌ NEVER ignore flaky tests (fix root cause)
   ✅ Flaky = likely race condition → fix the code