use std::sync::Arc;

use crate::config::AppConfig;
use crate::events::EventProducer;
use crate::repositories::bet_repo::BetRepository;
use crate::services::bet_service::BetService;
use crate::services::cashout_service::CashoutService;
use crate::services::settlement_service::SettlementService;

#[derive(Clone)]
pub struct AppState {
    inner: Arc<AppStateInner>,
}

struct AppStateInner {
    config: AppConfig,
    db_pool: sqlx::PgPool,
    bet_service: BetService,
    cashout_service: CashoutService,
    settlement_service: SettlementService,
    event_producer: Option<EventProducer>,
}

impl AppState {
    pub fn new(config: AppConfig, db_pool: sqlx::PgPool) -> Self {
        let bet_repo = BetRepository::new(db_pool.clone());
        let bet_service = BetService::new(bet_repo.clone());
        let cashout_service = CashoutService::new(bet_repo.clone());
        let settlement_service = SettlementService::new(bet_repo);

        let event_producer = EventProducer::new(&config.redpanda_brokers).ok();

        Self {
            inner: Arc::new(AppStateInner {
                config,
                db_pool,
                bet_service,
                cashout_service,
                settlement_service,
                event_producer,
            }),
        }
    }

    pub fn config(&self) -> &AppConfig {
        &self.inner.config
    }

    pub fn db_pool(&self) -> &sqlx::PgPool {
        &self.inner.db_pool
    }

    pub fn bet_service(&self) -> &BetService {
        &self.inner.bet_service
    }

    pub fn cashout_service(&self) -> &CashoutService {
        &self.inner.cashout_service
    }

    pub fn settlement_service(&self) -> &SettlementService {
        &self.inner.settlement_service
    }

    pub fn event_producer(&self) -> Option<&EventProducer> {
        self.inner.event_producer.as_ref()
    }
}
