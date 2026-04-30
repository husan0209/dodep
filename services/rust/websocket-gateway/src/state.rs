use std::sync::Arc;

use tokio::sync::broadcast;

use crate::config::AppConfig;
use crate::infrastructure::connection_manager::SubscriptionManager;
use crate::infrastructure::kafka_consumer::{KafkaBroadcaster, KafkaMessage};

#[derive(Clone)]
pub struct AppState {
    inner: Arc<AppStateInner>,
}

struct AppStateInner {
    config: AppConfig,
    subscription_manager: SubscriptionManager,
    kafka_tx: broadcast::Sender<KafkaMessage>,
}

impl AppState {
    pub fn new(
        config: AppConfig,
        kafka_broadcaster: KafkaBroadcaster,
    ) -> Self {
        let (kafka_tx, _) = broadcast::channel::<KafkaMessage>(4096);
        let subscription_manager = SubscriptionManager::new();

        // Spawn Kafka consumer task
        let kafka_tx_clone = kafka_tx.clone();
        tokio::spawn(async move {
            kafka_broadcaster.run(kafka_tx_clone).await;
        });

        Self {
            inner: Arc::new(AppStateInner {
                config,
                subscription_manager,
                kafka_tx,
            }),
        }
    }

    pub fn config(&self) -> &AppConfig {
        &self.inner.config
    }

    pub fn subscription_manager(&self) -> &SubscriptionManager {
        &self.inner.subscription_manager
    }

    pub fn kafka_receiver(&self) -> broadcast::Receiver<KafkaMessage> {
        self.inner.kafka_tx.subscribe()
    }
}
