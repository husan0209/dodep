


# SKILL #2 — rust-general.skill.md

Базовый skill для всех Rust-сервисов. Загружается каждым агентом, работающим с Rust.

---

```markdown
# rust-general.skill.md
# GAMBLING PLATFORM — RUST GENERAL CONVENTIONS
# Version: 1.0.0
# Updated: 2025
# Loaded by: All Rust agents
# Prerequisites: architecture-overview.skill.md

# ============================================================
# SECTION 1: ROLE AND CONTEXT
# ============================================================

## WHO YOU ARE

You are a Senior Rust Backend Developer building the critical path
of a high-scale online gambling platform.

Your code handles:
- 80% of all platform traffic
- All financial operations (wallets, bets, settlements)
- All real-time operations (WebSocket, odds, live betting)
- All fraud detection (real-time scoring)

Your code MUST be:
- Zero-copy where possible
- Lock-free where possible
- Allocation-aware (minimize heap allocations in hot paths)
- Panic-free in production (all errors handled explicitly)

## PERFORMANCE EXPECTATIONS

```text
Your services run on:
  CPU: 4-8 cores per instance
  RAM: 150-500 MB per instance (NOT gigabytes)
  Instances: 3-20 per service (auto-scaled)

Your code must achieve:
  p99 latency:        < 5ms for core operations
  Throughput:          10,000+ requests/sec per instance
  Memory:             No growth over time (no leaks)
  Startup time:       < 2 seconds
  Graceful shutdown:  < 10 seconds (drain connections)
```

# ============================================================
# SECTION 2: PROJECT SETUP
# ============================================================

## CARGO WORKSPACE

```toml
# Root Cargo.toml for Rust services
[workspace]
resolver = "2"
members = [
    "services/betting-engine",
    "services/wallet-service",
    "services/websocket-gateway",
    "services/odds-feed-service",
    "services/risk-engine",
    "libs/rust-platform/crates/*",
]

[workspace.package]
version = "0.1.0"
edition = "2021"
rust-version = "1.80"
license = "Proprietary"

[workspace.dependencies]
# Async runtime
tokio = { version = "1.40", features = ["full"] }
tokio-util = { version = "0.7" }

# Web framework
axum = { version = "0.8", features = ["macros", "ws"] }
axum-extra = { version = "0.10", features = ["typed-header"] }
tower = { version = "0.5", features = ["full"] }
tower-http = { version = "0.6", features = [
    "trace", "cors", "compression-gzip", "request-id",
    "timeout", "limit", "catch-panic"
] }
hyper = { version = "1.4" }

# gRPC
tonic = { version = "0.12", features = ["gzip", "tls"] }
tonic-build = { version = "0.12" }
prost = { version = "0.13" }
prost-types = { version = "0.13" }

# Serialization
serde = { version = "1.0", features = ["derive"] }
serde_json = { version = "1.0" }
rkyv = { version = "0.8", features = ["validation"] }

# Database
sqlx = { version = "0.8", features = [
    "runtime-tokio", "tls-rustls", "postgres",
    "chrono", "uuid", "rust_decimal", "json"
] }

# Cache
fred = { version = "9.0", features = ["enable-rustls"] }

# Message broker
rdkafka = { version = "0.36", features = ["cmake-build", "ssl"] }

# Observability
tracing = { version = "0.1" }
tracing-subscriber = { version = "0.3", features = [
    "json", "env-filter"
] }
tracing-opentelemetry = { version = "0.26" }
opentelemetry = { version = "0.25" }
opentelemetry-otlp = { version = "0.25" }
metrics = { version = "0.23" }
metrics-exporter-prometheus = { version = "0.15" }

# Error handling
thiserror = { version = "2.0" }
anyhow = { version = "1.0" }

# Utilities
uuid = { version = "1.10", features = ["v4", "v7", "serde"] }
chrono = { version = "0.4", features = ["serde"] }
rust_decimal = { version = "1.36", features = ["serde-with-str"] }
validator = { version = "0.18", features = ["derive"] }
derive_more = { version = "1.0", features = ["display", "from", "error"] }
strum = { version = "0.26", features = ["derive"] }
bytes = { version = "1.7" }
dashmap = { version = "6.0" }
parking_lot = { version = "0.12" }
once_cell = { version = "1.19" }
dotenvy = { version = "0.15" }
config = { version = "0.14" }
base64 = { version = "0.22" }
hex = { version = "0.4" }
rand = { version = "0.8" }
regex = { version = "1.10" }
url = { version = "2.5" }
itertools = { version = "0.13" }
pin-project-lite = { version = "0.2" }

# Crypto
argon2 = { version = "0.5" }
ed25519-dalek = { version = "2.1", features = ["serde"] }
aes-gcm = { version = "0.10" }
sha2 = { version = "0.10" }
hmac = { version = "0.12" }
jsonwebtoken = { version = "9.3" }

# Testing
tokio-test = { version = "0.4" }
testcontainers = { version = "0.22" }
testcontainers-modules = { version = "0.10", features = [
    "postgres", "redis", "kafka"
] }
fake = { version = "3.0", features = ["derive", "chrono", "uuid"] }
wiremock = { version = "0.6" }
assert_matches = { version = "1.5" }
proptest = { version = "1.5" }
criterion = { version = "0.5" }
pretty_assertions = { version = "1.4" }

# Sentry
sentry = { version = "0.34", features = ["tower", "tracing"] }

[workspace.lints.rust]
unsafe_code = "forbid"
missing_docs = "warn"
unused_must_use = "deny"
unused_imports = "deny"
dead_code = "warn"

[workspace.lints.clippy]
all = { level = "deny", priority = -1 }
pedantic = { level = "warn", priority = -1 }
nursery = { level = "warn", priority = -1 }

# Allow specific clippy lints with reason
module_name_repetitions = "allow"
must_use_candidate = "allow"
missing_errors_doc = "allow"
missing_panics_doc = "allow"

[profile.release]
lto = "thin"
codegen-units = 1
strip = "symbols"
panic = "abort"
opt-level = 3

[profile.release.package."*"]
opt-level = 3
```

## INDIVIDUAL SERVICE Cargo.toml

```toml
# services/betting-engine/Cargo.toml
[package]
name = "betting-engine"
version.workspace = true
edition.workspace = true
rust-version.workspace = true

[dependencies]
# Internal crates
platform-common = { path = "../../libs/rust-platform/crates/platform-common" }
platform-auth = { path = "../../libs/rust-platform/crates/platform-auth" }
platform-db = { path = "../../libs/rust-platform/crates/platform-db" }
platform-cache = { path = "../../libs/rust-platform/crates/platform-cache" }
platform-events = { path = "../../libs/rust-platform/crates/platform-events" }

# Workspace dependencies
tokio.workspace = true
axum.workspace = true
tower.workspace = true
tower-http.workspace = true
tonic.workspace = true
prost.workspace = true
serde.workspace = true
serde_json.workspace = true
sqlx.workspace = true
fred.workspace = true
tracing.workspace = true
thiserror.workspace = true
uuid.workspace = true
chrono.workspace = true
rust_decimal.workspace = true
validator.workspace = true
dashmap.workspace = true
metrics.workspace = true

[dev-dependencies]
tokio-test.workspace = true
testcontainers.workspace = true
testcontainers-modules.workspace = true
fake.workspace = true
wiremock.workspace = true
pretty_assertions.workspace = true
criterion.workspace = true

[build-dependencies]
tonic-build.workspace = true

[lints]
workspace = true

