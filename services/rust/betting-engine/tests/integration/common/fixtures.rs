use betting_engine::domain::bet::*;
use rust_decimal::Decimal;
use uuid::Uuid;

pub fn single_bet_request() -> PlaceBetRequest {
    PlaceBetRequest {
        bet_type: BetType::Single,
        selections: vec![SelectionRequest {
            event_id: 100,
            market_id: 1,
            outcome_id: 1,
            odds: Decimal::try_from("2.50").unwrap(),
        }],
        stake: Decimal::from(100),
        currency_code: "USD".into(),
        idempotency_key: Uuid::new_v4(),
        accept_odds_changes: AcceptOddsChanges::None,
        ip_address: Some("127.0.0.1".into()),
        device_fingerprint: Some("test".into()),
    }
}

pub fn accumulator_bet_request() -> PlaceBetRequest {
    PlaceBetRequest {
        bet_type: BetType::Accumulator,
        selections: vec![
            SelectionRequest {
                event_id: 100,
                market_id: 1,
                outcome_id: 1,
                odds: Decimal::try_from("1.50").unwrap(),
            },
            SelectionRequest {
                event_id: 200,
                market_id: 1,
                outcome_id: 1,
                odds: Decimal::try_from("2.00").unwrap(),
            },
        ],
        stake: Decimal::from(50),
        currency_code: "USD".into(),
        idempotency_key: Uuid::new_v4(),
        accept_odds_changes: AcceptOddsChanges::None,
        ip_address: None,
        device_fingerprint: None,
    }
}

pub fn bet_request_with_stake(stake: Decimal) -> PlaceBetRequest {
    let mut req = single_bet_request();
    req.stake = stake;
    req
}

pub fn bet_request_with_idempotency(key: Uuid) -> PlaceBetRequest {
    let mut req = single_bet_request();
    req.idempotency_key = key;
    req
}
