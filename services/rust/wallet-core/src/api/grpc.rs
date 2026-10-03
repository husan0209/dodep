//! gRPC service implementation

use std::sync::Arc;
use tonic::{Request, Response, Status};
use uuid::Uuid;

use crate::api::AppState;
use crate::domain::*;
use crate::infrastructure::*;
use crate::service::*;

// Import generated proto types
use crate::proto::common::v1 as common_proto;
use crate::proto::common::v1::{Money, TransactionId, UserId};
use crate::proto::wallet::v1::wallet_core_service_server::{
    WalletCoreService, WalletCoreServiceServer,
};
use crate::proto::wallet::v1::*;

/// gRPC Wallet Core Service
pub struct WalletGrpcService {
    wallet_service: Arc<WalletService>,
}

impl WalletGrpcService {
    pub fn new(state: Arc<AppState>) -> Self {
        let wallet_repo = Arc::new(WalletRepository::new(state.db_pool.clone()));
        let transaction_repo = Arc::new(TransactionRepository::new(state.db_pool.clone()));
        let ledger_repo = Arc::new(LedgerRepository::new(state.db_pool.clone()));
        let lock_repo = Arc::new(LockRepository::new());
        let outbox_repo = Arc::new(OutboxRepository::new(state.db_pool.clone()));
        let idempotency = Arc::new(IdempotencyService::new(
            state.redis_client.clone(),
            3600, // 1 hour TTL
        ));

        let wallet_service = Arc::new(WalletService::new(
            wallet_repo,
            transaction_repo,
            ledger_repo,
            lock_repo,
            outbox_repo,
            idempotency,
        ));

        Self { wallet_service }
    }

    pub fn into_service(self) -> WalletCoreServiceServer<Self> {
        WalletCoreServiceServer::new(self)
    }

