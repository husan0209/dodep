use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use sqlx::Type;
use uuid::Uuid;
use validator::Validate;

use super::selection::{Selection, SelectionResult};

// ── Newtype IDs ──

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct BetId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct UserId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct EventId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct MarketId(pub i64);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct OutcomeId(pub i64);

impl std::fmt::Display for BetId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl std::fmt::Display for UserId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl std::fmt::Display for EventId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl From<i64> for BetId {
    fn from(id: i64) -> Self {
        Self(id)
    }
}

impl From<i64> for UserId {
    fn from(id: i64) -> Self {
        Self(id)
    }
}

impl From<i64> for EventId {
    fn from(id: i64) -> Self {
        Self(id)
    }
}

// ── Enums ──

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Type)]
#[sqlx(type_name = "bet_type_enum", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
pub enum BetType {
    Single,
    Accumulator,
    System,
    Chain,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Type)]
#[sqlx(type_name = "bet_status_enum", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
pub enum BetStatus {
    Pending,
    Active,
    Won,
    Lost,
    Void,
    Cashout,
    Rejected,
}

impl BetStatus {
    pub fn can_transition_to(&self, target: BetStatus) -> bool {
        matches!(
            (self, target),
            (BetStatus::Pending, BetStatus::Active)
                | (BetStatus::Pending, BetStatus::Rejected)
                | (BetStatus::Active, BetStatus::Won)
                | (BetStatus::Active, BetStatus::Lost)
                | (BetStatus::Active, BetStatus::Void)
                | (BetStatus::Active, BetStatus::Cashout)
        )
    }
}

#[derive(Debug, Clone, Copy, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AcceptOddsChanges {
    None,
    Higher,
    Any,
}

impl Default for AcceptOddsChanges {
    fn default() -> Self {
        Self::None
    }
}

// ── Core Entity ──

#[derive(Debug, Clone, Serialize)]
pub struct Bet {
    pub id: BetId,
    pub user_id: UserId,
    pub bet_type: BetType,
    pub status: BetStatus,
    pub stake: Decimal,
    pub combined_odds: Decimal,
    pub potential_win: Decimal,
    pub actual_win: Decimal,
    pub currency_code: String,
    pub sport_id: Option<i32>,
    pub event_id: Option<EventId>,
    pub idempotency_key: Uuid,
    pub ip_address: Option<String>,
    pub device_fingerprint: Option<String>,
    pub placed_at: DateTime<Utc>,
    pub settled_at: Option<DateTime<Utc>>,
    pub selections: Vec<Selection>,
}

// ── DB Row (matches SQLx query_as!) ──

#[derive(Debug, sqlx::FromRow)]
pub struct BetRow {
    pub id: i64,
    pub user_id: i64,
    pub bet_type: BetType,
    pub status: BetStatus,
    pub stake: Decimal,
    pub potential_win: Decimal,
    pub actual_win: Decimal,
    pub odds: Decimal,
    pub currency_code: String,
    pub sport_id: Option<i32>,
    pub event_id: Option<i64>,
    pub idempotency_key: Uuid,
    pub ip_address: Option<String>,
    pub device_fingerprint: Option<String>,
    pub placed_at: DateTime<Utc>,
    pub settled_at: Option<DateTime<Utc>>,
}

impl From<BetRow> for Bet {
    fn from(row: BetRow) -> Self {
        Self {
            id: BetId(row.id),
            user_id: UserId(row.user_id),
            bet_type: row.bet_type,
            status: row.status,
            stake: row.stake,
            combined_odds: row.odds,
            potential_win: row.potential_win,
            actual_win: row.actual_win,
            currency_code: row.currency_code,
            sport_id: row.sport_id,
            event_id: row.event_id.map(EventId),
            idempotency_key: row.idempotency_key,
            ip_address: row.ip_address,
            device_fingerprint: row.device_fingerprint,
            placed_at: row.placed_at,
            settled_at: row.settled_at,
            selections: Vec::new(),
        }
    }
}

// ── Request DTOs ──

#[derive(Debug, Deserialize, Validate)]
pub struct PlaceBetRequest {
    pub bet_type: BetType,

    #[validate(length(min = 1, max = 20, message = "1-20 selections required"))]
    pub selections: Vec<SelectionRequest>,

    #[validate(custom(function = "validate_positive_decimal"))]
    pub stake: Decimal,

    #[validate(length(equal = 3, message = "Currency must be 3 chars"))]
    pub currency_code: String,

    pub idempotency_key: Uuid,

