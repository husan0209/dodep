//! Sports feed read models (events / markets / odds).
//!
//! These types define the **REST contract** for the read-only sports feed
//! consumed by mobile (`GET /api/v1/sports/events*`):
//!
//! - All odds and money values serialize as **decimal strings**
//!   (e.g. `"2.50"`). There is deliberately no `f64` anywhere in this
//!   module — floats must never carry odds or money (CONVENTIONS.md NEVER-6).
//! - The backing `events` / `markets` / `outcomes` tables do **not** exist
//!   yet in `libs/migrations` (the only sports tables present,
//!   `sports_events` / `sports_markets` from `013_admin_bff.sql`, are owned
//!   by the admin domain, use UUID keys, and carry no outcome odds).
//!   Until a betting-owned feed schema lands, the HTTP handlers return
//!   `501 Not Implemented` with [`FeedUnavailable`] as the body.
//! - Pure helpers in this module ([`implied_probability`],
//!   [`calculate_margin`]) use `rust_decimal::Decimal` arithmetic only.

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};

// ── Pagination ────────────────────────────────────────────────

/// Default page size for the events feed.
pub const DEFAULT_PAGE_SIZE: i64 = 20;
/// Hard cap for the events feed page size.
pub const MAX_PAGE_SIZE: i64 = 100;

/// Normalize a client-supplied `limit` into `[1, MAX_PAGE_SIZE]`.
#[must_use]
pub fn normalize_limit(limit: Option<i64>) -> i64 {
    limit.unwrap_or(DEFAULT_PAGE_SIZE).clamp(1, MAX_PAGE_SIZE)
}

/// Decode an opaque cursor produced by [`encode_cursor`].
///
/// Returns `None` for missing or malformed cursors (caller restarts from
/// the head of the feed instead of failing the request).
#[must_use]
pub fn decode_cursor(cursor: Option<&str>) -> Option<i64> {
    cursor?.trim().parse::<i64>().ok()
}

/// Encode the next-page cursor from the last visible event id.
#[must_use]
pub fn encode_cursor(last_event_id: i64) -> String {
    last_event_id.to_string()
}

// ── Pure odds helpers (Decimal only, no floats) ───────────────

/// Implied probability of decimal odds: `1 / odds`.
///
/// Returns `None` for non-positive odds (invalid input, never panics).
#[must_use]
pub fn implied_probability(odds: Decimal) -> Option<Decimal> {
    if odds <= Decimal::ZERO {
        return None;
    }
    Some(Decimal::ONE / odds)
}

/// Bookmaker margin (overround) of a market: `sum(1/odds) - 1`.
///
/// Example: `[2.10, 3.40, 3.20]` → `≈ 0.0828` (8.28%).
/// Returns `None` if any odds value is non-positive.
#[must_use]
pub fn calculate_margin(odds: &[Decimal]) -> Option<Decimal> {
    if odds.is_empty() || odds.iter().any(|o| *o <= Decimal::ZERO) {
        return None;
    }
    let implied_sum: Decimal = odds.iter().map(|o| Decimal::ONE / *o).sum();
    Some(implied_sum - Decimal::ONE)
}

// ── Feed status ───────────────────────────────────────────────

/// Lifecycle status of a sports event as exposed to clients.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EventFeedStatus {
    Scheduled,
    Open,
    Live,
    Suspended,
    Finished,
    Resulted,
    Cancelled,
    Postponed,
}

impl EventFeedStatus {
    /// Whether clients may currently see betting markets for this event.
    #[must_use]
    pub fn is_bettable(self) -> bool {
        matches!(self, Self::Open | Self::Live)
    }
}

impl std::fmt::Display for EventFeedStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            Self::Scheduled => "scheduled",
            Self::Open => "open",
            Self::Live => "live",
            Self::Suspended => "suspended",
            Self::Finished => "finished",
            Self::Resulted => "resulted",
            Self::Cancelled => "cancelled",
            Self::Postponed => "postponed",
        };
        write!(f, "{s}")
    }
}

/// Lifecycle status of a market as exposed to clients.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MarketFeedStatus {
    Open,
    Suspended,
    Closed,
    Resulted,
    Settled,
    Void,
}

