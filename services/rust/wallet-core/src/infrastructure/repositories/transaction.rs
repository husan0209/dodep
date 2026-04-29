//! Transaction repository

use sqlx::{PgPool, Postgres};
use uuid::Uuid;

use crate::domain::{Transaction, TransactionStatus};

/// Transaction repository
pub struct TransactionRepository {
    pub pool: PgPool,
}

impl TransactionRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }
    
    /// Get transaction by ID
    pub async fn get_by_id(&self, id: Uuid) -> Result<Option<Transaction>, sqlx::Error> {
        let transaction = sqlx::query_as!(
            Transaction,
            r#"
            SELECT 
                id, user_id, wallet_id, wallet_type as "WalletType: _",
                transaction_type as "TransactionType: _",
                amount as "Decimal: rust_decimal::Decimal",
                currency, status as "TransactionStatus: _",
                reference_id, reference_type, idempotency_key,
                description, metadata as "serde_json::Value: _",
                created_at, updated_at, completed_at
            FROM transactions
            WHERE id = $1
            "#,
            id
        )
        .fetch_optional(&self.pool)
        .await?;
        
        Ok(transaction)
    }
    
    /// Get transaction by idempotency key
    pub async fn get_by_idempotency_key(
        &self,
        idempotency_key: &str,
    ) -> Result<Option<Transaction>, sqlx::Error> {
        let transaction = sqlx::query_as!(
            Transaction,
            r#"
            SELECT 
                id, user_id, wallet_id, wallet_type as "WalletType: _",
                transaction_type as "TransactionType: _",
                amount as "Decimal: rust_decimal::Decimal",
                currency, status as "TransactionStatus: _",
                reference_id, reference_type, idempotency_key,
                description, metadata as "serde_json::Value: _",
                created_at, updated_at, completed_at
            FROM transactions
            WHERE idempotency_key = $1
            "#,
            idempotency_key
        )
        .fetch_optional(&self.pool)
        .await?;
        
        Ok(transaction)
    }

    /// Save transaction within an existing database transaction
    pub async fn save_with_tx(
        tx: &mut sqlx::Transaction<'_, Postgres>,
        transaction: &Transaction,
    ) -> Result<Transaction, crate::domain::WalletError> {
        let updated = sqlx::query_as!(
            Transaction,
            r#"
            INSERT INTO transactions (
                id, user_id, wallet_id, wallet_type, transaction_type,
                amount, currency, status, reference_id, reference_type,
                idempotency_key, description, metadata
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
            )
            RETURNING 
                id, user_id, wallet_id, wallet_type as "WalletType: _",
                transaction_type as "TransactionType: _",
                amount as "Decimal: rust_decimal::Decimal",
                currency, status as "TransactionStatus: _",
                reference_id, reference_type, idempotency_key,
                description, metadata as "serde_json::Value: _",
                created_at, updated_at, completed_at
            "#,
            transaction.id,
            transaction.user_id,
            transaction.wallet_id,
            transaction.wallet_type as _,
            transaction.transaction_type as _,
            transaction.amount,
            transaction.currency,
            transaction.status as _,
            transaction.reference_id,
            transaction.reference_type,
            transaction.idempotency_key,
            transaction.description,
            transaction.metadata as _,
        )
        .fetch_one(&mut **tx)
        .await
        .map_err(|e| crate::domain::WalletError::DatabaseError(e.to_string()))?;

        Ok(updated)
    }
}
