//! Domain enums

use serde::{Deserialize, Serialize};
use sqlx::Type;

/// Wallet type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Type)]
#[sqlx(type_name = "wallet_type", rename_all = "snake_case")]
pub enum WalletType {
    /// Main wallet (real money)
    Main,
    /// Bonus wallet (bonus funds)
    Bonus,
    /// Free spins wallet
    FreeSpins,
    /// Cashback wallet
    Cashback,
}

impl WalletType {
    pub fn as_str(&self) -> &'static str {
        match self {
            WalletType::Main => "main",
            WalletType::Bonus => "bonus",
            WalletType::FreeSpins => "free_spins",
            WalletType::Cashback => "cashback",
        }
    }

    /// Parse from the snake_case wire/DB representation.
    pub fn parse(s: &str) -> Option<Self> {
        match s.to_lowercase().as_str() {
            "main" => Some(WalletType::Main),
            "bonus" => Some(WalletType::Bonus),
            "free_spins" => Some(WalletType::FreeSpins),
            "cashback" => Some(WalletType::Cashback),
            _ => None,
        }
    }
}

impl std::str::FromStr for WalletType {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        Self::parse(s).ok_or_else(|| format!("unknown wallet type: {s}"))
    }
}

/// Transaction type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Type)]
#[sqlx(type_name = "transaction_type", rename_all = "snake_case")]
pub enum TransactionType {
    Deposit,
    Withdrawal,
    BetPlace,
    BetWin,
    BetRefund,
    BonusCredit,
    BonusDebit,
    Transfer,
    TransferIn,
    TransferOut,
    Adjustment,
    Fee,
}

impl TransactionType {
    pub fn as_str(&self) -> &'static str {
        match self {
            TransactionType::Deposit => "deposit",
            TransactionType::Withdrawal => "withdrawal",
            TransactionType::BetPlace => "bet_place",
            TransactionType::BetWin => "bet_win",
            TransactionType::BetRefund => "bet_refund",
            TransactionType::BonusCredit => "bonus_credit",
            TransactionType::BonusDebit => "bonus_debit",
            TransactionType::Transfer => "transfer",
            TransactionType::TransferIn => "transfer_in",
            TransactionType::TransferOut => "transfer_out",
            TransactionType::Adjustment => "adjustment",
            TransactionType::Fee => "fee",
        }
    }
}

/// Transaction status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Type)]
#[sqlx(type_name = "transaction_status", rename_all = "snake_case")]
pub enum TransactionStatus {
    Pending,
    Processing,
    Completed,
    Failed,
    Cancelled,
}

impl TransactionStatus {
    /// `Processing` covers the window between acceptance and completion.
    pub fn is_terminal(&self) -> bool {
        matches!(
            self,
            TransactionStatus::Completed | TransactionStatus::Failed | TransactionStatus::Cancelled
        )
    }

    pub fn as_str(&self) -> &'static str {
        match self {
            TransactionStatus::Pending => "pending",
            TransactionStatus::Processing => "processing",
            TransactionStatus::Completed => "completed",
            TransactionStatus::Failed => "failed",
            TransactionStatus::Cancelled => "cancelled",
        }
    }
}

/// Ledger entry type (for double-entry bookkeeping)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Type)]
#[sqlx(type_name = "ledger_entry_type", rename_all = "snake_case")]
pub enum LedgerEntryType {
    /// Money leaves account
    Debit,
    /// Money enters account
    Credit,
}

impl LedgerEntryType {
    pub fn as_str(&self) -> &'static str {
        match self {
            LedgerEntryType::Debit => "debit",
            LedgerEntryType::Credit => "credit",
        }
    }
}

/// Account type for ledger entries
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Type)]
#[sqlx(type_name = "account_type", rename_all = "snake_case")]
pub enum AccountType {
    /// User wallet account
    UserWallet,
    /// House revenue account
    HouseRevenue,
    /// House hold account (for pending bets)
    HouseHold,
    /// Payment gateway transit (in-flight funds)
    PaymentGatewayTransit,
    /// Tax reserve
    TaxReserve,
    /// Bonus pool
    BonusPool,
}

impl AccountType {
    pub fn as_str(&self) -> &'static str {
        match self {
            AccountType::UserWallet => "user_wallet",
            AccountType::HouseRevenue => "house_revenue",
            AccountType::HouseHold => "house_hold",
            AccountType::PaymentGatewayTransit => "payment_gateway_transit",
            AccountType::TaxReserve => "tax_reserve",
            AccountType::BonusPool => "bonus_pool",
        }
    }
}