// ── Read models (future 200 OK contract) ──────────────────────

/// Serialize a `Decimal` as a plain decimal string (`"2.50"`).
///
/// Central helper so no response struct can accidentally emit odds as a
/// JSON number.
#[must_use]
pub fn decimal_string(value: Decimal) -> String {
    value.to_string()
}

/// Single event row in `GET /api/v1/sports/events`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SportEventSummary {
    /// Opaque event id (string: future feed tables may use BIGINT or UUID).
    pub event_id: String,
    pub sport: String,
    pub league: String,
    pub home_team: String,
    pub away_team: String,
    pub status: EventFeedStatus,
    pub is_live: bool,
    pub start_time: DateTime<Utc>,
    pub market_count: i32,
}

/// Full event with markets for `GET /api/v1/sports/events/{id}`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SportEventDetail {
    #[serde(flatten)]
    pub summary: SportEventSummary,
    pub markets: Vec<MarketSnapshot>,
}

/// One market with its outcomes.
///
/// `margin` is the overround as a decimal string (e.g. `"0.0828"`),
/// or `None` when it cannot be computed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MarketSnapshot {
    pub market_id: String,
    pub name: String,
    pub status: MarketFeedStatus,
    /// Handicap / total line as decimal string (e.g. `"2.5"`), if any.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line: Option<String>,
    pub outcomes: Vec<OutcomeSnapshot>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub margin: Option<String>,
}

impl MarketSnapshot {
    /// Build a snapshot from Decimal odds, computing the margin.
    ///
    /// Odds are converted to strings at construction time so serialization
    /// can never leak a float.
    #[must_use]
    pub fn from_odds(
        market_id: String,
        name: String,
        status: MarketFeedStatus,
        line: Option<Decimal>,
        outcomes: Vec<(String, String, Decimal)>,
    ) -> Self {
        let odds_values: Vec<Decimal> = outcomes.iter().map(|(_, _, o)| *o).collect();
        let margin = calculate_margin(&odds_values).map(decimal_string);
        Self {
            market_id,
            name,
            status,
            line: line.map(decimal_string),
            outcomes: outcomes
                .into_iter()
                .map(|(outcome_id, outcome_name, odds)| OutcomeSnapshot {
                    outcome_id,
                    name: outcome_name,
                    odds: decimal_string(odds),
                    status: OutcomeFeedStatus::Active,
                })
                .collect(),
            margin,
        }
    }
}

/// One outcome with decimal odds serialized as a string.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutcomeSnapshot {
    pub outcome_id: String,
    pub name: String,
    /// Decimal odds as string (e.g. `"2.50"`). Never a JSON number.
    pub odds: String,
    pub status: OutcomeFeedStatus,
}

/// Lifecycle status of an outcome as exposed to clients.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OutcomeFeedStatus {
    Active,
    Suspended,
    Closed,
    Resulted,
}

/// Odds payload for `GET /api/v1/sports/events/{id}/odds`.
///
/// `ttl_secs` tells the client how long the snapshot may be cached:
/// 5s for live events, 30s for pre-match.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventOddsResponse {
    pub event_id: String,
    pub is_live: bool,
    pub ttl_secs: u64,
    pub markets: Vec<MarketSnapshot>,
}

// ── 501 contract (returned until feed tables exist) ───────────

/// Redpanda topic carrying live odds updates (see `docs/api/betting-engine.md`).
pub const ODDS_UPDATED_TOPIC: &str = "events.odds_updated";
/// DragonflyDB key pattern for cached odds snapshots.
pub const ODDS_CACHE_KEY_PATTERN: &str = "odds:{event_id}:{market_id}:{outcome_id}";
/// Cache TTL for live-event odds.
pub const ODDS_LIVE_TTL_SECS: u64 = 5;
/// Cache TTL for pre-match odds.
pub const ODDS_PREMATCH_TTL_SECS: u64 = 30;

/// Error envelope returned with HTTP 501 while the feed is unavailable.
///
/// The shape matches `AppError`'s `{ "error": { "code", "message",
/// "details" } }` so mobile can reuse its existing error-state handling.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeedUnavailable {
    pub error: FeedUnavailableBody,
}

