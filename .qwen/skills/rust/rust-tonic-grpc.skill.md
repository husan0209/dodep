# SKILL #10 — rust-tonic-grpc.skill.md

```markdown
# rust-tonic-grpc.skill.md
# GAMBLING PLATFORM — RUST TONIC gRPC PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

All service-to-service communication uses gRPC + Protobuf via Tonic.
Proto files live in shared proto/ directory.
Code generated at build time via tonic-build.

# ============================================================
# SECTION 2: BUILD SETUP
# ============================================================

```rust
// build.rs
fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .type_attribute(".", "#[derive(serde::Serialize, serde::Deserialize)]")
        .compile_protos(
            &[
                "../../proto/betting/v1/betting.proto",
                "../../proto/wallet/v1/wallet.proto",
                "../../proto/risk/v1/risk.proto",
            ],
            &["../../proto"],
        )?;
    Ok(())
}
toml

# Cargo.toml
[build-dependencies]
tonic-build = { workspace = true }
============================================================
SECTION 3: gRPC SERVER
============================================================
Rust

// src/grpc/server.rs
use tonic::{transport::Server, Request, Response, Status};

// Generated proto module
pub mod betting_proto {
    tonic::include_proto!("platform.betting.v1");
}

use betting_proto::betting_service_server::{BettingService, BettingServiceServer};
use betting_proto::*;

pub struct BettingGrpcService {
    bet_service: BetService,
}

impl BettingGrpcService {
    pub fn new(bet_service: BetService) -> Self {
        Self { bet_service }
    }
}

#[tonic::async_trait]
impl BettingService for BettingGrpcService {
    async fn place_bet(
        &self,
        request: Request<PlaceBetRequest>,
    ) -> Result<Response<PlaceBetResponse>, Status> {
        let req = request.into_inner();
        
        // Extract trace context from metadata
        let user_id = req.user_id;
        
        // Convert proto → domain
        let domain_req = req.try_into()
            .map_err(|e: ValidationError| Status::invalid_argument(e.to_string()))?;
        
        // Call service
        let bet = self.bet_service
            .place_bet(UserId(user_id), domain_req)
            .await
            .map_err(|e| map_app_error_to_status(e))?;
        
        // Convert domain → proto
        Ok(Response::new(PlaceBetResponse::from(bet)))
    }

    async fn get_bet(
        &self,
        request: Request<GetBetRequest>,
    ) -> Result<Response<GetBetResponse>, Status> {
        let req = request.into_inner();
        
        let bet = self.bet_service
            .get_bet(UserId(req.user_id), BetId(req.bet_id))
            .await
            .map_err(|e| map_app_error_to_status(e))?;
        
        Ok(Response::new(GetBetResponse::from(bet)))
    }

    async fn settle_bet(
        &self,
        request: Request<SettleBetRequest>,
    ) -> Result<Response<SettleBetResponse>, Status> {
        let req = request.into_inner();
        
        let result = self.bet_service
            .settle(BetId(req.bet_id), req.result.try_into()?)
            .await
            .map_err(|e| map_app_error_to_status(e))?;
        
        Ok(Response::new(SettleBetResponse::from(result)))
    }
}

// Start gRPC server
pub async fn start(state: AppState, port: u16) -> Result<(), tonic::transport::Error> {
    let addr = format!("0.0.0.0:{port}").parse().unwrap();
    
    let service = BettingGrpcService::new(state.bet_service().clone());
    
    tracing::info!(%addr, "gRPC server listening");
    
    Server::builder()
        .timeout(std::time::Duration::from_secs(30))
        .add_service(BettingServiceServer::new(service))
        .serve(addr)
        .await
}
ERROR MAPPING
Rust

fn map_app_error_to_status(err: AppError) -> Status {
    match err {
        AppError::Validation(fields) => {
            let msg = serde_json::to_string(&fields).unwrap_or_default();
            Status::invalid_argument(msg)
        }
        AppError::NotFound { entity, id } => {
            Status::not_found(format!("{entity} {id} not found"))
        }
        AppError::Unauthorized { reason } => Status::unauthenticated(reason),
        AppError::Forbidden { reason } => Status::permission_denied(reason),
        AppError::InsufficientBalance { .. } => {
            Status::failed_precondition(err.to_string())
        }
        AppError::BetOddsChanged { .. } => {
            Status::aborted(err.to_string())
        }
        AppError::Conflict { .. } | AppError::BetAlreadySettled => {
            Status::already_exists(err.to_string())
        }
        AppError::RateLimited { .. } => Status::resource_exhausted(err.to_string()),
        AppError::ServiceUnavailable(svc) => {
            Status::unavailable(format!("{svc} unavailable"))
        }
        _ => {
            tracing::error!(error = %err, "Internal gRPC error");
            Status::internal("Internal error")
        }
    }
}
============================================================
SECTION 4: gRPC CLIENT
============================================================
Rust

// src/grpc/clients.rs
use tonic::transport::{Channel, Endpoint};
use std::time::Duration;

pub mod wallet_proto {
    tonic::include_proto!("platform.wallet.v1");
}

use wallet_proto::wallet_service_client::WalletServiceClient;

pub async fn create_wallet_client(
    address: &str,
) -> Result<WalletServiceClient<Channel>, tonic::transport::Error> {
    let channel = Endpoint::from_shared(address.to_string())?
        .timeout(Duration::from_secs(5))
        .connect_timeout(Duration::from_secs(5))
        .keep_alive_timeout(Duration::from_secs(20))
        .keep_alive_while_idle(true)
        .connect()
        .await?;
    
    Ok(WalletServiceClient::new(channel))
}

