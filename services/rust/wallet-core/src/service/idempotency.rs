//! Idempotency service for preventing duplicate transactions
//! 
//! Standards: data-consistency.skill.md
//! 
//! PATTERN:
//! 1. Client generates UUID idempotency_key before request
//! 2. Server checks Redis: GET idempotency:{key}
//! 3. If exists → return cached transaction ID (no re-execution)
//! 4. If not → execute operation within DB transaction
//! 5. Store transaction ID: SET idempotency:{key} {txn_id} EX 86400
//! 6. UNIQUE constraint on idempotency_key in PostgreSQL (safety net)

use redis::AsyncCommands;
use std::time::Duration;
use uuid::Uuid;
use tracing::{debug, warn};

use crate::infrastructure::RedisClient;

/// Idempotency service
pub struct IdempotencyService {
    client: RedisClient,
    ttl_secs: u64,
    key_prefix: String,
}

impl IdempotencyService {
    pub fn new(client: RedisClient, ttl_secs: u64) -> Self {
        Self {
            client,
            ttl_secs,
            key_prefix: "wallet:idempotency:".to_string(),
        }
    }
    
    /// Get transaction ID for idempotency key
    /// Returns Some(txn_id) if key exists (operation was already processed)
    /// Returns None if key doesn't exist (new operation)
    pub async fn get(&self, key: &str) -> Result<Option<Uuid>, redis::RedisError> {
        let mut conn = self.client.get_multiplexed_tokio_connection().await?;
        let full_key = format!("{}{}", self.key_prefix, key);
        
        let value: Option<String> = conn.get(&full_key).await?;
        
        match value {
            Some(v) => {
                debug!(key = %key, txn_id = %v, "Found idempotent key");
                Ok(Uuid::parse_str(&v).ok())
            }
            None => {
                debug!(key = %key, "Idempotency key not found");
                Ok(None)
            }
        }
    }
    
    /// Set transaction ID for idempotency key
    /// Should be called AFTER successful DB transaction commit
    pub async fn set(&self, key: &str, txn_id: Uuid) -> Result<(), redis::RedisError> {
        let mut conn = self.client.get_multiplexed_tokio_connection().await?;
        let full_key = format!("{}{}", self.key_prefix, key);
        
        // SET with EX (expiry) - 24 hours by default
        conn.set_ex(&full_key, txn_id.to_string(), self.ttl_secs).await?;
        
        debug!(key = %key, txn_id = %txn_id, "Set idempotency key");
        
        Ok(())
    }
    
    /// Delete idempotency key (for cleanup if needed)
    pub async fn delete(&self, key: &str) -> Result<(), redis::RedisError> {
        let mut conn = self.client.get_multiplexed_tokio_connection().await?;
        let full_key = format!("{}{}", self.key_prefix, key);
        
        let _: () = conn.del(&full_key).await?;
        
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_idempotency_get_set() {
        // This test requires a running Redis instance
        // Use testcontainers for integration tests
    }
}
