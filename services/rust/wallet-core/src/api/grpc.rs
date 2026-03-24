//! gRPC service implementation

use std::sync::Arc;
use tonic::{Request, Response, Status};
use uuid::Uuid;

use crate::api::AppState;
use crate::domain::*;
use crate::infrastructure::*;
use crate::service::*;

// Import generated proto types
use wallet_proto::wallet_core_service_server::{WalletCoreService, WalletCoreServiceServer};
use wallet_proto::*;
use common_proto::common::v1::*;

/// gRPC Wallet Core Service
pub struct WalletGrpcService {
    wallet_service: Arc<WalletService>,
}

impl WalletGrpcService {
    pub fn new(state: Arc<AppState>) -> Self {
        let wallet_repo = Arc::new(WalletRepository::new(state.db_pool.clone()));
        let transaction_repo = Arc::new(TransactionRepository::new(state.db_pool.clone()));
        let lock_repo = Arc::new(LockRepository::new(state.db_pool.clone()));
        let idempotency = Arc::new(IdempotencyService::new(
            state.redis_client.clone(),
            3600, // 1 hour TTL
        ));
        let event_publisher = Arc::new(EventPublisher::new(true));
        
        let wallet_service = Arc::new(WalletService::new(
            wallet_repo,
            transaction_repo,
            lock_repo,
            idempotency,
            event_publisher,
        ));
        
        Self { wallet_service }
    }
    
    pub fn into_service(self) -> WalletCoreServiceServer<Self> {
        WalletCoreServiceServer::new(self)
    }
    
    /// Serve gRPC server
    pub async fn serve(addr: std::net::SocketAddr, state: Arc<AppState>) -> Result<(), Box<dyn std::error::Error>> {
        let service = Self::new(state);
        
        tonic::transport::Server::builder()
            .add_service(service.into_service())
            .serve(addr)
            .await?;
        
        Ok(())
    }
}

#[tonic::async_trait]
impl WalletCoreService for WalletGrpcService {
    async fn get_balance(
        &self,
        request: Request<GetBalanceRequest>,
    ) -> Result<Response<GetBalanceResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let wallet_type = parse_wallet_type(request::WalletType::try_from(req.wallet_type).map_err(|_| Status::invalid_argument("Invalid wallet_type"))?)?;
        
