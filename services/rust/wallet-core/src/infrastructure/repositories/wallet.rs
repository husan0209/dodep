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
        user_id: Uuid,
        wallet_type: WalletType,
    ) -> Result<Option<Wallet>, WalletError> {
        let wallet = sqlx::query_as!(
            Wallet,
            r#"
            SELECT 
                id, user_id, wallet_type as "WalletType: _",
                currency, balance_available as "Decimal: rust_decimal::Decimal",
                balance_locked as "Decimal: rust_decimal::Decimal",
                balance_bonus as "Decimal: rust_decimal::Decimal",
                version, is_active, created_at, updated_at
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            "#,
            user_id,
            wallet_type as _
        )
        .fetch_optional(&self.pool)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        Ok(wallet)
    }
    
    /// Get wallet by user and type within a transaction (with row lock)
    pub async fn get_by_user_and_type_internal(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
        tx: &mut Transaction<'_, Postgres>,
    ) -> Result<Option<Wallet>, WalletError> {
        let wallet = sqlx::query_as!(
            Wallet,
            r#"
            SELECT 
                id, user_id, wallet_type as "WalletType: _",
                currency, balance_available as "Decimal: rust_decimal::Decimal",
                balance_locked as "Decimal: rust_decimal::Decimal",
                balance_bonus as "Decimal: rust_decimal::Decimal",
                version, is_active, created_at, updated_at
            FROM wallets
            WHERE user_id = $1 AND wallet_type = $2 AND is_active = true
            FOR UPDATE
            "#,
            user_id,
            wallet_type as _
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
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
        let result = sqlx::query!(
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
            id,
            version,
            available as _,
            locked as _,
            bonus as _,
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
