//! Ledger repository for double-entry bookkeeping

use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::{LedgerEntry, ReconciliationResult};

/// Columns selected wherever a `ledger_entries` row is loaded.
const LEDGER_COLUMNS: &str = "id, transaction_id, account_type, account_id, entry_type, amount, \
                              currency, balance_after, reference_type, reference_id, \
                              idempotency_key, created_at";

/// Ledger repository
pub struct LedgerRepository {
    pub pool: PgPool,
}

impl LedgerRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    /// Insert a single ledger entry within a transaction.
    async fn insert_entry(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        entry: &LedgerEntry,
    ) -> Result<(), sqlx::Error> {
        sqlx::query(
            r#"
            INSERT INTO ledger_entries (
                id, transaction_id, account_type, account_id,
                entry_type, amount, currency, balance_after,
                reference_type, reference_id, idempotency_key
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            "#,
        )
        .bind(entry.id)
        .bind(entry.transaction_id)
        .bind(entry.account_type)
        .bind(&entry.account_id)
        .bind(entry.entry_type)
        .bind(entry.amount)
        .bind(&entry.currency)
        .bind(entry.balance_after)
        .bind(entry.reference_type.as_deref())
        .bind(entry.reference_id)
        .bind(entry.idempotency_key.as_deref())
        .execute(&mut **tx)
        .await?;

        Ok(())
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
        self.insert_entry(tx, &debit).await?;
        self.insert_entry(tx, &credit).await?;

        Ok(())
    }

    /// Get entries by transaction ID
    pub async fn get_by_transaction(
        &self,
        transaction_id: Uuid,
    ) -> Result<Vec<LedgerEntry>, sqlx::Error> {
        let sql = format!(
            "SELECT {LEDGER_COLUMNS} FROM ledger_entries WHERE transaction_id = $1 \
             ORDER BY created_at"
        );
        let entries = sqlx::query_as::<_, LedgerEntry>(&sql)
            .bind(transaction_id)
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
        let sql = format!(
            "SELECT {LEDGER_COLUMNS} FROM ledger_entries \
             WHERE account_type = $1 AND account_id = $2 \
             ORDER BY created_at DESC LIMIT $3"
        );
        let entries = sqlx::query_as::<_, LedgerEntry>(&sql)
            .bind(account_type)
            .bind(account_id)
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;

        Ok(entries)
    }

    /// Run reconciliation check
    pub async fn reconcile_wallet(
        &self,
        wallet_id: Uuid,
    ) -> Result<ReconciliationResult, sqlx::Error> {
        let sql = "SELECT * FROM wallet_reconciliation WHERE wallet_id = $1";
        let result = sqlx::query_as::<_, ReconciliationResult>(sql)
            .bind(wallet_id)
            .fetch_optional(&self.pool)
            .await?;

        result.ok_or(sqlx::Error::RowNotFound)
    }

    /// Get all reconciliation alerts (discrepancy > $0.01)
    pub async fn get_reconciliation_alerts(
        &self,
    ) -> Result<Vec<ReconciliationResult>, sqlx::Error> {
        sqlx::query_as::<_, ReconciliationResult>("SELECT * FROM wallet_reconciliation_alerts")
            .fetch_all(&self.pool)
            .await
    }
}