[[bench]]
name = "bet_placement"
harness = false
```

# ============================================================
# SECTION 3: APPLICATION BOOTSTRAP
# ============================================================

## MAIN.RS PATTERN

```rust
//! Betting Engine Service
//!
//! Handles bet placement, settlement, and cashout operations.
//! Critical path service — p99 latency target: < 5ms.

use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use tokio::signal;
use tracing::info;

mod config;
mod domain;
mod errors;
mod events;
mod grpc;
mod handlers;
mod middleware;
mod repositories;
mod router;
mod services;
mod state;

use config::AppConfig;
use state::AppState;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // 1. Load configuration
    let config = AppConfig::load()?;

    // 2. Initialize observability (BEFORE anything else)
    platform_common::telemetry::init(&config.service_name, &config.otel_endpoint)?;

    info!(
        service = %config.service_name,
        version = %config.version,
        environment = %config.environment,
        "Starting service"
    );

    // 3. Initialize dependencies
    let db_pool = platform_db::create_pool(&config.database).await?;
    let cache = platform_cache::create_client(&config.cache).await?;
    let event_producer = platform_events::create_producer(&config.redpanda)?;

    // 4. Run database migrations
    platform_db::run_migrations(&db_pool).await?;

    // 5. Build application state
    let state = AppState::new(
        config.clone(),
        db_pool,
        cache,
        event_producer,
    );

    // 6. Build router
    let app = router::build(state.clone());

    // 7. Start gRPC server (separate port)
    let grpc_handle = tokio::spawn(
        grpc::server::start(state.clone(), config.grpc_port)
    );

    // 8. Start event consumers
    let consumer_handle = tokio::spawn(
        events::consumer::start(state.clone())
    );

    // 9. Start HTTP server
    let addr = SocketAddr::from(([0, 0, 0, 0], config.http_port));
    let listener = tokio::net::TcpListener::bind(addr).await?;

    info!(%addr, "HTTP server listening");

    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await?;

    // 10. Graceful shutdown
    info!("Shutting down gracefully...");
    grpc_handle.abort();
    consumer_handle.abort();
    platform_common::telemetry::shutdown();

    info!("Service stopped");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => info!("Received Ctrl+C"),
        () = terminate => info!("Received SIGTERM"),
    }
}
```

## CONFIG PATTERN

```rust
// src/config.rs

use serde::Deserialize;
use platform_common::config::{DatabaseConfig, CacheConfig, RedpandaConfig};

#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    #[serde(default = "default_service_name")]
    pub service_name: String,

    #[serde(default = "default_version")]
    pub version: String,

    #[serde(default = "default_environment")]
    pub environment: String,

    #[serde(default = "default_http_port")]
    pub http_port: u16,

    #[serde(default = "default_grpc_port")]
    pub grpc_port: u16,

    pub otel_endpoint: String,

    pub database: DatabaseConfig,
    pub cache: CacheConfig,
    pub redpanda: RedpandaConfig,

    pub betting: BettingConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct BettingConfig {
    #[serde(default = "default_max_selections")]
    pub max_selections_per_bet: usize,

    #[serde(default = "default_min_odds")]
    pub min_odds: rust_decimal::Decimal,

    #[serde(default = "default_max_odds")]
    pub max_odds: rust_decimal::Decimal,

    #[serde(default = "default_max_payout")]
    pub max_payout_usd: rust_decimal::Decimal,
}

impl AppConfig {
    pub fn load() -> anyhow::Result<Self> {
        let config = config::Config::builder()
            .add_source(config::File::with_name("config/default"))
            .add_source(
                config::File::with_name(&format!(
                    "config/{}",
                    std::env::var("ENVIRONMENT").unwrap_or_else(|_| "dev".into())
                ))
                .required(false),
            )
            .add_source(config::Environment::with_prefix("APP").separator("__"))
            .build()?;

        let app_config: Self = config.try_deserialize()?;
        app_config.validate()?;
        Ok(app_config)
    }

    fn validate(&self) -> anyhow::Result<()> {
        anyhow::ensure!(self.http_port > 0, "HTTP port must be > 0");
        anyhow::ensure!(self.grpc_port > 0, "gRPC port must be > 0");
        anyhow::ensure!(
            self.http_port != self.grpc_port,
            "HTTP and gRPC ports must be different"
        );
        Ok(())
    }
}

fn default_service_name() -> String { "betting-engine".into() }
fn default_version() -> String { env!("CARGO_PKG_VERSION").into() }
fn default_environment() -> String { "dev".into() }
fn default_http_port() -> u16 { 8080 }
fn default_grpc_port() -> u16 { 9090 }
fn default_max_selections() -> usize { 20 }
fn default_min_odds() -> rust_decimal::Decimal { rust_decimal::Decimal::new(101, 2) } // 1.01
fn default_max_odds() -> rust_decimal::Decimal { rust_decimal::Decimal::new(100000, 2) } // 1000.00
fn default_max_payout() -> rust_decimal::Decimal { rust_decimal::Decimal::new(100_000, 0) }
```

## APP STATE PATTERN

```rust
// src/state.rs

use std::sync::Arc;

use crate::config::AppConfig;
use crate::repositories::bet_repo::BetRepository;
use crate::repositories::event_repo::EventRepository;
use crate::services::bet_service::BetService;
use crate::services::cashout_service::CashoutService;
use crate::services::settlement_service::SettlementService;

/// Shared application state, passed to all handlers.
///
/// Uses `Arc` internally — cloning is cheap (reference count increment).
/// All inner types are `Send + Sync`.
#[derive(Clone)]
pub struct AppState {
    inner: Arc<AppStateInner>,
}

struct AppStateInner {
    pub config: AppConfig,
    pub bet_service: BetService,
    pub cashout_service: CashoutService,
    pub settlement_service: SettlementService,
}

impl AppState {
    pub fn new(
        config: AppConfig,
        db_pool: sqlx::PgPool,
        cache: fred::clients::RedisClient,
        event_producer: platform_events::Producer,
    ) -> Self {
        // Build repositories
        let bet_repo = BetRepository::new(db_pool.clone());
        let event_repo = EventRepository::new(db_pool.clone());

        // Build services (with dependencies injected)
        let bet_service = BetService::new(
            config.betting.clone(),
            bet_repo.clone(),
            event_repo.clone(),
            cache.clone(),
            event_producer.clone(),
        );

        let cashout_service = CashoutService::new(
            bet_repo.clone(),
            cache.clone(),
            event_producer.clone(),
        );

        let settlement_service = SettlementService::new(
            bet_repo.clone(),
            event_producer.clone(),
        );

        Self {
            inner: Arc::new(AppStateInner {
                config,
                bet_service,
                cashout_service,
                settlement_service,
            }),
        }
    }

    pub fn config(&self) -> &AppConfig { &self.inner.config }
    pub fn bet_service(&self) -> &BetService { &self.inner.bet_service }
    pub fn cashout_service(&self) -> &CashoutService { &self.inner.cashout_service }
    pub fn settlement_service(&self) -> &SettlementService { &self.inner.settlement_service }
}
```

# ============================================================
# SECTION 4: HANDLER PATTERNS
# ============================================================

## RULES FOR HANDLERS

```text
1. Handlers are THIN — extract input, call service, return output
2. Handler NEVER contains business logic
3. Handler NEVER constructs SQL queries
4. Handler NEVER calls repository directly
5. Handler validates REQUEST FORMAT (via validator derive)
6. Handler does NOT validate BUSINESS RULES (service does that)
7. Handler returns Result<Json<T>, AppError>
8. Handler uses extractors for input (Path, Query, Json, State)
9. Handler function signature must fit on 1-3 lines
10. Handler body should be 3-10 lines
```

## HANDLER EXAMPLES

```rust
// src/handlers/bet_handler.rs

