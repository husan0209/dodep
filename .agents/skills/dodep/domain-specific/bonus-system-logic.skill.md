# SKILL #55 — bonus-system-logic.skill.md

```markdown
# bonus-system-logic.skill.md
# GAMBLING PLATFORM — BONUS SYSTEM LOGIC
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent, Frontend Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Bonuses attract and retain players. But they're also the #1 target
for fraud (bonus abuse). Every bonus has wagering requirements
to prevent deposit-bonus-withdraw schemes.

Bonus Service is written in Go.

# ============================================================
# SECTION 2: BONUS TYPES
# ============================================================

```text
WELCOME BONUS (first deposit):
  "100% match up to $500, 30x wagering"
  deposit $100 → get $100 bonus → must wager $3,000 before withdraw

RELOAD BONUS (subsequent deposits):
  "50% match up to $200, 25x wagering"

FREE SPINS:
  "50 free spins on Slot X, spin value $0.20, 40x wagering on winnings"
  Total free spin value: 50 × $0.20 = $10.00
  Winnings from spins subject to wagering
  Max win cap: e.g., $100

FREE BET (sports):
  "$10 free bet, min odds 1.50, 7 day expiry"
  If free bet wins: user gets profit only (not stake)
  If free bet loses: nothing lost (was free)

CASHBACK:
  "10% weekly cashback up to $100, 5x wagering"
  Calculated: (deposits - withdrawals - balance) × 10%
  Only on net losses

NO DEPOSIT BONUS:
  "$10 free, 60x wagering, max win $50"
  Given without deposit (for new users)
  Highest wagering requirement (abuse risk)

LOYALTY POINTS:
  1 point per $1 wagered
  100 points = $1 bonus
  Points never expire (or expire after 12 months inactivity)
============================================================
SECTION 3: WAGERING ENGINE
============================================================
text

WAGERING REQUIREMENT = bonus_amount × multiplier
  Example: $100 bonus × 30x = $3,000 must be wagered

WAGERING PROGRESS:
  progress = total_qualifying_bets / wagering_requirement
  When progress >= 1.0 → bonus "completed" → funds become real money

QUALIFYING CRITERIA:
  Sports: bet must have odds >= 1.50
  Casino: certain games excluded (see below)
  Max bet from bonus: $5 per bet (prevents high-risk clearing)
  
GAME CONTRIBUTION (casino):
  Slots:        100% (every $1 wagered counts as $1)
  Table Games:  10% ($1 wagered counts as $0.10)
  Live Casino:  10%
  Video Poker:  10%
  Excluded:     Specific high-RTP games (Mega Joker, Blood Suckers)
  
  WHY: Low-edge games let players clear wagering with minimal loss.
  High contribution on slots because slots have higher house edge.
DATA MODEL
Go

type BonusCampaign struct {
    ID             int64
    Name           string
    Type           BonusType    // welcome, reload, free_spins, etc.
    MatchPercent   int          // 100 = 100% match
    MaxAmount      Decimal      // max bonus amount
    MinDeposit     Decimal      // minimum qualifying deposit
    WageringMulti  int          // 30 = 30x
    MaxBet         Decimal      // max bet while bonus active
    MinOdds        Decimal      // min odds for qualifying bets (sports)
    ExpiryDays     int          // bonus expires after N days
    MaxWin         *Decimal     // cap on winnings (for free spins/no deposit)
    GameContrib    map[string]int // game_type → contribution %
    ExcludedGames  []int64      // game IDs that don't count
    Countries      []string     // eligible countries (empty = all)
    StartDate      time.Time
    EndDate        *time.Time
    Active         bool
}

type UserBonus struct {
    ID              int64
    UserID          int64
    CampaignID      int64
    BonusAmount     Decimal
    WageringRequired Decimal    // bonus_amount × multiplier
    WageringCompleted Decimal   // how much wagered so far
    Status          BonusStatus // pending, active, completed, expired, forfeited
    ActivatedAt     time.Time
    ExpiresAt       time.Time
    CompletedAt     *time.Time
}

type BonusStatus string
const (
    BonusPending   BonusStatus = "pending"   // waiting for qualifying deposit
    BonusActive    BonusStatus = "active"     // wagering in progress
    BonusCompleted BonusStatus = "completed"  // wagering done, funds released
    BonusExpired   BonusStatus = "expired"    // time ran out
    BonusForfeited BonusStatus = "forfeited"  // user cancelled or violated rules
)
============================================================
SECTION 4: CLAIMING FLOW
============================================================
text

WELCOME BONUS FLOW:
  1. User registers
  2. System shows available welcome bonus
  3. User opts in (explicit claim or automatic)
  4. User makes qualifying deposit (>= min_deposit)
  5. System calculates bonus: min(deposit × match%, max_amount)
  6. Bonus credited to bonus_balance (separate from real_balance)
  7. Wagering counter starts
  8. User plays: qualifying bets count toward wagering
  9. When wagering complete: bonus_balance → real_balance
  10. User can now withdraw

