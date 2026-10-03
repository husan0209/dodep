//! Fund lock repository
//!
//! Lock lifecycle:
//!   active ──unlock──▶ released (money returned to available)
//!   active ──consume─▶ consumed (money left the platform, booked as a debit)
//!
//! Both terminal transitions are single-shot: `WHERE is_active = true` guards
//! the UPDATE, so a replayed unlock/consume affects zero rows instead of
//! double-crediting the wallet.

use rust_decimal::Decimal;
use sqlx::{Postgres, Transaction};
use uuid::Uuid;

use crate::domain::{FundLock, WalletError};

/// Fund lock repository
pub struct LockRepository {}

impl LockRepository {
    pub fn new() -> Self {
        Self {}
    }

    /// Find the active lock owned by (user, reference_type, reference_id).
    pub async fn get_active_by_reference_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        user_id: i64,
        reference_type: &str,
        reference_id: &str,
    ) -> Result<Option<FundLock>, WalletError> {
        let lock = sqlx::query_as!(
            FundLock,
            r#"
            SELECT
                id, wallet_id, user_id, amount as "amount: Decimal",
                reference_id, reference_type, is_active,
                created_at, released_at, consumed_at, settlement_transaction_id
            FROM fund_locks
            WHERE user_id = $1
              AND reference_type = $2
              AND reference_id = $3
              AND is_active = true
            FOR UPDATE
            "#,
            user_id,
            reference_type,
            reference_id,
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(lock)
    }

    /// Look up a lock by its own handle (id).
    pub async fn get_by_id_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        lock_id: Uuid,
    ) -> Result<Option<FundLock>, WalletError> {
        let lock = sqlx::query_as!(
            FundLock,
            r#"
            SELECT
                id, wallet_id, user_id, amount as "amount: Decimal",
                reference_id, reference_type, is_active,
                created_at, released_at, consumed_at, settlement_transaction_id
            FROM fund_locks
            WHERE id = $1
            "#,
            lock_id,
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(lock)
    }

    /// Insert a new active lock.
    pub async fn insert_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        lock: &FundLock,
    ) -> Result<(), WalletError> {
        sqlx::query!(
            r#"
            INSERT INTO fund_locks (
                id, wallet_id, user_id, amount,
                reference_id, reference_type, is_active
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            "#,
            lock.id,
            lock.wallet_id,
            lock.user_id,
            lock.amount as _,
            lock.reference_id,
            &lock.reference_type,
            lock.is_active,
        )
        .execute(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(())
    }

    /// Return the locked amount to the available balance.
    /// Returns true when this call performed the transition.
    pub async fn release_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        lock_id: Uuid,
    ) -> Result<bool, WalletError> {
        let result = sqlx::query!(
            r#"
            UPDATE fund_locks
            SET is_active = false, released_at = NOW()
            WHERE id = $1 AND is_active = true
            "#,
            lock_id,
        )
        .execute(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(result.rows_affected() > 0)
    }

    /// Mark the lock as settled by `settlement_transaction_id`.
    /// Returns true when this call performed the transition.
    pub async fn consume_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        lock_id: Uuid,
        settlement_transaction_id: Uuid,
    ) -> Result<bool, WalletError> {
        let result = sqlx::query!(
            r#"
            UPDATE fund_locks
            SET is_active = false,
                consumed_at = NOW(),
                settlement_transaction_id = $2
            WHERE id = $1 AND is_active = true
            "#,
            lock_id,
            settlement_transaction_id,
        )
        .execute(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(result.rows_affected() > 0)
    }

    /// Amount currently locked by the user (diagnostics / reconciliation).
    pub async fn total_active_locked(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        user_id: i64,
    ) -> Result<Decimal, WalletError> {
        let row = sqlx::query!(
            r#"
            SELECT COALESCE(SUM(amount), 0) AS total
            FROM fund_locks
            WHERE user_id = $1 AND is_active = true
            "#,
            user_id,
        )
        .fetch_one(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        Ok(row.total.unwrap_or(Decimal::ZERO))
    }
}

impl Default for LockRepository {
    fn default() -> Self {
        Self::new()
    }
}