use axum::{
    extract::{Json, Path, Query, State},
    http::StatusCode,
};
use uuid::Uuid;
use validator::Validate;

use crate::domain::bet::{BetId, PlaceBetRequest, PlaceBetResponse};
use crate::domain::pagination::PaginationParams;
use crate::errors::AppError;
use crate::middleware::auth::AuthUser;
use crate::state::AppState;

/// Place a new bet.
///
/// Validates the request, checks odds, debits wallet, creates bet.
/// Idempotent by `idempotency_key`.
#[tracing::instrument(
    name = "handler.place_bet",
    skip(state, req),
    fields(user_id = %user.id, idempotency_key = %req.idempotency_key)
)]
pub async fn place_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Json(req): Json<PlaceBetRequest>,
) -> Result<(StatusCode, Json<PlaceBetResponse>), AppError> {
    req.validate()?;
    let bet = state.bet_service().place_bet(user.id, req).await?;
    Ok((StatusCode::CREATED, Json(PlaceBetResponse::from(bet))))
}

/// Get bet by ID.
///
/// User can only see their own bets (enforced in service layer).
#[tracing::instrument(
    name = "handler.get_bet",
    skip(state),
    fields(user_id = %user.id, bet_id = %bet_id)
)]
pub async fn get_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Path(bet_id): Path<BetId>,
) -> Result<Json<PlaceBetResponse>, AppError> {
    let bet = state.bet_service().get_bet(user.id, bet_id).await?;
    Ok(Json(PlaceBetResponse::from(bet)))
}

/// Get user's bet history.
#[tracing::instrument(
    name = "handler.get_bet_history",
    skip(state),
    fields(user_id = %user.id)
)]
pub async fn get_bet_history(
    State(state): State<AppState>,
    user: AuthUser,
    Query(params): Query<PaginationParams>,
) -> Result<Json<platform_common::PaginatedResponse<PlaceBetResponse>>, AppError> {
    params.validate()?;
    let result = state.bet_service().get_history(user.id, params).await?;
    Ok(Json(result))
}

/// Request cashout for an active bet.
#[tracing::instrument(
    name = "handler.cashout_bet",
    skip(state),
    fields(user_id = %user.id, bet_id = %bet_id)
)]
pub async fn cashout_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Path(bet_id): Path<BetId>,
) -> Result<Json<CashoutResponse>, AppError> {
    let result = state.cashout_service().cashout(user.id, bet_id).await?;
    Ok(Json(result))
}
```

## ANTI-PATTERNS IN HANDLERS

```rust
// ❌ BAD: Business logic in handler
pub async fn place_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Json(req): Json<PlaceBetRequest>,
) -> Result<Json<PlaceBetResponse>, AppError> {
    // ❌ Checking business rules in handler
    if req.selections.len() > 20 {
        return Err(AppError::Validation("too many selections".into()));
    }

    // ❌ Direct database call in handler
    let balance = sqlx::query_scalar!(
        "SELECT balance FROM wallets WHERE user_id = $1",
        user.id.0
    )
    .fetch_one(&state.db_pool)
    .await?;

    // ❌ Business logic in handler
    if balance < req.stake {
        return Err(AppError::InsufficientBalance);
    }

    // ❌ More business logic...
    let odds = state.cache.get(&format!("odds:{}", req.event_id)).await?;

    // ... 50 more lines of logic that belongs in service
    Ok(Json(response))
}

// ❌ BAD: Handler doing too many things
pub async fn register_and_send_email_and_create_wallet(
    // ... this is three operations, should be orchestrated in service
) -> Result<Json<Response>, AppError> {
    // ❌ Multiple responsibilities
}

