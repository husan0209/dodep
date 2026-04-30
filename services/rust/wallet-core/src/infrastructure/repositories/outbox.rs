//! Outbox repository for reliable event publishing

use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::OutboxEvent;

/// Outbox repository
pub struct OutboxRepository {
    pub pool: PgPool,
}

impl OutboxRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }
    
    /// Insert an outbox event
    pub async fn insert(&self, event: &OutboxEvent) -> Result<(), sqlx::Error> {
        let mut tx = self.pool.begin().await?;
        
        self.insert_internal(&mut tx, event).await?;
        
        tx.commit().await?;
        
        Ok(())
    }
    
    /// Insert an outbox event within a transaction
    pub async fn insert_internal(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        event: &OutboxEvent,
    ) -> Result<(), sqlx::Error> {
        sqlx::query!(
            r#"
            INSERT INTO outbox (topic, event_key, payload, headers)
            VALUES ($1, $2, $3, $4)
            "#,
            &event.topic,
            &event.event_key,
            &event.payload,
            &event.headers,
        )
        .execute(&mut **tx)
        .await?;
        
        Ok(())
    }
    
    /// Get unsent events for worker
    pub async fn get_unsent(&self, limit: i64) -> Result<Vec<OutboxEvent>, sqlx::Error> {
        let events = sqlx::query_as!(
            OutboxEvent,
            r#"
            SELECT
                id, topic, event_key,
                payload as "payload: Vec<u8>",
                headers as "headers: serde_json::Value",
                created_at, sent_at, retries, last_error
            FROM outbox
            WHERE sent_at IS NULL
            ORDER BY created_at
            LIMIT $1
            "#,
            limit
        )
        .fetch_all(&self.pool)
        .await?;
        
        Ok(events)
    }
    
    /// Mark event as sent
    pub async fn mark_sent(&self, id: i64) -> Result<(), sqlx::Error> {
        sqlx::query!(
            r#"
            UPDATE outbox
            SET sent_at = NOW(), retries = 0, last_error = NULL
            WHERE id = $1
            "#,
            id
        )
        .execute(&self.pool)
        .await?;
        
        Ok(())
    }
    
    /// Increment retry count
    pub async fn increment_retry(&self, id: i64, error: &str) -> Result<(), sqlx::Error> {
        sqlx::query!(
            r#"
            UPDATE outbox
            SET retries = retries + 1, last_error = $2
            WHERE id = $1
            "#,
            id,
            error
        )
        .execute(&self.pool)
        .await?;
        
        Ok(())
    }
    
    /// Get events with too many retries (dead letter candidates)
    pub async fn get_dead_letter(&self, max_retries: i32) -> Result<Vec<OutboxEvent>, sqlx::Error> {
        let events = sqlx::query_as!(
            OutboxEvent,
            r#"
            SELECT
                id, topic, event_key,
                payload as "payload: Vec<u8>",
                headers as "headers: serde_json::Value",
                created_at, sent_at, retries, last_error
            FROM outbox
            WHERE retries > $1
            "#,
            max_retries
        )
        .fetch_all(&self.pool)
        .await?;
        
        Ok(events)
    }
}