    /// Serve gRPC server
    pub async fn serve(
        addr: std::net::SocketAddr,
        state: Arc<AppState>,
    ) -> Result<(), Box<dyn std::error::Error>> {
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

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let wallet_type = parse_wallet_type_from_i32(req.wallet_type)?;

        match self.wallet_service.get_balance(user_id, wallet_type).await {
            Ok(balance) => {
                let response = GetBalanceResponse {
                    balance: Some(common_proto::Balance {
                        available: Some(money_from_decimal(balance.available, "USD")),
                        locked: Some(money_from_decimal(balance.locked, "USD")),
                        bonus: Some(money_from_decimal(balance.bonus, "USD")),
                        total: Some(money_from_decimal(
                            balance.available + balance.locked + balance.bonus,
                            "USD",
                        )),
                    }),
                    version: 0,
                    updated_at: Some(dt_to_timestamp(chrono::Utc::now())),
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

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        // Get all wallets for user
        let wallets = self
            .wallet_service
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

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let wallet_type = parse_wallet_type_from_i32(req.wallet_type)?;

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;

        match self
            .wallet_service
            .credit(
                user_id,
                wallet_type,
                amount,
                "USD",
                reference_id,
                req.reference_type.as_str(),
                &req.idempotency_key,
            )
            .await
        {
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

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let wallet_type = parse_wallet_type_from_i32(req.wallet_type)?;

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;

        match self
            .wallet_service
            .debit(
                user_id,
                wallet_type,
                amount,
                "USD",
                reference_id,
                req.reference_type.as_str(),
                &req.idempotency_key,
            )
            .await
        {
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

    async fn lock(&self, request: Request<LockRequest>) -> Result<Response<LockResponse>, Status> {
        let req = request.into_inner();

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let wallet_type = parse_wallet_type_from_i32(req.wallet_type)?;

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;

        let reference_type = parse_reference_type(&req.reference_type, "lock");

        match self
            .wallet_service
            .lock(user_id, wallet_type, amount, reference_id, reference_type)
            .await
        {
            Ok(lock) => {
                let response = LockResponse {
                    success: true,
                    new_available: None,
                    new_locked: None,
                    error: None,
                    lock_id: lock.id.to_string(),
                };
                Ok(Response::new(response))
            }
            Err(WalletError::InsufficientAvailableBalance { .. }) => {
                Err(Status::failed_precondition("Insufficient balance"))
            }
            Err(WalletError::LockReferenceExists(_)) | Err(WalletError::AlreadyExists { .. }) => {
                Err(Status::already_exists("Lock reference already exists"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }

    async fn unlock(
        &self,
        request: Request<UnlockRequest>,
    ) -> Result<Response<UnlockResponse>, Status> {
        let req = request.into_inner();

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;
        let reference_type = parse_reference_type(&req.reference_type, "lock");

        match self
            .wallet_service
            .unlock(user_id, reference_type, reference_id)
            .await
        {
            Ok(_released) => {
                let response = UnlockResponse {
                    success: true,
                    new_available: None,
                    new_locked: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::LockReferenceNotFound(_)) => Err(Status::not_found("Lock not found")),
            // Compensation replays are expected: the lock is already terminal,
            // so the money is safe. Report success instead of an error.
            Err(WalletError::LockAlreadySettled(_)) => {
                let response = UnlockResponse {
                    success: true,
                    new_available: None,
                    new_locked: None,
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::LockOwnershipMismatch { .. }) => {
                Err(Status::permission_denied("Lock belongs to another user"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }

    async fn consume_lock(
        &self,
        request: Request<ConsumeLockRequest>,
    ) -> Result<Response<ConsumeLockResponse>, Status> {
        let req = request.into_inner();

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;
        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;
        let reference_type = parse_reference_type(&req.reference_type, "lock");
        if req.idempotency_key.trim().is_empty() {
            return Err(Status::invalid_argument("idempotency_key is required"));
        }
        let description = match req.description.trim() {
            "" => None,
            other => Some(other.to_string()),
        };

        match self
            .wallet_service
            .consume_lock(
                user_id,
                reference_type,
                reference_id,
                req.idempotency_key.clone(),
                description,
            )
            .await
        {
            Ok(txn) => {
                let response = ConsumeLockResponse {
                    success: true,
                    transaction: Some(transaction_to_proto(&txn)),
                    error: None,
                };
                Ok(Response::new(response))
            }
            Err(WalletError::LockReferenceNotFound(_)) => Err(Status::not_found("Lock not found")),
            Err(WalletError::LockOwnershipMismatch { .. }) => {
                Err(Status::permission_denied("Lock belongs to another user"))
            }
            Err(WalletError::LockAlreadySettled(_)) => {
                Err(Status::failed_precondition("Lock already settled"))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }

    async fn transfer(
        &self,
        request: Request<TransferRequest>,
    ) -> Result<Response<TransferResponse>, Status> {
        let req = request.into_inner();

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let from_wallet_type = parse_wallet_type_from_i32(req.from_wallet)?;
        let to_wallet_type = parse_wallet_type_from_i32(req.to_wallet)?;

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id =
            parse_reference_id(&req.reference_id).map_err(|e| Status::invalid_argument(e))?;

        match self
            .wallet_service
            .transfer(
                user_id,
                from_wallet_type,
                to_wallet_type,
                amount,
                "USD",
                reference_id,
                Some(req.idempotency_key),
            )
            .await
        {
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

        let user_id = parse_user_id(&req.user_id).map_err(|e| Status::invalid_argument(e))?;

        let (limit, offset) = match req.pagination {
            Some(p) => {
                let l = p.page_size.max(1).min(100) as i64;
                let o = p.cursor.parse::<i64>().unwrap_or(0).max(0);
                (l, o)
            }
            None => (20, 0),
        };

        let transactions = self
            .wallet_service
            .get_transactions(user_id, limit, offset)
            .await
            .unwrap_or_default();

        let response = GetTransactionsResponse {
            transactions: transactions.iter().map(transaction_to_proto).collect(),
            pagination: Some(common_proto::PageResponse {
                next_cursor: (offset + limit).to_string(),
                prev_cursor: String::new(),
                has_more: transactions.len() as i64 == limit,
                total_count: Some(0),
                current_page: None,
                total_pages: None,
            }),
        };

        Ok(Response::new(response))
    }

    async fn get_transaction(
        &self,
        request: Request<GetTransactionRequest>,
    ) -> Result<Response<GetTransactionResponse>, Status> {
        let req = request.into_inner();

        let tx_id = req
            .transaction_id
            .as_ref()
            .and_then(|id| Uuid::parse_str(&id.value).ok())
            .ok_or_else(|| Status::invalid_argument("Invalid transaction_id"))?;

        let transaction =
            self.wallet_service
                .get_transaction(tx_id)
                .await
                .map_err(|e| match e {
                    WalletError::TransactionNotFound(_) => {
                        Status::not_found("Transaction not found")
                    }
                    _ => Status::internal(e.to_string()),
                })?;

        let response = GetTransactionResponse {
            transaction: Some(transaction_to_proto(&transaction)),
        };

        Ok(Response::new(response))
    }
}

// Helper functions

/// Platform users are identified by `users.id` (BIGINT). Callers send the
/// decimal form in `common.v1.UserId`, e.g. "42".
fn parse_user_id(proto: &Option<UserId>) -> Result<i64, String> {
    let raw = proto
        .as_ref()
        .map(|id| id.value.trim())
        .filter(|v| !v.is_empty())
        .ok_or_else(|| "user_id is required".to_string())?;
    raw.parse::<i64>()
        .ok()
        .filter(|id| *id > 0)
        .ok_or_else(|| format!("Invalid user_id: {raw} (expected a positive integer)"))
}

/// Business references belong to the calling service and are opaque to
/// wallet-core; they are not necessarily UUIDs.
fn parse_reference_id(proto: &str) -> Result<String, String> {
    let trimmed = proto.trim();
    if trimmed.is_empty() {
        return Err("reference_id is required".to_string());
    }
    if trimmed.len() > 255 {
        return Err("reference_id must be at most 255 characters".to_string());
    }
    Ok(trimmed.to_string())
}

fn parse_reference_type(proto: &str, fallback: &str) -> String {
    let trimmed = proto.trim();
    if trimmed.is_empty() {
        fallback.to_string()
    } else {
        trimmed.to_string()
    }
}

fn parse_wallet_type_from_i32(val: i32) -> Result<WalletType, Status> {
    let proto = common_proto::WalletType::try_from(val)
        .map_err(|_| Status::invalid_argument("Invalid wallet_type"))?;
    match proto {
        common_proto::WalletType::Unspecified => Ok(WalletType::Main),
        common_proto::WalletType::Main => Ok(WalletType::Main),
        common_proto::WalletType::Bonus => Ok(WalletType::Bonus),
        common_proto::WalletType::FreeSpins => Ok(WalletType::FreeSpins),
        common_proto::WalletType::Cashback => Ok(WalletType::Cashback),
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

fn wallet_to_proto(wallet: &Wallet) -> common_proto::Wallet {
    common_proto::Wallet {
        user_id: Some(UserId {
            value: wallet.user_id.to_string(),
        }),
        r#type: wallet_type_to_proto(wallet.wallet_type),
        balance: Some(common_proto::Balance {
            available: Some(money_from_decimal(
                wallet.balance_available,
                &wallet.currency,
            )),
            locked: Some(money_from_decimal(wallet.balance_locked, &wallet.currency)),
            bonus: Some(money_from_decimal(
                rust_decimal::Decimal::ZERO,
                &wallet.currency,
            )),
            total: Some(money_from_decimal(
                wallet.balance_available + wallet.balance_locked,
                &wallet.currency,
            )),
        }),
        currency: wallet.currency.clone(),
        created_at: Some(dt_to_timestamp(wallet.created_at)),
        updated_at: Some(dt_to_timestamp(wallet.updated_at)),
        is_active: wallet.is_active,
    }
}

fn dt_to_timestamp(dt: chrono::DateTime<chrono::Utc>) -> prost_types::Timestamp {
    prost_types::Timestamp {
        seconds: dt.timestamp(),
        nanos: dt.timestamp_subsec_nanos() as i32,
    }
}

fn wallet_type_to_proto(wallet_type: WalletType) -> i32 {
    match wallet_type {
        WalletType::Main => common_proto::WalletType::Main as i32,
        WalletType::Bonus => common_proto::WalletType::Bonus as i32,
        WalletType::FreeSpins => common_proto::WalletType::FreeSpins as i32,
        WalletType::Cashback => common_proto::WalletType::Cashback as i32,
    }
}

fn transaction_to_proto(txn: &Transaction) -> common_proto::Transaction {
    common_proto::Transaction {
        id: Some(TransactionId {
            value: txn.id.to_string(),
        }),
        user_id: Some(UserId {
            value: txn.user_id.to_string(),
        }),
        wallet_type: wallet_type_to_proto(txn.wallet_type) as i32,
        amount: Some(money_from_decimal(txn.amount, &txn.currency)),
        r#type: transaction_type_to_proto(txn.transaction_type) as i32,
        reference_id: txn.reference_id.clone().unwrap_or_default(),
        reference_type: txn.reference_type.clone().unwrap_or_default(),
        idempotency_key: txn.idempotency_key.clone().unwrap_or_default(),
        created_at: Some(dt_to_timestamp(txn.created_at)),
        updated_at: Some(dt_to_timestamp(txn.updated_at)),
        status: transaction_status_to_proto(&txn.status) as i32,
        description: txn.description.clone().unwrap_or_default(),
        metadata: std::collections::HashMap::new(),
    }
}

fn transaction_type_to_proto(t: TransactionType) -> common_proto::TransactionType {
    match t {
        TransactionType::Deposit => common_proto::TransactionType::Deposit,
        TransactionType::Withdrawal => common_proto::TransactionType::Withdrawal,
        TransactionType::BetPlace => common_proto::TransactionType::BetPlace,
        TransactionType::BetWin => common_proto::TransactionType::BetWin,
        TransactionType::BetRefund => common_proto::TransactionType::BetRefund,
        TransactionType::BonusCredit => common_proto::TransactionType::BonusCredit,
        TransactionType::BonusDebit => common_proto::TransactionType::BonusDebit,
        TransactionType::Transfer => common_proto::TransactionType::Transfer,
        TransactionType::Adjustment | TransactionType::Fee => {
            common_proto::TransactionType::Adjustment
        }
    }
}

fn transaction_status_to_proto(s: &TransactionStatus) -> common_proto::TransactionStatus {
    match s {
        TransactionStatus::Pending => common_proto::TransactionStatus::Pending,
        TransactionStatus::Processing => common_proto::TransactionStatus::Processing,
        TransactionStatus::Completed => common_proto::TransactionStatus::Completed,
        TransactionStatus::Failed => common_proto::TransactionStatus::Failed,
        TransactionStatus::Cancelled => common_proto::TransactionStatus::Cancelled,
    }
}
