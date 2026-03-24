


# SKILL #4 — betting-engine-logic.skill.md

Доменный skill для всех агентов, работающих с логикой ставок.

---

```markdown
# betting-engine-logic.skill.md
# GAMBLING PLATFORM — BETTING ENGINE DOMAIN LOGIC
# Version: 1.0.0
# Updated: 2025
# Loaded by: Rust Core Agent, Go Business Agent, Frontend Agent, QA Agent
# Prerequisites: architecture-overview.skill.md, rust-general.skill.md

# ============================================================
# SECTION 1: BETTING DOMAIN OVERVIEW
# ============================================================

## WHAT THIS SKILL COVERS

```text
Everything about sports betting logic:
- Data hierarchy (Sport → Event → Market → Outcome)
- Odds formats and calculations
- Bet types (single, accumulator, system)
- Bet lifecycle (state machine)
- Settlement logic
- Cashout logic
- Liability management
- Margin calculation
- Live betting specifics
- Edge cases and race conditions
```

## DATA HIERARCHY

```text
┌─────────────────────────────────────────────────────────┐
│                    BETTING DATA MODEL                    │
│                                                         │
│  Sport                                                  │
│  ├── Football                                           │
│  │   ├── Category: International                        │
│  │   │   ├── Tournament: UEFA Champions League          │
│  │   │   │   ├── Event: Real Madrid vs Barcelona        │
│  │   │   │   │   ├── Market: Match Result (1X2)         │
│  │   │   │   │   │   ├── Outcome: Real Madrid   @2.10  │
│  │   │   │   │   │   ├── Outcome: Draw           @3.40  │
│  │   │   │   │   │   └── Outcome: Barcelona      @3.20  │
│  │   │   │   │   ├── Market: Over/Under 2.5 Goals       │
│  │   │   │   │   │   ├── Outcome: Over 2.5      @1.85  │
│  │   │   │   │   │   └── Outcome: Under 2.5     @1.95  │
│  │   │   │   │   ├── Market: Both Teams to Score        │
│  │   │   │   │   ├── Market: Correct Score              │
│  │   │   │   │   ├── Market: First Goalscorer           │
│  │   │   │   │   └── ... (50-200 markets per event)     │
│  │   │   │   └── Event: Bayern vs PSG                   │
│  │   │   └── Tournament: FIFA World Cup                 │
│  │   └── Category: England                              │
│  │       └── Tournament: Premier League                 │
│  ├── Basketball                                         │
│  ├── Tennis                                             │
│  └── ... (30+ sports)                                   │
└─────────────────────────────────────────────────────────┘

CARDINALITY (typical):
  Sports:              30-50
  Categories/sport:    10-30
  Tournaments/category: 5-20
  Events/tournament:   2-500 (seasonal)
  Markets/event:       50-200 (football), 20-80 (tennis)
  Outcomes/market:     2-100 (correct score has ~100)
  
  Total active events at any time: 5,000-20,000
  Total active markets:            500,000-2,000,000
  Total active outcomes:           1,500,000-6,000,000
```

## CORE ENTITIES (Rust structs)

```rust
// ── Sport ──

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Sport {
    pub id: SportId,
    pub name: String,
    pub slug: String,            // "football", "basketball"
    pub display_order: i32,
    pub active: bool,
    pub event_count: i32,        // cached count of active events
    pub icon_url: Option<String>,
}

// ── Event ──

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    pub id: EventId,
    pub external_id: String,     // Sportradar ID
    pub sport_id: SportId,
    pub category_id: CategoryId,
    pub tournament_id: TournamentId,
    pub name: String,            // "Real Madrid vs Barcelona"
    pub home_team: String,
    pub away_team: String,
    pub status: EventStatus,
    pub start_time: DateTime<Utc>,
    pub is_live: bool,
    pub live_data: Option<LiveData>,
    pub market_count: i32,
    pub metadata: serde_json::Value,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type, strum::Display)]
#[sqlx(type_name = "event_status", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
pub enum EventStatus {
    /// Event is scheduled but not yet open for betting
    Scheduled,
    /// Event is open for pre-match betting
    Open,
    /// Event is live (in-play betting available)
    Live,
    /// Event is temporarily suspended (odds being recalculated)
    Suspended,
    /// Event has finished, awaiting result confirmation
    Finished,
    /// Results confirmed, bets can be settled
    Resulted,
    /// Event was cancelled (all bets void)
    Cancelled,
    /// Event was postponed (bets may be void or remain)
    Postponed,
}

impl EventStatus {
    /// Can users place bets on this event?
    pub fn is_bettable(&self) -> bool {
        matches!(self, EventStatus::Open | EventStatus::Live)
    }
    
    /// Can this event be settled?
    pub fn can_settle(&self) -> bool {
        matches!(self, EventStatus::Resulted)
    }
    
    /// Should bets be voided for this status?
    pub fn should_void_bets(&self) -> bool {
        matches!(self, EventStatus::Cancelled)
    }
}

// ── Live Data ──

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiveData {
    pub score_home: i32,
    pub score_away: i32,
    pub period: String,          // "1st Half", "2nd Half", "1st Set"
    pub minute: Option<i32>,     // match minute (football)
    pub game_score: Option<String>, // "40-30" (tennis)
    pub set_scores: Option<Vec<SetScore>>, // tennis sets
    pub stats: Option<serde_json::Value>,
    pub last_update: DateTime<Utc>,
}

