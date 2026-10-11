//! Wallet Service - Core business logic
//!
//! Standards: wallet-financial-ops.skill.md, data-consistency.skill.md
//!
//! CRITICAL RULES:
//! 1. Idempotency checked FIRST (before any business logic)
//! 2. Every operation creates debit+credit ledger entries
//! 3. All operations in DB transaction
//! 4. Events written to outbox (not published directly)
//! 5. Optimistic locking with retry

use rust_decimal::Decimal;
use std::sync::Arc;
use tracing::{info, instrument};
use uuid::Uuid;

use crate::domain::*;
use crate::infrastructure::repositories::*;
use crate::service::idempotency::IdempotencyService;

/// Request to move funds between two of a user's wallets.
#[derive(Debug, Clone)]
pub struct TransferRequest {
    pub user_id: Uuid,
    pub from_wallet_type: WalletType,
    pub to_wallet_type: WalletType,
    pub amount: Decimal,
    pub currency: String,
    pub reference_id: Uuid,
    pub idempotency_key: Option<String>,
}

/// Request to credit funds into a user's wallet.
#[derive(Debug, Clone)]
pub struct CreditRequest {
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub amount: Decimal,
    pub currency: String,
    pub reference_id: Uuid,
    pub reference_type: String,
    pub idempotency_key: String,
}

/// Request to debit funds from a user's wallet.
#[derive(Debug, Clone)]
pub struct DebitRequest {
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub amount: Decimal,
    pub currency: String,
    pub reference_id: Uuid,
    pub reference_type: String,
    pub idempotency_key: String,
}

/// Wallet service
pub struct WalletService {
    wallet_repo: Arc<WalletRepository>,
    transaction_repo: Arc<TransactionRepository>,
    ledger_repo: Arc<LedgerRepository>,
    /// Exposed so fund-lock lifecycle helpers can share this instance.
    pub(crate) lock_repo: Arc<LockRepository>,
    outbox_repo: Arc<OutboxRepository>,
    idempotency: Arc<IdempotencyService>,
}

impl WalletService {
    pub fn new(
        wallet_repo: Arc<WalletRepository>,
        transaction_repo: Arc<TransactionRepository>,
        ledger_repo: Arc<LedgerRepository>,
        lock_repo: Arc<LockRepository>,
        outbox_repo: Arc<OutboxRepository>,
        idempotency: Arc<IdempotencyService>,
    ) -> Self {
        Self {
            wallet_repo,
            transaction_repo,
            ledger_repo,
            lock_repo,
            outbox_repo,
            idempotency,
        }
    }

    /// Get or create wallet for user
    #[instrument(skip(self), fields(user_id = %user_id, wallet_type = ?wallet_type))]
    pub async fn get_or_create_wallet(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
        currency: &str,
    ) -> Result<Wallet, WalletError> {
        // Try to get existing wallet
        if let Some(wallet) = self
            .wallet_repo
            .get_by_user_and_type(user_id, wallet_type)
            .await?
        {
            return Ok(wallet);
        }

        // Create new wallet in transaction
        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let wallet = Wallet::new(user_id, wallet_type, currency.to_string());

        // Insert wallet
        sqlx::query(
            r#"
            INSERT INTO wallets (id, user_id, wallet_type, currency, version, is_active)
            VALUES ($1, $2, $3, $4, 0, true)
            "#,
        )
        .bind(wallet.id)
        .bind(wallet.user_id)
        .bind(wallet.wallet_type)
        .bind(&wallet.currency)
        .execute(&mut *tx)
        .await
        .map_err(|e| {
            if let Some(code) = e.as_database_error().and_then(|de| de.code()) {
                if code.as_ref() == "23505" {
                    // unique_violation
                    return WalletError::AlreadyExists {
                        user_id,
                        wallet_type,
                    };
                }
            }
            WalletError::DatabaseError(e.to_string())
        })?;

        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        info!("Created new wallet for user");

        Ok(wallet)
    }

