use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

use axum::extract::ws::Message;
use dashmap::DashMap;
use tokio::sync::mpsc;

use crate::domain::channel::Topic;

/// Per-connection sender
pub type ClientSender = mpsc::Sender<Message>;

/// Global subscription map: topic -> set of client senders
pub struct SubscriptionManager {
    subscriptions: DashMap<Topic, Vec<ClientSender>>,
    connection_count: Arc<AtomicU64>,
    topic_counts: DashMap<i64, usize>, // per-user subscription counts (key=user_id from user_bets/user_balance topics)
}

impl SubscriptionManager {
    pub fn new() -> Self {
        Self {
            subscriptions: DashMap::new(),
            connection_count: Arc::new(AtomicU64::new(0)),
            topic_counts: DashMap::new(),
        }
    }

    pub fn subscribe(&self, topic: Topic, sender: ClientSender) {
        self.subscriptions
            .entry(topic)
            .or_default()
            .push(sender);
    }

    pub fn unsubscribe(&self, topic: &Topic, sender: &ClientSender) {
        if let Some(mut senders) = self.subscriptions.get_mut(topic) {
            let sender_ptr = sender as *const _;
            senders.retain(|s| {
                if s.is_closed() {
                    return false;
                }
                let s_ptr = s as *const _;
                sender_ptr != s_ptr
            });
        }
    }

    /// Broadcast binary message to all subscribers of a topic
    pub fn broadcast(&self, topic: &Topic, message: &[u8]) {
        if let Some(mut senders) = self.subscriptions.get_mut(topic) {
            let msg = Message::Binary(message.to_vec());

            senders.retain(|sender| {
                if sender.is_closed() {
                    return false;
                }
                // try_send: non-blocking, drops if buffer full (slow client protection)
                sender.try_send(msg.clone()).is_ok()
            });
        }
    }

    pub fn connection_count(&self) -> u64 {
        self.connection_count.load(Ordering::Relaxed)
    }

    pub fn increment_connections(&self) {
        self.connection_count.fetch_add(1, Ordering::Relaxed);
    }

    pub fn decrement_connections(&self) {
        self.connection_count.fetch_sub(1, Ordering::Relaxed);
    }

    pub fn subscriber_count(&self, topic: &Topic) -> usize {
        self.subscriptions
            .get(topic)
            .map(|s| s.len())
            .unwrap_or(0)
    }

    pub fn total_subscriptions(&self) -> usize {
        self.subscriptions.iter().map(|entry| entry.value().len()).sum()
    }
}

impl Clone for SubscriptionManager {
    fn clone(&self) -> Self {
        Self {
            subscriptions: DashMap::new(),
            connection_count: Arc::clone(&self.connection_count),
            topic_counts: DashMap::new(),
        }
    }
}
