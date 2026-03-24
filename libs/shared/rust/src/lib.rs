//! Opus Casino Shared Libraries
//! 
//! This crate provides shared types, validators, constants, and utilities
//! for the Opus Casino gambling platform.
//! 
//! # Example
//! 
//! ```rust
//! use opus_shared::types::{UserId, Money};
//! use opus_shared::validators::{is_valid_email, is_valid_uuid};
//! use opus_shared::helpers::generate_uuid;
//! 
//! let user_id: UserId = generate_uuid();
//! let balance = Money::new("100.00", "USD");
//! 
//! assert!(is_valid_email("user@example.com"));
//! assert!(is_valid_uuid(&user_id.to_string()));
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