// Usage in service layer
pub async fn lock_wallet_funds(
    client: &mut WalletServiceClient<Channel>,
    user_id: i64,
    amount: Decimal,
    idempotency_key: Uuid,
) -> Result<i64, AppError> {
    let request = tonic::Request::new(wallet_proto::LockRequest {
        user_id,
        currency_code: "USD".into(),
        amount: amount.to_string(),
        idempotency_key: idempotency_key.to_string(),
        reference_type: "bet".into(),
        reference_id: 0,
    });
    
    // Set deadline
    request.set_timeout(Duration::from_secs(3));
    
    let response = client
        .lock(request)
        .await
        .map_err(|status| match status.code() {
            tonic::Code::FailedPrecondition => AppError::InsufficientBalance {
                required: amount,
                available: Decimal::ZERO,
            },
            tonic::Code::Unavailable => AppError::ServiceUnavailable("wallet".into()),
            tonic::Code::DeadlineExceeded => AppError::ServiceUnavailable("wallet timeout".into()),
            _ => AppError::Internal(anyhow::anyhow!("wallet gRPC: {}", status.message())),
        })?;
    
    Ok(response.into_inner().lock_id)
}
============================================================
SECTION 5: INTERCEPTORS (middleware)
============================================================
Rust

use tonic::{service::Interceptor, metadata::MetadataValue};

// Client interceptor: inject trace context
#[derive(Clone)]
pub struct TraceInterceptor;

impl Interceptor for TraceInterceptor {
    fn call(&mut self, mut request: tonic::Request<()>) -> Result<tonic::Request<()>, Status> {
        // Inject OpenTelemetry trace context
        let span = tracing::Span::current();
        if let Some(trace_id) = span.context().span().span_context().trace_id().to_string().into() {
            request.metadata_mut().insert(
                "x-trace-id",
                MetadataValue::try_from(&trace_id).unwrap(),
            );
        }
        Ok(request)
    }
}

// Server interceptor: extract auth from metadata
pub fn auth_interceptor(req: Request<()>) -> Result<Request<()>, Status> {
    let token = req.metadata()
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .ok_or_else(|| Status::unauthenticated("Missing auth token"))?;
    
    // Validate token (fast, in-memory check)
    // In practice, internal services use mTLS via Istio, not JWT
    Ok(req)
}

// Usage with interceptor
let client = WalletServiceClient::with_interceptor(channel, TraceInterceptor);
============================================================
SECTION 6: PROTO ↔ DOMAIN CONVERSION
============================================================
Rust

// Always convert between proto types and domain types.
// Proto types are auto-generated, domain types are hand-written.
// NEVER use proto types in service/repository layer.

impl TryFrom<betting_proto::PlaceBetRequest> for domain::PlaceBetInput {
    type Error = ValidationError;
    
    fn try_from(proto: betting_proto::PlaceBetRequest) -> Result<Self, Self::Error> {
        let stake = Decimal::from_str(&proto.stake)
            .map_err(|_| ValidationError::new("stake", "Invalid decimal"))?;
        
        let selections = proto.selections
            .into_iter()
            .map(|s| domain::SelectionInput {
                event_id: EventId(s.event_id),
                market_id: MarketId(s.market_id),
                outcome_id: OutcomeId(s.outcome_id),
                odds: Decimal::from_str(&s.odds).unwrap_or_default(),
            })
            .collect();
        
        Ok(domain::PlaceBetInput { stake, selections, /* ... */ })
    }
}

impl From<domain::Bet> for betting_proto::PlaceBetResponse {
    fn from(bet: domain::Bet) -> Self {
        Self {
            bet_id: bet.id.0,
            status: bet.status.to_string(),
            stake: bet.stake.to_string(),
            odds: bet.combined_odds.to_string(),
            potential_win: bet.potential_win.to_string(),
        }
    }
}
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER use proto types in service/repository layer (always convert)
❌ NEVER skip setting timeout/deadline on gRPC calls
❌ NEVER create new Channel per request (reuse via connection pool)
❌ NEVER return Status::internal with detailed error message to external callers
❌ NEVER use unary call for streaming data (use server streaming)
❌ NEVER ignore tonic::Code in error handling (map each code properly)
❌ NEVER hardcode service addresses (use config / service discovery)
❌ NEVER skip trace context propagation in interceptors
❌ NEVER put business logic in gRPC service impl (delegate to service layer)
============================================================
SECTION 8: TESTING
============================================================
Rust

#[tokio::test]
async fn test_place_bet_grpc() {
    // Start test gRPC server
    let state = create_test_state().await;
    let service = BettingGrpcService::new(state.bet_service().clone());
    
    let (client, server) = tokio::io::duplex(1024);
    
    tokio::spawn(async move {
        Server::builder()
            .add_service(BettingServiceServer::new(service))
            .serve_with_incoming(futures::stream::once(async { Ok::<_, std::io::Error>(server) }))
            .await
    });
    
    let mut client = BettingServiceClient::new(
        Endpoint::try_from("http://[::]:50051")?
            .connect_with_connector(tower::service_fn(move |_| {
                let client = client.clone();
                async { Ok::<_, std::io::Error>(client) }
            }))
            .await?
    );
    
    let response = client.place_bet(PlaceBetRequest {
        user_id: 1,
        stake: "100.00".into(),
        // ...
    }).await.unwrap();
    
    assert!(response.into_inner().bet_id > 0);
}