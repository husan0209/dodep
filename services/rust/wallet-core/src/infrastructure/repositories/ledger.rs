//! Ledger repository for double-entry bookkeeping

use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::{LedgerEntry, ReconciliationResult};

/// Ledger repository
pub struct LedgerRepository {
    pub pool: PgPool,
}

impl LedgerRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }
    
    /// Insert a ledger entry pair (debit + credit)
    pub async fn insert_pair(
        &self,
        debit: LedgerEntry,
        credit: LedgerEntry,
    ) -> Result<(), sqlx::Error> {
        let mut tx = self.pool.begin().await?;
        
        self.insert_pair_internal(&mut tx, debit, credit).await?;
        
        tx.commit().await?;
        
        Ok(())
    }
    
    /// Insert a ledger entry pair within a transaction
    pub async fn insert_pair_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        debit: LedgerEntry,
        credit: LedgerEntry,
    ) -> Result<(), sqlx::Error> {
        // Insert debit entry
        sqlx::query!(
            r#"
            INSERT INTO ledger_entries (
                id, transaction_id, account_type, account_id,
                entry_type, amount, currency, balance_after,
                reference_type, reference_id, idempotency_key
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            "#,
            debit.id,
            debit.transaction_id,
            debit.account_type as _,
            &debit.account_id,
            debit.entry_type as _,
            debit.amount as _,
            &debit.currency,
            debit.balance_after as Option<_>,
            debit.reference_type.as_deref(),
            debit.reference_id,
            debit.idempotency_key,
        )
        .execute(&mut **tx)
        .await?;
        
        // Insert credit entry
        sqlx::query!(
            r#"
            INSERT INTO ledger_entries (
                id, transaction_id, account_type, account_id,
                entry_type, amount, currency, balance_after,
                reference_type, reference_id, idempotency_key
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            "#,
            credit.id,
            credit.transaction_id,
            credit.account_type as _,
            &credit.account_id,
            credit.entry_type as _,
            credit.amount as _,
            &credit.currency,
            credit.balance_after as Option<_>,
            credit.reference_type.as_deref(),
            credit.reference_id,
            credit.idempotency_key,
        )
        .execute(&mut **tx)
        .await?;
        
        Ok(())
    }
    
    /// Get entries by transaction ID
    pub async fn get_by_transaction(&self, transaction_id: Uuid) -> Result<Vec<LedgerEntry>, sqlx::Error> {
        let entries = sqlx::query_as!(
            LedgerEntry,
            r#"
            SELECT
                id, transaction_id,
                account_type as "AccountType: _",
                account_id,
                entry_type as "LedgerEntryType: _",
                amount as "Decimal: rust_decimal::Decimal",
                currency, balance_after as "Decimal: rust_decimal::Decimal",
                reference_type, reference_id, idempotency_key,
                created_at
            FROM ledger_entries
            WHERE transaction_id = $1
            ORDER BY created_at
            "#,
            transaction_id
        )
        .fetch_all(&self.pool)
        .await?;
        
        Ok(entries)
    }
    
    /// Get entries by account
    pub async fn get_by_account(
        &self,
        account_type: crate::domain::AccountType,
        account_id: &str,
        limit: i64,
    ) -> Result<Vec<LedgerEntry>, sqlx::Error> {
        let entries = sqlx::query_as!(
            LedgerEntry,
            r#"
            SELECT
                id, transaction_id,
                account_type as "AccountType: _",
                account_id,
                entry_type as "LedgerEntryType: _",
                amount as "Decimal: rust_decimal::Decimal",
                currency, balance_after as "Decimal: rust_decimal::Decimal",
                reference_type, reference_id, idempotency_key,
                created_at
            FROM ledger_entries
            WHERE account_type = $1 AND account_id = $2
            ORDER BY created_at DESC
            LIMIT $3
            "#,
            account_type as _,
            account_id,
            limit
        )
        .fetch_all(&self.pool)
        .await?;
        
        Ok(entries)
    }
    
    /// Run reconciliation check
    pub async fn reconcile_wallet(
        &self,
        wallet_id: Uuid,
    ) -> Result<ReconciliationResult, sqlx::Error> {
        let result = sqlx::query!(
            r#"
            SELECT * FROM wallet_reconciliation
            WHERE wallet_id = $1
            "#,
            wallet_id
        )
        .fetch_optional(&self.pool)
        .await?;
        
        match result {
            Some(row) => {
                use sqlx::Row;
                Ok(ReconciliationResult {
                    wallet_id: row.try_get("wallet_id")?,
                    user_id: row.try_get("user_id")?,
                    wallet_type: row.try_get("wallet_type")?,
                    currency: row.try_get("currency")?,
                    actual_balance: row.try_get("actual_balance")?,
                    expected_balance: row.try_get("expected_balance")?,
                    discrepancy: row.try_get("discrepancy")?,
                })
            }
            None => Err(sqlx::Error::RowNotFound),
        }
    }
    
    /// Get all reconciliation alerts (discrepancy > $0.01)
    pub async fn get_reconciliation_alerts(
        &self,
    ) -> Result<Vec<ReconciliationResult>, sqlx::Error> {
        let results = sqlx::query!(
            r#"
            SELECT * FROM wallet_reconciliation_alerts
            "#
        )
        .fetch_all(&self.pool)
        .await?;
        
        // Map to ReconciliationResult
        // Note: This requires custom mapping since we're using a view
        Ok(Vec::new())  // TODO: implement proper mapping
    }
}
