//! Integration tests for bet placement flow

mod common;
use common::setup::TestApp;
use common::fixtures::*;

use betting_engine::domain::bet::*;
use betting_engine::errors::AppError;
use rust_decimal::Decimal;
use uuid::Uuid;

// ============================================================
// Bet Placement
// ============================================================

#[tokio::test]
async fn test_place_single_bet_success() {
    let app = TestApp::start().await;
    app.create_test_user(1).await;

    let result = app
        .bet_service
        .place_bet(UserId(1), single_bet_request())
        .await;

    assert!(result.is_ok(), "Place bet should succeed");
    let bet = result.unwrap();
    assert_eq!(bet.user_id.0, 1);
    assert_eq!(bet.status, BetStatus::Pending);
    assert_eq!(bet.stake, Decimal::from(100));
    assert_eq!(bet.combined_odds, Decimal::try_from("2.50").unwrap());
    assert_eq!(bet.potential_win, Decimal::from(250));
    assert_eq!(bet.currency_code, "USD");
    assert!(!bet.selections.is_empty());
}

#[tokio::test]
async fn test_place_accumulator_bet_success() {
    let app = TestApp::start().await;
    app.create_test_user(2).await;

    let result = app
        .bet_service
        .place_bet(UserId(2), accumulator_bet_request())
        .await;

    assert!(result.is_ok());
    let bet = result.unwrap();
    assert_eq!(bet.bet_type, BetType::Accumulator);
    // 1.50 * 2.00 = 3.00
    assert_eq!(bet.combined_odds, Decimal::from(3));
    assert_eq!(bet.potential_win, Decimal::from(150)); // 50 * 3.00
    assert_eq!(bet.selections.len(), 2);
}

// ============================================================
// Idempotency
// ============================================================

#[tokio::test]
async fn test_place_bet_idempotent() {
    let app = TestApp::start().await;
    app.create_test_user(3).await;

    let key = Uuid::new_v4();
    let req1 = bet_request_with_idempotency(key);
    let req2 = bet_request_with_idempotency(key);

    let bet1 = app
        .bet_service
        .place_bet(UserId(3), req1)
        .await
        .unwrap();

    // Second call with same key should succeed (ON CONFLICT DO NOTHING)
    let bet2 = app
        .bet_service
        .place_bet(UserId(3), req2)
        .await
        .unwrap();

    assert_eq!(bet1.id, bet2.id, "Idempotent call should return same bet");
}

// ============================================================
// Stake Validation
// ============================================================

#[tokio::test]
async fn test_place_bet_stake_too_low() {
    let app = TestApp::start().await;
    app.create_test_user(4).await;

    let result = app
        .bet_service
        .place_bet(UserId(4), bet_request_with_stake(Decimal::try_from("0.01").unwrap()))
        .await;

    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        AppError::BetStakeTooLow { .. }
    ));
}

#[tokio::test]
async fn test_place_bet_stake_too_high() {
    let app = TestApp::start().await;
    app.create_test_user(5).await;

    let result = app
        .bet_service
        .place_bet(UserId(5), bet_request_with_stake(Decimal::from(999999)))
        .await;

    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        AppError::BetStakeTooHigh { .. }
    ));
}

#[tokio::test]
async fn test_place_bet_max_payout_exceeded() {
    let app = TestApp::start().await;
    app.create_test_user(6).await;

    // Stake 50000 * odds 10.0 = 500000 > 100000 max payout
    let mut req = single_bet_request();
    req.stake = Decimal::from(50000);
    req.selections[0].odds = Decimal::from(10);

    let result = app.bet_service.place_bet(UserId(6), req).await;

    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        AppError::BetMaxPayoutExceeded { .. }
    ));
}

#[tokio::test]
async fn test_place_bet_empty_selections_rejected() {
    let app = TestApp::start().await;
    app.create_test_user(7).await;

    let mut req = single_bet_request();
    req.selections.clear();

    let result = app.bet_service.place_bet(UserId(7), req).await;
    assert!(result.is_err());
}

// ============================================================
// Bet Retrieval
// ============================================================