// ❌ BAD: Not using extractors
pub async fn place_bet(
    req: axum::http::Request<axum::body::Body>,
) -> Result<Json<PlaceBetResponse>, AppError> {
    // ❌ Manual parsing
    let body = axum::body::to_bytes(req.into_body(), 1024 * 1024).await?;
    let bet_req: PlaceBetRequest = serde_json::from_slice(&body)?;
    // ...
}
```

# ============================================================
# SECTION 5: SERVICE LAYER PATTERNS
# ============================================================

## RULES FOR SERVICES

```text
1. Service contains ALL business logic
2. Service validates BUSINESS RULES (not format — that's handler)
3. Service orchestrates multiple repositories and external calls
4. Service is responsible for transactions (begin/commit/rollback)
5. Service publishes domain events
6. Service uses dependency injection (repos passed via constructor)
7. Service methods are async and return Result<T, AppError>
8. Service NEVER knows about HTTP/gRPC (no StatusCode, no Request)
9. Service CAN call other services via gRPC clients
10. Service logs at info/warn level (debug for details)
```

## SERVICE EXAMPLE

```rust
// src/services/bet_service.rs

use rust_decimal::Decimal;
use tracing::{info, warn};
use uuid::Uuid;

use crate::config::BettingConfig;
use crate::domain::bet::*;
use crate::errors::AppError;
use crate::repositories::bet_repo::BetRepository;
use crate::repositories::event_repo::EventRepository;
use platform_cache::CacheClient;
use platform_events::Producer;

#[derive(Clone)]
pub struct BetService {
    config: BettingConfig,
    bet_repo: BetRepository,
    event_repo: EventRepository,
    cache: CacheClient,
    producer: Producer,
    // gRPC clients for other services
    wallet_client: wallet_proto::wallet_service_client::WalletServiceClient<tonic::transport::Channel>,
    risk_client: risk_proto::risk_service_client::RiskServiceClient<tonic::transport::Channel>,
}

impl BetService {
    pub fn new(
        config: BettingConfig,
        bet_repo: BetRepository,
        event_repo: EventRepository,
        cache: CacheClient,
        producer: Producer,
    ) -> Self {
        // ... initialization
        Self { config, bet_repo, event_repo, cache, producer,
               wallet_client, risk_client }
    }

    /// Place a new bet.
    ///
    /// Flow:
    /// 1. Check idempotency
    /// 2. Validate selections (events active, markets open)
    /// 3. Get current odds, compare with submitted odds
    /// 4. Validate stake (min/max)
    /// 5. Risk check
    /// 6. Lock funds in wallet
    /// 7. Store bet
    /// 8. Publish event
    ///
    /// Idempotent by `idempotency_key`.
    pub async fn place_bet(
        &self,
        user_id: UserId,
        req: PlaceBetRequest,
    ) -> Result<Bet, AppError> {
        // 1. Idempotency check
        if let Some(cached) = self.cache
            .get_idempotency::<Bet>(&req.idempotency_key)
            .await?
        {
            info!(
                user_id = %user_id,
                idempotency_key = %req.idempotency_key,
                "Returning cached idempotent response"
            );
            return Ok(cached);
        }

        // 2. Validate all selections
        let validated_selections = self
            .validate_selections(&req.selections)
            .await?;

        // 3. Get current odds and compare
        let current_odds = self
            .get_and_compare_odds(&validated_selections, req.accept_odds_changes)
            .await?;

        // 4. Calculate combined odds
        let combined_odds = self.calculate_combined_odds(&current_odds, req.bet_type);
        let potential_win = req.stake * combined_odds;

        // 5. Validate stake limits
        self.validate_stake(user_id, req.stake, potential_win)?;

        // 6. Risk check via gRPC
        let risk_result = self.risk_client
            .check_bet(risk_proto::CheckBetRequest {
                user_id: user_id.into(),
                stake: req.stake.to_string(),
                odds: combined_odds.to_string(),
                selections_count: validated_selections.len() as u32,
            })
            .await
            .map_err(|e| {
                warn!(error = %e, "Risk service unavailable, applying default policy");
                AppError::ServiceUnavailable("risk-engine".into())
            })?;

        if risk_result.into_inner().rejected {
            return Err(AppError::BetRejected {
                reason: "Risk check failed".into(),
            });
        }

        // 7. Lock funds in wallet via gRPC
        let lock_result = self.wallet_client
            .lock(wallet_proto::LockRequest {
                user_id: user_id.into(),
                currency_code: req.currency_code.clone(),
                amount: req.stake.to_string(),
                idempotency_key: req.idempotency_key.to_string(),
                reference_type: "bet".into(),
                reference_id: 0, // will be updated after bet creation
            })
            .await
            .map_err(|status| match status.code() {
                tonic::Code::FailedPrecondition => AppError::InsufficientBalance {
                    required: req.stake,
                    available: Decimal::ZERO, // actual balance in status.message()
                },
                _ => AppError::ServiceUnavailable("wallet-service".into()),
            })?;

        let lock_id = lock_result.into_inner().lock_id;

        // 8. Store bet in database
        let bet = match self.bet_repo.create_bet(CreateBetParams {
            user_id,
            bet_type: req.bet_type,
            stake: req.stake,
            combined_odds,
            potential_win,
            currency_code: req.currency_code,
            selections: validated_selections,
            idempotency_key: req.idempotency_key,
            lock_id,
            ip_address: req.ip_address,
            device_fingerprint: req.device_fingerprint,
        }).await {
            Ok(bet) => bet,
            Err(e) => {
                // Compensate: unlock wallet funds
                warn!(error = %e, lock_id = %lock_id, "Bet creation failed, unlocking funds");
                let _ = self.wallet_client.unlock(wallet_proto::UnlockRequest {
                    lock_id,
                    idempotency_key: Uuid::new_v4().to_string(),
                }).await;
                return Err(e.into());
            }
        };

        // 9. Cache idempotency result
        self.cache
            .set_idempotency(&req.idempotency_key, &bet, 86400)
            .await?;

        // 10. Publish domain event
        self.producer.publish(
            "bets.placed",
            &bet.user_id.to_string(),
            &BetPlacedEvent::from(&bet),
        ).await?;

        info!(
            user_id = %user_id,
            bet_id = %bet.id,
            stake = %req.stake,
            odds = %combined_odds,
            "Bet placed successfully"
        );

        Ok(bet)
    }

    /// Validate that all selections are valid (events active, markets open).
    async fn validate_selections(
        &self,
        selections: &[SelectionRequest],
    ) -> Result<Vec<ValidatedSelection>, AppError> {
        if selections.is_empty() {
            return Err(AppError::Validation(vec![
                FieldError::new("selections", "At least one selection required"),
            ]));
        }

        if selections.len() > self.config.max_selections_per_bet {
            return Err(AppError::Validation(vec![
                FieldError::new(
                    "selections",
                    format!("Maximum {} selections allowed", self.config.max_selections_per_bet),
                ),
            ]));
        }

        let mut validated = Vec::with_capacity(selections.len());

        for selection in selections {
            let event = self.event_repo
                .get_event(selection.event_id)
                .await?
                .ok_or_else(|| AppError::BetEventNotFound {
                    event_id: selection.event_id,
                })?;

            if event.status != EventStatus::Active {
                return Err(AppError::BetEventSuspended {
                    event_id: selection.event_id,
                });
            }

            let market = event.markets
                .iter()
                .find(|m| m.id == selection.market_id)
                .ok_or_else(|| AppError::BetMarketNotFound {
                    market_id: selection.market_id,
                })?;

            if market.status != MarketStatus::Open {
                return Err(AppError::BetMarketClosed {
                    market_id: selection.market_id,
                });
            }

            validated.push(ValidatedSelection {
                event_id: selection.event_id,
                market_id: selection.market_id,
                outcome_id: selection.outcome_id,
                submitted_odds: selection.odds,
                event_name: event.name.clone(),
                market_name: market.name.clone(),
            });
        }

        Ok(validated)
    }

    fn validate_stake(
        &self,
        user_id: UserId,
        stake: Decimal,
        potential_win: Decimal,
    ) -> Result<(), AppError> {
        let min_stake = Decimal::new(1, 1); // 0.1
        let max_stake = Decimal::new(10_000, 0); // 10,000

        if stake < min_stake {
            return Err(AppError::BetStakeTooLow { min: min_stake, actual: stake });
        }

        if stake > max_stake {
            return Err(AppError::BetStakeTooHigh { max: max_stake, actual: stake });
        }

        if potential_win > self.config.max_payout_usd {
            return Err(AppError::BetMaxPayoutExceeded {
                max_payout: self.config.max_payout_usd,
                potential_win,
            });
        }

        Ok(())
    }

    fn calculate_combined_odds(
        &self,
        selections: &[CurrentOdds],
        bet_type: BetType,
    ) -> Decimal {
        match bet_type {
            BetType::Single => selections[0].odds,
            BetType::Accumulator => {
                selections.iter()
                    .map(|s| s.odds)
                    .fold(Decimal::ONE, |acc, odds| acc * odds)
            }
            BetType::System { .. } => {
                // System bet calculation (more complex)
                todo!("Implement system bet odds calculation")
            }
        }
    }
}
```

# ============================================================
# SECTION 6: REPOSITORY PATTERNS
# ============================================================

## RULES FOR REPOSITORIES

```text
1. Repository handles ALL database operations
2. Repository uses SQLx with compile-time checked queries
3. Repository NEVER contains business logic
4. Repository returns domain types (NOT database rows)
5. Repository methods are async and return Result<T, sqlx::Error>
   (service layer maps to AppError)
6. Repository uses transactions where needed (passed as argument)
7. For complex queries: use sqlx::query! macro (compile-time check)
8. For dynamic queries: use sqlx::QueryBuilder
```

## REPOSITORY EXAMPLE

```rust
// src/repositories/bet_repo.rs

use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::bet::*;

#[derive(Clone)]
pub struct BetRepository {
    pool: PgPool,
}

impl BetRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    pub async fn create_bet(&self, params: CreateBetParams) -> Result<Bet, sqlx::Error> {
        let mut tx = self.pool.begin().await?;

        // Insert bet
        let bet = sqlx::query_as!(
            BetRow,
            r#"
            INSERT INTO bets (
                user_id, bet_type, status, stake, combined_odds,
                potential_win, currency_code, idempotency_key,
                lock_id, ip_address, device_fingerprint
            )
            VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7, $8, $9, $10)
            RETURNING
                id, user_id, bet_type as "bet_type: BetType",
                status as "status: BetStatus", stake, combined_odds,
                potential_win, actual_win, currency_code,
                idempotency_key, lock_id, placed_at, settled_at
            "#,
            params.user_id.0,
            params.bet_type as BetType,
            params.stake,
            params.combined_odds,
            params.potential_win,
            params.currency_code,
            params.idempotency_key,
            params.lock_id,
            params.ip_address.map(|ip| ip.to_string()),
            params.device_fingerprint,
        )
        .fetch_one(&mut *tx)
        .await?;

        // Insert selections
        for selection in &params.selections {
            sqlx::query!(
                r#"
                INSERT INTO bet_selections (
                    bet_id, event_id, market_id, outcome_id,
                    odds, event_name, market_name
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7)
                "#,
                bet.id,
                selection.event_id.0,
                selection.market_id.0,
                selection.outcome_id.0,
                selection.submitted_odds,
                selection.event_name,
                selection.market_name,
            )
            .fetch_optional(&mut *tx)
            .await?;
        }

        tx.commit().await?;

        Ok(Bet::from(bet))
    }

    pub async fn get_bet_by_id(
        &self,
        bet_id: BetId,
        user_id: UserId,
    ) -> Result<Option<Bet>, sqlx::Error> {
        let row = sqlx::query_as!(
            BetRow,
            r#"
            SELECT
                id, user_id, bet_type as "bet_type: BetType",
                status as "status: BetStatus", stake, combined_odds,
                potential_win, actual_win, currency_code,
                idempotency_key, lock_id, placed_at, settled_at
            FROM bets
            WHERE id = $1 AND user_id = $2
            "#,
            bet_id.0,
            user_id.0,
        )
        .fetch_optional(&self.pool)
        .await?;

        Ok(row.map(Bet::from))
    }

    pub async fn get_user_bets(
        &self,
        user_id: UserId,
        params: &PaginationParams,
    ) -> Result<(Vec<Bet>, i64), sqlx::Error> {
        let total = sqlx::query_scalar!(
            "SELECT COUNT(*) FROM bets WHERE user_id = $1",
            user_id.0,
        )
        .fetch_one(&self.pool)
        .await?
        .unwrap_or(0);

        let rows = sqlx::query_as!(
            BetRow,
            r#"
            SELECT
                id, user_id, bet_type as "bet_type: BetType",
                status as "status: BetStatus", stake, combined_odds,
                potential_win, actual_win, currency_code,
                idempotency_key, lock_id, placed_at, settled_at
            FROM bets
            WHERE user_id = $1
            ORDER BY placed_at DESC
            LIMIT $2 OFFSET $3
            "#,
            user_id.0,
            params.limit(),
            params.offset(),
        )
        .fetch_all(&self.pool)
        .await?;

        Ok((rows.into_iter().map(Bet::from).collect(), total))
    }

    /// Update bet status using state machine validation.
    ///
    /// Returns error if the transition is invalid.
    pub async fn update_bet_status(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        bet_id: BetId,
        from_status: BetStatus,
        to_status: BetStatus,
        actual_win: Option<rust_decimal::Decimal>,
    ) -> Result<Bet, sqlx::Error> {
        let row = sqlx::query_as!(
            BetRow,
            r#"
            UPDATE bets
            SET
                status = $3,
                actual_win = COALESCE($4, actual_win),
                settled_at = CASE WHEN $3 IN ('won', 'lost', 'void', 'cashout')
                             THEN NOW() ELSE settled_at END,
                updated_at = NOW()
            WHERE id = $1 AND status = $2
            RETURNING
                id, user_id, bet_type as "bet_type: BetType",
                status as "status: BetStatus", stake, combined_odds,
                potential_win, actual_win, currency_code,
                idempotency_key, lock_id, placed_at, settled_at
            "#,
            bet_id.0,
            from_status as BetStatus,
            to_status as BetStatus,
            actual_win,
        )
        .fetch_optional(&mut **tx)
        .await?;

        row.map(Bet::from).ok_or_else(|| {
            sqlx::Error::RowNotFound // service will interpret as state conflict
        })
    }
}
```

# ============================================================
# SECTION 7: DOMAIN TYPES
# ============================================================

## RULES FOR DOMAIN TYPES

```text
1. Domain types are plain structs/enums with derive macros
2. Domain types have NO external dependencies (no sqlx, no axum)
3. Use newtypes for IDs: struct UserId(pub i64)
4. Use rust_decimal::Decimal for ALL money values
5. Use chrono::DateTime<Utc> for ALL timestamps
6. Use uuid::Uuid for ALL external-facing IDs
7. Use enums with strum for ALL status fields
8. Implement Display for all public types
9. Implement From<Row> for conversion from DB rows
10. Implement Into<Response> for conversion to API responses
```

## DOMAIN EXAMPLES

```rust
// src/domain/bet.rs

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use sqlx::Type;
use strum::{Display, EnumString};
use uuid::Uuid;
use validator::Validate;

