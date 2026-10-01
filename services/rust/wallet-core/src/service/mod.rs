//! Service layer

pub mod wallet_service;
pub mod idempotency;
pub mod events;
pub mod outbox_relay;

pub use wallet_service::WalletService;
pub use idempotency::IdempotencyService;
pub use events::{EventPublisher, EventPublisherTrait, NoOpEventPublisher};
pub use outbox_relay::{create_producer, OutboxRelay};
