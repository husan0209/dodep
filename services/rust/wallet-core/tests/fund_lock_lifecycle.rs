//! Fund-lock lifecycle tests against a real PostgreSQL + Redis pair.
//!
//! These cover the money-critical invariants of the reservation flow:
//!   * Lock is idempotent per (user, reference_type, reference) — a retry must
//!     not reserve the funds twice.
//!   * ConsumeLock settles the reservation ONCE. Before this operation existed,
//!     a successful payout called Debit on top of the already reserved amount,
//!     charging the player twice.
//!   * Unlock returns the funds and is safe to replay.
//!   * A reservation can never be released or settled by another user.
//!
//! Requires DATABASE_URL (PostgreSQL with the wallet schema applied) and
//! REDIS_URL (idempotency cache) — the same prerequisites the crate already
//! needs to compile, because sqlx checks its queries against a live database.

use std::sync::Arc;

use rust_decimal::Decimal;
use sqlx::PgPool;
use uuid::Uuid;

use wallet_core::domain::{WalletError, WalletType};
use wallet_core::infrastructure::repositories::{
    LedgerRepository, LockRepository, OutboxRepository, TransactionRepository, WalletRepository,
};
use wallet_core::infrastructure::RedisClient;
use wallet_core::service::{IdempotencyService, WalletService};

struct Fixture {
    svc: WalletService,
    pool: PgPool,
    user_id: i64,
}

async fn fixture(amount: &str) -> Fixture {
    let db_url = std::env::var("DATABASE_URL")
        .expect("DATABASE_URL must point at PostgreSQL with the wallet schema");
    let redis_url =
        std::env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".to_string());

    let pool = PgPool::connect(&db_url).await.expect("connect postgres");
    let redis: RedisClient = redis::Client::open(redis_url).expect("redis url");

    let svc = WalletService::new(
        Arc::new(WalletRepository::new(pool.clone())),
        Arc::new(TransactionRepository::new(pool.clone())),
        Arc::new(LedgerRepository::new(pool.clone())),
        Arc::new(LockRepository::new()),
        Arc::new(OutboxRepository::new(pool.clone())),
        Arc::new(IdempotencyService::new(redis, 3600)),
    );

    // Isolated user per test keeps balances independent.
    let user_id = -(i64::from(u16::from_be_bytes(
        Uuid::new_v4().as_bytes()[0..2].try_into().expect("2 bytes"),
    )) as i64)
        - 10_000;

    let wallet = svc
        .credit(
            user_id,
            WalletType::Main,
            Decimal::from_str_exact(amount).expect("amount"),
            "USD",
            format!("test-credit-{user_id}"),
            "deposit",
            &format!("idem-credit-{user_id}"),
        )
        .await
        .expect("seed wallet");

    assert_eq!(wallet.user_id, user_id);
    let _ = pool;
    Fixture { svc, pool, user_id }
}

async fn balance_of(fx: &Fixture) -> (Decimal, Decimal) {
    let b = fx
        .svc
        .get_balance(fx.user_id, WalletType::Main)
        .await
        .expect("balance");
    (b.available, b.locked)
}

async fn active_locks(fx: &Fixture, reference_id: &str) -> i64 {
    sqlx::query_scalar::<_, i64>(
        "SELECT COUNT(*) FROM fund_locks WHERE user_id = $1 AND reference_id = $2 AND is_active = true",
    )
    .bind(fx.user_id)
    .bind(reference_id)
    .fetch_one(&fx.pool)
    .await
    .expect("count locks")
}

/// Locking twice for the same business reference must reserve the funds once.
#[tokio::test]
async fn lock_is_idempotent_for_the_same_reference() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    let first = fx
        .svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("first lock");

    let second = fx
        .svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("replayed lock");

    assert_eq!(
        first.id, second.id,
        "replay must return the same reservation"
    );
    let (available, locked) = balance_of(&fx).await;
    assert_eq!(available, Decimal::from(600), "funds reserved once");
    assert_eq!(locked, Decimal::from(400), "locked once, not twice");
    assert_eq!(active_locks(&fx, &reference).await, 1);
}

/// A replay for the same reference with a different amount is a caller bug and
/// must not silently reserve the new amount.
#[tokio::test]
async fn lock_rejects_amount_mismatch_for_active_reference() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("first lock");

    let err = fx
        .svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(700),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect_err("amount mismatch must be rejected");

    assert!(
        matches!(err, WalletError::BusinessRuleViolation(_)),
        "got {err:?}"
    );
    let (available, locked) = balance_of(&fx).await;
    assert_eq!(
        (available, locked),
        (Decimal::from(600), Decimal::from(400))
    );
}

/// The regression this service exists for: a confirmed crypto payout must
/// settle the reservation exactly once, not debit the player again.
#[tokio::test]
async fn consume_lock_settles_the_reserved_amount_once() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("lock");

    let settlement_key = format!("settle-{}", Uuid::new_v4());
    let txn = fx
        .svc
        .consume_lock(
            fx.user_id,
            "withdrawal".into(),
            reference.clone(),
            settlement_key.clone(),
            Some("crypto payout".into()),
        )
        .await
        .expect("settle");

    // The amount left the platform exactly once: total drops by 400, not 800.
    let (available, locked) = balance_of(&fx).await;
    assert_eq!(
        available,
        Decimal::from(600),
        "available was reduced at lock time"
    );
    assert_eq!(locked, Decimal::ZERO, "reservation settled");
    assert_eq!(txn.amount, Decimal::from(400));

    // Replaying the same idempotency key returns the same transaction and does
    // not touch the balances again.
    let replay = fx
        .svc
        .consume_lock(
            fx.user_id,
            "withdrawal".into(),
            reference.clone(),
            settlement_key,
            Some("crypto payout".into()),
        )
        .await
        .expect("replayed settlement");
    assert_eq!(replay.id, txn.id);
    let (available_after, locked_after) = balance_of(&fx).await;
    assert_eq!(available_after, available);
    assert_eq!(locked_after, locked);

    // The lock is terminal and no longer active.
    assert_eq!(active_locks(&fx, &reference).await, 0);
    let consumed_at: Option<chrono::DateTime<chrono::Utc>> = sqlx::query_scalar(
        "SELECT consumed_at FROM fund_locks WHERE user_id = $1 AND reference_id = $2",
    )
    .bind(fx.user_id)
    .bind(&reference)
    .fetch_one(&fx.pool)
    .await
    .expect("lock row");
    assert!(
        consumed_at.is_some(),
        "settlement must be recorded on the lock"
    );
}