// ── Newtype IDs ──

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct BetId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct UserId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct EventId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct MarketId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct OutcomeId(pub i64);

impl std::fmt::Display for BetId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl From<i64> for BetId {
    fn from(id: i64) -> Self { Self(id) }
}

impl From<BetId> for i64 {
    fn from(id: BetId) -> Self { id.0 }
}

// ── Enums ──

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Type, Display, EnumString)]
#[sqlx(type_name = "bet_type", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
#[strum(serialize_all = "snake_case")]
pub enum BetType {
    Single,
    Accumulator,
    System,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Type, Display, EnumString)]
#[sqlx(type_name = "bet_status", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
#[strum(serialize_all = "snake_case")]
pub enum BetStatus {
    Pending,
    Active,
    Won,
    Lost,
    Void,
    Cashout,
    Rejected,
}

impl BetStatus {
    /// Validate that a state transition is allowed.
    pub fn can_transition_to(&self, target: BetStatus) -> bool {
        matches!(
            (self, target),
            (BetStatus::Pending, BetStatus::Active)
                | (BetStatus::Pending, BetStatus::Rejected)
                | (BetStatus::Active, BetStatus::Won)
                | (BetStatus::Active, BetStatus::Lost)
                | (BetStatus::Active, BetStatus::Void)
                | (BetStatus::Active, BetStatus::Cashout)
        )
    }
}

// ── Core Entity ──

#[derive(Debug, Clone, Serialize)]
pub struct Bet {
    pub id: BetId,
    pub user_id: UserId,
    pub bet_type: BetType,
    pub status: BetStatus,
    pub stake: Decimal,
    pub combined_odds: Decimal,
    pub potential_win: Decimal,
    pub actual_win: Decimal,
    pub currency_code: String,
    pub idempotency_key: Uuid,
    pub lock_id: i64,
    pub placed_at: DateTime<Utc>,
    pub settled_at: Option<DateTime<Utc>>,
    pub selections: Vec<Selection>,
}

#[derive(Debug, Clone, Serialize)]
pub struct Selection {
    pub event_id: EventId,
    pub market_id: MarketId,
    pub outcome_id: OutcomeId,
    pub odds: Decimal,
    pub event_name: String,
    pub market_name: String,
    pub result: Option<SelectionResult>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SelectionResult {
    Won,
    Lost,
    Void,
    HalfWon,
    HalfLost,
}

// ── Request DTOs ──

#[derive(Debug, Deserialize, Validate)]
pub struct PlaceBetRequest {
    pub bet_type: BetType,

    #[validate(length(min = 1, max = 20, message = "1-20 selections required"))]
    pub selections: Vec<SelectionRequest>,

    #[validate(custom(function = "validate_positive_decimal"))]
    pub stake: Decimal,

    pub currency_code: String,

    #[serde(default = "default_accept_odds")]
    pub accept_odds_changes: AcceptOddsChanges,

    pub idempotency_key: Uuid,

    // Populated by middleware, not by client
    #[serde(skip_deserializing)]
    pub ip_address: Option<std::net::IpAddr>,

    #[serde(skip_deserializing)]
    pub device_fingerprint: Option<String>,
}

#[derive(Debug, Deserialize, Validate)]
pub struct SelectionRequest {
    pub event_id: EventId,
    pub market_id: MarketId,
    pub outcome_id: OutcomeId,

    #[validate(custom(function = "validate_positive_decimal"))]
    pub odds: Decimal,
}

#[derive(Debug, Clone, Copy, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AcceptOddsChanges {
    None,
    Higher,
    Any,
}

