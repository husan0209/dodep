
---

# SKILL #58 — odds-calculation.skill.md

```markdown
# odds-calculation.skill.md
# GAMBLING PLATFORM — ODDS CALCULATION & MANAGEMENT
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent, Frontend Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Odds are the core product. Wrong odds = financial loss.
Stale odds = regulatory violation. Slow odds = lost customers.

This service is Rust. Latency target: odds update < 50ms end-to-end.

# ============================================================
# SECTION 2: ODDS FORMATS
# ============================================================

```text
INTERNAL FORMAT: Decimal (always). Stored as Decimal(12,6) in DB.
DISPLAY FORMAT: Converted per user preference on frontend.

Decimal  │ Fractional │ American │ Implied Probability
─────────┼────────────┼──────────┼────────────────────
1.50     │ 1/2        │ -200     │ 66.67%
2.00     │ 1/1 (Evens)│ +100     │ 50.00%
2.50     │ 3/2        │ +150     │ 40.00%
3.00     │ 2/1        │ +200     │ 33.33%
5.00     │ 4/1        │ +400     │ 20.00%
10.00    │ 9/1        │ +900     │ 10.00%

CONVERSION FORMULAS:
  decimal → implied:    1 / decimal_odds
  decimal → american:   if >= 2.0: (decimal - 1) × 100
                        if < 2.0:  -100 / (decimal - 1)
  decimal → fractional: (decimal - 1) as simplified fraction

  use rust_decimal::Decimal;
use rust_decimal_macros::dec;

pub fn to_american(odds: Decimal) -> String {
    if odds >= dec!(2.0) {
        format!("+{}", ((odds - dec!(1)) * dec!(100)).round_dp(0))
    } else {
        format!("-{}", (dec!(100) / (odds - dec!(1))).round_dp(0))
    }
}

pub fn implied_probability(odds: Decimal) -> Decimal {
    (dec!(1) / odds * dec!(100)).round_dp(2)
}

pub fn from_american(american: i64) -> Decimal {
    if american > 0 {
        Decimal::new(american, 0) / dec!(100) + dec!(1)
    } else {
        dec!(100) / Decimal::new(american.abs(), 0) + dec!(1)
    }
}


Margin is how the platform profits regardless of outcome.

FAIR MARKET (coin flip):
  Heads: 2.00 (50%) + Tails: 2.00 (50%) = 100% total

WITH 5% MARGIN:
  Heads: 1.909 (52.38%) + Tails: 1.909 (52.38%) = 104.76%
  Margin = 104.76% - 100% = 4.76%

MARGIN TARGETS BY MARKET:
  Football 1X2:         5-8%
  Football Over/Under:  6-9%
  Tennis Match Winner:  5-7%
  Basketball Spread:    5-7%
  Correct Score:        15-25%
  Player Props:         8-15%
  Live markets:         +2-3% vs pre-match


  /// Calculate margin from a set of odds.
pub fn calculate_margin(odds: &[Decimal]) -> Decimal {
    let implied_sum: Decimal = odds.iter().map(|o| dec!(1) / *o).sum();
    ((implied_sum - dec!(1)) * dec!(100)).round_dp(2) // as percentage
}

/// Apply margin to fair probabilities.
pub fn apply_margin(fair_probs: &[Decimal], margin_pct: Decimal) -> Vec<Decimal> {
    let margin = margin_pct / dec!(100);
    fair_probs.iter().map(|&prob| {
        let adjusted = prob * (dec!(1) + margin);
        (dec!(1) / adjusted).round_dp(2).max(dec!(1.01))
    }).collect()
}


PIPELINE: Sportradar → Receive → Normalize → Apply Margin → 
          Risk Adjust → Validate → Cache → Publish → Push

STEP 1: RECEIVE
  Pre-match: REST polling every 30 seconds
  Live: WebSocket push (real-time)
  
STEP 2: NORMALIZE
  Convert Sportradar format → internal format
  Map external IDs → internal IDs
  Decimal odds, UTC timestamps

STEP 3: APPLY MARGIN
  fair_odds from Sportradar × platform margin config
  Different margins per sport/market/tournament

STEP 4: RISK ADJUST
  If high liability on one outcome → reduce its odds
  If sharp money detected → adjust odds toward sharp position
  If event is live → add live premium (extra margin)

STEP 5: VALIDATE
  odds >= 1.01 and <= 1001.00
  odds changed by reasonable amount (no 10x jumps without reason)
  all outcomes in market present

STEP 6: CACHE
  DragonflyDB: key = odds:{event_id}:{market_id}:{outcome_id}
  TTL: live = 3-5s, pre-match = 30s
  Format: Protobuf serialized

STEP 7: PUBLISH
  Redpanda topic: events.odds_updated
  Key: event_id (partition by event)

STEP 8: PUSH
  WebSocket gateway reads from Redpanda
  Pushes to subscribed clients



  // Key structure in DragonflyDB
const ODDS_KEY: &str = "odds:{event_id}:{market_id}:{outcome_id}";
const MARKET_STATUS_KEY: &str = "market_status:{market_id}";
const EVENT_STATUS_KEY: &str = "event_status:{event_id}";

// Batch update odds for an event (pipeline for efficiency)
pub async fn update_event_odds(
    cache: &CacheClient,
    event_id: EventId,
    markets: &[MarketUpdate],
) -> Result<(), CacheError> {
    let mut pipe = cache.pipeline();
    
    for market in markets {
        pipe.set(
            &format!("market_status:{}", market.id),
            &market.status.to_string(),
            market.ttl_seconds,
        );
        
        for outcome in &market.outcomes {
            pipe.set(
                &format!("odds:{}:{}:{}", event_id, market.id, outcome.id),
                &outcome.odds.to_string(),
                market.ttl_seconds,
            );
        }
    }
    
    pipe.execute().await
}




SUSPENSIONS:
  Goal scored → suspend all markets 10-30 seconds
  Red card → suspend relevant markets 20-60 seconds
  Penalty → suspend all markets until resolved
  VAR review → suspend all markets until resolved
  
  CHECK: market status from cache BEFORE accepting any bet
  CACHE TTL for suspended status: same as suspension duration

DANGEROUS MOMENTS:
  Corner kick, free kick near box → brief suspension (5-10s)
  End of half/period → close period-specific markets
  
ODDS DRIFT PROTECTION:
  If odds change > 20% in < 10 seconds → auto-suspend
  Alert risk team for manual review
  Prevents bad data from feed causing incorrect odds


  ❌ NEVER serve odds from database (too slow) → USE cache (DragonflyDB)
❌ NEVER accept bet without checking market status from cache
❌ NEVER display odds older than TTL to client
❌ NEVER apply margin < 2% (unsustainable, regulatory risk)
❌ NEVER accept odds = 1.00 or odds < 1.01 (no margin, guaranteed loss)
❌ NEVER skip validation step (corrupt feed data happens)
❌ NEVER block on Sportradar API call in bet placement path
❌ NEVER store display-format odds in database (always decimal internally)


MUST TEST:
  ✅ Margin calculation correct for 2-way, 3-way, N-way markets
  ✅ Odds conversion: decimal ↔ fractional ↔ american (round-trip)
  ✅ Pipeline processes 50K+ odds updates/second
  ✅ Suspended market rejects bets immediately
  ✅ Cache TTL expiry doesn't serve stale odds
  ✅ Odds drift protection triggers on > 20% change
  ✅ Feed outage detected within 60 seconds (no updates alert)
  ✅ Concurrent odds update + bet placement is consistent



  