//! Wallet repository

use rust_decimal::Decimal;
use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::{Wallet, WalletError, WalletType};

/// Columns selected wherever a `wallets` row is loaded.
const WALLET_COLUMNS: &str = "id, user_id, wallet_type, currency, balance_available, \
                             balance_locked, balance_bonus, version, is_active, created_at, \
                             updated_at";

/// Wallet repository
pub struct WalletRepository {
    pub pool: PgPool,
}

impl WalletRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    /// Get wallet by user and type
    pub async fn get_by_user_and_type(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
    ) -> Result<Option<Wallet>, WalletError> {
        let sql = format!(
            r#"
            SELECT {WALLET_COLUMNS}
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            "#
        );

        let wallet = sqlx::query_as::<_, Wallet>(&sql)
            .bind(user_id)
            .bind(wallet_type)
            .fetch_optional(&self.pool)
            .await?;

        Ok(wallet)
    }

    /// Get wallet by user and type within a transaction (with row lock)
    pub async fn get_by_user_and_type_internal(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
        tx: &mut Transaction<'_, Postgres>,
    ) -> Result<Option<Wallet>, WalletError> {
        let sql = format!(
            r#"
            SELECT {WALLET_COLUMNS}
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            FOR UPDATE
            "#
        );

        let wallet = sqlx::query_as::<_, Wallet>(&sql)
            .bind(user_id)
            .bind(wallet_type)
            .fetch_optional(&mut **tx)
            .await?;

        Ok(wallet)
    }

    /// Update wallet balance with optimistic locking
    pub async fn update_balances(
        &self,
        id: Uuid,
        available: Decimal,
        locked: Decimal,
        bonus: Decimal,
        version: i32,
    ) -> Result<(), WalletError> {
        let result = sqlx::query(
            r#"
            UPDATE wallets
            SET 
                balance_available = $3,
                balance_locked = $4,
                balance_bonus = $5,
                version = version + 1,
                updated_at = NOW()
            WHERE id = $1 AND version = $2
            RETURNING id
            "#,
        )
        .bind(id)
        .bind(version)
        .bind(available)
        .bind(locked)
        .bind(bonus)
        .fetch_optional(&self.pool)
        .await?;

        if result.is_none() {
            return Err(WalletError::concurrency_conflict());
        }

        Ok(())
    }
}