fn default_accept_odds() -> AcceptOddsChanges {
    AcceptOddsChanges::None
}

// ── Response DTOs ──

#[derive(Debug, Serialize)]
pub struct PlaceBetResponse {
    pub bet_id: BetId,
    pub status: BetStatus,
    pub stake: Decimal,
    pub odds: Decimal,
    pub potential_win: Decimal,
    pub currency_code: String,
    pub placed_at: DateTime<Utc>,
    pub selections: Vec<SelectionResponse>,
}

impl From<Bet> for PlaceBetResponse {
    fn from(bet: Bet) -> Self {
        Self {
            bet_id: bet.id,
            status: bet.status,
            stake: bet.stake,
            odds: bet.combined_odds,
            potential_win: bet.potential_win,
            currency_code: bet.currency_code,
            placed_at: bet.placed_at,
            selections: bet.selections.into_iter().map(Into::into).collect(),
        }
    }
}

// ── Validation helpers ──

fn validate_positive_decimal(value: &Decimal) -> Result<(), validator::ValidationError> {
    if *value <= Decimal::ZERO {
        return Err(validator::ValidationError::new("must_be_positive"));
    }
    Ok(())
}
```

# ============================================================
# SECTION 8: ERROR HANDLING
# ============================================================

## ERROR TYPE

```rust
// src/errors.rs

use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use rust_decimal::Decimal;
use serde::Serialize;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum AppError {
    // ── Validation ──
    #[error("Validation failed")]
    Validation(Vec<FieldError>),

    // ── Authentication / Authorization ──
    #[error("Unauthorized: {reason}")]
    Unauthorized { reason: String },

    #[error("Forbidden: {reason}")]
    Forbidden { reason: String },

    // ── Not Found ──
    #[error("{entity} not found: {id}")]
    NotFound { entity: String, id: String },

    // ── Betting specific ──
    #[error("Event {event_id} not found")]
    BetEventNotFound { event_id: EventId },

    #[error("Event {event_id} is suspended")]
    BetEventSuspended { event_id: EventId },

    #[error("Market {market_id} is closed")]
    BetMarketClosed { market_id: MarketId },

    #[error("Odds changed for selection")]
    BetOddsChanged {
        selection_index: usize,
        submitted_odds: Decimal,
        current_odds: Decimal,
    },

    #[error("Stake {actual} is below minimum {min}")]
    BetStakeTooLow { min: Decimal, actual: Decimal },

    #[error("Stake {actual} exceeds maximum {max}")]
    BetStakeTooHigh { max: Decimal, actual: Decimal },

    #[error("Potential win {potential_win} exceeds max payout {max_payout}")]
    BetMaxPayoutExceeded { max_payout: Decimal, potential_win: Decimal },

    #[error("Bet rejected: {reason}")]
    BetRejected { reason: String },

    #[error("Bet already settled")]
    BetAlreadySettled,

    #[error("Cashout unavailable for this bet")]
    CashoutUnavailable,

    // ── Wallet specific ──
    #[error("Insufficient balance: required {required}, available {available}")]
    InsufficientBalance { required: Decimal, available: Decimal },

    // ── External services ──
    #[error("Service unavailable: {0}")]
    ServiceUnavailable(String),

    // ── Rate limiting ──
    #[error("Rate limit exceeded")]
    RateLimited { retry_after: std::time::Duration },

    // ── Conflict ──
    #[error("Conflict: {reason}")]
    Conflict { reason: String },

    // ── Internal ──
    #[error("Internal error")]
    Internal(#[from] anyhow::Error),

    #[error("Database error")]
    Database(#[from] sqlx::Error),

    #[error("Cache error")]
    Cache(#[from] fred::error::RedisError),
}

// HTTP error response body
#[derive(Debug, Serialize)]
struct ErrorResponse {
    error: ErrorBody,
    meta: Meta,
}

#[derive(Debug, Serialize)]
struct ErrorBody {
    code: &'static str,
    message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    details: Option<serde_json::Value>,
}

#[derive(Debug, Serialize)]
struct Meta {
    request_id: String,
    timestamp: String,
}

#[derive(Debug, Serialize)]
pub struct FieldError {
    pub field: String,
    pub message: String,
}

impl FieldError {
    pub fn new(field: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            field: field.into(),
            message: message.into(),
        }
    }
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code, message, details) = match &self {
            // 400
            AppError::Validation(fields) => (
                StatusCode::BAD_REQUEST,
                "VALIDATION_FAILED",
                "Validation failed".into(),
                Some(serde_json::to_value(fields).unwrap_or_default()),
            ),

            // 401
            AppError::Unauthorized { reason } => (
                StatusCode::UNAUTHORIZED,
                "AUTH_UNAUTHORIZED",
                reason.clone(),
                None,
            ),

            // 403
            AppError::Forbidden { reason } => (
                StatusCode::FORBIDDEN,
                "AUTH_FORBIDDEN",
                reason.clone(),
                None,
            ),

            // 404
            AppError::NotFound { entity, id } => (
                StatusCode::NOT_FOUND,
                "NOT_FOUND",
                format!("{entity} not found"),
                Some(serde_json::json!({ "id": id })),
            ),

            // Betting 400s / 409s / 422s
            AppError::BetEventNotFound { event_id } => (
                StatusCode::NOT_FOUND,
                "BET_EVENT_NOT_FOUND",
                format!("Event {event_id} not found"),
                None,
            ),
            AppError::BetEventSuspended { event_id } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_EVENT_SUSPENDED",
                format!("Event {event_id} is suspended"),
                None,
            ),
            AppError::BetMarketClosed { market_id } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_MARKET_CLOSED",
                format!("Market {market_id} is closed"),
                None,
            ),
            AppError::BetOddsChanged { selection_index, submitted_odds, current_odds } => (
                StatusCode::CONFLICT,
                "BET_ODDS_CHANGED",
                "Odds have changed".into(),
                Some(serde_json::json!({
                    "selection_index": selection_index,
                    "submitted_odds": submitted_odds.to_string(),
                    "current_odds": current_odds.to_string(),
                })),
            ),
            AppError::BetStakeTooLow { min, actual } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_STAKE_TOO_LOW",
                format!("Minimum stake is {min}"),
                Some(serde_json::json!({ "min": min.to_string(), "actual": actual.to_string() })),
            ),
            AppError::BetStakeTooHigh { max, actual } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_STAKE_TOO_HIGH",
                format!("Maximum stake is {max}"),
                Some(serde_json::json!({ "max": max.to_string(), "actual": actual.to_string() })),
            ),
            AppError::BetMaxPayoutExceeded { max_payout, potential_win } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_MAX_PAYOUT_EXCEEDED",
                format!("Maximum payout is {max_payout}"),
                Some(serde_json::json!({
                    "max_payout": max_payout.to_string(),
                    "potential_win": potential_win.to_string(),
                })),
            ),
            AppError::BetRejected { reason } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_REJECTED",
                reason.clone(),
                None,
            ),
            AppError::BetAlreadySettled => (
                StatusCode::CONFLICT,
                "BET_ALREADY_SETTLED",
                "Bet is already settled".into(),
                None,
            ),
            AppError::CashoutUnavailable => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_CASHOUT_UNAVAILABLE",
                "Cashout is not available for this bet".into(),
                None,
            ),

            // Wallet
            AppError::InsufficientBalance { required, available } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "WALLET_INSUFFICIENT_BALANCE",
                "Insufficient balance".into(),
                Some(serde_json::json!({
                    "required": required.to_string(),
                    "available": available.to_string(),
                })),
            ),

            // 429
            AppError::RateLimited { retry_after } => (
                StatusCode::TOO_MANY_REQUESTS,
                "RATE_LIMITED",
                format!("Rate limit exceeded. Retry after {} seconds", retry_after.as_secs()),
                None,
            ),

            // 409
            AppError::Conflict { reason } => (
                StatusCode::CONFLICT,
                "CONFLICT",
                reason.clone(),
                None,
            ),

            // 503
            AppError::ServiceUnavailable(service) => (
                StatusCode::SERVICE_UNAVAILABLE,
                "SERVICE_UNAVAILABLE",
                format!("Service temporarily unavailable: {service}"),
                None,
            ),

            // 500 (NEVER expose internal details)
            AppError::Internal(e) => {
                tracing::error!(error = %e, "Internal error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "INTERNAL_ERROR",
                    "An internal error occurred".into(),
                    None,
                )
            }
            AppError::Database(e) => {
                tracing::error!(error = %e, "Database error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "INTERNAL_ERROR",
                    "An internal error occurred".into(),
                    None,
                )
            }
            AppError::Cache(e) => {
                tracing::error!(error = %e, "Cache error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "INTERNAL_ERROR",
                    "An internal error occurred".into(),
                    None,
                )
            }
        };

        let body = ErrorResponse {
            error: ErrorBody { code, message, details },
            meta: Meta {
                request_id: String::new(), // populated by middleware
                timestamp: chrono::Utc::now().to_rfc3339(),
            },
        };

        (status, Json(body)).into_response()
    }
}

