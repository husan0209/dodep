use serde::{Deserialize, Serialize};

/// Topics a client can subscribe to
#[derive(Debug, Clone, Hash, Eq, PartialEq)]
pub enum Topic {
    EventOdds(i64),
    EventStats(i64),
    SportScores(i64),
    UserBets(i64),
    UserBalance(i64),
}

impl std::fmt::Display for Topic {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Topic::EventOdds(id) => write!(f, "event_odds:{id}"),
            Topic::EventStats(id) => write!(f, "event_stats:{id}"),
            Topic::SportScores(id) => write!(f, "sport_scores:{id}"),
            Topic::UserBets(id) => write!(f, "user_bets:{id}"),
            Topic::UserBalance(id) => write!(f, "user_balance:{id}"),
        }
    }
}

/// Client sends JSON messages to subscribe/unsubscribe
#[derive(Debug, Deserialize)]
#[serde(tag = "action")]
pub enum ClientMessage {
    #[serde(rename = "subscribe")]
    Subscribe { topics: Vec<TopicRequest> },

    #[serde(rename = "unsubscribe")]
    Unsubscribe { topics: Vec<TopicRequest> },

    #[serde(rename = "ping")]
    Ping,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct TopicRequest {
    #[serde(rename = "type")]
    pub topic_type: String,
    pub id: i64,
}

/// Server response messages
#[derive(Debug, Serialize)]
#[serde(tag = "action")]
pub enum ServerMessage {
    #[serde(rename = "pong")]
    Pong,

    #[serde(rename = "subscribed")]
    Subscribed { topics: Vec<TopicResponse> },

    #[serde(rename = "unsubscribed")]
    Unsubscribed { topics: Vec<TopicResponse> },

    #[serde(rename = "error")]
    Error { code: String, message: String },
}

#[derive(Debug, Serialize)]
pub struct TopicResponse {
    #[serde(rename = "type")]
    pub topic_type: String,
    pub id: i64,
}

pub fn parse_topic(req: &TopicRequest) -> Option<Topic> {
    match req.topic_type.as_str() {
        "event_odds" => Some(Topic::EventOdds(req.id)),
        "event_stats" => Some(Topic::EventStats(req.id)),
        "sport_scores" => Some(Topic::SportScores(req.id)),
        "user_bets" => Some(Topic::UserBets(req.id)),
        "user_balance" => Some(Topic::UserBalance(req.id)),
        _ => None,
    }
}

/// Extract Kafka topic and key from message to determine which Topic to broadcast to
pub fn kafka_to_topic(kafka_topic: &str, key: &str) -> Option<Topic> {
    let id: i64 = key.parse().ok()?;
    match kafka_topic {
        "events.odds_updated" => Some(Topic::EventOdds(id)),
        "bets.bet.placed" | "bets.bet.settled" | "bets.bet.cashout" => {
            Some(Topic::UserBets(id))
        }
        "analytics.events" => Some(Topic::EventStats(id)),
        _ => None,
    }
}

/// Kafka topics the gateway subscribes to
pub const KAFKA_TOPICS: &[&str] = &[
    "events.odds_updated",
    "bets.bet.placed",
    "bets.bet.settled",
    "bets.bet.cashout",
    "analytics.events",
];