    #[serde(default)]
    pub accept_odds_changes: AcceptOddsChanges,

    #[serde(skip)]
    pub ip_address: Option<String>,

    #[serde(skip)]
    pub device_fingerprint: Option<String>,
}

#[derive(Debug, Deserialize, Validate)]
pub struct SelectionRequest {
    pub event_id: i64,
    pub market_id: i64,
    pub outcome_id: i64,

    #[validate(custom(function = "validate_positive_decimal"))]
    pub odds: Decimal,
}

#[derive(Debug, Deserialize)]
pub struct SettleBetRequest {
    pub bet_id: i64,
    pub result: String,
    pub actual_win: Decimal,
}

// ── Response DTOs ──

#[derive(Debug, Serialize)]
pub struct BetResponse {
    pub bet_id: i64,
    pub user_id: i64,
    pub bet_type: String,
    pub status: String,
    pub stake: String,
    pub odds: String,
    pub potential_win: String,
    pub actual_win: String,
    pub currency_code: String,
    pub placed_at: String,
    pub settled_at: Option<String>,
    pub selections: Vec<SelectionResponse>,
}

#[derive(Debug, Serialize)]
pub struct SelectionResponse {
    pub event_id: i64,
    pub market_id: i64,
    pub outcome_id: i64,
    pub odds: String,
    pub result: Option<String>,
}

impl From<Bet> for BetResponse {
    fn from(bet: Bet) -> Self {
        Self {
            bet_id: bet.id.0,
            user_id: bet.user_id.0,
            bet_type: format!("{:?}", bet.bet_type).to_lowercase(),
            status: format!("{:?}", bet.status).to_lowercase(),
            stake: bet.stake.to_string(),
            odds: bet.combined_odds.to_string(),
            potential_win: bet.potential_win.to_string(),
            actual_win: bet.actual_win.to_string(),
            currency_code: bet.currency_code,
            placed_at: bet.placed_at.to_rfc3339(),
            settled_at: bet.settled_at.map(|t| t.to_rfc3339()),
            selections: bet.selections.into_iter().map(Into::into).collect(),
        }
    }
}

impl From<Selection> for SelectionResponse {
    fn from(sel: Selection) -> Self {
        Self {
            event_id: sel.event_id.0,
            market_id: sel.market_id.0,
            outcome_id: sel.outcome_id.0,
            odds: sel.odds.to_string(),
            result: sel.result.map(|r| format!("{:?}", r).to_lowercase()),
        }
    }
}

// ── Paginated response ──

#[derive(Debug, Serialize)]
pub struct PaginatedResponse<T: Serialize> {
    pub data: Vec<T>,
    pub total: i64,
    pub page_size: i64,
    pub cursor: Option<String>,
}

// ── Validation helpers ──

fn validate_positive_decimal(value: &Decimal) -> Result<(), validator::ValidationError> {
    if *value <= Decimal::ZERO {
        return Err(validator::ValidationError::new("must_be_positive"));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bet_status_transitions() {
        assert!(BetStatus::Pending.can_transition_to(BetStatus::Active));
        assert!(BetStatus::Pending.can_transition_to(BetStatus::Rejected));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Won));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Lost));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Void));
        assert!(BetStatus::Active.can_transition_to(BetStatus::Cashout));

        // Invalid transitions
        assert!(!BetStatus::Won.can_transition_to(BetStatus::Lost));
        assert!(!BetStatus::Lost.can_transition_to(BetStatus::Active));
        assert!(!BetStatus::Active.can_transition_to(BetStatus::Pending));
        assert!(!BetStatus::Rejected.can_transition_to(BetStatus::Active));
    }

    #[test]
    fn test_bet_response_from_bet() {
        let bet = Bet {
            id: BetId(1),
            user_id: UserId(42),
            bet_type: BetType::Single,
            status: BetStatus::Pending,
            stake: Decimal::from(100),
            combined_odds: Decimal::try_from("2.50").unwrap(),
            potential_win: Decimal::from(250),
            actual_win: Decimal::ZERO,
            currency_code: "USD".into(),
            sport_id: Some(1),
            event_id: Some(EventId(100)),
            idempotency_key: Uuid::new_v4(),
            ip_address: None,
            device_fingerprint: None,
            placed_at: Utc::now(),
            settled_at: None,
            selections: vec![],
        };

        let resp = BetResponse::from(bet);
        assert_eq!(resp.bet_id, 1);
        assert_eq!(resp.user_id, 42);
        assert_eq!(resp.status, "pending");
    }
}
