//! Database connection pool

use sqlx::postgres::{PgPool, PgPoolOptions};
use std::time::Duration;

use crate::config::DatabaseConfig;

pub type DbPool = PgPool;

/// Create database connection pool
pub async fn create_db_pool(config: &DatabaseConfig) -> Result<DbPool, sqlx::Error> {
    PgPoolOptions::new()
        .max_connections(config.max_connections)
        .min_connections(config.min_connections)
        .acquire_timeout(Duration::from_secs(config.connect_timeout_secs))
        .idle_timeout(Duration::from_secs(config.idle_timeout_secs))
        .connect(&config.connection_string())
        .await
}

/// Run database migrations (centralised in libs/migrations/postgresql).
///
/// `MigrateError` does not convert into `sqlx::Error`, so callers that expect a
/// single error type get the migration failure boxed.
pub async fn run_migrations(pool: &DbPool) -> Result<(), Box<dyn std::error::Error>> {
    sqlx::migrate!("../../../libs/migrations/postgresql")
        .run(pool)
        .await?;
    Ok(())
}