/// Inner `{ code, message, details }` of [`FeedUnavailable`].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeedUnavailableBody {
    pub code: String,
    pub message: String,
    pub details: FeedUnavailableDetails,
}

/// Machine-readable details: what was requested and where to get the
/// same data over WebSocket instead.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeedUnavailableDetails {
    pub resource: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub event_id: Option<String>,
    pub retryable: bool,
    pub docs: String,
    pub ws_alternative: WsAlternative,
}

/// WebSocket fallback contract for mobile.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WsAlternative {
    pub transport: String,
    pub redpanda_topic: String,
    pub topic_key: String,
    /// Ordered field list of each `events.odds_updated` message.
    pub fields: Vec<String>,
    /// Odds are decimal strings, e.g. `"2.50"`.
    pub odds_format: String,
    pub cache_key_pattern: String,
    pub ttl_live_secs: u64,
    pub ttl_prematch_secs: u64,
}

impl FeedUnavailable {
    fn details(resource: &str, event_id: Option<String>) -> FeedUnavailableDetails {
        FeedUnavailableDetails {
            resource: resource.to_string(),
            event_id,
            retryable: false,
            docs: "docs/api/betting-engine.md#sports-feed".to_string(),
            ws_alternative: WsAlternative {
                transport: "websocket".to_string(),
                redpanda_topic: ODDS_UPDATED_TOPIC.to_string(),
                topic_key: "event_id".to_string(),
                fields: [
                    "event_id",
                    "market_id",
                    "outcome_id",
                    "odds",
                    "status",
                    "updated_at",
                ]
                .iter()
                .map(|s| (*s).to_string())
                .collect(),
                odds_format: "decimal string, e.g. \"2.50\"".to_string(),
                cache_key_pattern: ODDS_CACHE_KEY_PATTERN.to_string(),
                ttl_live_secs: ODDS_LIVE_TTL_SECS,
                ttl_prematch_secs: ODDS_PREMATCH_TTL_SECS,
            },
        }
    }

    /// 501 body for `GET /api/v1/sports/events`.
    #[must_use]
    pub fn list() -> Self {
        Self {
            error: FeedUnavailableBody {
                code: "SPORTS_FEED_UNAVAILABLE".to_string(),
                message: "Sports events feed is not implemented yet".to_string(),
                details: Self::details("sports_events", None),
            },
        }
    }

    /// 501 body for `GET /api/v1/sports/events/{id}`.
    #[must_use]
    pub fn detail(event_id: String) -> Self {
        Self {
            error: FeedUnavailableBody {
                code: "SPORTS_FEED_UNAVAILABLE".to_string(),
                message: format!("Sports event {event_id} is not available yet"),
                details: Self::details("sports_event", Some(event_id)),
            },
        }
    }