    /// Get wallet balance
    #[instrument(skip(self), fields(user_id = %user_id, wallet_type = ?wallet_type))]
    pub async fn get_balance(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
    ) -> Result<Balance, WalletError> {
        let wallet = self
            .wallet_repo
            .get_by_user_and_type(user_id, wallet_type)
            .await?
            .ok_or(WalletError::NotFound { user_id })?;

        Ok(Balance::new(
            wallet.balance_available,
            wallet.balance_locked,
            wallet.balance_bonus,
        ))
    }

    /// Credit wallet (deposit, win, bonus)
    ///
    /// FLOW:
    /// 1. Check idempotency FIRST
    /// 2. Get or create wallet
    /// 3. Create transaction
    /// 4. Update wallet balance
    /// 5. Create ledger entries (debit + credit)
    /// 6. Write event to outbox
    /// 7. Commit transaction
    /// 8. Cache idempotency result
    #[instrument(skip(self, req), fields(user_id = %req.user_id, amount = %req.amount))]
    pub async fn credit(&self, req: CreditRequest) -> Result<Transaction, WalletError> {
        let CreditRequest {
            user_id,
            wallet_type,
            amount,
            currency,
            reference_id,
            reference_type,
            idempotency_key,
        } = req;
        let currency = currency.as_str();
        let reference_type = reference_type.as_str();
        let idempotency_key = idempotency_key.as_str();
        // =====================================================================
        // STEP 1: Check idempotency FIRST (before ANY business logic)
        // =====================================================================
        if let Some(txn_id) = self.idempotency.get(idempotency_key).await? {
            info!(txn_id = %txn_id, "Returning cached idempotent transaction");
            return self
                .transaction_repo
                .get_by_id(txn_id)
                .await
                .map_err(|e| WalletError::DatabaseError(e.to_string()))?
                .ok_or(WalletError::NotFound { user_id });
        }

        // =====================================================================
        // STEP 2-7: Database transaction
        // =====================================================================
        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        // Get or create wallet
        let wallet = self
            .get_or_create_wallet_internal(user_id, wallet_type, currency, &mut tx)
            .await?;

        // Validate amount
        if amount <= Decimal::ZERO {
            return Err(WalletError::InvalidAmount(
                "Amount must be positive".to_string(),
            ));
        }

        // Create transaction record
        let mut transaction = Transaction::new(NewTransaction {
            user_id,
            wallet_id: wallet.id,
            wallet_type,
            transaction_type: TransactionType::Deposit,
            amount,
            currency: currency.to_string(),
            reference_id: Some(reference_id),
            reference_type: Some(reference_type.to_string()),
            idempotency_key: Some(idempotency_key.to_string()),
        });

        // Mark transaction as completed for sync flow before saving
        transaction.complete();

        // Insert transaction
        self.insert_transaction_internal(&mut tx, &transaction)
            .await?;

        // Update wallet balance
        let new_available = wallet.balance_available + amount;
        self.update_wallet_balance_internal(
            &mut tx,
            wallet.id,
            new_available,
            wallet.balance_locked,
            wallet.balance_bonus,
            wallet.version,
        )
        .await?;

        // Create ledger entries (DOUBLE-ENTRY BOOKKEEPING)
        // Credit: user wallet (money enters)
        let credit_entry = LedgerEntry::credit_with_ref(LedgerEntryWithRef {
            transaction_id: transaction.id,
            account_type: AccountType::UserWallet,
            account_id: format!("user_wallet:{}:{:?}:{}", user_id, wallet_type, currency),
            amount,
            currency: currency.to_string(),
            balance_after: Some(new_available),
            reference_type: reference_type.to_string(),
            reference_id,
            idempotency_key: Some(idempotency_key.to_string()),
        });

        // Debit: payment gateway transit or house revenue (money leaves)
        let debit_account = match reference_type {
            "deposit" => AccountType::PaymentGatewayTransit,
            "bet_win" => AccountType::HouseRevenue,
            "bonus" => AccountType::BonusPool,
            _ => AccountType::HouseRevenue,
        };

        let debit_entry = LedgerEntry::debit_with_ref(LedgerEntryWithRef {
            transaction_id: transaction.id,
            account_type: debit_account,
            account_id: format!("{}:{}", debit_account.as_str(), reference_id),
            amount,
            currency: currency.to_string(),
            balance_after: None,
            reference_type: reference_type.to_string(),
            reference_id,
            idempotency_key: Some(idempotency_key.to_string()),
        });

        // Insert ledger entries
        self.ledger_repo
            .insert_pair_internal(&mut tx, debit_entry, credit_entry)
            .await?;

        // Write event to outbox (TRANSACTIONAL OUTBOX PATTERN)
        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "wallet_credited",
            "transaction_id": transaction.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_type": wallet_type.as_str(),
            "amount": amount.to_string(),
            "currency": currency,
            "reference_id": reference_id.to_string(),
            "reference_type": reference_type,
        }))
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );

        self.outbox_repo
            .insert_internal(&mut tx, &outbox_event)
            .await?;

        // Commit transaction
        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        // =====================================================================
        // STEP 8: Cache idempotency result (after successful commit)
        // =====================================================================
        self.idempotency
            .set(idempotency_key, transaction.id)
            .await?;

        info!("Wallet credited successfully");

        Ok(transaction)
    }

    /// Debit wallet (withdrawal, bet, fee)
    ///
    /// FLOW:
    /// 1. Check idempotency FIRST
    /// 2. Get wallet
    /// 3. Check balance (MUST be >= amount)
    /// 4. Create transaction
    /// 5. Update wallet balance
    /// 6. Create ledger entries
    /// 7. Write event to outbox
    /// 8. Commit
    /// 9. Cache idempotency
    #[instrument(skip(self, req), fields(user_id = %req.user_id, amount = %req.amount))]
    pub async fn debit(&self, req: DebitRequest) -> Result<Transaction, WalletError> {
        let DebitRequest {
            user_id,
            wallet_type,
            amount,
            currency,
            reference_id,
            reference_type,
            idempotency_key,
        } = req;
        let currency = currency.as_str();
        let reference_type = reference_type.as_str();
        let idempotency_key = idempotency_key.as_str();

        // STEP 1: Check idempotency FIRST
        if let Some(txn_id) = self.idempotency.get(idempotency_key).await? {
            info!(txn_id = %txn_id, "Returning cached idempotent transaction");
            return self
                .transaction_repo
                .get_by_id(txn_id)
                .await
                .map_err(|e| WalletError::DatabaseError(e.to_string()))?
                .ok_or(WalletError::NotFound { user_id });
        }

        // STEP 2-7: Database transaction
        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        // Get wallet
        let wallet = self
            .wallet_repo
            .get_by_user_and_type_internal(user_id, wallet_type, &mut tx)
            .await?
            .ok_or(WalletError::NotFound { user_id })?;

        // STEP 3: Check balance
        if !wallet.has_available_balance(amount) {
            return Err(WalletError::InsufficientAvailableBalance {
                required: amount,
                available: wallet.balance_available,
            });
        }

        // Create transaction
        let mut transaction = Transaction::new(NewTransaction {
            user_id,
            wallet_id: wallet.id,
            wallet_type,
            transaction_type: TransactionType::BetPlace,
            amount,
            currency: currency.to_string(),
            reference_id: Some(reference_id),
            reference_type: Some(reference_type.to_string()),
            idempotency_key: Some(idempotency_key.to_string()),
        });

        // Mark transaction as completed for sync flow before saving
        transaction.complete();

        self.insert_transaction_internal(&mut tx, &transaction)
            .await?;

        // Update wallet balance
        let new_available = wallet.balance_available - amount;
        self.update_wallet_balance_internal(
            &mut tx,
            wallet.id,
            new_available,
            wallet.balance_locked,
            wallet.balance_bonus,
            wallet.version,
        )
        .await?;

        // Create ledger entries
        // Debit: user wallet (money leaves)
        let debit_entry = LedgerEntry::debit_with_ref(LedgerEntryWithRef {
            transaction_id: transaction.id,
            account_type: AccountType::UserWallet,
            account_id: format!("user_wallet:{}:{:?}:{}", user_id, wallet_type, currency),
            amount,
            currency: currency.to_string(),
            balance_after: Some(new_available),
            reference_type: reference_type.to_string(),
            reference_id,
            idempotency_key: Some(idempotency_key.to_string()),
        });

        // Credit: house hold (for pending bets) or house revenue
        let credit_account = if reference_type == "bet" {
            AccountType::HouseHold
        } else {
            AccountType::HouseRevenue
        };

        let credit_entry = LedgerEntry::credit_with_ref(LedgerEntryWithRef {
            transaction_id: transaction.id,
            account_type: credit_account,
            account_id: format!("{}:{}", credit_account.as_str(), reference_id),
            amount,
            currency: currency.to_string(),
            balance_after: None,
            reference_type: reference_type.to_string(),
            reference_id,
            idempotency_key: Some(idempotency_key.to_string()),
        });

        self.ledger_repo
            .insert_pair_internal(&mut tx, debit_entry, credit_entry)
            .await?;

        // Write event to outbox
        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "wallet_debited",
            "transaction_id": transaction.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_type": wallet_type.as_str(),
            "amount": amount.to_string(),
            "currency": currency,
            "reference_id": reference_id.to_string(),
        }))
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );

        self.outbox_repo
            .insert_internal(&mut tx, &outbox_event)
            .await?;

        // Commit
        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        // Cache idempotency
        self.idempotency
            .set(idempotency_key, transaction.id)
            .await?;

        info!("Wallet debited successfully");

        Ok(transaction)
    }

    /// Lock funds for a pending operation.
    ///
    /// Idempotent on `reference_id`: replaying the same reference returns the
    /// existing lock instead of double-locking the balance. (An
    /// `idempotency_key` is therefore not needed here; the reference *is* the
    /// idempotency key.)
    pub async fn lock(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
        amount: Decimal,
        reference_id: Uuid,
    ) -> Result<FundLock, WalletError> {
        if let Some(existing) = self.lock_repo.get_active_by_reference(reference_id).await? {
            return Ok(existing);
        }

        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let wallet = self
            .wallet_repo
            .get_by_user_and_type_internal(user_id, wallet_type, &mut tx)
            .await?
            .ok_or(WalletError::NotFound { user_id })?;

        if !wallet.has_available_balance(amount) {
            return Err(WalletError::InsufficientAvailableBalance {
                required: amount,
                available: wallet.balance_available,
            });
        }

        let new_available = wallet.balance_available - amount;
        let new_locked = wallet.balance_locked + amount;

        self.update_wallet_balance_internal(
            &mut tx,
            wallet.id,
            new_available,
            new_locked,
            wallet.balance_bonus,
            wallet.version,
        )
        .await?;

        let lock = FundLock::new(wallet.id, user_id, amount, reference_id, "lock".to_string());

        sqlx::query(
            r#"INSERT INTO fund_locks (id, wallet_id, user_id, amount, reference_id, reference_type, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7)"#,
        )
        .bind(lock.id)
        .bind(lock.wallet_id)
        .bind(lock.user_id)
        .bind(lock.amount)
        .bind(lock.reference_id)
        .bind(&lock.reference_type)
        .bind(lock.is_active)
        .execute(&mut *tx)
        .await?;

        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        Ok(lock)
    }

    pub async fn unlock(&self, user_id: Uuid, reference_id: Uuid) -> Result<(), WalletError> {
        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let lock = sqlx::query_as::<_, FundLock>(
            r#"SELECT id, wallet_id, user_id, amount, reference_id, reference_type, is_active, created_at, released_at FROM fund_locks WHERE reference_id = $1 AND is_active = true FOR UPDATE"#,
        )
        .bind(reference_id)
        .fetch_optional(&mut *tx)
        .await?
        .ok_or(WalletError::LockReferenceNotFound(
            reference_id.to_string(),
        ))?;

        let wallet = sqlx::query_as::<_, Wallet>(
            r#"SELECT id, user_id, wallet_type, currency, balance_available, balance_locked, balance_bonus, version, is_active, created_at, updated_at FROM wallets WHERE id = $1 FOR UPDATE"#,
        )
        .bind(lock.wallet_id)
        .fetch_optional(&mut *tx)
        .await?
        .ok_or(WalletError::NotFound { user_id })?;

        let new_available = wallet.balance_available + lock.amount;
        let new_locked = wallet.balance_locked - lock.amount;

        self.update_wallet_balance_internal(
            &mut tx,
            wallet.id,
            new_available,
            new_locked,
            wallet.balance_bonus,
            wallet.version,
        )
        .await?;

        sqlx::query("UPDATE fund_locks SET is_active = false, released_at = NOW() WHERE id = $1")
            .bind(lock.id)
            .execute(&mut *tx)
            .await?;

        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        Ok(())
    }

    pub async fn transfer(
        &self,
        req: TransferRequest,
    ) -> Result<(Transaction, Transaction), WalletError> {
        let TransferRequest {
            user_id,
            from_wallet_type,
            to_wallet_type,
            amount,
            currency,
            reference_id,
            idempotency_key,
        } = req;
        let currency = currency.as_str();
        let mut tx = self
            .wallet_repo
            .pool
            .begin()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let from_wallet = self
            .wallet_repo
            .get_by_user_and_type_internal(user_id, from_wallet_type, &mut tx)
            .await?
            .ok_or(WalletError::NotFound { user_id })?;
        let to_wallet = self
            .get_or_create_wallet_internal(user_id, to_wallet_type, currency, &mut tx)
            .await?;

        if !from_wallet.has_available_balance(amount) {
            return Err(WalletError::InsufficientAvailableBalance {
                required: amount,
                available: from_wallet.balance_available,
            });
        }

        let new_from_available = from_wallet.balance_available - amount;
        self.update_wallet_balance_internal(
            &mut tx,
            from_wallet.id,
            new_from_available,
            from_wallet.balance_locked,
            from_wallet.balance_bonus,
            from_wallet.version,
        )
        .await?;

        let new_to_available = to_wallet.balance_available + amount;
        self.update_wallet_balance_internal(
            &mut tx,
            to_wallet.id,
            new_to_available,
            to_wallet.balance_locked,
            to_wallet.balance_bonus,
            to_wallet.version,
        )
        .await?;

        let mut debit_txn = Transaction::new(NewTransaction {
            user_id,
            wallet_id: from_wallet.id,
            wallet_type: from_wallet_type,
            transaction_type: TransactionType::TransferOut,
            amount,
            currency: currency.to_string(),
            reference_id: Some(reference_id),
            reference_type: Some("transfer".to_string()),
            idempotency_key: idempotency_key.clone(),
        });
        debit_txn.complete();
        self.insert_transaction_internal(&mut tx, &debit_txn)
            .await?;

        let mut credit_txn = Transaction::new(NewTransaction {
            user_id,
            wallet_id: to_wallet.id,
            wallet_type: to_wallet_type,
            transaction_type: TransactionType::TransferIn,
            amount,
            currency: currency.to_string(),
            reference_id: Some(reference_id),
            reference_type: Some("transfer".to_string()),
            idempotency_key,
        });
        credit_txn.complete();
        self.insert_transaction_internal(&mut tx, &credit_txn)
            .await?;

        tx.commit()
            .await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        Ok((debit_txn, credit_txn))
    }

    pub async fn get_transactions(
        &self,
        user_id: Uuid,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Transaction>, WalletError> {
        let transactions = sqlx::query_as::<_, Transaction>(
            r#"SELECT id, user_id, wallet_id, wallet_type, transaction_type, amount, currency, status, reference_id, reference_type, idempotency_key, description, metadata, created_at, updated_at, completed_at FROM transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"#,
        )
        .bind(user_id)
        .bind(limit)
        .bind(offset)
        .fetch_all(&self.wallet_repo.pool)
        .await?;
        Ok(transactions)
    }

    /// Get a single transaction by its ID.
    pub async fn get_transaction(&self, transaction_id: Uuid) -> Result<Transaction, WalletError> {
        self.transaction_repo
            .get_by_id(transaction_id)
            .await?
            .ok_or_else(|| WalletError::TransactionNotFound(transaction_id.to_string()))
    }

    // =====================================================================
    // INTERNAL HELPER METHODS
    // =====================================================================

    async fn get_or_create_wallet_internal(
        &self,
        user_id: Uuid,
        wallet_type: WalletType,
        currency: &str,
        tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    ) -> Result<Wallet, WalletError> {
        if let Some(wallet) = self
            .wallet_repo
            .get_by_user_and_type_internal(user_id, wallet_type, tx)
            .await?
        {
            return Ok(wallet);
        }

        let wallet = Wallet::new(user_id, wallet_type, currency.to_string());

        sqlx::query(
            r#"
            INSERT INTO wallets (id, user_id, wallet_type, currency, version, is_active)
            VALUES ($1, $2, $3, $4, 0, true)
            "#,
        )
        .bind(wallet.id)
        .bind(wallet.user_id)
        .bind(wallet.wallet_type)
        .bind(&wallet.currency)
        .execute(&mut **tx)
        .await
        .map_err(|e| {
            if let Some(code) = e.as_database_error().and_then(|de| de.code()) {
                if code.as_ref() == "23505" {
                    return WalletError::AlreadyExists {
                        user_id,
                        wallet_type,
                    };
                }
            }
            WalletError::DatabaseError(e.to_string())
        })?;

        Ok(wallet)
    }

    async fn insert_transaction_internal(
        &self,
        tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
        transaction: &Transaction,
    ) -> Result<(), WalletError> {
        sqlx::query(
            r#"
            INSERT INTO transactions (
                id, user_id, wallet_id, wallet_type,
                transaction_type, amount, currency, status,
                reference_id, reference_type, idempotency_key
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            "#,
        )
        .bind(transaction.id)
        .bind(transaction.user_id)
        .bind(transaction.wallet_id)
        .bind(transaction.wallet_type)
        .bind(transaction.transaction_type)
        .bind(transaction.amount)
        .bind(&transaction.currency)
        .bind(transaction.status)
        .bind(transaction.reference_id)
        .bind(&transaction.reference_type)
        .bind(&transaction.idempotency_key)
        .execute(&mut **tx)
        .await?;

        Ok(())
    }

    async fn update_wallet_balance_internal(
        &self,
        tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
        wallet_id: Uuid,
        available: Decimal,
        locked: Decimal,
        bonus: Decimal,
        version: i32,
    ) -> Result<(), WalletError> {
        let result = sqlx::query(
            r#"
            UPDATE wallets
            SET 
                balance_available = $3,
                balance_locked = $4,
                balance_bonus = $5,
                version = version + 1,
                updated_at = NOW()
            WHERE id = $1 AND version = $2
            RETURNING id
            "#,
        )
        .bind(wallet_id)
        .bind(version)
        .bind(available)
        .bind(locked)
        .bind(bonus)
        .fetch_optional(&mut **tx)
        .await?;

        if result.is_none() {
            return Err(WalletError::concurrency_conflict());
        }

        Ok(())
    }
}
