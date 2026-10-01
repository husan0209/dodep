//! Wallet repository

use chrono::Utc;
use rust_decimal::Decimal;
use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::{Wallet, WalletType, WalletError};

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
        user_id: i64,
        wallet_type: WalletType,
    ) -> Result<Option<Wallet>, WalletError> {
        // Manual mapping: sqlx checked macros decode the custom
        // `wallet_type` enum as String; parse it back via FromStr.
        let row = sqlx::query!(
            r#"
            SELECT
                id, user_id, wallet_type::text AS wallet_type,
                currency, balance_available,
                balance_locked,
                balance_bonus,
                version, is_active, created_at, updated_at
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            "#,
            user_id,
            wallet_type as WalletType
        )
        .fetch_optional(&self.pool)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        match row {
            Some(r) => Ok(Some(Wallet {
                id: r.id,
                user_id: r.user_id,
                wallet_type: r.wallet_type.as_deref().and_then(|s| s.parse().ok()).unwrap_or(WalletType::Main),
                currency: r.currency,
                balance_available: r.balance_available,
                balance_locked: r.balance_locked,
                balance_bonus: r.balance_bonus,
                version: r.version,
                is_active: r.is_active,
                created_at: r.created_at,
                updated_at: r.updated_at,
            })),
            None => Ok(None),
        }
    }
    
    /// Get wallet by user and type within a transaction (with row lock)
    pub async fn get_by_user_and_type_internal(
        &self,
        user_id: i64,
        wallet_type: WalletType,
        tx: &mut Transaction<'_, Postgres>,
    ) -> Result<Option<Wallet>, WalletError> {
        let row = sqlx::query!(
            r#"
            SELECT
                id, user_id, wallet_type::text AS wallet_type,
                currency, balance_available,
                balance_locked,
                balance_bonus,
                version, is_active, created_at, updated_at
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            FOR UPDATE
            "#,
            user_id,
            wallet_type as WalletType
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        match row {
            Some(r) => Ok(Some(Wallet {
                id: r.id,
                user_id: r.user_id,
                wallet_type: r.wallet_type.as_deref().and_then(|s| s.parse().ok()).unwrap_or(WalletType::Main),
                currency: r.currency,
                balance_available: r.balance_available,
                balance_locked: r.balance_locked,
                balance_bonus: r.balance_bonus,
                version: r.version,
                is_active: r.is_active,
                created_at: r.created_at,
                updated_at: r.updated_at,
            })),
            None => Ok(None),
        }
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
        let result = sqlx::query!(
            r#"
            UPDATE wallets
            SET 
                balance_available = $2,
                balance_locked = $3,
                balance_bonus = $4,
                version = version + 1,
                updated_at = NOW()
            WHERE id = $1 AND version = $5
            RETURNING id
            "#,
            id,
            available,
            locked,
            bonus,
            version,
        )
        .fetch_optional(&self.pool)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        if result.is_none() {
            return Err(WalletError::ConcurrencyConflict);
        }
        
        Ok(())
    }
}