#[tokio::test]
async fn test_get_bet_by_id() {
    let app = TestApp::start().await;
    app.create_test_user(8).await;

    let placed = app
        .bet_service
        .place_bet(UserId(8), single_bet_request())
        .await
        .unwrap();

    let found = app
        .bet_service
        .get_bet(UserId(8), placed.id)
        .await;

    assert!(found.is_ok());
    assert_eq!(found.unwrap().id, placed.id);
}

#[tokio::test]
async fn test_get_bet_not_found() {
    let app = TestApp::start().await;

    let result = app
        .bet_service
        .get_bet(UserId(999), BetId(999999))
        .await;

    assert!(matches!(result, Err(AppError::NotFound { .. })));
}

// ============================================================
// Bet History
// ============================================================

#[tokio::test]
async fn test_get_bet_history() {
    let app = TestApp::start().await;
    app.create_test_user(9).await;

    // Place 3 bets
    for _ in 0..3 {
        app.bet_service
            .place_bet(UserId(9), single_bet_request())
            .await
            .unwrap();
    }

    let history = app
        .bet_service
        .get_history(UserId(9), 10, None, None)
        .await
        .unwrap();

    assert_eq!(history.data.len(), 3);
    assert_eq!(history.total, 3);
}

// ============================================================
// Settlement
// ============================================================

#[tokio::test]
async fn test_settle_bet_won() {
    let app = TestApp::start().await;
    app.create_test_user(10).await;

    let bet = app
        .bet_service
        .place_bet(UserId(10), single_bet_request())
        .await
        .unwrap();

    let settled = app
        .settlement_service
        .settle_bet(bet.id, "won", Decimal::from(250))
        .await;

    assert!(settled.is_ok());
    assert_eq!(settled.unwrap().status, BetStatus::Won);
}

#[tokio::test]
async fn test_settle_bet_lost() {
    let app = TestApp::start().await;
    app.create_test_user(11).await;

    let bet = app
        .bet_service
        .place_bet(UserId(11), single_bet_request())
        .await
        .unwrap();

    let settled = app
        .settlement_service
        .settle_bet(bet.id, "lost", Decimal::ZERO)
        .await;

    assert!(settled.is_ok());
    assert_eq!(settled.unwrap().status, BetStatus::Lost);
}

#[tokio::test]
async fn test_void_bet() {
    let app = TestApp::start().await;
    app.create_test_user(12).await;

    let bet = app
        .bet_service
        .place_bet(UserId(12), single_bet_request())
        .await
        .unwrap();

    let voided = app.settlement_service.void_bet(bet.id).await;

    assert!(voided.is_ok());
    assert_eq!(voided.unwrap().status, BetStatus::Void);
}

#[tokio::test]
async fn test_settle_already_settled_returns_conflict() {
    let app = TestApp::start().await;
    app.create_test_user(13).await;

    let bet = app
        .bet_service
        .place_bet(UserId(13), single_bet_request())
        .await
        .unwrap();

    // First settle
    app.settlement_service
        .settle_bet(bet.id, "won", Decimal::from(250))
        .await
        .unwrap();

    // Second settle should fail
    let result = app
        .settlement_service
        .settle_bet(bet.id, "lost", Decimal::ZERO)
        .await;

    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), AppError::Conflict { .. }));
}

// ============================================================
// Cashout
// ============================================================

#[tokio::test]
async fn test_cashout_active_bet() {
    let app = TestApp::start().await;
    app.create_test_user(14).await;

    let bet = app
        .bet_service
        .place_bet(UserId(14), single_bet_request())
        .await
        .unwrap();

    let result = app
        .cashout_service
        .cashout(UserId(14), bet.id)
        .await;

    assert!(result.is_ok());
    let cashout = result.unwrap();
    assert_eq!(cashout.status, "cashout");
    assert!(cashout.cashout_value.parse::<f64>().unwrap() > 0.0);
}

#[tokio::test]
async fn test_cashout_already_settled_fails() {
    let app = TestApp::start().await;
    app.create_test_user(15).await;

    let bet = app
        .bet_service
        .place_bet(UserId(15), single_bet_request())
        .await
        .unwrap();

    // Settle first
    app.settlement_service
        .settle_bet(bet.id, "won", Decimal::from(250))
        .await
        .unwrap();

    // Cashout should fail
    let result = app
        .cashout_service
        .cashout(UserId(15), bet.id)
        .await;

    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), AppError::CashoutUnavailable));
}
