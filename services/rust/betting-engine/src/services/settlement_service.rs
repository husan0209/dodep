use rust_decimal::Decimal;
use tracing::info;

use crate::domain::bet::*;
use crate::errors::AppError;
use crate::repositories::bet_repo::BetRepository;

#[derive(Clone)]
pub struct SettlementService {
    bet_repo: BetRepository,
}

impl SettlementService {
    pub fn new(bet_repo: BetRepository) -> Self {
        Self { bet_repo }
    }

    #[tracing::instrument(name = "service.settle_bet", skip(self), fields(bet_id = %bet_id))]
    pub async fn settle_bet(
        &self,
        bet_id: BetId,
        result: &str,
        actual_win: Decimal,
    ) -> Result<Bet, AppError> {
        let target_status = match result {
            "won" => BetStatus::Won,
            "lost" => BetStatus::Lost,
            "void" => BetStatus::Void,
            _ => {
                return Err(AppError::Validation(vec![crate::errors::FieldError {
                    field: "result".into(),
                    message: "Must be 'won', 'lost', or 'void'".into(),
                }]))
            }
        };

        let mut tx = self
            .bet_repo
            .pool_ref()
            .begin()
            .await
            .map_err(|e| AppError::Database(e))?;

        let bet = self
            .bet_repo
            .update_bet_status(&mut tx, bet_id, BetStatus::Active, target_status, Some(actual_win))
            .await
            .map_err(|e| match e {
                sqlx::Error::RowNotFound => AppError::Conflict {
                    reason: "Bet not in active state or not found".into(),
                },
                other => AppError::Database(other),
            })?;

        tx.commit().await.map_err(|e| AppError::Database(e))?;

        info!(
            bet_id = %bet_id,
            result = result,
            actual_win = %actual_win,
            "Bet settled"
        );

        Ok(bet)
    }

    pub async fn void_bet(&self, bet_id: BetId) -> Result<Bet, AppError> {
        let mut tx = self
            .bet_repo
            .pool_ref()
            .begin()
            .await
            .map_err(|e| AppError::Database(e))?;

        let bet = self
            .bet_repo
            .update_bet_status(&mut tx, bet_id, BetStatus::Active, BetStatus::Void, None)
            .await
            .map_err(|e| match e {
                sqlx::Error::RowNotFound => AppError::Conflict {
                    reason: "Bet not in active state".into(),
                },
                other => AppError::Database(other),
            })?;

        tx.commit().await.map_err(|e| AppError::Database(e))?;

        info!(bet_id = %bet_id, "Bet voided");

        Ok(bet)
    }
}