// Convert validator::ValidationErrors to AppError
impl From<validator::ValidationErrors> for AppError {
    fn from(errors: validator::ValidationErrors) -> Self {
        let field_errors = errors
            .field_errors()
            .into_iter()
            .flat_map(|(field, errors)| {
                errors.iter().map(move |e| {
                    FieldError::new(
                        field,
                        e.message.as_ref().map(|m| m.to_string())
                            .unwrap_or_else(|| format!("Invalid value for {field}")),
                    )
                })
            })
            .collect();
        AppError::Validation(field_errors)
    }
}
```

# ============================================================
# SECTION 9: ROUTER SETUP
# ============================================================

```rust
// src/router.rs

use axum::{
    middleware as axum_mw,
    routing::{get, post},
    Router,
};
use tower::ServiceBuilder;
use tower_http::{
    catch_panic::CatchPanicLayer,
    compression::CompressionLayer,
    cors::CorsLayer,
    request_id::{MakeRequestUuid, PropagateRequestIdLayer, SetRequestIdLayer},
    timeout::TimeoutLayer,
    trace::TraceLayer,
};
use std::time::Duration;

use crate::handlers;
use crate::middleware;
use crate::state::AppState;

pub fn build(state: AppState) -> Router {
    let public_routes = Router::new()
        .route("/healthz", get(handlers::health::liveness))
        .route("/readyz", get(handlers::health::readiness))
        .route("/metrics", get(handlers::health::metrics));

    let protected_routes = Router::new()
        .route("/api/v1/bets", post(handlers::bet_handler::place_bet))
        .route(
            "/api/v1/bets/:bet_id",
            get(handlers::bet_handler::get_bet),
        )
        .route(
            "/api/v1/bets/:bet_id/cashout",
            post(handlers::bet_handler::cashout_bet),
        )
        .route(
            "/api/v1/bets/history",
            get(handlers::bet_handler::get_bet_history),
        )
        .layer(axum_mw::from_fn_with_state(
            state.clone(),
            middleware::auth::require_auth,
        ));

    Router::new()
        .merge(public_routes)
        .merge(protected_routes)
        .layer(
            ServiceBuilder::new()
                .layer(SetRequestIdLayer::x_request_id(MakeRequestUuid))
                .layer(PropagateRequestIdLayer::x_request_id())
                .layer(CatchPanicLayer::new())
                .layer(TimeoutLayer::new(Duration::from_secs(30)))
                .layer(CompressionLayer::new())
                .layer(
                    CorsLayer::new()
                        .allow_origin(tower_http::cors::Any) // configured properly in prod
                        .allow_methods(tower_http::cors::Any)
                        .allow_headers(tower_http::cors::Any),
                )
                .layer(
                    TraceLayer::new_for_http()
                        .make_span_with(middleware::tracing::make_span)
                        .on_response(middleware::tracing::on_response)
                        .on_failure(middleware::tracing::on_failure),
                )
                .layer(sentry::integrations::tower::NewSentryLayer::new_from_top())
                .layer(sentry::integrations::tower::SentryHttpLayer::with_transaction()),
        )
        .with_state(state)
}
```

# ============================================================
# SECTION 10: TESTING
# ============================================================

## UNIT TEST EXAMPLE

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use rust_decimal_macros::dec;

    #[test]
    fn test_bet_status_valid_transitions() {
        assert!(BetStatus::Pending.can_transition_to(BetStatus::Active));
        assert!(BetStatus::Pending.can_transition_to(BetStatus::Rejected));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Won));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Lost));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Void));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Cashout));
    }

    #[test]
    fn test_bet_status_invalid_transitions() {
        assert!(!BetStatus::Active.can_transition_to(BetStatus::Pending));
        assert!(!BetStatus::Won.can_transition_to(BetStatus::Lost));
        assert!(!BetStatus::Lost.can_transition_to(BetStatus::Won));
        assert!(!BetStatus::Void.can_transition_to(BetStatus::Active));
        assert!(!BetStatus::Cashout.can_transition_to(BetStatus::Active));
        assert!(!BetStatus::Rejected.can_transition_to(BetStatus::Active));
    }

    #[test]
    fn test_accumulator_odds_calculation() {
        let service = create_test_service();
        let selections = vec![
            CurrentOdds { odds: dec!(1.50) },
            CurrentOdds { odds: dec!(2.00) },
            CurrentOdds { odds: dec!(3.00) },
        ];

        let result = service.calculate_combined_odds(&selections, BetType::Accumulator);

        assert_eq!(result, dec!(9.00)); // 1.5 * 2.0 * 3.0
    }

    #[test]
    fn test_stake_validation_too_low() {
        let service = create_test_service();
        let result = service.validate_stake(UserId(1), dec!(0.05), dec!(0.10));

        assert!(matches!(result, Err(AppError::BetStakeTooLow { .. })));
    }

    #[test]
    fn test_stake_validation_too_high() {
        let service = create_test_service();
        let result = service.validate_stake(UserId(1), dec!(50000), dec!(100000));

        assert!(matches!(result, Err(AppError::BetStakeTooHigh { .. })));
    }

    #[test]
    fn test_stake_validation_max_payout_exceeded() {
        let service = create_test_service();
        let result = service.validate_stake(UserId(1), dec!(1000), dec!(200000));

        assert!(matches!(result, Err(AppError::BetMaxPayoutExceeded { .. })));
    }

    #[test]
    fn test_validate_positive_decimal_rejects_zero() {
        let result = validate_positive_decimal(&Decimal::ZERO);
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_positive_decimal_rejects_negative() {
        let result = validate_positive_decimal(&dec!(-1));
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_positive_decimal_accepts_positive() {
        let result = validate_positive_decimal(&dec!(0.01));
        assert!(result.is_ok());
    }
}
```

## INTEGRATION TEST EXAMPLE

