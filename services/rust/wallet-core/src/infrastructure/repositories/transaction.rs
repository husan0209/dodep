//! Transaction repository

use sqlx::{PgPool, Postgres};
use uuid::Uuid;

use crate::domain::{Transaction, WalletError};

/// Columns selected wherever a `transactions` row is loaded.
const TRANSACTION_COLUMNS: &str = "id, user_id, wallet_id, wallet_type, transaction_type, \
                                  amount, currency, status, reference_id, reference_type, \
                                  idempotency_key, description, metadata, created_at, \
                                  updated_at, completed_at";

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
        let sql = format!("SELECT {TRANSACTION_COLUMNS} FROM transactions WHERE id = $1");
        let transaction = sqlx::query_as::<_, Transaction>(&sql)
            .bind(id)
            .fetch_optional(&self.pool)
            .await?;

        Ok(transaction)
    }

    /// Get transaction by idempotency key
    pub async fn get_by_idempotency_key(
        &self,
        idempotency_key: &str,
    ) -> Result<Option<Transaction>, sqlx::Error> {
        let sql =
            format!("SELECT {TRANSACTION_COLUMNS} FROM transactions WHERE idempotency_key = $1");
        let transaction = sqlx::query_as::<_, Transaction>(&sql)
            .bind(idempotency_key)
            .fetch_optional(&self.pool)
            .await?;

        Ok(transaction)
    }

    /// Save transaction within an existing database transaction
    pub async fn save_with_tx(
        tx: &mut sqlx::Transaction<'_, Postgres>,
        transaction: &Transaction,
    ) -> Result<Transaction, WalletError> {
        let sql = format!(
            r#"
            INSERT INTO transactions (
                id, user_id, wallet_id, wallet_type, transaction_type,
                amount, currency, status, reference_id, reference_type,
                idempotency_key, description, metadata
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
            )
            RETURNING {TRANSACTION_COLUMNS}
            "#
        );

        let updated = sqlx::query_as::<_, Transaction>(&sql)
            .bind(transaction.id)
            .bind(transaction.user_id)
            .bind(transaction.wallet_id)
            .bind(transaction.wallet_type)
            .bind(transaction.transaction_type)
            .bind(transaction.amount)
            .bind(&transaction.currency)
            .bind(transaction.status)
            .bind(transaction.reference_id)
            .bind(&transaction.reference_type)
            .bind(&transaction.idempotency_key)
            .bind(&transaction.description)
            .bind(&transaction.metadata)
            .fetch_one(&mut **tx)
            .await?;

        Ok(updated)
    }
}