        match self.wallet_service.get_balance(user_id, wallet_type).await {
            Ok(balance) => {
                let response = GetBalanceResponse {
                    available: Some(money_from_decimal(balance.available, "USD")),
                    locked: Some(money_from_decimal(balance.locked, "USD")),
                    bonus: Some(money_from_decimal(balance.bonus, "USD")),
                    version: 0,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::NotFound { .. }) => Err(Status::not_found("Wallet not found")),
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn get_wallets(
        &self,
        request: Request<GetWalletsRequest>,
    ) -> Result<Response<GetWalletsResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        // Get all wallets for user
        let wallets = self.wallet_service
            .get_or_create_wallet(user_id, WalletType::Main, "USD")
            .await
            .map(|w| vec![w])
            .unwrap_or_default();
        
        let response = GetWalletsResponse {
            wallets: wallets.iter().map(wallet_to_proto).collect(),
        };
        
        Ok(Response::new(response))
    }
    
    async fn credit(
        &self,
        request: Request<CreditRequest>,
    ) -> Result<Response<CreditResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let wallet_type = parse_wallet_type(WalletType::try_from(req.wallet_type).map_err(|_| Status::invalid_argument("Invalid wallet_type"))?)?;
        
        let amount = parse_money(&req.amount)
            .map_err(|_| Status::invalid_argument("Invalid amount"))?;
        
        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;
        
        match self.wallet_service.credit(
            user_id,
            wallet_type,
            amount,
            "USD",
            reference_id,
            req.reference_type.as_str(),
            Some(req.idempotency_key),
        ).await {
            Ok(txn) => {
                let response = CreditResponse {
                    transaction: Some(transaction_to_proto(&txn)),
                    new_balance: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn debit(
        &self,
        request: Request<DebitRequest>,
    ) -> Result<Response<DebitResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let wallet_type = parse_wallet_type(WalletType::try_from(req.wallet_type).map_err(|_| Status::invalid_argument("Invalid wallet_type"))?)?;
        
        let amount = parse_money(&req.amount)
            .map_err(|_| Status::invalid_argument("Invalid amount"))?;
        
        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;
        
        match self.wallet_service.debit(
            user_id,
            wallet_type,
            amount,
            "USD",
            reference_id,
            req.reference_type.as_str(),
            Some(req.idempotency_key),
        ).await {
            Ok(txn) => {
                let response = DebitResponse {
                    transaction: Some(transaction_to_proto(&txn)),
                    new_balance: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::InsufficientAvailableBalance { .. }) => {
                Err(Status::failed_precondition("Insufficient balance"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn lock(
        &self,
        request: Request<LockRequest>,
    ) -> Result<Response<LockResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let wallet_type = parse_wallet_type(WalletType::try_from(req.wallet_type).map_err(|_| Status::invalid_argument("Invalid wallet_type"))?)?;
        
        let amount = parse_money(&req.amount)
            .map_err(|_| Status::invalid_argument("Invalid amount"))?;
        
        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;
        
        match self.wallet_service.lock(
            user_id,
            wallet_type,
            amount,
            reference_id,
            Some(req.idempotency_key),
        ).await {
            Ok(lock) => {
                let response = LockResponse {
                    success: true,
                    new_available: None,
                    new_locked: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::InsufficientAvailableBalance { .. }) => {
                Err(Status::failed_precondition("Insufficient balance"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn unlock(
        &self,
        request: Request<UnlockRequest>,
    ) -> Result<Response<UnlockResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;
        
        match self.wallet_service.unlock(user_id, reference_id).await {
            Ok(_) => {
                let response = UnlockResponse {
                    success: true,
                    new_available: None,
                    new_locked: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::LockReferenceNotFound(_)) => {
                Err(Status::not_found("Lock not found"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn transfer(
        &self,
        request: Request<TransferRequest>,
    ) -> Result<Response<TransferResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let from_wallet_type = parse_wallet_type(WalletType::try_from(req.from_wallet).map_err(|_| Status::invalid_argument("Invalid from_wallet"))?)?;
        let to_wallet_type = parse_wallet_type(WalletType::try_from(req.to_wallet).map_err(|_| Status::invalid_argument("Invalid to_wallet"))?)?;
        
        let amount = parse_money(&req.amount)
            .map_err(|_| Status::invalid_argument("Invalid amount"))?;
        
        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;
        
        match self.wallet_service.transfer(
            user_id,
            from_wallet_type,
            to_wallet_type,
            amount,
            "USD",
            reference_id,
            Some(req.idempotency_key),
        ).await {
            Ok((debit_txn, credit_txn)) => {
                let response = TransferResponse {
                    debit_transaction: Some(transaction_to_proto(&debit_txn)),
                    credit_transaction: Some(transaction_to_proto(&credit_txn)),
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }
    
    async fn get_transactions(
        &self,
        request: Request<GetTransactionsRequest>,
    ) -> Result<Response<GetTransactionsResponse>, Status> {
        let req = request.into_inner();
        
        let user_id = parse_uuid(&req.user_id)
            .map_err(|_| Status::invalid_argument("Invalid user_id"))?;
        
        let limit = req.limit.max(1).min(100) as i64;
        let offset = req.offset.max(0) as i64;
        
        let transactions = self.wallet_service
            .get_transactions(user_id, limit, offset)
            .await
            .unwrap_or_default();
        
        let response = GetTransactionsResponse {
            transactions: transactions.iter().map(transaction_to_proto).collect(),
            total: transactions.len() as i32,
        };
        
        Ok(Response::new(response))
    }
}

// Helper functions

fn parse_uuid(proto: &Option<UserId>) -> Result<Uuid, String> {
    proto
        .as_ref()
        .and_then(|id| Uuid::parse_str(&id.value).ok())
        .ok_or_else(|| "Invalid UUID".to_string())
}

fn parse_wallet_type(proto: common_proto::common::v1::WalletType) -> Result<WalletType, String> {
    match proto {
        common_proto::common::v1::WalletType::Unspecified => Ok(WalletType::Main),
        common_proto::common::v1::WalletType::Main => Ok(WalletType::Main),
        common_proto::common::v1::WalletType::Bonus => Ok(WalletType::Bonus),
        common_proto::common::v1::WalletType::FreeSpins => Ok(WalletType::FreeSpins),
        common_proto::common::v1::WalletType::Cashback => Ok(WalletType::Cashback),
    }
}

fn parse_money(proto: &Option<Money>) -> Result<rust_decimal::Decimal, String> {
    proto
        .as_ref()
        .and_then(|m| rust_decimal::Decimal::from_str_exact(&m.amount).ok())
        .ok_or_else(|| "Invalid money amount".to_string())
}

fn money_from_decimal(amount: rust_decimal::Decimal, currency: &str) -> Money {
    Money {
        amount: amount.to_string(),
        currency: currency.to_string(),
    }
}

fn wallet_to_proto(wallet: &Wallet) -> common_proto::common::v1::Wallet {
    common_proto::common::v1::Wallet {
        user_id: Some(UserId { value: wallet.user_id.to_string() }),
        r#type: wallet_type_to_proto(wallet.wallet_type),
        balance: Some(money_from_decimal(wallet.balance_available, &wallet.currency)),
        locked: Some(money_from_decimal(wallet.balance_locked, &wallet.currency)),
        updated_at: Some(prost_types::Timestamp::from(wallet.updated_at)),
    }
}

fn wallet_type_to_proto(wallet_type: WalletType) -> i32 {
    match wallet_type {
        WalletType::Main => common_proto::common::v1::WalletType::Main as i32,
        WalletType::Bonus => common_proto::common::v1::WalletType::Bonus as i32,
        WalletType::FreeSpins => common_proto::common::v1::WalletType::FreeSpins as i32,
        WalletType::Cashback => common_proto::common::v1::WalletType::Cashback as i32,
    }
}

fn transaction_to_proto(txn: &Transaction) -> common_proto::common::v1::Transaction {
    common_proto::common::v1::Transaction {
        id: Some(TransactionId { value: txn.id.to_string() }),
        user_id: Some(UserId { value: txn.user_id.to_string() }),
        wallet_type: wallet_type_to_proto(txn.wallet_type) as i32,
        amount: Some(money_from_decimal(txn.amount, &txn.currency)),
        r#type: txn.transaction_type.as_str().to_string(),
        reference_id: txn.reference_id.map(|id| id.to_string()).unwrap_or_default(),
        reference_type: txn.reference_type.clone().unwrap_or_default(),
        created_at: Some(prost_types::Timestamp::from(txn.created_at)),
        status: txn.status.as_str().to_string(),
    }
}