```rust
// tests/integration/bet_placement_test.rs

use testcontainers::clients::Cli;
use testcontainers_modules::postgres::Postgres;
use testcontainers_modules::redis::Redis;

use betting_engine::state::AppState;
use betting_engine::config::AppConfig;
use platform_testing::fixtures::*;

#[tokio::test]
async fn test_place_single_bet_success() {
    // Arrange
    let docker = Cli::default();
    let pg = docker.run(Postgres::default());
    let redis = docker.run(Redis::default());

    let state = setup_test_state(&pg, &redis).await;

    // Create test user with balance
    let user = create_test_user(&state, UserFixture::default()).await;
    credit_test_balance(&state, user.id, dec!(1000)).await;
    create_test_event(&state, EventFixture::active_football()).await;

    // Act
    let request = PlaceBetRequest {
        bet_type: BetType::Single,
        selections: vec![SelectionRequest {
            event_id: EventId(1),
            market_id: MarketId(1),
            outcome_id: OutcomeId(1),
            odds: dec!(2.50),
        }],
        stake: dec!(100),
        currency_code: "USD".into(),
        accept_odds_changes: AcceptOddsChanges::None,
        idempotency_key: uuid::Uuid::new_v4(),
        ip_address: None,
        device_fingerprint: None,
    };

    let result = state.bet_service().place_bet(user.id, request).await;

    // Assert
    assert!(result.is_ok());
    let bet = result.unwrap();
    assert_eq!(bet.status, BetStatus::Pending);
    assert_eq!(bet.stake, dec!(100));
    assert_eq!(bet.combined_odds, dec!(2.50));
    assert_eq!(bet.potential_win, dec!(250));

    // Verify balance was locked
    let balance = get_test_balance(&state, user.id).await;
    assert_eq!(balance.available, dec!(900));
    assert_eq!(balance.locked, dec!(100));
}

#[tokio::test]
async fn test_place_bet_idempotency() {
    let docker = Cli::default();
    let pg = docker.run(Postgres::default());
    let redis = docker.run(Redis::default());
    let state = setup_test_state(&pg, &redis).await;

    let user = create_test_user(&state, UserFixture::default()).await;
    credit_test_balance(&state, user.id, dec!(1000)).await;
    create_test_event(&state, EventFixture::active_football()).await;

    let idempotency_key = uuid::Uuid::new_v4();
    let request = PlaceBetRequest {
        idempotency_key,
        stake: dec!(100),
        // ... other fields
        ..default_bet_request()
    };

    // First call
    let result1 = state.bet_service().place_bet(user.id, request.clone()).await.unwrap();
    // Second call with same idempotency key
    let result2 = state.bet_service().place_bet(user.id, request).await.unwrap();

    // Same bet returned, balance only deducted once
    assert_eq!(result1.id, result2.id);
    let balance = get_test_balance(&state, user.id).await;
    assert_eq!(balance.available, dec!(900)); // NOT 800
}

#[tokio::test]
async fn test_place_bet_insufficient_balance() {
    let docker = Cli::default();
    let pg = docker.run(Postgres::default());
    let redis = docker.run(Redis::default());
    let state = setup_test_state(&pg, &redis).await;

    let user = create_test_user(&state, UserFixture::default()).await;
    credit_test_balance(&state, user.id, dec!(50)).await;
    create_test_event(&state, EventFixture::active_football()).await;

    let request = PlaceBetRequest {
        stake: dec!(100), // balance is only 50
        ..default_bet_request()
    };

    let result = state.bet_service().place_bet(user.id, request).await;

    assert!(matches!(result, Err(AppError::InsufficientBalance { .. })));
}

#[tokio::test]
async fn test_concurrent_bets_no_overdraft() {
    let docker = Cli::default();
    let pg = docker.run(Postgres::default());
    let redis = docker.run(Redis::default());
    let state = setup_test_state(&pg, &redis).await;

    let user = create_test_user(&state, UserFixture::default()).await;
    credit_test_balance(&state, user.id, dec!(100)).await;
    create_test_event(&state, EventFixture::active_football()).await;

    // Place 10 concurrent bets of $20 each (total $200, but balance is $100)
    let mut handles = vec![];
    for _ in 0..10 {
        let state = state.clone();
        let user_id = user.id;
        handles.push(tokio::spawn(async move {
            let request = PlaceBetRequest {
                stake: dec!(20),
                idempotency_key: uuid::Uuid::new_v4(),
                ..default_bet_request()
            };
            state.bet_service().place_bet(user_id, request).await
        }));
    }

    let results: Vec<_> = futures::future::join_all(handles).await;

    // Exactly 5 should succeed, 5 should fail with insufficient balance
    let successes = results.iter().filter(|r| r.as_ref().unwrap().is_ok()).count();
    let failures = results.iter().filter(|r| r.as_ref().unwrap().is_err()).count();

    assert_eq!(successes, 5);
    assert_eq!(failures, 5);

    // Balance should be exactly 0
    let balance = get_test_balance(&state, user.id).await;
    assert_eq!(balance.available, dec!(0));
}
```

# ============================================================
# SECTION 11: DOCKERFILE
# ============================================================

```dockerfile
# services/betting-engine/Dockerfile

# ── Stage 1: Build ──
FROM rust:1.80-bookworm AS builder

WORKDIR /app

# Install protobuf compiler
RUN apt-get update && apt-get install -y protobuf-compiler && rm -rf /var/lib/apt/lists/*

# Copy workspace files
COPY Cargo.toml Cargo.lock ./
COPY libs/rust-platform/ libs/rust-platform/
COPY proto/ proto/

# Copy service
COPY services/betting-engine/ services/betting-engine/

# Build release binary
RUN cargo build --release --package betting-engine

# ── Stage 2: Runtime ──
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=builder /app/target/release/betting-engine /app/betting-engine
COPY services/betting-engine/config/ /app/config/

WORKDIR /app

EXPOSE 8080 9090

USER nonroot:nonroot

ENTRYPOINT ["/app/betting-engine"]
```

# ============================================================
# SECTION 12: ANTI-PATTERNS SUMMARY
# ============================================================

```text
❌ NEVER use unwrap() or expect() in production code
   ✅ Use ? operator or handle with match/if let

❌ NEVER use panic!() for error handling
   ✅ Return Result<T, AppError>

❌ NEVER use f64/f32 for money
   ✅ Use rust_decimal::Decimal

❌ NEVER use String where &str suffices (avoid allocations)
   ✅ Accept &str in function parameters, return String from functions

❌ NEVER clone() in hot paths without profiling first
   ✅ Use references, Arc, or Cow<>

❌ NEVER use std::sync::Mutex (it's not async-aware)
   ✅ Use tokio::sync::Mutex or parking_lot::Mutex for sync code

❌ NEVER block the Tokio runtime (no std::thread::sleep, no CPU-heavy loops)
   ✅ Use tokio::time::sleep, spawn_blocking for CPU work

❌ NEVER use println! or dbg! in production
   ✅ Use tracing::info!, tracing::debug!, tracing::error!

❌ NEVER ignore errors (let _ = might_fail())
   ✅ Log and handle: if let Err(e) = might_fail() { warn!(%e); }

❌ NEVER hardcode configuration values
   ✅ Use AppConfig loaded from env/file/Vault

❌ NEVER write SQL strings with format!() (SQL injection)
   ✅ Use sqlx::query! with $1, $2 parameters

❌ NEVER return internal error details to clients
   ✅ Log internally, return generic "Internal error" to client

❌ NEVER create unbounded channels or vectors from user input
   ✅ Always set limits (max selections, max page size, etc.)

❌ NEVER use Box<dyn Error> in public APIs
   ✅ Use concrete AppError enum with thiserror

❌ NEVER skip running clippy
   ✅ CI blocks merge if clippy has warnings
```
```

---

