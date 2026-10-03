//! Fund lock repository

use sqlx::PgPool;
use uuid::Uuid;

use crate::domain::FundLock;

/// Columns selected wherever a `fund_locks` row is loaded.
const LOCK_COLUMNS: &str =
    "id, wallet_id, user_id, amount, reference_id, reference_type, is_active, created_at, \
     released_at";

/// Fund lock repository
pub struct LockRepository {
    pub pool: PgPool,
}

impl LockRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    /// Create a new lock
    pub async fn create(&self, lock: &FundLock) -> Result<(), sqlx::Error> {
        sqlx::query(
            r#"
            INSERT INTO fund_locks (
                id, wallet_id, user_id, amount,
                reference_id, reference_type, is_active
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            "#,
        )
        .bind(lock.id)
        .bind(lock.wallet_id)
        .bind(lock.user_id)
        .bind(lock.amount)
        .bind(lock.reference_id)
        .bind(&lock.reference_type)
        .bind(lock.is_active)
        .execute(&self.pool)
        .await?;

        Ok(())
    }

    /// Get active lock by reference ID
    pub async fn get_active_by_reference(
        &self,
        reference_id: Uuid,
    ) -> Result<Option<FundLock>, sqlx::Error> {
        let sql = format!(
            "SELECT {LOCK_COLUMNS} FROM fund_locks WHERE reference_id = $1 AND is_active = true"
        );
        let lock = sqlx::query_as::<_, FundLock>(&sql)
            .bind(reference_id)
            .fetch_optional(&self.pool)
            .await?;

        Ok(lock)
    }

    /// Release a lock
    pub async fn release(&self, reference_id: Uuid) -> Result<(), sqlx::Error> {
        sqlx::query(
            r#"
            UPDATE fund_locks
            SET is_active = false, released_at = NOW()
            WHERE reference_id = $1 AND is_active = true
            "#,
        )
        .bind(reference_id)
        .execute(&self.pool)
        .await?;

        Ok(())
    }
}
