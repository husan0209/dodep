//! HTTP handlers for the gateway.
//!
//! This module file is required: `api/mod.rs` declares `pub mod handlers;` while the
//! handlers live in `api/handlers/*.rs`, so Rust looks for either `handlers.rs` or
//! `handlers/mod.rs`. Without it the crate fails to compile with
//! "failed to resolve mod `handlers`".

pub mod health_handler;
pub mod ws_handler;

pub use health_handler::{liveness, readiness};
pub use ws_handler::ws_handler;