BALANCE PRIORITY:
  When user places bet:
    1. Debit from real_balance first
    2. Then from bonus_balance
  When user wins:
    Credit to real_balance (if bet was from real)
    Credit to bonus_balance (if bet was from bonus)
  
  WHY: If user bets with real money, winnings are withdrawable.
  If user bets with bonus, winnings subject to wagering.

CANCELLATION:
  User can forfeit bonus at any time
  → bonus_balance set to 0
  → wagering progress reset
  → any winnings from bonus removed
  → real_balance untouched
WAGERING TRACKING
Go

func (s *BonusService) RecordWager(
    ctx context.Context, 
    userID int64, 
    betAmount Decimal,
    betOdds Decimal,
    gameType string,
    gameID int64,
) error {
    bonus, err := s.repo.GetActiveBonus(ctx, userID)
    if err != nil || bonus == nil {
        return nil // no active bonus, nothing to track
    }
    
    campaign, _ := s.repo.GetCampaign(ctx, bonus.CampaignID)
    
    // Check qualifying criteria
    if betOdds.LessThan(campaign.MinOdds) {
        return nil // bet odds too low, doesn't count
    }
    
    if contains(campaign.ExcludedGames, gameID) {
        return nil // excluded game
    }
    
    // Check max bet
    if betAmount.GreaterThan(campaign.MaxBet) {
        // This is a VIOLATION — flag for review
        s.producer.Publish(ctx, "fraud.signals", &BonusAbuseSignal{
            UserID: userID, 
            Reason: "max_bet_exceeded_during_bonus",
            Amount: betAmount,
        })
        return domain.ErrBonusMaxBetExceeded
    }
    
    // Apply game contribution
    contribution := campaign.GameContrib[gameType]
    if contribution == 0 {
        contribution = 100 // default 100%
    }
    qualifyingAmount := betAmount.Mul(Decimal.NewFromInt(int64(contribution))).Div(dec(100))
    
    // Update wagering progress
    newProgress := bonus.WageringCompleted.Add(qualifyingAmount)
    
    if newProgress.GreaterThanOrEqual(bonus.WageringRequired) {
        // WAGERING COMPLETE — release bonus to real balance
        return s.completeBonusWagering(ctx, bonus)
    }
    
    return s.repo.UpdateWageringProgress(ctx, bonus.ID, newProgress)
}

func (s *BonusService) completeBonusWagering(ctx context.Context, bonus *UserBonus) error {
    // 1. Update bonus status
    s.repo.UpdateBonusStatus(ctx, bonus.ID, BonusCompleted)
    
    // 2. Move bonus_balance → real_balance
    s.walletClient.ConvertBonusToReal(ctx, bonus.UserID, bonus.BonusAmount)
    
    // 3. Apply max win cap if applicable
    campaign, _ := s.repo.GetCampaign(ctx, bonus.CampaignID)
    if campaign.MaxWin != nil {
        // Check if winnings exceed cap
        // If so, cap and remove excess
    }
    
    // 4. Notify user
    s.producer.Publish(ctx, "bonuses.completed", &BonusCompletedEvent{
        UserID:  bonus.UserID,
        BonusID: bonus.ID,
        Amount:  bonus.BonusAmount,
    })
    
    return nil
}
============================================================
SECTION 5: EXPIRATION AND FORFEITURE
============================================================
text

EXPIRATION:
  Cron job runs every hour
  Check: WHERE status = 'active' AND expires_at < NOW()
  Action:
    1. Set status = 'expired'
    2. Remove bonus_balance
    3. Remove any winnings from bonus bets
    4. Notify user
    
FORFEITURE (user-initiated):
  User clicks "Cancel Bonus"
  Same as expiration but immediate
  Real balance untouched

AUTOMATIC FORFEITURE:
  If user requests withdrawal while bonus active:
    Option A: Cancel bonus, then process withdrawal
    Option B: Block withdrawal until wagering complete
    Decision: Option A (better UX, most platforms do this)
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER credit bonus directly to real_balance (always bonus_balance)
❌ NEVER allow withdrawal during active wagering without forfeiting bonus
❌ NEVER skip max bet check during bonus (abuse vector)
❌ NEVER allow same user to claim welcome bonus twice
❌ NEVER set wagering multiplier < 5x (instant cash out)
❌ NEVER forget game contribution rates (some games are excluded for a reason)
❌ NEVER calculate wagering from locked/pending bets (only settled bets count)
❌ NEVER apply bonus retroactively to past deposits
❌ NEVER allow bonus stacking (one active bonus at a time, unless explicitly designed)
============================================================
SECTION 7: TESTING
============================================================
text

MUST TEST:
  ✅ Welcome bonus: deposit → credit bonus → wager → complete → withdraw
  ✅ Wagering progress tracks correctly with game contribution %
  ✅ Max bet violation during bonus is blocked and flagged
  ✅ Low-odds bets (< min_odds) don't count toward wagering
  ✅ Excluded games don't count toward wagering
  ✅ Bonus expires correctly after expiry_days
  ✅ Forfeiture removes bonus_balance but keeps real_balance
  ✅ Max win cap applied correctly for free spins
  ✅ Cannot claim welcome bonus twice (even with different deposit)
  ✅ Withdrawal during active bonus → auto-forfeit → process withdrawal