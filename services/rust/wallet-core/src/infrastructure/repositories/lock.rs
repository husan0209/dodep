//! Fund lock repository

use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::FundLock;

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
        .execute(&self.pool)
        .await?;
        
        Ok(())
    }
    
    /// Get active lock by reference ID
    pub async fn get_active_by_reference(
        &self,
        reference_id: Uuid,
    ) -> Result<Option<FundLock>, sqlx::Error> {
        let lock = sqlx::query_as!(
            FundLock,
            r#"
            SELECT 
                id, wallet_id, user_id, amount as "Decimal: rust_decimal::Decimal",
                reference_id, reference_type, is_active,
                created_at, released_at
            FROM fund_locks
            WHERE reference_id = $1 AND is_active = true
            "#,
            reference_id
        )
        .fetch_optional(&self.pool)
        .await?;
        
        Ok(lock)
    }
    
    /// Release a lock
    pub async fn release(&self, reference_id: Uuid) -> Result<(), sqlx::Error> {
        sqlx::query!(
            r#"
            UPDATE fund_locks
            SET is_active = false, released_at = NOW()
            WHERE reference_id = $1 AND is_active = true
            "#,
            reference_id
        )
        .execute(&self.pool)
        .await?;
        
        Ok(())
    }
}
