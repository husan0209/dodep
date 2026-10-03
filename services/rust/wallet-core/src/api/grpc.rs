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
        let outbox_repo = Arc::new(OutboxRepository::new(state.db_pool.clone()));
        let idempotency = Arc::new(IdempotencyService::new(
            state.redis_client.clone(),
            3600, // 1 hour TTL
        ));

        let wallet_service = Arc::new(WalletService::new(
            wallet_repo,
            transaction_repo,
            ledger_repo,
            outbox_repo,
            idempotency,
        ));

        Self { wallet_service }
    }

    pub fn into_service(self) -> WalletCoreServiceServer<Self> {
        WalletCoreServiceServer::new(self)
    }

    /// Serve gRPC server
    ///
    /// Returns the concrete `tonic::transport::Error` rather than
    /// `Box<dyn Error>`: the caller spawns this future, and a bare
    /// `Box<dyn Error>` is neither `Send` nor `Sync`, so it cannot cross the
    /// task boundary.
    pub async fn serve(
        addr: std::net::SocketAddr,
        state: Arc<AppState>,
    ) -> Result<(), tonic::transport::Error> {
        let service = Self::new(state);

        tonic::transport::Server::builder()
            .add_service(service.into_service())
            .serve(addr)
            .await
    }
}

#[tonic::async_trait]
impl WalletCoreService for WalletGrpcService {
    async fn get_balance(
        &self,
        request: Request<GetBalanceRequest>,
    ) -> Result<Response<GetBalanceResponse>, Status> {
        let req = request.into_inner();

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.wallet_type)
                .map_err(|_| Status::invalid_argument("Invalid wallet_type"))?,
        );

        match self.wallet_service.get_balance(user_id, wallet_type).await {
            Ok(balance) => {
                // GetBalanceResponse carries a single `balance` message
                // (available/locked/bonus/total), not three top-level amounts.
                let response = GetBalanceResponse {
                    balance: Some(common_proto::Balance {
                        available: Some(money_from_decimal(balance.available, "USD")),
                        locked: Some(money_from_decimal(balance.locked, "USD")),
                        bonus: Some(money_from_decimal(balance.bonus, "USD")),
                        total: Some(money_from_decimal(balance.total(), "USD")),
                    }),
                    version: 0,
                    updated_at: None,
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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.wallet_type)
                .map_err(|_| Status::invalid_argument("Invalid wallet_type"))?,
        );

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;

        // A missing idempotency key must be rejected, not defaulted to "":
        // every keyless credit would then share one cache entry and the second
        // one would be answered with the first one's transaction.
        let idempotency_key = if req.idempotency_key.is_empty() {
            return Err(Status::invalid_argument("idempotency_key is required"));
        } else {
            req.idempotency_key.as_str()
        };

        match self
            .wallet_service
            .credit(
                user_id,
                wallet_type,
                amount,
                "USD",
                reference_id,
                req.reference_type.as_str(),
                idempotency_key,
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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.wallet_type)
                .map_err(|_| Status::invalid_argument("Invalid wallet_type"))?,
        );

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;

        // See the note in credit(): an absent idempotency key is rejected rather
        // than defaulted, otherwise all keyless debits collide on one entry.
        let idempotency_key = if req.idempotency_key.is_empty() {
            return Err(Status::invalid_argument("idempotency_key is required"));
        } else {
            req.idempotency_key.as_str()
        };

        match self
            .wallet_service
            .debit(
                user_id,
                wallet_type,
                amount,
                "USD",
                reference_id,
                req.reference_type.as_str(),
                idempotency_key,
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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.wallet_type)
                .map_err(|_| Status::invalid_argument("Invalid wallet_type"))?,
        );

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;

        match self
            .wallet_service
            .lock(
                user_id,
                wallet_type,
                amount,
                reference_id,
                Some(req.idempotency_key),
            )
            .await
        {
            Ok(_lock) => {
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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

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
            Err(WalletError::LockReferenceNotFound(_)) => Err(Status::not_found("Lock not found")),
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }

    async fn transfer(
        &self,
        request: Request<TransferRequest>,
    ) -> Result<Response<TransferResponse>, Status> {
        let req = request.into_inner();

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let from_wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.from_wallet)
                .map_err(|_| Status::invalid_argument("Invalid from_wallet"))?,
        );
        let to_wallet_type = parse_wallet_type(
            common_proto::WalletType::try_from(req.to_wallet)
                .map_err(|_| Status::invalid_argument("Invalid to_wallet"))?,
        );

        let amount =
            parse_money(&req.amount).map_err(|_| Status::invalid_argument("Invalid amount"))?;

        let reference_id = Uuid::parse_str(&req.reference_id)
            .map_err(|_| Status::invalid_argument("Invalid reference_id"))?;

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

        let user_id =
            parse_uuid(&req.user_id).map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let (limit, offset) = match req.pagination {
            Some(p) => {
                let l = p.page_size.clamp(1, 100) as i64;
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

        let has_more = transactions.len() as i64 == limit;
        let response = GetTransactionsResponse {
            transactions: transactions.iter().map(transaction_to_proto).collect(),
            pagination: Some(common_proto::PageResponse {
                next_cursor: (offset + limit).to_string(),
                prev_cursor: offset.saturating_sub(limit).max(0).to_string(),
                has_more,
                // total_count is `optional int64` in the contract; leaving it
                // unset is cheaper than counting the whole table per request.
                total_count: None,
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

fn parse_uuid(proto: &Option<UserId>) -> Result<Uuid, String> {
    proto
        .as_ref()
        .and_then(|id| Uuid::parse_str(&id.value).ok())
        .ok_or_else(|| "Invalid UUID".to_string())
}

/// Maps a proto wallet type onto the domain one.
///
/// Infallible by construction: the proto enum's variants are covered
/// exhaustively, and an unspecified type means "the main wallet".
fn parse_wallet_type(proto: common_proto::WalletType) -> WalletType {
    match proto {
        common_proto::WalletType::Unspecified | common_proto::WalletType::Main => WalletType::Main,
        common_proto::WalletType::Bonus => WalletType::Bonus,
        common_proto::WalletType::FreeSpins => WalletType::FreeSpins,
        common_proto::WalletType::Cashback => WalletType::Cashback,
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
        wallet_type: wallet_type_to_proto(txn.wallet_type),
        amount: Some(money_from_decimal(txn.amount, &txn.currency)),
        r#type: transaction_type_to_proto(txn.transaction_type) as i32,
        reference_id: txn
            .reference_id
            .map(|id| id.to_string())
            .unwrap_or_default(),
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
        TransactionType::Adjustment => common_proto::TransactionType::Adjustment,
        // The wire contract has no fee type, so a fee is reported as
        // unspecified rather than being silently folded into another type.
        TransactionType::Fee => common_proto::TransactionType::Unspecified,
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
