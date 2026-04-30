use rust_decimal::Decimal;
use tracing::{info, warn};
use uuid::Uuid;

use crate::domain::bet::*;
use crate::domain::odds::{calculate_combined_odds, calculate_potential_win};
use crate::errors::AppError;
use crate::repositories::bet_repo::{BetRepository, CreateBetParams, CreateSelectionParams};

const MIN_STAKE: &str = "0.10";
const MAX_STAKE: &str = "10000.00";
const MAX_PAYOUT: &str = "100000.00";

#[derive(Clone)]
pub struct BetService {
    bet_repo: BetRepository,
}

impl BetService {
    pub fn new(bet_repo: BetRepository) -> Self {
        Self { bet_repo }
    }

    #[tracing::instrument(name = "service.place_bet", skip(self, req), fields(user_id = %user_id))]
    pub async fn place_bet(
        &self,
        user_id: UserId,
        req: PlaceBetRequest,
    ) -> Result<Bet, AppError> {
        req.validate()?;

        if req.selections.is_empty() {
            return Err(AppError::Validation(vec![crate::errors::FieldError {
                field: "selections".into(),
                message: "At least one selection required".into(),
            }]));
        }

        let combined_odds = calculate_combined_odds(&req.selections, req.bet_type);
        let potential_win = calculate_potential_win(req.stake, combined_odds);

        self.validate_stake(req.stake, potential_win)?;

        let event_id = if req.selections.len() == 1 {
            Some(EventId(req.selections[0].event_id))
        } else {
            None
        };

        let selections: Vec<CreateSelectionParams> = req
            .selections
            .iter()
            .map(|s| CreateSelectionParams {
                event_id: EventId(s.event_id),
                market_id: MarketId(s.market_id),
                outcome_id: OutcomeId(s.outcome_id),
                odds: s.odds,
            })
            .collect();

        let bet = self
            .bet_repo
            .create_bet(CreateBetParams {
                user_id,
                bet_type: req.bet_type,
                stake: req.stake,
                combined_odds,
                potential_win,
                currency_code: req.currency_code.clone(),
                sport_id: None,
                event_id,
                selections,
                idempotency_key: req.idempotency_key,
                ip_address: req.ip_address,
                device_fingerprint: req.device_fingerprint,
            })
            .await?;

        info!(
            user_id = %user_id,
            bet_id = %bet.id,
            stake = %req.stake,
            odds = %combined_odds,
            "Bet placed"
        );

        Ok(bet)
    }

    #[tracing::instrument(name = "service.get_bet", skip(self))]
    pub async fn get_bet(
        &self,
        user_id: UserId,
        bet_id: BetId,
    ) -> Result<Bet, AppError> {
        self.bet_repo
            .get_bet_by_id(bet_id, user_id)
            .await?
            .ok_or_else(|| AppError::NotFound {
                entity: "Bet".into(),
                id: bet_id.to_string(),
            })
    }

    #[tracing::instrument(name = "service.get_history", skip(self))]
    pub async fn get_history(
        &self,
        user_id: UserId,
        limit: i64,
        cursor: Option<i64>,
        status: Option<BetStatus>,
    ) -> Result<PaginatedResponse<BetResponse>, AppError> {
        let (bets, total) = self
            .bet_repo
            .get_user_bets(user_id, limit, cursor, status)
            .await?;

        let next_cursor = bets.last().map(|b| b.id.0.to_string());

        Ok(PaginatedResponse {
            data: bets.into_iter().map(BetResponse::from).collect(),
            total,
            page_size: limit,
            cursor: next_cursor,
        })
    }

    fn validate_stake(&self, stake: Decimal, potential_win: Decimal) -> Result<(), AppError> {
        let min = Decimal::try_from(MIN_STAKE).unwrap();
        let max = Decimal::try_from(MAX_STAKE).unwrap();
        let max_payout = Decimal::try_from(MAX_PAYOUT).unwrap();

        if stake < min {
            return Err(AppError::BetStakeTooLow { min, actual: stake });
        }

        if stake > max {
            return Err(AppError::BetStakeTooHigh { max, actual: stake });
        }

        if potential_win > max_payout {
            return Err(AppError::BetMaxPayoutExceeded {
                max: max_payout,
                potential: potential_win,
            });
        }

        Ok(())
    }
}
