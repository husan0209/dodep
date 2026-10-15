//! Read-only sports feed handlers (`GET /api/v1/sports/events*`).
//!
//! Mobile (`/sportsbook`, `/sportsbook/live`, `/sportsbook/event/{id}`)
//! needs this catalog to render anything. The backing `events` / `markets`
//! tables do not exist yet (see `domain::sports_feed`), so every handler
//! below returns an honest `501 Not Implemented` whose JSON body carries
//! the WebSocket fallback contract instead of leaving mobile to hit an
//! empty route.
//!
//! These routes are intentionally **public** (no `require_auth`): the
//! sports catalog contains no user data and is browsed pre-login, exactly
//! like the web sportsbook. Placing bets stays behind auth.

use axum::{
    extract::{Path, Query},
    http::StatusCode,
    Json,
};
use serde::Deserialize;

use crate::domain::sports_feed::{normalize_limit, FeedUnavailable};

/// Query params for `GET /api/v1/sports/events`.
///
/// All fields are optional; unknown sports are simply ignored once the
/// feed exists. `limit` is clamped to `[1, 100]` (default 20), `cursor`
/// is the opaque next-page token from the previous response.
#[derive(Debug, Deserialize)]
pub struct SportsEventsQuery {
    /// Sport slug, e.g. `football`, `tennis`.
    pub sport: Option<String>,
    /// `true` → live events only, `false` → pre-match only.
    pub live: Option<bool>,
    pub limit: Option<i64>,
    pub cursor: Option<String>,
}

/// List sports events (paginated, filterable by sport and live flag).
#[tracing::instrument(name = "handler.list_sports_events", skip(query))]
pub async fn list_sports_events(
    Query(query): Query<SportsEventsQuery>,
) -> (StatusCode, Json<FeedUnavailable>) {
    let _limit = normalize_limit(query.limit);
    let _cursor = query.cursor;
    (StatusCode::NOT_IMPLEMENTED, Json(FeedUnavailable::list()))
}

/// Get one sports event with its markets.
#[tracing::instrument(name = "handler.get_sports_event", skip_all, fields(event_id = %event_id))]
pub async fn get_sports_event(Path(event_id): Path<String>) -> (StatusCode, Json<FeedUnavailable>) {
    (
        StatusCode::NOT_IMPLEMENTED,
        Json(FeedUnavailable::detail(event_id)),
    )
}

/// Get current odds for one event (live / pre-match resync).
#[tracing::instrument(name = "handler.get_event_odds", skip_all, fields(event_id = %event_id))]
pub async fn get_event_odds(Path(event_id): Path<String>) -> (StatusCode, Json<FeedUnavailable>) {
    (
        StatusCode::NOT_IMPLEMENTED,
        Json(FeedUnavailable::odds(event_id)),
    )
}