// ── Market ──

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Market {
    pub id: MarketId,
    pub external_id: String,
    pub event_id: EventId,
    pub name: String,            // "Match Result", "Over/Under 2.5"
    pub market_type: MarketType,
    pub status: MarketStatus,
    pub line: Option<Decimal>,   // handicap or total line (e.g., 2.5)
    pub outcomes: Vec<Outcome>,
    pub display_order: i32,
    pub is_main: bool,           // main market for the event
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
#[sqlx(type_name = "market_type", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
pub enum MarketType {
    MatchResult,       // 1X2 (home/draw/away)
    Moneyline,         // home/away (no draw)
    Spread,            // handicap/spread
    TotalOverUnder,    // over/under a line
    BothTeamsToScore,  // yes/no
    CorrectScore,      // exact final score
    FirstGoalscorer,   // player markets
    HalfTimeResult,    // 1X2 at half time
    DoubleChance,      // 1X, X2, 12
    DrawNoBet,         // home/away (draw = void)
    AsianHandicap,     // Asian handicap variants
    AsianTotal,        // Asian total variants
    PlayerProps,       // player-specific markets
    Custom,            // custom/special markets
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
#[sqlx(type_name = "market_status", rename_all = "snake_case")]
#[serde(rename_all = "snake_case")]
pub enum MarketStatus {
    /// Market is open for betting
    Open,
    /// Market is temporarily suspended (odds change, dangerous moment)
    Suspended,
    /// Market is closed (no more bets, e.g., event started for pre-match)
    Closed,
    /// Market is resulted (outcome determined)
    Resulted,
    /// Market is settled (all bets paid out)
    Settled,
    /// Market is void (cancelled, rules not met)
    Void,
}

impl MarketStatus {
    pub fn accepts_bets(&self) -> bool {
        matches!(self, MarketStatus::Open)
    }
}

// ── Outcome ──

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Outcome {
    pub id: OutcomeId,
    pub external_id: String,
    pub market_id: MarketId,
    pub name: String,            // "Real Madrid", "Over 2.5", "2-1"
    pub odds: Decimal,           // current decimal odds (e.g., 2.10)
    pub previous_odds: Option<Decimal>, // for showing odds movement
    pub status: OutcomeStatus,
    pub result: Option<OutcomeResult>,
    pub display_order: i32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OutcomeStatus {
    Active,
    Suspended,
    Closed,
    Resulted,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OutcomeResult {
    /// This outcome won
    Win,
    /// This outcome lost
    Loss,
    /// This outcome is void (push, cancelled)
    Void,
    /// Half win (Asian handicap)
    HalfWin,
    /// Half loss (Asian handicap)
    HalfLoss,
    /// Dead heat (reduced payout)
    DeadHeat { reduction_factor: Decimal },
}
```

# ============================================================
# SECTION 2: ODDS
# ============================================================

## ODDS FORMATS

```text
The platform stores and calculates in DECIMAL format internally.
Display conversion happens at the frontend level.

DECIMAL (European):
  Represents total return per unit staked.
  Stake $10 at 2.50 → Return $25.00, Profit $15.00
  Minimum: 1.01 (almost certain)
  Maximum: 1001.00 (extremely unlikely)
  
FRACTIONAL (UK):
  5/2 = decimal 3.50
  1/4 = decimal 1.25
  Conversion: decimal = (numerator/denominator) + 1

AMERICAN (US):
  +200 = decimal 3.00 (positive: underdog)
  -150 = decimal 1.667 (negative: favorite)
  Conversion:
    positive: decimal = (american/100) + 1
    negative: decimal = (100/|american|) + 1
```

## ODDS CONVERSION (Rust implementation)

```rust
use rust_decimal::Decimal;
use rust_decimal_macros::dec;

/// Convert decimal odds to all display formats
pub struct OddsConverter;

impl OddsConverter {
    /// Decimal → Fractional
    /// 2.50 → "3/2"
    /// 1.25 → "1/4"
    pub fn to_fractional(decimal_odds: Decimal) -> String {
        let profit = decimal_odds - Decimal::ONE;
        
        // Find simplest fraction
        let (num, den) = Self::simplify_fraction(profit);
        format!("{}/{}", num, den)
    }
    
    /// Decimal → American
    /// 2.50 → "+150"
    /// 1.50 → "-200"
    pub fn to_american(decimal_odds: Decimal) -> String {
        if decimal_odds >= dec!(2.0) {
            // Positive (underdog)
            let american = (decimal_odds - Decimal::ONE) * dec!(100);
            format!("+{}", american.round_dp(0))
        } else {
            // Negative (favorite)
            let american = dec!(100) / (decimal_odds - Decimal::ONE);
            format!("-{}", american.round_dp(0))
        }
    }
    
    /// Fractional → Decimal
    pub fn from_fractional(numerator: i64, denominator: i64) -> Decimal {
        Decimal::new(numerator, 0) / Decimal::new(denominator, 0) + Decimal::ONE
    }
    
    /// American → Decimal
    pub fn from_american(american: i64) -> Decimal {
        if american > 0 {
            Decimal::new(american, 0) / dec!(100) + Decimal::ONE
        } else {
            dec!(100) / Decimal::new(american.abs(), 0) + Decimal::ONE
        }
    }
    
    fn simplify_fraction(decimal: Decimal) -> (i64, i64) {
        // Convert to integer fraction with precision
        let multiplied = (decimal * dec!(100)).to_i64().unwrap_or(0);
        let denominator: i64 = 100;
        let gcd = Self::gcd(multiplied.unsigned_abs(), denominator as u64) as i64;
        (multiplied / gcd, denominator / gcd)
    }
    
    fn gcd(a: u64, b: u64) -> u64 {
        if b == 0 { a } else { Self::gcd(b, a % b) }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_decimal_to_american() {
        assert_eq!(OddsConverter::to_american(dec!(2.50)), "+150");
        assert_eq!(OddsConverter::to_american(dec!(1.50)), "-200");
        assert_eq!(OddsConverter::to_american(dec!(2.00)), "+100");
        assert_eq!(OddsConverter::to_american(dec!(3.00)), "+200");
        assert_eq!(OddsConverter::to_american(dec!(1.25)), "-400");
    }
    
    #[test]
    fn test_american_to_decimal() {
        assert_eq!(OddsConverter::from_american(150), dec!(2.50));
        assert_eq!(OddsConverter::from_american(-200), dec!(1.50));
        assert_eq!(OddsConverter::from_american(100), dec!(2.00));
    }
}
```

## MARGIN CALCULATION

```text
WHAT IS MARGIN (overround / vig / juice):

Fair odds:
  Coin flip: Head 2.00, Tail 2.00
  Implied probability: 50% + 50% = 100%

With 5% margin:
  Head 1.909, Tail 1.909
  Implied probability: 52.38% + 52.38% = 104.76%
  Margin = 104.76% - 100% = 4.76%

The margin is how the platform makes money.
Higher margin = more profit per bet, but less competitive odds.
```

```rust
/// Calculate the margin (overround) for a set of odds.
/// 
/// margin = (sum of implied probabilities) - 1.0
/// 
/// Example: odds [2.10, 3.40, 3.20] (1X2 football)
/// implied = 1/2.10 + 1/3.40 + 1/3.20 = 0.4762 + 0.2941 + 0.3125 = 1.0828
/// margin = 1.0828 - 1.0 = 0.0828 = 8.28%
pub fn calculate_margin(odds: &[Decimal]) -> Decimal {
    let implied_sum: Decimal = odds.iter()
        .map(|o| Decimal::ONE / *o)
        .sum();
    
    implied_sum - Decimal::ONE
}

/// Apply margin to fair odds.
/// 
/// Given fair probability and target margin, calculate display odds.
/// 
/// fair_odds = 1 / fair_probability
/// display_odds = fair_odds / (1 + margin)
/// 
/// But we use proportional margin application:
/// display_odds = fair_odds * (1 - margin_per_outcome)
pub fn apply_margin(fair_odds: &[Decimal], target_margin: Decimal) -> Vec<Decimal> {
    let n = Decimal::new(fair_odds.len() as i64, 0);
    
    // Proportional margin: each outcome gets proportional share
    fair_odds.iter().map(|&fair| {
        let implied_prob = Decimal::ONE / fair;
        let adjusted_prob = implied_prob * (Decimal::ONE + target_margin);
        let display_odds = Decimal::ONE / adjusted_prob;
        
        // Round to 2 decimal places, minimum 1.01
        display_odds.round_dp(2).max(dec!(1.01))
    }).collect()
}

/// Remove margin from displayed odds to get fair odds.
/// Used for cashout calculation.
pub fn remove_margin(displayed_odds: &[Decimal]) -> Vec<Decimal> {
    let margin = calculate_margin(displayed_odds);
    let adjustment = Decimal::ONE + margin;
    
    displayed_odds.iter().map(|&odds| {
        let implied = Decimal::ONE / odds;
        let fair_implied = implied / adjustment * Decimal::ONE;
        (Decimal::ONE / fair_implied).round_dp(2)
    }).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_margin_calculation() {
        let odds = vec![dec!(2.10), dec!(3.40), dec!(3.20)];
        let margin = calculate_margin(&odds);
        
        // Expected: ~8.28%
        assert!(margin > dec!(0.08));
        assert!(margin < dec!(0.09));
    }
    
    #[test]
    fn test_fair_odds_no_margin() {
        let fair_odds = vec![dec!(2.00), dec!(2.00)]; // coin flip
        let margin = calculate_margin(&fair_odds);
        
        assert_eq!(margin, dec!(0));
    }
    
    #[test]
    fn test_apply_margin() {
        let fair_odds = vec![dec!(2.00), dec!(2.00)];
        let display_odds = apply_margin(&fair_odds, dec!(0.05)); // 5% margin
        
        // Both should be less than 2.00
        assert!(display_odds[0] < dec!(2.00));
        assert!(display_odds[1] < dec!(2.00));
        
        // Margin should be ~5%
        let actual_margin = calculate_margin(&display_odds);
        assert!((actual_margin - dec!(0.05)).abs() < dec!(0.01));
    }
}
```

# ============================================================
# SECTION 3: BET TYPES AND CALCULATIONS
# ============================================================

## BET TYPES

```text
SINGLE BET:
  One selection, one stake.
  Payout = stake × odds
  Example: $10 on Real Madrid @2.50 → win $25.00

ACCUMULATOR (PARLAY/COMBO):
  Multiple selections, all must win.
  Combined odds = odds₁ × odds₂ × ... × oddsₙ
  Payout = stake × combined_odds
  Example: $10 on 3 selections @1.80, @2.20, @1.50
           Combined = 1.80 × 2.20 × 1.50 = 5.94
           Win = $10 × 5.94 = $59.40
  
  Rules:
  - Minimum 2 selections, maximum 20
  - Selections must be from DIFFERENT events
  - If one selection loses → entire bet loses
  - If one selection is void → odds become 1.00 (removed from acca)
  - Related contingencies blocked (e.g., same match, correlated markets)

SYSTEM BET:
  Multiple accumulators from a set of selections.
  Example: System 2/3 from selections A, B, C:
    Generates 3 doubles: AB, AC, BC
    Stake per combo: total_stake / number_of_combos
    Some combos can win even if not all selections win.
  
  Common systems:
    2/3 (Trixie without singles): 3 doubles + 1 treble = 4 bets
    2/4 (Yankee): 6 doubles + 4 trebles + 1 fourfold = 11 bets
    Trixie: 3 doubles + 1 treble = 4 bets
    Patent: 3 singles + 3 doubles + 1 treble = 7 bets
    Yankee: 6 doubles + 4 trebles + 1 fourfold = 11 bets
    Lucky 15: 4 singles + 6 doubles + 4 trebles + 1 fourfold = 15 bets
```

## BET CALCULATION ENGINE

```rust
use rust_decimal::Decimal;
use rust_decimal_macros::dec;
use itertools::Itertools;

/// Calculate payout for different bet types.
pub struct BetCalculator;

impl BetCalculator {
    /// Single bet payout.
    pub fn single_payout(stake: Decimal, odds: Decimal) -> Decimal {
        stake * odds
    }
    
    /// Accumulator combined odds.
    /// Multiply all individual odds together.
    pub fn accumulator_odds(selection_odds: &[Decimal]) -> Decimal {
        selection_odds.iter()
            .fold(Decimal::ONE, |acc, &odds| acc * odds)
    }
    
    /// Accumulator payout.
    pub fn accumulator_payout(stake: Decimal, selection_odds: &[Decimal]) -> Decimal {
        stake * Self::accumulator_odds(selection_odds)
    }
    
    /// System bet: generate all combinations and calculate.
    /// 
    /// system_type: minimum selections that must win (e.g., 2 for "2/3")
    /// selections: number of total selections
    /// Returns total number of individual bets in the system.
    pub fn system_bet_count(system_type: usize, total_selections: usize) -> usize {
        // Number of combinations = C(n, k) for k from system_type to total_selections
        (system_type..=total_selections)
            .map(|k| Self::combinations(total_selections, k))
            .sum()
    }
    
    /// Calculate system bet payout.
    /// 
    /// stake_per_bet: stake for each individual combination
    /// selection_odds: odds for each selection
    /// selection_results: whether each selection won (true) or lost (false)
    /// system_type: minimum fold (e.g., 2 for doubles)
    pub fn system_payout(
        stake_per_bet: Decimal,
        selection_odds: &[Decimal],
        selection_results: &[SelectionOutcome],
        system_type: usize,
    ) -> Decimal {
        let n = selection_odds.len();
        let mut total_payout = Decimal::ZERO;
        
        // Generate all combinations of size system_type to n
        for size in system_type..=n {
            for combo in (0..n).combinations(size) {
                // Check if all selections in this combo won
                let all_won = combo.iter().all(|&i| {
                    matches!(selection_results[i], SelectionOutcome::Won)
                });
                
                if all_won {
                    // Calculate payout for this combination
                    let combo_odds: Decimal = combo.iter()
                        .map(|&i| {
                            match selection_results[i] {
                                SelectionOutcome::Won => selection_odds[i],
                                SelectionOutcome::Void => Decimal::ONE,
                                SelectionOutcome::HalfWon => {
                                    // Half stake wins at full odds, half returned
                                    (selection_odds[i] + Decimal::ONE) / dec!(2)
                                }
                                _ => Decimal::ZERO,
                            }
                        })
                        .fold(Decimal::ONE, |acc, odds| acc * odds);
                    
                    total_payout += stake_per_bet * combo_odds;
                }
            }
        }
        
        total_payout
    }
    
    /// Handle void selections in accumulators.
    /// Void selection odds become 1.00 (effectively removed).
    pub fn accumulator_with_voids(
        stake: Decimal,
        selection_odds: &[Decimal],
        selection_results: &[SelectionOutcome],
    ) -> Decimal {
        let effective_odds: Decimal = selection_odds.iter()
            .zip(selection_results.iter())
            .map(|(&odds, result)| {
                match result {
                    SelectionOutcome::Won => odds,
                    SelectionOutcome::Void => Decimal::ONE, // void = removed
                    SelectionOutcome::HalfWon => (odds + Decimal::ONE) / dec!(2),
                    SelectionOutcome::HalfLost => Decimal::ONE / dec!(2),
                    SelectionOutcome::Lost => Decimal::ZERO,
                    SelectionOutcome::DeadHeat { factor } => {
                        Decimal::ONE + (odds - Decimal::ONE) * factor
                    }
                }
            })
            .fold(Decimal::ONE, |acc, odds| acc * odds);
        
        stake * effective_odds
    }
    
    /// Calculate C(n, k) — binomial coefficient.
    fn combinations(n: usize, k: usize) -> usize {
        if k > n { return 0; }
        if k == 0 || k == n { return 1; }
        let k = k.min(n - k); // optimization: C(n,k) = C(n,n-k)
        let mut result: usize = 1;
        for i in 0..k {
            result = result * (n - i) / (i + 1);
        }
        result
    }
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum SelectionOutcome {
    /// Selection won — full payout at odds
    Won,
    /// Selection lost — no payout
    Lost,
    /// Selection void — odds become 1.00
    Void,
    /// Half won — Asian handicap push (half stake wins, half returned)
    HalfWon,
    /// Half lost — Asian handicap push (half stake lost, half returned)
    HalfLost,
    /// Dead heat — reduced payout by factor
    DeadHeat { factor: Decimal },
    /// Not yet settled
    Pending,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_single_payout() {
        assert_eq!(
            BetCalculator::single_payout(dec!(10), dec!(2.50)),
            dec!(25.00)
        );
    }
    
    #[test]
    fn test_accumulator_odds() {
        let odds = vec![dec!(1.80), dec!(2.20), dec!(1.50)];
        let combined = BetCalculator::accumulator_odds(&odds);
        assert_eq!(combined, dec!(5.940));
    }
    
    #[test]
    fn test_accumulator_with_one_void() {
        let stake = dec!(10);
        let odds = vec![dec!(1.80), dec!(2.20), dec!(1.50)];
        let results = vec![
            SelectionOutcome::Won,
            SelectionOutcome::Void, // this one voided → odds = 1.00
            SelectionOutcome::Won,
        ];
        
        let payout = BetCalculator::accumulator_with_voids(stake, &odds, &results);
        // Effective odds: 1.80 × 1.00 × 1.50 = 2.70
        assert_eq!(payout, dec!(27.000));
    }
    
    #[test]
    fn test_accumulator_with_one_loss() {
        let stake = dec!(10);
        let odds = vec![dec!(1.80), dec!(2.20), dec!(1.50)];
        let results = vec![
            SelectionOutcome::Won,
            SelectionOutcome::Lost,
            SelectionOutcome::Won,
        ];
        
        let payout = BetCalculator::accumulator_with_voids(stake, &odds, &results);
        // One loss → entire acca loses
        assert_eq!(payout, dec!(0));
    }
    
    #[test]
    fn test_system_bet_count() {
        // System 2/3 = C(3,2) + C(3,3) = 3 + 1 = 4
        assert_eq!(BetCalculator::system_bet_count(2, 3), 4);
        
        // Yankee = System 2/4 = C(4,2)+C(4,3)+C(4,4) = 6+4+1 = 11
        assert_eq!(BetCalculator::system_bet_count(2, 4), 11);
    }
    
    #[test]
    fn test_dead_heat() {
        let stake = dec!(10);
        let odds = vec![dec!(3.00)];
        let results = vec![
            SelectionOutcome::DeadHeat { factor: dec!(0.5) }, // 2-way dead heat
        ];
        
        let payout = BetCalculator::accumulator_with_voids(stake, &odds, &results);
        // Dead heat: 1 + (3.00 - 1) × 0.5 = 1 + 1.0 = 2.0
        assert_eq!(payout, dec!(20.0));
    }
}
```

# ============================================================
# SECTION 4: BET LIFECYCLE (STATE MACHINE)
# ============================================================

```text
BET STATE MACHINE:

                    ┌──────────┐
          ┌────────│ REJECTED  │
          │         └──────────┘
          │ (risk check
          │  failed)
          │
    ┌─────────┐          ┌──────────┐
    │ PENDING │─────────▶│  ACTIVE  │
    └─────────┘          └──────────┘
     (bet placed,         (funds locked,│
      validating)          waiting for  │
                           result)      │
                                        │
              ┌─────────────────────────┼──────────────────┐
              │                         │                  │
              ▼                         ▼                  ▼
        ┌──────────┐           ┌──────────┐        ┌──────────┐
        │   WON    │           │   LOST   │        │   VOID   │
        └──────────┘           └──────────┘        └──────────┘
        (credit user)          (release lock,      (refund stake)
                                no payout)
              │
              ▼ (partial, before settlement)
        ┌──────────┐
        │ CASHOUT  │
        └──────────┘
        (partial payout,
         close bet early)

STATE TRANSITIONS:
  pending  → active    : validation passed, funds locked
  pending  → rejected  : validation failed (risk, odds, balance)
  active   → won       : all selections won (or enough for system)
  active   → lost      : one+ selection lost (single/acca) or not enough (system)
  active   → void      : event cancelled, market voided
  active   → cashout   : user requested cashout, accepted

TERMINAL STATES: won, lost, void, cashout, rejected
  Once in terminal state, bet CANNOT change.

SETTLEMENT PRIORITY:
  void > won/lost (if event cancelled, always void regardless of score)
```

```rust
/// Bet state machine with validated transitions.
impl BetStatus {
    /// Check if this is a terminal (final) state.
    pub fn is_terminal(&self) -> bool {
        matches!(
            self,
            BetStatus::Won | BetStatus::Lost | BetStatus::Void 
            | BetStatus::Cashout | BetStatus::Rejected
        )
    }
    
    /// Check if bet can be cashed out.
    pub fn can_cashout(&self) -> bool {
        matches!(self, BetStatus::Active)
    }
    
    /// Check if bet is awaiting settlement.
    pub fn awaits_settlement(&self) -> bool {
        matches!(self, BetStatus::Active)
    }
    
    /// Validate state transition and return error if invalid.
    pub fn transition_to(&self, target: BetStatus) -> Result<BetStatus, BetError> {
        if self.can_transition_to(target) {
            Ok(target)
        } else {
            Err(BetError::InvalidTransition {
                from: *self,
                to: target,
            })
        }
    }
    
    pub fn can_transition_to(&self, target: BetStatus) -> bool {
        matches!(
            (*self, target),
            (BetStatus::Pending, BetStatus::Active)
                | (BetStatus::Pending, BetStatus::Rejected)
                | (BetStatus::Active, BetStatus::Won)
                | (BetStatus::Active, BetStatus::Lost)
                | (BetStatus::Active, BetStatus::Void)
                | (BetStatus::Active, BetStatus::Cashout)
        )
    }
}
```

# ============================================================
# SECTION 5: SETTLEMENT ENGINE
# ============================================================

```text
SETTLEMENT FLOW:

1. Sportradar sends result for an event
2. Odds Feed Service receives result, publishes events.resulted
3. Settlement Engine picks up the event:
   a. Get all markets for this event
   b. For each market, determine outcome results (win/loss/void)
   c. Find all unsettled bets that include selections from this event
   d. For SINGLE bets on this event: settle immediately
   e. For ACCUMULATOR bets: check if ALL selections are now settled
      - If yes → calculate final result
      - If no → mark this selection as settled, wait for others
   f. For each settled bet:
      - Calculate payout
      - Call Wallet Service to settle (unlock + credit/debit)
      - Update bet status
      - Publish bets.settled event
      - Notify user (if win > threshold)

SETTLEMENT RULES:
  - Settlement is IDEMPOTENT (same event settled twice = no double payout)
  - Settlement processes in BATCHES (per event, max 1000 bets per batch)
  - Bets are settled in PARALLEL (independent bets on same event)
  - Settlement can be REVERSED only by creating a "void + resettle"
  - All settlements are AUDITED (immutable log)
```

```rust
/// Settlement engine — determines bet outcomes from event results.
pub struct SettlementEngine;

impl SettlementEngine {
    /// Settle all bets for a given event.
    /// 
    /// This is the main entry point called when an event is resulted.
    pub async fn settle_event(
        &self,
        event_id: EventId,
        market_results: &[MarketResult],
        bet_repo: &BetRepository,
        wallet_client: &WalletClient,
        producer: &Producer,
    ) -> Result<SettlementReport, SettlementError> {
        let mut report = SettlementReport::new(event_id);
        
        // 1. Get all active bets with selections on this event
        let bets = bet_repo.get_bets_for_event(event_id, BetStatus::Active).await?;
        
        tracing::info!(
            event_id = %event_id,
            bet_count = bets.len(),
            "Starting settlement"
        );
        
        // 2. Process each bet
        for bet in &bets {
            match self.settle_single_bet(bet, market_results, bet_repo, wallet_client).await {
                Ok(result) => {
                    report.add_success(bet.id, result);
                    
                    // Publish event
                    let _ = producer.publish(
                        "bets.settled",
                        &bet.user_id.to_string(),
                        &BetSettledEvent {
                            bet_id: bet.id,
                            user_id: bet.user_id,
                            result: result.status,
                            payout: result.payout,
                            settled_at: Utc::now(),
                        },
                    ).await;
                }
                Err(e) => {
                    tracing::error!(bet_id = %bet.id, error = %e, "Settlement failed");
                    report.add_failure(bet.id, e.to_string());
                }
            }
        }
        
        tracing::info!(
            event_id = %event_id,
            settled = report.success_count,
            failed = report.failure_count,
            total_payout = %report.total_payout,
            "Settlement completed"
        );
        
        Ok(report)
    }
    
    /// Settle a single bet based on market results.
    async fn settle_single_bet(
        &self,
        bet: &Bet,
        market_results: &[MarketResult],
        bet_repo: &BetRepository,
        wallet_client: &WalletClient,
    ) -> Result<BetSettlementResult, SettlementError> {
        // 1. Determine outcome for each selection
        let mut selection_outcomes = Vec::new();
        let mut all_selections_settled = true;
        
        for selection in &bet.selections {
            let market_result = market_results.iter()
                .find(|mr| mr.market_id == selection.market_id);
            
            match market_result {
                Some(mr) => {
                    let outcome_result = mr.outcome_results.iter()
                        .find(|or| or.outcome_id == selection.outcome_id)
                        .map(|or| or.result)
                        .unwrap_or(SelectionOutcome::Void);
                    
                    selection_outcomes.push(outcome_result);
                }
                None => {
                    // Market not yet resulted (different event in accumulator)
                    all_selections_settled = false;
                    selection_outcomes.push(SelectionOutcome::Pending);
                }
            }
        }
        
        // 2. For accumulators, only settle when ALL selections are determined
        if !all_selections_settled {
            // Update individual selection results but don't settle the bet yet
            bet_repo.update_selection_results(bet.id, &selection_outcomes).await?;
            return Ok(BetSettlementResult {
                status: BetStatus::Active, // still active
                payout: Decimal::ZERO,
                partial: true,
            });
        }
        
        // 3. Calculate final result
        let (final_status, payout) = self.calculate_bet_result(
            bet.bet_type,
            bet.stake,
            &bet.selection_odds(),
            &selection_outcomes,
        );
        
        // 4. Settle in wallet
        wallet_client.settle(WalletSettleRequest {
            lock_id: bet.lock_id,
            settlement_amount: payout,
            idempotency_key: format!("settle_{}_{}", bet.id, Uuid::new_v4()),
        }).await?;
        
        // 5. Update bet status in database
        let mut tx = bet_repo.begin_tx().await?;
        bet_repo.update_bet_status(
            &mut tx, bet.id, BetStatus::Active, final_status, Some(payout)
        ).await?;
        bet_repo.update_selection_results_tx(&mut tx, bet.id, &selection_outcomes).await?;
        tx.commit().await?;
        
        Ok(BetSettlementResult {
            status: final_status,
            payout,
            partial: false,
        })
    }
    
    /// Calculate bet result from selection outcomes.
    fn calculate_bet_result(
        &self,
        bet_type: BetType,
        stake: Decimal,
        selection_odds: &[Decimal],
        outcomes: &[SelectionOutcome],
    ) -> (BetStatus, Decimal) {
        match bet_type {
            BetType::Single => {
                self.calculate_single_result(stake, selection_odds[0], outcomes[0])
            }
            BetType::Accumulator => {
                self.calculate_accumulator_result(stake, selection_odds, outcomes)
            }
            BetType::System => {
                // System bets use BetCalculator::system_payout
                todo!("System bet settlement")
            }
        }
    }
    
    fn calculate_single_result(
        &self,
        stake: Decimal,
        odds: Decimal,
        outcome: SelectionOutcome,
    ) -> (BetStatus, Decimal) {
        match outcome {
            SelectionOutcome::Won => {
                (BetStatus::Won, stake * odds)
            }
            SelectionOutcome::Lost => {
                (BetStatus::Lost, Decimal::ZERO)
            }
            SelectionOutcome::Void => {
                (BetStatus::Void, stake) // return stake
            }
            SelectionOutcome::HalfWon => {
                // Half stake wins at full odds, half stake returned
                let half = stake / dec!(2);
                let payout = half * odds + half;
                (BetStatus::Won, payout)
            }
            SelectionOutcome::HalfLost => {
                // Half stake lost, half returned
                let half = stake / dec!(2);
                (BetStatus::Lost, half)
            }
            SelectionOutcome::DeadHeat { factor } => {
                let payout = stake * (Decimal::ONE + (odds - Decimal::ONE) * factor);
                (BetStatus::Won, payout)
            }
            SelectionOutcome::Pending => unreachable!("Single bet should not be pending"),
        }
    }
    
    fn calculate_accumulator_result(
        &self,
        stake: Decimal,
        odds: &[Decimal],
        outcomes: &[SelectionOutcome],
    ) -> (BetStatus, Decimal) {
        // Check for any loss
        let has_loss = outcomes.iter().any(|o| matches!(o, SelectionOutcome::Lost));
        if has_loss {
            return (BetStatus::Lost, Decimal::ZERO);
        }
        
        // Check if all void
        let all_void = outcomes.iter().all(|o| matches!(o, SelectionOutcome::Void));
        if all_void {
            return (BetStatus::Void, stake);
        }
        
        // Calculate payout with voids and special outcomes
        let payout = BetCalculator::accumulator_with_voids(stake, odds, outcomes);
        
        if payout > Decimal::ZERO {
            (BetStatus::Won, payout)
        } else {
            (BetStatus::Lost, Decimal::ZERO)
        }
    }
}
```

# ============================================================
# SECTION 6: CASHOUT
# ============================================================

```text
CASHOUT allows users to settle a bet early, before all events finish.

CASHOUT CALCULATION:
  cashout_value = stake × (current_odds_achieved / original_odds) × (1 - cashout_margin)

  Where:
  - current_odds_achieved: based on how the bet is performing
  - original_odds: odds at bet placement
  - cashout_margin: 5-10% (platform profit on cashout)

EXAMPLES:
  Bet: $10 on Team A @3.00 (potential win: $30)
  Team A is winning 1-0 at half time
  New odds for Team A: 1.40 (they're likely to win)
  
  Fair cashout = $10 × 3.00 / 1.40 = $21.43
  With 5% margin = $21.43 × 0.95 = $20.36
  
  User takes $20.36 instead of risking the full $30 or $0.

RULES:
  - Cashout available ONLY for active bets
  - Cashout value recalculated every few seconds (live) or minutes (pre-match)
  - Value can change between display and confirmation (race condition!)
  - Partial cashout: cash out a portion of the bet
  - Auto-cashout: set a threshold, system cashes out automatically
  - NOT available for: free bets, bonus bets, certain markets
```

```rust
/// Cashout calculator.
pub struct CashoutCalculator {
    cashout_margin: Decimal, // e.g., 0.05 (5%)
}

impl CashoutCalculator {
    pub fn new(cashout_margin: Decimal) -> Self {
        Self { cashout_margin }
    }
    
    /// Calculate cashout value for a single bet.
    pub fn calculate_single(
        &self,
        stake: Decimal,
        original_odds: Decimal,
        current_odds: Decimal,
    ) -> Option<Decimal> {
        // Can't cash out if current odds are zero or negative
        if current_odds <= Decimal::ZERO {
            return None;
        }
        
        let fair_value = stake * original_odds / current_odds;
        let cashout_value = fair_value * (Decimal::ONE - self.cashout_margin);
        
        // Minimum cashout is $0.10
        if cashout_value < dec!(0.10) {
            return None;
        }
        
        // Cashout can't exceed potential win
        let max_value = stake * original_odds;
        Some(cashout_value.min(max_value).round_dp(2))
    }
    
    /// Calculate cashout value for an accumulator.
    /// 
    /// For accumulators with mixed settled/unsettled selections:
    /// - Settled winning selections: use their odds (already achieved)
    /// - Unsettled selections: use current live/pre-match odds
    /// - Settled losing selections: cashout impossible
    pub fn calculate_accumulator(
        &self,
        stake: Decimal,
        selections: &[CashoutSelection],
    ) -> Option<Decimal> {
        let mut achieved_odds = Decimal::ONE;
        let mut remaining_odds = Decimal::ONE;
        let mut original_remaining_odds = Decimal::ONE;
        
        for sel in selections {
            match sel.status {
                CashoutSelectionStatus::Won => {
                    achieved_odds *= sel.original_odds;
                }
                CashoutSelectionStatus::Lost => {
                    return None; // can't cash out lost acca
                }
                CashoutSelectionStatus::Void => {
                    // Void selection = odds 1.00, doesn't affect calculation
                }
                CashoutSelectionStatus::Pending => {
                    remaining_odds *= sel.current_odds;
                    original_remaining_odds *= sel.original_odds;
                }
            }
        }
        
        if remaining_odds <= Decimal::ZERO {
            return None;
        }
        
        // Cashout = stake × achieved_odds × (original_remaining / current_remaining)
        // With margin applied
        let fair_value = stake * achieved_odds * original_remaining_odds / remaining_odds;
        let cashout_value = fair_value * (Decimal::ONE - self.cashout_margin);
        
        if cashout_value < dec!(0.10) {
            return None;
        }
        
        Some(cashout_value.round_dp(2))
    }
    
    /// Validate that cashout value hasn't changed too much since display.
    /// Prevents stale cashout acceptance.
    pub fn validate_cashout_value(
        &self,
        displayed_value: Decimal,
        current_value: Decimal,
        tolerance: Decimal, // e.g., 0.02 (2%)
    ) -> CashoutValidation {
        let diff = ((current_value - displayed_value) / displayed_value).abs();
        
        if diff <= tolerance {
            CashoutValidation::Accepted { value: current_value }
        } else if current_value > displayed_value {
            CashoutValidation::IncreasedValue { new_value: current_value }
        } else {
            CashoutValidation::DecreasedValue { new_value: current_value }
        }
    }
}

#[derive(Debug)]
pub enum CashoutValidation {
    /// Cashout accepted at this value
    Accepted { value: Decimal },
    /// Value increased — user gets more (auto-accept)
    IncreasedValue { new_value: Decimal },
    /// Value decreased — ask user to confirm new value
    DecreasedValue { new_value: Decimal },
}

#[derive(Debug)]
pub struct CashoutSelection {
    pub original_odds: Decimal,
    pub current_odds: Decimal,
    pub status: CashoutSelectionStatus,
}

#[derive(Debug)]
pub enum CashoutSelectionStatus {
    Won,
    Lost,
    Void,
    Pending,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_single_cashout_favorable() {
        let calc = CashoutCalculator::new(dec!(0.05));
        
        // Bet on Team A @3.00, now Team A winning, odds dropped to 1.40
        let cashout = calc.calculate_single(dec!(10), dec!(3.00), dec!(1.40));
        
        assert!(cashout.is_some());
        let value = cashout.unwrap();
        // Fair: 10 × 3.00 / 1.40 = 21.43
        // With 5% margin: 21.43 × 0.95 = 20.36
        assert!(value > dec!(20) && value < dec!(21));
    }
    
    #[test]
    fn test_single_cashout_unfavorable() {
        let calc = CashoutCalculator::new(dec!(0.05));
        
        // Bet on Team A @2.00, Team A losing, odds increased to 5.00
        let cashout = calc.calculate_single(dec!(10), dec!(2.00), dec!(5.00));
        
        assert!(cashout.is_some());
        let value = cashout.unwrap();
        // Fair: 10 × 2.00 / 5.00 = 4.00
        // With 5% margin: 4.00 × 0.95 = 3.80
        assert!(value > dec!(3.5) && value < dec!(4.0));
    }
    
    #[test]
    fn test_accumulator_cashout_partial_settled() {
        let calc = CashoutCalculator::new(dec!(0.05));
        
        let selections = vec![
            CashoutSelection {
                original_odds: dec!(1.80),
                current_odds: dec!(1.80),
                status: CashoutSelectionStatus::Won, // already won
            },
            CashoutSelection {
                original_odds: dec!(2.20),
                current_odds: dec!(1.50), // odds dropped (favorable)
                status: CashoutSelectionStatus::Pending,
            },
            CashoutSelection {
                original_odds: dec!(1.50),
                current_odds: dec!(1.50),
                status: CashoutSelectionStatus::Pending,
            },
        ];
        
        let cashout = calc.calculate_accumulator(dec!(10), &selections);
        assert!(cashout.is_some());
        let value = cashout.unwrap();
        assert!(value > dec!(10)); // should be profitable
    }
    
    #[test]
    fn test_cashout_impossible_with_loss() {
        let calc = CashoutCalculator::new(dec!(0.05));
        
        let selections = vec![
            CashoutSelection {
                original_odds: dec!(1.80),
                current_odds: dec!(1.80),
                status: CashoutSelectionStatus::Won,
            },
            CashoutSelection {
                original_odds: dec!(2.20),
                current_odds: dec!(2.20),
                status: CashoutSelectionStatus::Lost, // lost!
            },
        ];
        
        let cashout = calc.calculate_accumulator(dec!(10), &selections);
        assert!(cashout.is_none()); // can't cash out
    }
}
```

# ============================================================
# SECTION 7: LIABILITY MANAGEMENT
# ============================================================

```text
LIABILITY = the maximum amount the platform could pay out.

For each outcome:
  liability = SUM(potential_payout) for all bets on that outcome - SUM(stakes on other outcomes)

SIMPLIFIED per-outcome:
  exposure = SUM(stake × odds) for all bets on this outcome
  
The risk team monitors liability per:
  - Individual outcome
  - Market (all outcomes combined)
  - Event (all markets combined)
  - Sport (aggregate)
  - User (individual user exposure)

AUTOMATIC ACTIONS:
  Liability > 80% of max:
    → Reduce max stake for this market
    → Alert risk team
  
  Liability > 95% of max:
    → Suspend market
    → Alert risk team (P1)
  
  Sharp bettor detected (consistently profitable):
    → Lower individual limits
    → Enhanced monitoring
```

```rust
use dashmap::DashMap;
use std::sync::Arc;

/// Real-time liability tracker.
/// Uses DashMap for lock-free concurrent access.
pub struct LiabilityTracker {
    /// outcome_id → current exposure (sum of potential payouts)
    outcome_exposure: Arc<DashMap<OutcomeId, Decimal>>,
    
    /// event_id → total event liability
    event_liability: Arc<DashMap<EventId, Decimal>>,
    
    /// Configuration
    max_payout_per_event: Decimal,
    max_payout_per_outcome: Decimal,
    alert_threshold: Decimal, // e.g., 0.80 (80%)
}

impl LiabilityTracker {
    pub fn new(config: &LiabilityConfig) -> Self {
        Self {
            outcome_exposure: Arc::new(DashMap::new()),
            event_liability: Arc::new(DashMap::new()),
            max_payout_per_event: config.max_payout_per_event,
            max_payout_per_outcome: config.max_payout_per_outcome,
            alert_threshold: config.alert_threshold,
        }
    }
    
    /// Check if a bet can be accepted based on liability.
    pub fn check_bet(&self, bet: &ProposedBet) -> LiabilityCheck {
        for selection in &bet.selections {
            let current = self.outcome_exposure
                .get(&selection.outcome_id)
                .map(|v| *v)
                .unwrap_or(Decimal::ZERO);
            
            let new_exposure = current + bet.stake * selection.odds;
            
            if new_exposure > self.max_payout_per_outcome {
                return LiabilityCheck::Rejected {
                    reason: format!(
                        "Outcome {} exceeds max liability", 
                        selection.outcome_id
                    ),
                    max_allowed_stake: self.calculate_max_stake(
                        current, selection.odds
                    ),
                };
            }
            
            let ratio = new_exposure / self.max_payout_per_outcome;
            if ratio > self.alert_threshold {
                tracing::warn!(
                    outcome_id = %selection.outcome_id,
                    exposure = %new_exposure,
                    max = %self.max_payout_per_outcome,
                    ratio = %ratio,
                    "Liability threshold breached"
                );
            }
        }
        
        LiabilityCheck::Accepted
    }
    
    /// Record a bet's liability after acceptance.
    pub fn record_bet(&self, bet: &Bet) {
        for selection in &bet.selections {
            self.outcome_exposure
                .entry(selection.outcome_id)
                .and_modify(|v| *v += bet.stake * selection.odds)
                .or_insert(bet.stake * selection.odds);
        }
        
        self.event_liability
            .entry(bet.event_id())
            .and_modify(|v| *v += bet.potential_win)
            .or_insert(bet.potential_win);
    }
    
    /// Release liability when bet is settled or voided.
    pub fn release_bet(&self, bet: &Bet) {
        for selection in &bet.selections {
            self.outcome_exposure
                .entry(selection.outcome_id)
                .and_modify(|v| *v = (*v - bet.stake * selection.odds).max(Decimal::ZERO));
        }
        
        self.event_liability
            .entry(bet.event_id())
            .and_modify(|v| *v = (*v - bet.potential_win).max(Decimal::ZERO));
    }
    
    fn calculate_max_stake(&self, current_exposure: Decimal, odds: Decimal) -> Decimal {
        let remaining = self.max_payout_per_outcome - current_exposure;
        if remaining <= Decimal::ZERO {
            return Decimal::ZERO;
        }
        (remaining / odds).round_dp(2)
    }
}

#[derive(Debug)]
pub enum LiabilityCheck {
    Accepted,
    Rejected {
        reason: String,
        max_allowed_stake: Decimal,
    },
}
```

# ============================================================
# SECTION 8: LIVE BETTING SPECIFICS
# ============================================================

```text
LIVE BETTING differs from pre-match in several key ways:

1. ODDS CHANGE RAPIDLY
   - Updates every 1-5 seconds
   - Must handle stale odds gracefully
   - User sees odds that may be outdated by the time bet is placed

2. MARKET SUSPENSIONS
   - Markets suspend during "dangerous moments" (goal scored, penalty, red card)
   - Suspension lasts 10-60 seconds
   - Any bet placed during suspension MUST be rejected
   - Suspension status must be checked in real-time (from cache, not DB)

3. BET ACCEPTANCE DELAY
   - Live bets may have a deliberate delay (5-10 seconds)
   - During this delay, if odds change significantly → reject
   - Delay helps prevent exploitation of broadcast delay

4. ODDS ACCEPTANCE MODES
   - "No changes": reject if any odds changed
   - "Accept higher": accept only if odds went UP (user gets better deal)
   - "Accept any": accept any odds change (user's choice)

5. EVENT CLOCK
   - Need to track match time for various markets
   - Some markets close at certain times (e.g., "First Half Result" at half time)

6. FASTER SETTLEMENT
   - Some markets settle during the event (e.g., "Next Goal" market)
   - Must process settlements quickly while event is ongoing
```

```rust
/// Live bet acceptance with delay and odds validation.
pub struct LiveBetProcessor {
    acceptance_delay: Duration,
    max_odds_drift: Decimal, // maximum % change allowed
}

impl LiveBetProcessor {
    /// Process a live bet with acceptance delay.
    pub async fn process_live_bet(
        &self,
        bet_request: &PlaceBetRequest,
        odds_cache: &OddsCache,
    ) -> Result<LiveBetDecision, BetError> {
        // 1. Check market is open and not suspended
        for selection in &bet_request.selections {
            let market_status = odds_cache
                .get_market_status(selection.market_id)
                .await?;
            
            if market_status != MarketStatus::Open {
                return Ok(LiveBetDecision::Rejected {
                    reason: format!("Market {} is {}", selection.market_id, market_status),
                });
            }
        }
        
        // 2. Record submitted odds
        let submitted_odds: Vec<_> = bet_request.selections.iter()
            .map(|s| (s.outcome_id, s.odds))
            .collect();
        
        // 3. Wait for acceptance delay (for live events)
        if self.acceptance_delay > Duration::ZERO {
            tokio::time::sleep(self.acceptance_delay).await;
        }
        
        // 4. Re-check market status (may have suspended during delay)
        for selection in &bet_request.selections {
            let market_status = odds_cache
                .get_market_status(selection.market_id)
                .await?;
            
            if market_status != MarketStatus::Open {
                return Ok(LiveBetDecision::Rejected {
                    reason: format!("Market suspended during acceptance delay"),
                });
            }
        }
        
        // 5. Compare current odds with submitted odds
        let mut final_odds = Vec::new();
        for (outcome_id, submitted) in &submitted_odds {
            let current = odds_cache
                .get_outcome_odds(*outcome_id)
                .await?
                .ok_or(BetError::OddsNotFound { outcome_id: *outcome_id })?;
            
            match bet_request.accept_odds_changes {
                AcceptOddsChanges::None => {
                    if current != *submitted {
                        return Ok(LiveBetDecision::OddsChanged {
                            changes: vec![OddsChange {
                                outcome_id: *outcome_id,
                                submitted: *submitted,
                                current,
                            }],
                        });
                    }
                    final_odds.push(current);
                }
                AcceptOddsChanges::Higher => {
                    if current < *submitted {
                        return Ok(LiveBetDecision::OddsChanged {
                            changes: vec![OddsChange {
                                outcome_id: *outcome_id,
                                submitted: *submitted,
                                current,
                            }],
                        });
                    }
                    // Use current (higher) odds — better for user
                    final_odds.push(current);
                }
                AcceptOddsChanges::Any => {
                    // Check max drift
                    let drift = ((current - *submitted) / *submitted).abs();
                    if drift > self.max_odds_drift {
                        return Ok(LiveBetDecision::OddsChanged {
                            changes: vec![OddsChange {
                                outcome_id: *outcome_id,
                                submitted: *submitted,
                                current,
                            }],
                        });
                    }
                    final_odds.push(current);
                }
            }
        }
        
        Ok(LiveBetDecision::Accepted { final_odds })
    }
}

#[derive(Debug)]
pub enum LiveBetDecision {
    Accepted { final_odds: Vec<Decimal> },
    Rejected { reason: String },
    OddsChanged { changes: Vec<OddsChange> },
}

#[derive(Debug, Serialize)]
pub struct OddsChange {
    pub outcome_id: OutcomeId,
    pub submitted: Decimal,
    pub current: Decimal,
}
```

# ============================================================
# SECTION 9: RELATED CONTINGENCIES (CORRELATED MARKETS)
# ============================================================

```text
PROBLEM: Some markets on the same event are correlated.
If a user selects "Over 2.5 Goals" AND "Both Teams to Score: Yes",
these are correlated — if both teams score, there's a high chance
of over 2.5 goals.

Allowing correlated selections in accumulators gives the user
artificially high combined odds.

RULES:
  1. Within the SAME EVENT, certain market combinations are BLOCKED
  2. Different events are ALWAYS allowed (no correlation)
  3. The list of blocked combinations is per sport

BLOCKED COMBINATIONS (Football example):
  - Match Result + Double Chance (same event)
  - Over/Under + Correct Score (same event)
  - BTTS + Over/Under (same event)
  - Match Result + Draw No Bet (same event)
  - First Goalscorer + Correct Score (same event)
  - Any two markets that are mathematically correlated

IMPLEMENTATION:
  - Each market type has a "correlation group"
  - At most ONE selection per correlation group per event
```

```rust
/// Check if selections have related contingencies.
pub fn validate_no_related_contingencies(
    selections: &[ValidatedSelection],
) -> Result<(), BetError> {
    // Group selections by event
    let by_event: HashMap<EventId, Vec<&ValidatedSelection>> = selections.iter()
        .fold(HashMap::new(), |mut map, sel| {
            map.entry(sel.event_id).or_default().push(sel);
            map
        });
    
    for (event_id, event_selections) in &by_event {
        if event_selections.len() <= 1 {
            continue; // single selection per event is always OK
        }
        
        // Check correlation groups
        let mut seen_groups: HashSet<CorrelationGroup> = HashSet::new();
        for sel in event_selections {
            let group = get_correlation_group(sel.market_type);
            if let Some(group) = group {
                if !seen_groups.insert(group) {
                    return Err(BetError::RelatedContingency {
                        event_id: *event_id,
                        conflicting_markets: event_selections.iter()
                            .map(|s| s.market_type)
                            .collect(),
                    });
                }
            }
        }
    }
    
    Ok(())
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
enum CorrelationGroup {
    MatchResult,       // 1X2, Double Chance, Draw No Bet
    Goals,             // Over/Under, Correct Score, BTTS
    HalfTimeFullTime,  // HT Result, HT/FT
    Goalscorer,        // First/Last/Anytime Goalscorer
    Cards,             // Total Cards, Player Cards
    Corners,           // Total Corners
}

fn get_correlation_group(market_type: MarketType) -> Option<CorrelationGroup> {
    match market_type {
        MarketType::MatchResult | MarketType::DoubleChance | MarketType::DrawNoBet => {
            Some(CorrelationGroup::MatchResult)
        }
        MarketType::TotalOverUnder | MarketType::CorrectScore | MarketType::BothTeamsToScore => {
            Some(CorrelationGroup::Goals)
        }
        MarketType::HalfTimeResult => {
            Some(CorrelationGroup::HalfTimeFullTime)
        }
        MarketType::FirstGoalscorer => {
            Some(CorrelationGroup::Goalscorer)
        }
        // Markets without correlation (can combine freely)
        MarketType::Spread | MarketType::AsianHandicap | MarketType::PlayerProps => None,
        _ => None,
    }
}
```

# ============================================================
# SECTION 10: EDGE CASES AND RACE CONDITIONS
# ============================================================

```text
EDGE CASE 1: Event starts while bet is being placed
  Scenario: User clicks "place bet" → event kicks off → bet submitted
  Solution: Check event status AFTER locking funds
            If event started → unlock funds, reject bet

EDGE CASE 2: Odds change between display and placement
  Scenario: User sees odds 2.50, clicks place bet, odds now 2.30
  Solution: accept_odds_changes parameter
            If "none" → reject, show new odds
            If "higher" → reject (odds decreased)
            If "any" → accept at 2.30

EDGE CASE 3: Double settlement
  Scenario: Settlement runs twice for same event (retry, crash recovery)
  Solution: Idempotency key per settlement
            Database constraint: unique(bet_id, settlement_event)
            Check bet status before settling (must be "active")

EDGE CASE 4: Partial settlement of accumulator
  Scenario: Event A results at 15:00, Event B results at 17:00
            Accumulator has selections from both events
  Solution: Mark individual selections as settled
            Only finalize bet when ALL selections settled
            Use "pending_settlement_count" counter

EDGE CASE 5: Cashout during settlement
  Scenario: User clicks cashout at the same moment as settlement runs
  Solution: Optimistic locking on bet status
            Both operations try to transition from "active"
            First one wins, second gets "conflict" error
            Settlement always has priority (retry cashout fails gracefully)

EDGE CASE 6: Negative liability after void
  Scenario: Event cancelled, all bets voided
            Liability tracker goes negative
  Solution: Clamp to zero: max(0, current - released)

EDGE CASE 7: Accumulator with all selections void
  Scenario: All events in accumulator are cancelled
  Solution: Return full stake (not a "win", classified as "void")

EDGE CASE 8: Half-time / Full-time settlement
  Scenario: "First Half Over 0.5 Goals" market
            Half-time: 0-0 → market settles as "Under wins"
            But the event continues for second half
  Solution: Market-level settlement (not event-level)
            Each market has its own settlement trigger
            Some settle at half-time, some at full-time

EDGE CASE 9: User places bet, then self-excludes
  Scenario: Active bets exist when user self-excludes
  Solution: Option A: Void all active bets (refund stakes)
            Option B: Let bets run but block new bets
            Decision: Option B (regulatory depends on jurisdiction)

EDGE CASE 10: Maximum payout across multiple bets
  Scenario: User places 100 small bets on same outcome
            Individual liability OK, but combined exceeds max
  Solution: Track per-user per-outcome liability
            Not just per-bet limits
```

```text
RACE CONDITION PREVENTION CHECKLIST:

✅ Wallet operations use optimistic locking (version field)
✅ Bet creation uses idempotency key (UUID)
✅ Settlement uses idempotency key (event_id + bet_id)
✅ Status transitions use WHERE status = 'expected_status'
✅ Cashout validates bet status at execution time
✅ Odds are read from cache with known freshness (TTL)
✅ Market status checked before AND after acceptance delay
✅ Liability counter uses atomic operations (DashMap)
✅ Database constraints as last line of defense
✅ All critical operations wrapped in database transactions
```
```

---
