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

use std::sync::Arc;
use rust_decimal::Decimal;
use uuid::Uuid;
use tracing::{info, warn, instrument};

use crate::domain::*;
use crate::infrastructure::repositories::*;
use crate::service::idempotency::IdempotencyService;

/// Counter-account that receives the money leaving the player wallet when a
/// reservation is settled. Keeps double-entry bookkeeping symmetric with the
/// credit path (deposit -> gateway transit, bonus -> bonus pool, ...).
fn counter_account_for(reference_type: &str) -> AccountType {
    match reference_type {
        "withdrawal" => AccountType::PaymentGatewayTransit,
        "bonus" | "bonus_credit" => AccountType::BonusPool,
        "bet" | "bet_place" => AccountType::HouseHold,
        _ => AccountType::HouseRevenue,
    }
}

/// Wallet service
pub struct WalletService {
    wallet_repo: Arc<WalletRepository>,
    transaction_repo: Arc<TransactionRepository>,
    ledger_repo: Arc<LedgerRepository>,
    lock_repo: Arc<LockRepository>,
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
        user_id: i64,
        wallet_type: WalletType,
        currency: &str,
    ) -> Result<Wallet, WalletError> {
        // Try to get existing wallet
        if let Some(wallet) = self.wallet_repo.get_by_user_and_type(user_id, wallet_type).await? {
            return Ok(wallet);
        }
        
        // Create new wallet in transaction
        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        let wallet = Wallet::new(user_id, wallet_type, currency.to_string());
        
        // Insert wallet
        sqlx::query!(
            r#"
            INSERT INTO wallets (id, user_id, wallet_type, currency, version, is_active)
            VALUES ($1, $2, $3, $4, 0, true)
            "#,
            wallet.id,
            wallet.user_id,
            wallet.wallet_type as _,
            wallet.currency,
        )
        .execute(&mut *tx)
        .await
        .map_err(|e| {
            if let Some(code) = e.as_database_error().and_then(|de| de.code()) {
                if code.as_ref() == "23505" {  // unique_violation
                    return WalletError::AlreadyExists {
                        user_id,
                        wallet_type,
                    };
                }
            }
            WalletError::DatabaseError(e.to_string())
        })?;
        
        tx.commit().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        info!("Created new wallet for user");
        
        Ok(wallet)
    }
    
    /// Get wallet balance
    #[instrument(skip(self), fields(user_id = %user_id, wallet_type = ?wallet_type))]
    pub async fn get_balance(&self, user_id: i64, wallet_type: WalletType) -> Result<Balance, WalletError> {
        let wallet = self.wallet_repo
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
    #[instrument(skip(self), fields(user_id = %user_id, amount = %amount))]
    pub async fn credit(
        &self,
        user_id: i64,
        wallet_type: WalletType,
        amount: Decimal,
        currency: &str,
        reference_id: String,
        reference_type: &str,
        idempotency_key: &str,
    ) -> Result<Transaction, WalletError> {
        // =====================================================================
        // STEP 1: Check idempotency FIRST (before ANY business logic)
        // =====================================================================
        if let Some(txn_id) = self.idempotency.get(idempotency_key).await? {
            info!(txn_id = %txn_id, "Returning cached idempotent transaction");
            return self.transaction_repo.get_by_id(txn_id).await
                .map_err(|e| WalletError::DatabaseError(e.to_string()))?
                .ok_or(WalletError::NotFound { user_id });
        }
        
        // =====================================================================
        // STEP 2-7: Database transaction
        // =====================================================================
        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        // Get or create wallet
        let wallet = self.get_or_create_wallet_internal(user_id, wallet_type, currency, &mut tx).await?;
        
        // Validate amount
        if amount <= Decimal::ZERO {
            return Err(WalletError::InvalidAmount("Amount must be positive".to_string()));
        }
        
        // Create transaction record
        let mut transaction = Transaction::new(
            user_id,
            wallet.id,
            wallet_type,
            TransactionType::Deposit,
            amount,
            currency.to_string(),
            Some(reference_id.clone()),
            Some(reference_type.to_string()),
            Some(idempotency_key.to_string()),
        );
        
        // Mark transaction as completed for sync flow before saving
        transaction.complete();
        
        // Insert transaction
        self.insert_transaction_internal(&mut tx, &transaction).await?;
        
        // Update wallet balance
        let new_available = wallet.balance_available + amount;
        self.update_wallet_balance_internal(&mut tx, wallet.id, new_available, wallet.balance_locked, wallet.balance_bonus, wallet.version).await?;
        
        // Create ledger entries (DOUBLE-ENTRY BOOKKEEPING)
        // Credit: user wallet (money enters)
        let credit_entry = LedgerEntry::credit_with_ref(
            transaction.id,
            AccountType::UserWallet,
            format!("user_wallet:{}:{:?}:{}", user_id, wallet_type, currency),
            amount,
            currency.to_string(),
            Some(new_available),
            reference_type.to_string(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:credit")),
        );
        
        // Debit: payment gateway transit or house revenue (money leaves)
        let debit_account = match reference_type {
            "deposit" => AccountType::PaymentGatewayTransit,
            "bet_win" => AccountType::HouseRevenue,
            "bonus" => AccountType::BonusPool,
            _ => AccountType::HouseRevenue,
        };
        
        let debit_entry = LedgerEntry::debit_with_ref(
            transaction.id,
            debit_account,
            format!("{}:{}", debit_account.as_str(), reference_id),
            amount,
            currency.to_string(),
            None,
            reference_type.to_string(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:debit")),
        );
        
        // Insert ledger entries
        self.ledger_repo.insert_pair_internal(&mut tx, debit_entry, credit_entry).await?;
        
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
        })).map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );
        
        self.outbox_repo.insert_internal(&mut tx, &outbox_event).await?;
        
        // Commit transaction
        tx.commit().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        // =====================================================================
        // STEP 8: Cache idempotency result (after successful commit)
        // =====================================================================
        self.idempotency.set(idempotency_key, transaction.id).await?;
        
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
    #[instrument(skip(self), fields(user_id = %user_id, amount = %amount))]
    pub async fn debit(
        &self,
        user_id: i64,
        wallet_type: WalletType,
        amount: Decimal,
        currency: &str,
        reference_id: String,
        reference_type: &str,
        idempotency_key: &str,
    ) -> Result<Transaction, WalletError> {
        // STEP 1: Check idempotency FIRST
        if let Some(txn_id) = self.idempotency.get(idempotency_key).await? {
            info!(txn_id = %txn_id, "Returning cached idempotent transaction");
            return self.transaction_repo.get_by_id(txn_id).await
                .map_err(|e| WalletError::DatabaseError(e.to_string()))?
                .ok_or(WalletError::NotFound { user_id });
        }
        
        // STEP 2-7: Database transaction
        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        // Get wallet
        let wallet = self.wallet_repo
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
        let mut transaction = Transaction::new(
            user_id,
            wallet.id,
            wallet_type,
            TransactionType::BetPlace,
            amount,
            currency.to_string(),
            Some(reference_id.clone()),
            Some(reference_type.to_string()),
            Some(idempotency_key.to_string()),
        );
        
        // Mark transaction as completed for sync flow before saving
        transaction.complete();
        
        self.insert_transaction_internal(&mut tx, &transaction).await?;
        
        // Update wallet balance
        let new_available = wallet.balance_available - amount;
        self.update_wallet_balance_internal(&mut tx, wallet.id, new_available, wallet.balance_locked, wallet.balance_bonus, wallet.version).await?;
        
        // Create ledger entries
        // Debit: user wallet (money leaves)
        let debit_entry = LedgerEntry::debit_with_ref(
            transaction.id,
            AccountType::UserWallet,
            format!("user_wallet:{}:{:?}:{}", user_id, wallet_type, currency),
            amount,
            currency.to_string(),
            Some(new_available),
            reference_type.to_string(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:debit")),
        );
        
        // Credit: house hold (for pending bets) or house revenue
        let credit_account = if reference_type == "bet" {
            AccountType::HouseHold
        } else {
            AccountType::HouseRevenue
        };
        
        let credit_entry = LedgerEntry::credit_with_ref(
            transaction.id,
            credit_account,
            format!("{}:{}", credit_account.as_str(), reference_id),
            amount,
            currency.to_string(),
            None,
            reference_type.to_string(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:credit")),
        );
        
        self.ledger_repo.insert_pair_internal(&mut tx, debit_entry, credit_entry).await?;
        
        // Write event to outbox
        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "wallet_debited",
            "transaction_id": transaction.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_type": wallet_type.as_str(),
            "amount": amount.to_string(),
            "currency": currency,
            "reference_id": reference_id.to_string(),
        })).map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );
        
        self.outbox_repo.insert_internal(&mut tx, &outbox_event).await?;
        
        // Commit
        tx.commit().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        // Cache idempotency
        self.idempotency.set(idempotency_key, transaction.id).await?;
        
        info!("Wallet debited successfully");
        
        Ok(transaction)
    }
    
    /// Reserve funds: available -> locked.
    ///
    /// Idempotent per (user_id, reference_type, reference_id): if such a lock
    /// is already active the existing reservation is returned and no balance
    /// is touched, so a retried request (client timeout, duplicate webhook)
    /// cannot reserve the money twice.
    #[instrument(skip(self), fields(user_id = %user_id, reference_id = %reference_id))]
    pub async fn lock(
        &self,
        user_id: i64,
        wallet_type: WalletType,
        amount: Decimal,
        reference_id: String,
        reference_type: String,
    ) -> Result<FundLock, WalletError> {
        if amount <= Decimal::ZERO {
            return Err(WalletError::InvalidAmount("Lock amount must be positive".to_string()));
        }
        if reference_id.trim().is_empty() {
            return Err(WalletError::InvalidArgument("reference_id is required".to_string()));
        }
        if reference_type.trim().is_empty() {
            return Err(WalletError::InvalidArgument("reference_type is required".to_string()));
        }

        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let wallet = self.get_or_create_wallet_internal(user_id, wallet_type, "USD", &mut tx).await?;

        // Idempotency: reuse the active lock for this business reference.
        if let Some(existing) = self.lock_repo
            .get_active_by_reference_internal(&mut tx, user_id, &reference_type, &reference_id)
            .await?
        {
            if existing.amount != amount {
                return Err(WalletError::BusinessRuleViolation(format!(
                    "active lock for {reference_type}:{reference_id} holds {} but {amount} was requested",
                    existing.amount
                )));
            }
            tx.commit().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
            info!(lock_id = %existing.id, "Reusing active fund lock");
            return Ok(existing);
        }

        if !wallet.has_available_balance(amount) {
            return Err(WalletError::InsufficientAvailableBalance {
                required: amount,
                available: wallet.balance_available,
            });
        }

        let new_available = wallet.balance_available - amount;
        let new_locked = wallet.balance_locked + amount;

        self.update_wallet_balance_internal(&mut tx, wallet.id, new_available, new_locked, wallet.balance_bonus, wallet.version).await?;

        let lock = FundLock::new(
            wallet.id,
            user_id,
            amount,
            reference_id.clone(),
            reference_type.clone(),
        );
        self.lock_repo.insert_internal(&mut tx, &lock).await?;

        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "fund_locked",
            "lock_id": lock.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_id": wallet.id.to_string(),
            "amount": amount.to_string(),
            "currency": "USD",
            "reference_id": reference_id,
            "reference_type": reference_type,
        })).map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );
        self.outbox_repo.insert_internal(&mut tx, &outbox_event).await?;

        tx.commit().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        info!(lock_id = %lock.id, "Funds locked");
        Ok(lock)
    }

    /// Release a reservation: locked -> available.
    ///
    /// Idempotent: an already released or consumed lock is reported as
    /// `Ok(false)` instead of failing, so compensating retries are safe.
    #[instrument(skip(self), fields(user_id = %user_id, reference_id = %reference_id))]
    pub async fn unlock(
        &self,
        user_id: i64,
        reference_type: String,
        reference_id: String,
    ) -> Result<bool, WalletError> {
        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let lock = self.lock_repo
            .get_active_by_reference_internal(&mut tx, user_id, &reference_type, &reference_id)
            .await?
            .ok_or_else(|| WalletError::LockReferenceNotFound(format!("{reference_type}:{reference_id}")))?;

        let wallet = self.lock_holder_wallet_internal(&mut tx, &lock, user_id).await?;

        if wallet.balance_locked < lock.amount {
            return Err(WalletError::LockedBalanceUnderflow {
                locked: lock.amount,
                available: wallet.balance_locked,
            });
        }

        let new_available = wallet.balance_available + lock.amount;
        let new_locked = wallet.balance_locked - lock.amount;
        self.update_wallet_balance_internal(&mut tx, wallet.id, new_available, new_locked, wallet.balance_bonus, wallet.version).await?;

        let released = self.lock_repo.release_internal(&mut tx, lock.id).await?;
        if !released {
            // Lost the race against a concurrent settlement: no balance change.
            return Err(WalletError::LockAlreadySettled(format!("{reference_type}:{reference_id}")));
        }

        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "fund_unlocked",
            "lock_id": lock.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_id": wallet.id.to_string(),
            "amount": lock.amount.to_string(),
            "currency": wallet.currency,
            "reference_id": reference_id,
            "reference_type": reference_type,
        })).map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );
        self.outbox_repo.insert_internal(&mut tx, &outbox_event).await?;

        tx.commit().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        info!(lock_id = %lock.id, "Funds unlocked");
        Ok(true)
    }

    /// Settle a reservation: the locked amount left the platform and is
    /// booked as a debit transaction (crypto payout confirmed).
    ///
    /// This is NOT `unlock + debit`: the amount was already taken out of
    /// `available` when the lock was created, so settling only clears the
    /// `locked` bucket and writes the double-entry pair. Doing a second debit
    /// would charge the player twice.
    ///
    /// Idempotent per idempotency_key and per (user, reference_type,
    /// reference_id): replays return the transaction created by the first
    /// call without touching balances.
    #[instrument(skip(self), fields(user_id = %user_id, reference_id = %reference_id))]
    pub async fn consume_lock(
        &self,
        user_id: i64,
        reference_type: String,
        reference_id: String,
        idempotency_key: String,
        description: Option<String>,
    ) -> Result<Transaction, WalletError> {
        if idempotency_key.trim().is_empty() {
            return Err(WalletError::InvalidArgument("idempotency_key is required".to_string()));
        }

        // Idempotency: a replayed settlement returns the original transaction.
        if let Some(txn_id) = self.idempotency.get(&idempotency_key).await? {
            info!(txn_id = %txn_id, "Lock already settled (cached idempotency)");
            return self.transaction_repo.get_by_id(txn_id).await
                .map_err(|e| WalletError::DatabaseError(e.to_string()))?
                .ok_or(WalletError::NotFound { user_id });
        }

        let mut tx = self.wallet_repo.pool.begin().await
            .map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        let lock = self.lock_repo
            .get_active_by_reference_internal(&mut tx, user_id, &reference_type, &reference_id)
            .await?
            .ok_or_else(|| WalletError::LockReferenceNotFound(format!("{reference_type}:{reference_id}")))?;

        let wallet = self.lock_holder_wallet_internal(&mut tx, &lock, user_id).await?;

        if wallet.balance_locked < lock.amount {
            return Err(WalletError::LockedBalanceUnderflow {
                locked: lock.amount,
                available: wallet.balance_locked,
            });
        }

        // Clear the locked bucket; `available` was reduced at lock time.
        let new_locked = wallet.balance_locked - lock.amount;
        self.update_wallet_balance_internal(
            &mut tx,
            wallet.id,
            wallet.balance_available,
            new_locked,
            wallet.balance_bonus,
            wallet.version,
        ).await?;

        let mut transaction = Transaction::new(
            user_id,
            wallet.id,
            wallet.wallet_type,
            TransactionType::Withdrawal,
            lock.amount,
            wallet.currency.clone(),
            Some(reference_id.clone()),
            Some(reference_type.clone()),
            Some(idempotency_key.clone()),
        );
        transaction.description = description;
        transaction.complete();
        self.insert_transaction_internal(&mut tx, &transaction).await?;

        // Double entry: the reserved funds leave the player wallet, arriving
        // in the counter-account selected by the reservation type.
        let counter_account = counter_account_for(&reference_type);
        let debit_entry = LedgerEntry::debit_with_ref(
            transaction.id,
            AccountType::UserWallet,
            wallet.ledger_account_id(),
            lock.amount,
            wallet.currency.clone(),
            Some(wallet.balance_available),
            reference_type.clone(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:debit")),
        );
        let credit_entry = LedgerEntry::credit_with_ref(
            transaction.id,
            counter_account,
            format!("{}:{}", counter_account.as_str(), reference_id),
            lock.amount,
            wallet.currency.clone(),
            None,
            reference_type.clone(),
            reference_id.clone(),
            Some(format!("{idempotency_key}:credit")),
        );
        self.ledger_repo.insert_pair_internal(&mut tx, debit_entry, credit_entry).await?;

        let settled = self.lock_repo.consume_internal(&mut tx, lock.id, transaction.id).await?;
        if !settled {
            return Err(WalletError::LockAlreadySettled(format!("{reference_type}:{reference_id}")));
        }

        let event_payload = serde_json::to_vec(&serde_json::json!({
            "event": "fund_lock_consumed",
            "lock_id": lock.id.to_string(),
            "transaction_id": transaction.id.to_string(),
            "user_id": user_id.to_string(),
            "wallet_id": wallet.id.to_string(),
            "amount": lock.amount.to_string(),
            "currency": wallet.currency,
            "reference_id": reference_id,
            "reference_type": reference_type,
        })).map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        let outbox_event = OutboxEvent::new(
            "wallet.events".to_string(),
            user_id.to_string(),
            event_payload,
        );
        self.outbox_repo.insert_internal(&mut tx, &outbox_event).await?;

        tx.commit().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;

        self.idempotency.set(&idempotency_key, transaction.id).await?;
        info!(lock_id = %lock.id, transaction_id = %transaction.id, "Fund lock settled");
        Ok(transaction)
    }

    /// Load the wallet holding `lock` and verify it belongs to `user_id`.
    async fn lock_holder_wallet_internal(
        &self,
        tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
        lock: &FundLock,
        user_id: i64,
    ) -> Result<Wallet, WalletError> {
        // Manual mapping: the `wallet_type` enum is read as text and parsed
        // back, matching WalletRepository's approach.
        let row = sqlx::query!(
            r#"SELECT id, user_id, wallet_type::text AS wallet_type, currency,
                      balance_available, balance_locked, balance_bonus,
                      version, is_active, created_at, updated_at
               FROM wallets WHERE id = $1 FOR UPDATE"#,
            lock.wallet_id
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?
        .ok_or(WalletError::NotFound { user_id })?;

        if row.user_id != user_id {
            return Err(WalletError::LockOwnershipMismatch {
                reference: lock.reference_id.clone(),
                user_id,
            });
        }

        Ok(Wallet {
            id: row.id,
            user_id: row.user_id,
            wallet_type: row
                .wallet_type
                .as_deref()
                .and_then(|s| s.parse().ok())
                .unwrap_or(WalletType::Main),
            currency: row.currency,
            balance_available: row.balance_available,
            balance_locked: row.balance_locked,
            balance_bonus: row.balance_bonus,
            version: row.version,
            is_active: row.is_active,
            created_at: row.created_at,
            updated_at: row.updated_at,
        })
    }
    
    pub async fn transfer(
        &self,
        user_id: i64,
        from_wallet_type: WalletType,
        to_wallet_type: WalletType,
        amount: Decimal,
        currency: &str,
        reference_id: String,
        idempotency_key: Option<String>,
    ) -> Result<(Transaction, Transaction), WalletError> {
        let mut tx = self.wallet_repo.pool.begin().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        let from_wallet = self.wallet_repo.get_by_user_and_type_internal(user_id, from_wallet_type, &mut tx).await?.ok_or(WalletError::NotFound { user_id })?;
        let to_wallet = self.get_or_create_wallet_internal(user_id, to_wallet_type, currency, &mut tx).await?;
        
        if !from_wallet.has_available_balance(amount) {
            return Err(WalletError::InsufficientAvailableBalance { required: amount, available: from_wallet.balance_available });
        }
        
        let new_from_available = from_wallet.balance_available - amount;
        self.update_wallet_balance_internal(&mut tx, from_wallet.id, new_from_available, from_wallet.balance_locked, from_wallet.balance_bonus, from_wallet.version).await?;
        
        let new_to_available = to_wallet.balance_available + amount;
        self.update_wallet_balance_internal(&mut tx, to_wallet.id, new_to_available, to_wallet.balance_locked, to_wallet.balance_bonus, to_wallet.version).await?;
        
        let mut debit_txn = Transaction::new(user_id, from_wallet.id, from_wallet_type, TransactionType::Transfer, amount, currency.to_string(), Some(reference_id.clone()), Some("transfer".to_string()), idempotency_key.clone());
        debit_txn.complete();
        self.insert_transaction_internal(&mut tx, &debit_txn).await?;
        
        let mut credit_txn = Transaction::new(user_id, to_wallet.id, to_wallet_type, TransactionType::Transfer, amount, currency.to_string(), Some(reference_id), Some("transfer".to_string()), idempotency_key);
        credit_txn.complete();
        self.insert_transaction_internal(&mut tx, &credit_txn).await?;
        
        tx.commit().await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        Ok((debit_txn, credit_txn))
    }
    
    pub async fn get_transactions(&self, user_id: i64, limit: i64, offset: i64) -> Result<Vec<Transaction>, WalletError> {
        let transactions = sqlx::query_as!(
            Transaction,
            r#"SELECT id, user_id, wallet_id, wallet_type as "wallet_type: WalletType", transaction_type as "transaction_type: TransactionType", amount as "amount: Decimal", currency, status as "status: TransactionStatus", reference_id, reference_type, idempotency_key, description, metadata as "metadata: serde_json::Value", created_at, updated_at, completed_at FROM transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"#,
            user_id, limit, offset
        ).fetch_all(&self.wallet_repo.pool).await.map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        Ok(transactions)
    }

    pub async fn get_transaction(&self, id: Uuid) -> Result<Transaction, WalletError> {
        let transaction = sqlx::query_as!(
            Transaction,
            r#"SELECT id, user_id, wallet_id, wallet_type as "wallet_type: WalletType", transaction_type as "transaction_type: TransactionType", amount as "amount: Decimal", currency, status as "status: TransactionStatus", reference_id, reference_type, idempotency_key, description, metadata as "metadata: serde_json::Value", created_at, updated_at, completed_at FROM transactions WHERE id = $1"#,
            id
        ).fetch_optional(&self.wallet_repo.pool).await.map_err(|e| WalletError::DatabaseError(e.to_string()))?
        .ok_or_else(|| WalletError::TransactionNotFound(id.to_string()))?;
        Ok(transaction)
    }

    // =====================================================================
    // INTERNAL HELPER METHODS
    // =====================================================================
    
    async fn get_or_create_wallet_internal(
        &self,
        user_id: i64,
        wallet_type: WalletType,
        currency: &str,
        tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    ) -> Result<Wallet, WalletError> {
        if let Some(wallet) = self.wallet_repo
            .get_by_user_and_type_internal(user_id, wallet_type, tx)
            .await?
        {
            return Ok(wallet);
        }
        
        let wallet = Wallet::new(user_id, wallet_type, currency.to_string());
        
        sqlx::query!(
            r#"
            INSERT INTO wallets (id, user_id, wallet_type, currency, version, is_active)
            VALUES ($1, $2, $3, $4, 0, true)
            "#,
            wallet.id,
            wallet.user_id,
            wallet.wallet_type as _,
            wallet.currency,
        )
        .execute(&mut **tx)
        .await
        .map_err(|e| {
            if let Some(code) = e.as_database_error().and_then(|de| de.code()) {
                if code.as_ref() == "23505" {
                    return WalletError::AlreadyExists { user_id, wallet_type };
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
        sqlx::query!(
            r#"
            INSERT INTO transactions (
                id, user_id, wallet_id, wallet_type,
                transaction_type, amount, currency, status,
                reference_id, reference_type, idempotency_key
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            "#,
            transaction.id,
            transaction.user_id,
            transaction.wallet_id,
            transaction.wallet_type as _,
            transaction.transaction_type as _,
            transaction.amount as _,
            &transaction.currency,
            transaction.status as _,
            transaction.reference_id,
            transaction.reference_type,
            transaction.idempotency_key,
        )
        .execute(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
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
        let result = sqlx::query!(
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
            wallet_id,
            version,
            available as _,
            locked as _,
            bonus as _,
        )
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| WalletError::DatabaseError(e.to_string()))?;
        
        if result.is_none() {
            return Err(WalletError::ConcurrencyConflict);
        }
        
        Ok(())
    }
}
