//! Test infrastructure — spin up real Postgres via testcontainers

use sqlx::{PgPool, postgres::PgPoolOptions};
use std::sync::Arc;

use betting_engine::config::AppConfig;
use betting_engine::repositories::bet_repo::BetRepository;
use betting_engine::services::bet_service::BetService;
use betting_engine::services::cashout_service::CashoutService;
use betting_engine::services::settlement_service::SettlementService;
use betting_engine::state::AppState;

use rust_decimal::Decimal;

pub struct TestApp {
    pub pool: PgPool,
    pub bet_service: BetService,
    pub cashout_service: CashoutService,
    pub settlement_service: SettlementService,
    _container: testcontainers::ContainerAsync<testcontainers_modules::postgres::Postgres>,
}

impl TestApp {
    pub async fn start() -> Self {
        let container = testcontainers::ContainerAsync::start(
            testcontainers_modules::postgres::Postgres::default()
        )
        .await
        .expect("Failed to start Postgres container");

        let port = container.get_host_port_ipv4(5432).await.unwrap();
        let db_url = format!("postgres://postgres:postgres@localhost:{port}/postgres");

        let pool = PgPoolOptions::new()
            .max_connections(5)
            .connect(&db_url)
            .await
            .expect("Failed to connect to test database");

        // Run migrations
        Self::run_test_migrations(&pool).await;

        let bet_repo = BetRepository::new(pool.clone());
        let bet_service = BetService::new(bet_repo.clone());
        let cashout_service = CashoutService::new(bet_repo.clone());
        let settlement_service = SettlementService::new(bet_repo);

        Self {
            pool,
            bet_service,
            cashout_service,
            settlement_service,
            _container: container,
        }
    }

    async fn run_test_migrations(pool: &PgPool) {
        // Create enums
        sqlx::query(
            r#"
            DO $$ BEGIN
                CREATE TYPE bet_type_enum AS ENUM ('single','accumulator','system','chain');
            EXCEPTION WHEN duplicate_object THEN NULL; END $$;
            "#
        ).execute(pool).await.unwrap();

        sqlx::query(
            r#"
            DO $$ BEGIN
                CREATE TYPE bet_status_enum AS ENUM ('pending','active','won','lost','void','cashout','rejected');
            EXCEPTION WHEN duplicate_object THEN NULL; END $$;
            "#
        ).execute(pool).await.unwrap();

        // Create bets table
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS bets (
                id              BIGSERIAL PRIMARY KEY,
                user_id         BIGINT NOT NULL,
                bet_type        bet_type_enum NOT NULL,
                status          bet_status_enum NOT NULL DEFAULT 'pending',
                stake           NUMERIC(18,8) NOT NULL,
                potential_win   NUMERIC(18,8) NOT NULL,
                actual_win      NUMERIC(18,8) DEFAULT 0,
                odds            NUMERIC(12,6) NOT NULL,
                currency_code   CHAR(3) NOT NULL,
                sport_id        INTEGER,
                event_id        BIGINT,
                market_id       BIGINT,
                selection_id    BIGINT,
                idempotency_key UUID UNIQUE NOT NULL,
                ip_address      VARCHAR(45),
                device_fingerprint VARCHAR(64),
                placed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                settled_at      TIMESTAMPTZ,
                updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                metadata        JSONB DEFAULT '{}',
                CONSTRAINT ck_stake_positive CHECK (stake > 0)
            )
            "#
        ).execute(pool).await.unwrap();

        // Create bet_selections table
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS bet_selections (
                id          BIGSERIAL PRIMARY KEY,
                bet_id      BIGINT NOT NULL,
                user_id     BIGINT NOT NULL,
                event_id    BIGINT NOT NULL,
                market_id   BIGINT NOT NULL,
                selection_id BIGINT NOT NULL,
                odds        NUMERIC(12,6) NOT NULL,
                status      VARCHAR(20) DEFAULT 'active',
                result      VARCHAR(20),
                created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                settled_at  TIMESTAMPTZ
            )
            "#
        ).execute(pool).await.unwrap();

        // Create users table (minimal for FK)
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS users (
                id BIGSERIAL PRIMARY KEY,
                email VARCHAR(255) UNIQUE NOT NULL,
                status VARCHAR(20) DEFAULT 'active'
            )
            "#
        ).execute(pool).await.unwrap();

        // Insert test user
        sqlx::query("INSERT INTO users (id, email) VALUES (1, 'test@test.com') ON CONFLICT DO NOTHING")
            .execute(pool).await.unwrap();
    }

    pub async fn create_test_user(&self, user_id: i64) {
        sqlx::query(&format!(
            "INSERT INTO users (id, email) VALUES ({}, 'user{}@test.com') ON CONFLICT DO NOTHING",
            user_id, user_id
        ))
        .execute(&self.pool)
        .await
        .unwrap();
    }
}
