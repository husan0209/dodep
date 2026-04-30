//! Integration tests for Betting Engine
//!
//! These tests validate:
//! - Domain model correctness (bet state machine, odds calculation)
//! - API contract compliance
//! - Error response format

use betting_engine::domain::bet::*;
use betting_engine::domain::odds::*;
use betting_engine::errors::AppError;

use rust_decimal::Decimal;
use uuid::Uuid;

// ============================================================
// Domain: Bet State Machine
// ============================================================

#[test]
fn test_bet_state_machine_valid_transitions() {
    let cases = vec![
        (BetStatus::Pending, BetStatus::Active, true),
        (BetStatus::Pending, BetStatus::Rejected, true),
        (BetStatus::Active, BetStatus::Won, true),
        (BetStatus::Active, BetStatus::Lost, true),
        (BetStatus::Active, BetStatus::Void, true),
        (BetStatus::Active, BetStatus::Cashout, true),
        // Invalid transitions
        (BetStatus::Won, BetStatus::Lost, false),
        (BetStatus::Lost, BetStatus::Active, false),
        (BetStatus::Active, BetStatus::Pending, false),
        (BetStatus::Rejected, BetStatus::Active, false),
        (BetStatus::Void, BetStatus::Active, false),
        (BetStatus::Cashout, BetStatus::Active, false),
    ];

    for (from, to, expected) in cases {
        assert_eq!(
            from.can_transition_to(to),
            expected,
            "Transition {from:?} -> {to:?} should be {expected}"
        );
    }
}

// ============================================================
// Domain: Odds Calculation
// ============================================================

#[test]
fn test_single_bet_odds() {
    let selections = vec![SelectionRequest {
        event_id: 1,
        market_id: 1,
        outcome_id: 1,
        odds: Decimal::try_from("2.50").unwrap(),
    }];

    let combined = calculate_combined_odds(&selections, BetType::Single);
    assert_eq!(combined, Decimal::try_from("2.50").unwrap());
}

#[test]
fn test_accumulator_bet_odds() {
    let selections = vec![
        SelectionRequest {
            event_id: 1,
            market_id: 1,
            outcome_id: 1,
            odds: Decimal::try_from("1.50").unwrap(),
        },
        SelectionRequest {
            event_id: 2,
            market_id: 1,
            outcome_id: 1,
            odds: Decimal::try_from("2.00").unwrap(),
        },
        SelectionRequest {
            event_id: 3,
            market_id: 1,
            outcome_id: 1,
            odds: Decimal::try_from("3.00").unwrap(),
        },
    ];

    let combined = calculate_combined_odds(&selections, BetType::Accumulator);
    // 1.5 * 2.0 * 3.0 = 9.0
    assert_eq!(combined, Decimal::from(9));
}

#[test]
fn test_potential_win_calculation() {
    let stake = Decimal::from(50);
    let odds = Decimal::try_from("4.00").unwrap();
    let win = calculate_potential_win(stake, odds);
    assert_eq!(win, Decimal::from(200));
}

#[test]
fn test_cashout_value() {
    // Original odds 3.0, current odds 1.5, stake 100, margin 5%
    let current = Decimal::try_from("1.50").unwrap();
    let original = Decimal::try_from("3.00").unwrap();
    let stake = Decimal::from(100);
    let margin = Decimal::try_from("0.05").unwrap();

    let cashout = calculate_cashout_value(current, original, stake, margin);
    // 100 * (1.5/3.0) * 0.95 = 47.50
    assert_eq!(cashout, Decimal::try_from("47.50").unwrap());
}

#[test]
fn test_cashout_zero_original_odds() {
    let cashout = calculate_cashout_value(
        Decimal::from(2),
        Decimal::ZERO,
        Decimal::from(100),
        Decimal::try_from("0.05").unwrap(),
    );
    assert_eq!(cashout, Decimal::ZERO);
}

// ============================================================
// Domain: Bet Response
// ============================================================

#[test]
fn test_bet_response_serialization() {
    let bet = create_test_bet();
    let resp = BetResponse::from(bet);

    assert_eq!(resp.bet_id, 1);
    assert_eq!(resp.user_id, 42);
    assert_eq!(resp.status, "pending");
    assert_eq!(resp.stake, "100");
    assert_eq!(resp.currency_code, "USD");
}

#[test]
fn test_bet_response_with_selections() {
    let mut bet = create_test_bet();
    bet.selections = vec![betting_engine::domain::selection::Selection {
        id: Some(1),
        bet_id: BetId(1),
        event_id: EventId(100),
        market_id: MarketId(1),
        outcome_id: OutcomeId(1),
        odds: Decimal::try_from("2.50").unwrap(),
        event_name: Some("Real Madrid vs Barcelona".into()),
        market_name: Some("Match Result".into()),
        result: None,
        created_at: None,
        settled_at: None,
    }];

    let resp = BetResponse::from(bet);
    assert_eq!(resp.selections.len(), 1);
    assert_eq!(resp.selections[0].event_id, 100);
}

// ============================================================
// Error handling
// ============================================================

#[test]
fn test_error_code_mapping() {
    // Verify error types produce correct HTTP codes
    let err = AppError::BetStakeTooLow {
        min: Decimal::from(10),
        actual: Decimal::from(5),
    };
    let resp = err.into_response();
    assert_eq!(resp.status(), 422);
}

// ============================================================
// Helpers
// ============================================================

fn create_test_bet() -> Bet {
    Bet {
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
        ip_address: Some("192.168.1.1".into()),
        device_fingerprint: Some("test-fp".into()),
        placed_at: chrono::Utc::now(),
        settled_at: None,
        selections: vec![],
    }
}

use axum::response::IntoResponse;
