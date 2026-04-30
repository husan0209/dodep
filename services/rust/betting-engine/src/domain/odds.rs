use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};

use super::bet::{BetType, SelectionRequest};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OddsData {
    pub event_id: i64,
    pub market_id: i64,
    pub outcome_id: i64,
    pub odds: Decimal,
    pub updated_at: i64,
}

pub fn calculate_combined_odds(selections: &[SelectionRequest], bet_type: BetType) -> Decimal {
    match selections.len() {
        0 => Decimal::ZERO,
        1 => selections[0].odds,
        _ => match bet_type {
            BetType::Single => selections[0].odds,
            BetType::Accumulator | BetType::Chain => selections
                .iter()
                .map(|s| s.odds)
                .fold(Decimal::ONE, |acc, odds| acc * odds),
            BetType::System => {
                // Simplified: return average of all odds
                // Full implementation would calculate all combinations
                let sum: Decimal = selections.iter().map(|s| s.odds).sum();
                sum / Decimal::from(selections.len())
            }
        },
    }
}

pub fn calculate_potential_win(stake: Decimal, combined_odds: Decimal) -> Decimal {
    stake * combined_odds
}

pub fn calculate_cashout_value(
    current_odds: Decimal,
    original_odds: Decimal,
    stake: Decimal,
    margin: Decimal,
) -> Decimal {
    // Cashout = stake * (current_odds / original_odds) * (1 - margin)
    if original_odds.is_zero() {
        return Decimal::ZERO;
    }
    let ratio = current_odds / original_odds;
    stake * ratio * (Decimal::ONE - margin)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_single_bet_odds() {
        let selections = vec![SelectionRequest {
            event_id: 1,
            market_id: 1,
            outcome_id: 1,
            odds: Decimal::try_from("2.50").unwrap(),
        }];
        let odds = calculate_combined_odds(&selections, BetType::Single);
        assert_eq!(odds, Decimal::try_from("2.50").unwrap());
    }

    #[test]
    fn test_accumulator_odds() {
        let selections = vec![
            SelectionRequest {
                event_id: 1,
                market_id: 1,
                outcome_id: 1,
                odds: Decimal::try_from("2.00").unwrap(),
            },
            SelectionRequest {
                event_id: 2,
                market_id: 1,
                outcome_id: 1,
                odds: Decimal::try_from("3.00").unwrap(),
            },
        ];
        let odds = calculate_combined_odds(&selections, BetType::Accumulator);
        assert_eq!(odds, Decimal::from(6));
    }

    #[test]
    fn test_potential_win() {
        let stake = Decimal::from(100);
        let odds = Decimal::try_from("2.50").unwrap();
        let win = calculate_potential_win(stake, odds);
        assert_eq!(win, Decimal::from(250));
    }

    #[test]
    fn test_cashout_value() {
        let current = Decimal::try_from("1.50").unwrap();
        let original = Decimal::try_from("3.00").unwrap();
        let stake = Decimal::from(100);
        let margin = Decimal::try_from("0.05").unwrap();
        let cashout = calculate_cashout_value(current, original, stake, margin);
        // 100 * (1.5/3.0) * 0.95 = 47.5
        assert_eq!(cashout, Decimal::try_from("47.50").unwrap());
    }
}