/// A second settle with a *different* idempotency key must be refused: the
/// reservation is already terminal, so the money is not debited twice.
#[tokio::test]
async fn consume_lock_refuses_an_already_settled_reservation() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("lock");
    fx.svc
        .consume_lock(
            fx.user_id,
            "withdrawal".into(),
            reference.clone(),
            format!("settle-{}", Uuid::new_v4()),
            None,
        )
        .await
        .expect("settle");

    let err = fx
        .svc
        .consume_lock(
            fx.user_id,
            "withdrawal".into(),
            reference.clone(),
            format!("settle-{}", Uuid::new_v4()),
            None,
        )
        .await
        .expect_err("second settlement must be refused");
    assert!(
        matches!(
            err,
            WalletError::LockReferenceNotFound(_) | WalletError::LockAlreadySettled(_)
        ),
        "got {err:?}"
    );

    let (available, locked) = balance_of(&fx).await;
    assert_eq!((available, locked), (Decimal::from(600), Decimal::ZERO));
}

/// Unlock restores the funds and replaying the compensation is a no-op.
#[tokio::test]
async fn unlock_restores_funds_and_is_replayable() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("lock");

    assert!(fx
        .svc
        .unlock(fx.user_id, "withdrawal".into(), reference.clone())
        .await
        .expect("unlock"));

    let (available, locked) = balance_of(&fx).await;
    assert_eq!((available, locked), (Decimal::from(1000), Decimal::ZERO));

    // Compensation replay (e.g. a retried webhook): the reservation is gone,
    // so there is nothing to release and nothing to credit.
    let err = fx
        .svc
        .unlock(fx.user_id, "withdrawal".into(), reference.clone())
        .await
        .expect_err("already released reservation");
    assert!(
        matches!(err, WalletError::LockReferenceNotFound(_)),
        "got {err:?}"
    );
    let (available_after, locked_after) = balance_of(&fx).await;
    assert_eq!(
        (available_after, locked_after),
        (Decimal::from(1000), Decimal::ZERO)
    );
}

/// One user must never be able to release or settle another user's money.
#[tokio::test]
async fn reservations_are_scoped_to_their_owner() {
    let fx = fixture("1000").await;
    let attacker = fx.user_id - 1;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("lock");

    let unlock_err = fx
        .svc
        .unlock(attacker, "withdrawal".into(), reference.clone())
        .await
        .expect_err("cross-user unlock must fail");
    assert!(
        matches!(unlock_err, WalletError::LockReferenceNotFound(_)),
        "got {unlock_err:?}"
    );

    let settle_err = fx
        .svc
        .consume_lock(
            attacker,
            "withdrawal".into(),
            reference.clone(),
            format!("x-{}", Uuid::new_v4()),
            None,
        )
        .await
        .expect_err("cross-user settle must fail");
    assert!(
        matches!(settle_err, WalletError::LockReferenceNotFound(_)),
        "got {settle_err:?}"
    );

    // The owner's reservation is untouched.
    let (available, locked) = balance_of(&fx).await;
    assert_eq!(
        (available, locked),
        (Decimal::from(600), Decimal::from(400))
    );
}

/// A released reservation may be re-created for the same reference (the
/// business flow legitimately restarts), while the old lock stays terminal.
#[tokio::test]
async fn released_reference_can_be_locked_again() {
    let fx = fixture("1000").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    fx.svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(400),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("lock");
    fx.svc
        .unlock(fx.user_id, "withdrawal".into(), reference.clone())
        .await
        .expect("unlock");

    let relock = fx
        .svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(250),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect("re-lock after release");
    assert!(relock.is_active);

    let (available, locked) = balance_of(&fx).await;
    assert_eq!(
        (available, locked),
        (Decimal::from(750), Decimal::from(250))
    );
    assert_eq!(active_locks(&fx, &reference).await, 1);
}

/// Locking more than the available balance must fail and leave the wallet as is.
#[tokio::test]
async fn lock_rejects_insufficient_available_balance() {
    let fx = fixture("100").await;
    let reference = format!("withdrawal:{}", Uuid::new_v4());

    let err = fx
        .svc
        .lock(
            fx.user_id,
            WalletType::Main,
            Decimal::from(500),
            reference.clone(),
            "withdrawal".into(),
        )
        .await
        .expect_err("insufficient funds must be rejected");
    assert!(
        matches!(err, WalletError::InsufficientAvailableBalance { .. }),
        "got {err:?}"
    );

    let (available, locked) = balance_of(&fx).await;
    assert_eq!((available, locked), (Decimal::from(100), Decimal::ZERO));
    assert_eq!(active_locks(&fx, &reference).await, 0);
}
