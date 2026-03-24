//! Redis client for caching and distributed locks

use redis::{Client, ConnectionManager, RedisError};

use crate::config::RedisConfig;

pub type RedisClient = Client;
pub type RedisConnection = ConnectionManager;

/// Create Redis client
pub async fn create_redis_client(config: &RedisConfig) -> Result<RedisClient, RedisError> {
    Client::open(config.connection_string())
}

/// Get Redis connection manager
pub async fn get_connection(client: &RedisClient) -> Result<RedisConnection, RedisError> {
    client.get_connection_manager().await
}
