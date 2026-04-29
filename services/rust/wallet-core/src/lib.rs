//! Wallet Core Library

pub mod api;
pub mod config;
pub mod domain;
pub mod infrastructure;
pub mod service;
pub mod telemetry;

pub use config::Config;
pub use domain::{Wallet, Transaction, Balance, WalletType, TransactionType, FundLock};
pub use service::WalletService;

pub mod proto {
    pub mod common {
        pub mod v1 {
            tonic::include_proto!("common.v1");
        }
    }
    pub mod wallet {
        pub mod v1 {
            tonic::include_proto!("wallet.v1");
        }
    }
}
