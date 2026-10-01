//! Opus Casino Shared Libraries
//! 
//! This crate provides shared types, validators, constants, and utilities
//! for the Opus Casino gambling platform.
//! 
//! # Example
//!
//! ```
//! use opus_shared::types::{BetId, Money, UserId};
//! use opus_shared::validators::{is_valid_email, is_valid_uuid};
//! use opus_shared::helpers::generate_uuid;
//!
//! // Identifier newtypes wrap a UUID; each is a distinct type.
//! let user_id = UserId(generate_uuid());
//! let bet_id = BetId(generate_uuid());
//! assert_ne!(user_id.0, bet_id.0);
//!
//! // Money parses decimal strings and rejects negatives.
//! let balance = Money::new("100.00", "USD").unwrap();
//! assert!(Money::new("-1.00", "USD").is_err());
//!
//! assert!(is_valid_email("user@example.com"));
//! assert!(is_valid_uuid(&generate_uuid().to_string()));
//! ```

pub mod types;
pub mod validators;
pub mod constants;
pub mod helpers;
pub mod error;

// Re-export commonly used items
pub use types::*;
pub use validators::*;
pub use constants::*;
pub use helpers::*;
pub use error::{AppError, AppResult};
