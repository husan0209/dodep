use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};

use super::bet::{BetId, EventId, MarketId, OutcomeId};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Selection {
    pub id: Option<i64>,
    pub bet_id: BetId,
    pub event_id: EventId,
    pub market_id: MarketId,
    pub outcome_id: OutcomeId,
    pub odds: Decimal,
    pub event_name: Option<String>,
    pub market_name: Option<String>,
    pub result: Option<SelectionResult>,
    pub created_at: Option<DateTime<Utc>>,
    pub settled_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SelectionResult {
    Won,
    Lost,
    Void,
    HalfWon,
    HalfLost,
}

#[derive(Debug, sqlx::FromRow)]
pub struct SelectionRow {
    pub id: i64,
    pub bet_id: i64,
    pub event_id: i64,
    pub market_id: i64,
    pub outcome_id: i64,
    pub odds: Decimal,
    pub status: String,
    pub result: Option<String>,
    pub created_at: DateTime<Utc>,
    pub settled_at: Option<DateTime<Utc>>,
}

impl From<SelectionRow> for Selection {
    fn from(row: SelectionRow) -> Self {
        Self {
            id: Some(row.id),
            bet_id: BetId(row.bet_id),
            event_id: EventId(row.event_id),
            market_id: MarketId(row.market_id),
            outcome_id: OutcomeId(row.outcome_id),
            odds: row.odds,
            event_name: None,
            market_name: None,
            result: row.result.and_then(|r| match r.as_str() {
                "won" => Some(SelectionResult::Won),
                "lost" => Some(SelectionResult::Lost),
                "void" => Some(SelectionResult::Void),
                _ => None,
            }),
            created_at: Some(row.created_at),
            settled_at: row.settled_at,
        }
    }
}