    /// 501 body for `GET /api/v1/sports/events/{id}/odds`.
    #[must_use]
    pub fn odds(event_id: String) -> Self {
        Self {
            error: FeedUnavailableBody {
                code: "SPORTS_FEED_UNAVAILABLE".to_string(),
                message: format!("Odds for event {event_id} are not available via REST yet"),
                details: Self::details("event_odds", Some(event_id)),
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;

    fn d(s: &str) -> Decimal {
        Decimal::try_from(s).unwrap()
    }

    #[test]
    fn test_normalize_limit_defaults_and_clamps() {
        assert_eq!(normalize_limit(None), 20);
        assert_eq!(normalize_limit(Some(5)), 5);
        assert_eq!(normalize_limit(Some(0)), 1);
        assert_eq!(normalize_limit(Some(-10)), 1);
        assert_eq!(normalize_limit(Some(500)), 100);
    }

    #[test]
    fn test_decode_cursor_round_trip() {
        assert_eq!(decode_cursor(None), None);
        assert_eq!(decode_cursor(Some("abc")), None);
        assert_eq!(decode_cursor(Some("")), None);
        assert_eq!(decode_cursor(Some("  ")), None);
        let cursor = encode_cursor(12345);
        assert_eq!(decode_cursor(Some(&cursor)), Some(12345));
    }

    #[test]
    fn test_implied_probability_decimal_only() {
        let prob = implied_probability(d("2.50")).unwrap();
        assert_eq!(prob.round_dp(4), d("0.4000"));
        assert!(implied_probability(Decimal::ZERO).is_none());
        assert!(implied_probability(d("-1.50")).is_none());
    }

    #[test]
    fn test_calculate_margin_three_way() {
        // 1/2.10 + 1/3.40 + 1/3.20 ≈ 1.0828 → margin ≈ 0.0828
        let margin = calculate_margin(&[d("2.10"), d("3.40"), d("3.20")]).unwrap();
        assert!(margin > d("0.08") && margin < d("0.09"));
    }

    #[test]
    fn test_calculate_margin_rejects_invalid() {
        assert!(calculate_margin(&[]).is_none());
        assert!(calculate_margin(&[d("2.00"), Decimal::ZERO]).is_none());
        // Fair coin flip has zero margin.
        assert_eq!(
            calculate_margin(&[d("2.00"), d("2.00")]).unwrap(),
            Decimal::ZERO
        );
    }

    #[test]
    fn test_event_feed_status_bettable() {
        assert!(EventFeedStatus::Open.is_bettable());
        assert!(EventFeedStatus::Live.is_bettable());
        assert!(!EventFeedStatus::Suspended.is_bettable());
        assert!(!EventFeedStatus::Finished.is_bettable());
        assert!(!EventFeedStatus::Cancelled.is_bettable());
    }

    #[test]
    fn test_outcome_odds_serialize_as_string_never_float() {
        let market = MarketSnapshot::from_odds(
            "m1".into(),
            "Match Result".into(),
            MarketFeedStatus::Open,
            None,
            vec![
                ("o1".into(), "Home".into(), d("2.10")),
                ("o2".into(), "Draw".into(), d("3.40")),
                ("o3".into(), "Away".into(), d("3.20")),
            ],
        );
        let value: Value = serde_json::to_value(&market).unwrap();
        let outcomes = value["outcomes"].as_array().unwrap();
        assert_eq!(outcomes.len(), 3);
        for outcome in outcomes {
            let odds = &outcome["odds"];
            assert!(odds.is_string(), "odds must be a JSON string, got: {odds}");
        }
        assert_eq!(outcomes[0]["odds"], Value::String("2.10".into()));
        // Margin is also a decimal string.
        assert!(value["margin"].is_string());
        // No floats anywhere in the payload.
        let raw = serde_json::to_string(&value).unwrap();
        let reparsed: Value = serde_json::from_str(&raw).unwrap();
        assert!(reparsed["outcomes"][0]["odds"].is_string());
    }

    #[test]
    fn test_line_serializes_as_string() {
        let market = MarketSnapshot::from_odds(
            "m2".into(),
            "Over/Under 2.5".into(),
            MarketFeedStatus::Open,
            Some(d("2.5")),
            vec![("o1".into(), "Over".into(), d("1.85"))],
        );
        let value: Value = serde_json::to_value(&market).unwrap();
        assert_eq!(value["line"], Value::String("2.5".into()));
    }

    #[test]
    fn test_feed_unavailable_shape_matches_error_contract() {
        let body = FeedUnavailable::odds("evt-42".into());
        let value: Value = serde_json::to_value(&body).unwrap();
        assert_eq!(value["error"]["code"], "SPORTS_FEED_UNAVAILABLE");
        assert_eq!(value["error"]["details"]["event_id"], "evt-42");
        assert_eq!(value["error"]["details"]["retryable"], false);
        assert_eq!(
            value["error"]["details"]["ws_alternative"]["redpanda_topic"],
            "events.odds_updated"
        );
        let fields = value["error"]["details"]["ws_alternative"]["fields"]
            .as_array()
            .unwrap();
        assert!(fields.contains(&Value::String("odds".into())));
        assert_eq!(
            value["error"]["details"]["ws_alternative"]["ttl_live_secs"],
            5
        );
        assert_eq!(
            value["error"]["details"]["ws_alternative"]["ttl_prematch_secs"],
            30
        );
    }
}
