
# SKILL #57 — responsible-gambling.skill.md

```markdown
# responsible-gambling.skill.md
# GAMBLING PLATFORM — RESPONSIBLE GAMBLING
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent, Frontend Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Responsible gambling is a REGULATORY REQUIREMENT. These features
protect vulnerable players. Failure to implement = license loss.

These features MUST work perfectly. A self-excluded user placing
a bet is a regulatory violation that can shut down the platform.

Service: Go (part of User Service or standalone).

# ============================================================
# SECTION 2: LIMIT TYPES
# ============================================================

```text
DEPOSIT LIMITS:
  daily_deposit_limit:   max deposit per 24h rolling window
  weekly_deposit_limit:  max deposit per 7d rolling window
  monthly_deposit_limit: max deposit per 30d rolling window
  
  INCREASE: takes effect after 24-72 hour cooling period
  DECREASE: takes effect IMMEDIATELY
  WHY: prevents impulsive increases during gambling session

LOSS LIMITS:
  daily_loss_limit:   max net loss per 24h
  weekly_loss_limit:  max net loss per 7d
  monthly_loss_limit: max net loss per 30d
  
  Calculation: deposits - withdrawals - current_balance
  When reached: block deposits AND bets

WAGER LIMITS:
  daily_wager_limit:   max total wagered per 24h
  weekly_wager_limit:  max total wagered per 7d
  
  When reached: block new bets (can still withdraw)

SESSION LIMITS:
  session_duration_limit:  max continuous play time
  daily_duration_limit:    max total play time per 24h
  
  When reached: force logout, block login for remaining period
  Warning: popup at 80% of limit
```

## DATA MODEL

```go
type ResponsibleGamblingSettings struct {
    UserID               int64
    DepositLimitDaily    *Decimal
    DepositLimitWeekly   *Decimal
    DepositLimitMonthly  *Decimal
    LossLimitDaily       *Decimal
    LossLimitWeekly      *Decimal
    LossLimitMonthly     *Decimal
    WagerLimitDaily      *Decimal
    WagerLimitWeekly     *Decimal
    SessionDurationLimit *time.Duration // per session
    DailyDurationLimit   *time.Duration // total per day
    RealityCheckInterval *time.Duration // popup interval
    SelfExclusionUntil   *time.Time     // nil = not excluded
    SelfExclusionType    *string        // "temporary" or "permanent"
    CoolOffUntil         *time.Time     // nil = not cooling off
    UpdatedAt            time.Time
}

type PendingLimitChange struct {
    ID         int64
    UserID     int64
    LimitType  string    // "deposit_daily", "loss_weekly", etc.
    OldValue   *Decimal
    NewValue   Decimal
    Direction  string    // "increase" or "decrease"
    RequestedAt time.Time
    EffectiveAt time.Time // now for decrease, +24-72h for increase
    Applied    bool
}
```

# ============================================================
# SECTION 3: LIMIT ENFORCEMENT
# ============================================================

```go
// Called BEFORE every deposit
func (s *RGService) CheckDepositAllowed(
    ctx context.Context, userID int64, amount Decimal,
) error {
    settings, _ := s.repo.GetSettings(ctx, userID)
    
    // Check self-exclusion first
    if settings.SelfExclusionUntil != nil && time.Now().Before(*settings.SelfExclusionUntil) {
        return domain.ErrSelfExcluded
    }
    
    // Check cool-off
    if settings.CoolOffUntil != nil && time.Now().Before(*settings.CoolOffUntil) {
        return domain.ErrCoolOffActive
    }
    
    // Check deposit limits
    if settings.DepositLimitDaily != nil {
        deposited24h, _ := s.repo.GetDepositsInWindow(ctx, userID, 24*time.Hour)
        if deposited24h.Add(amount).GreaterThan(*settings.DepositLimitDaily) {
            return domain.WithDetails(domain.ErrDepositLimitExceeded, map[string]interface{}{
                "limit":     settings.DepositLimitDaily.String(),
                "used":      deposited24h.String(),
                "remaining": settings.DepositLimitDaily.Sub(deposited24h).String(),
            })
        }
    }
    
    // Same for weekly, monthly...
    return nil
}

// Called BEFORE every bet
func (s *RGService) CheckBetAllowed(
    ctx context.Context, userID int64, stakeAmount Decimal,
) error {
    settings, _ := s.repo.GetSettings(ctx, userID)
    
    // Self-exclusion check (CRITICAL — never skip)
    if settings.SelfExclusionUntil != nil && time.Now().Before(*settings.SelfExclusionUntil) {
        return domain.ErrSelfExcluded
    }
    
    // Cool-off: can't bet during cool-off
    if settings.CoolOffUntil != nil && time.Now().Before(*settings.CoolOffUntil) {
        return domain.ErrCoolOffActive
    }
    
    // Loss limit check
    if settings.LossLimitDaily != nil {
        netLoss, _ := s.calculateNetLoss(ctx, userID, 24*time.Hour)
        if netLoss.GreaterThanOrEqual(*settings.LossLimitDaily) {
            return domain.ErrLossLimitReached
        }
    }
    
    // Wager limit check
    if settings.WagerLimitDaily != nil {
        wagered24h, _ := s.repo.GetWageredInWindow(ctx, userID, 24*time.Hour)
        if wagered24h.Add(stakeAmount).GreaterThan(*settings.WagerLimitDaily) {
            return domain.ErrWagerLimitReached
        }
    }
    
    return nil
}

// Called BEFORE every casino game launch
func (s *RGService) CheckGameLaunchAllowed(ctx context.Context, userID int64) error {
    settings, _ := s.repo.GetSettings(ctx, userID)
    
    // Self-exclusion (same check everywhere)
    if settings.SelfExclusionUntil != nil && time.Now().Before(*settings.SelfExclusionUntil) {
        return domain.ErrSelfExcluded
    }
    
    // Session duration limit
    if settings.DailyDurationLimit != nil {
        playedToday, _ := s.repo.GetPlayTimeToday(ctx, userID)
        if playedToday >= *settings.DailyDurationLimit {
            return domain.ErrDailyDurationLimitReached
        }
    }
    
    return nil
}
```

# ============================================================
# SECTION 4: SELF-EXCLUSION
# ============================================================

```text
PERIODS: 24h, 7d, 30d, 6 months, 1 year, PERMANENT

PERMANENT self-exclusion:
  - CANNOT be reversed (regulatory requirement in most jurisdictions)
  - User must contact regulator to reverse (not platform)
  - Account permanently closed for gambling

WHEN SELF-EXCLUSION IS SET:
  1. Block login immediately
  2. Close all active sessions (force logout)
  3. Active bets: let them run (settle normally) — OR void (jurisdiction-dependent)
  4. Pending withdrawals: process them (user gets their money)
  5. Cancel any pending deposits
  6. Remove from all marketing lists
  7. Remove from affiliate tracking
  8. Block re-registration (email, phone, document match)
  9. Publish users.self_excluded event

BLOCK RE-REGISTRATION:
  On new registration, check against exclusion database:
    - email hash match
    - phone hash match
    - document number hash match (from KYC)
    - device fingerprint match
  If match found → block registration with generic error
  (don't reveal that account exists — privacy)
```

```go
func (s *RGService) SelfExclude(
    ctx context.Context, userID int64, period string,
) error {
    var until time.Time
    isPermanent := false
    
    switch period {
    case "24h":
        until = time.Now().Add(24 * time.Hour)
    case "7d":
        until = time.Now().Add(7 * 24 * time.Hour)
    case "30d":
        until = time.Now().Add(30 * 24 * time.Hour)
    case "6m":
        until = time.Now().AddDate(0, 6, 0)
    case "1y":
        until = time.Now().AddDate(1, 0, 0)
    case "permanent":
        until = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
        isPermanent = true
    default:
        return domain.NewValidationError(
            domain.FieldError{Field: "period", Message: "Invalid exclusion period"})
    }

    // Update settings
    s.repo.SetSelfExclusion(ctx, userID, until, isPermanent)

    // Kill all sessions
    s.sessionClient.DeleteAllForUser(ctx, userID)

    // Block user status
    s.userClient.UpdateStatus(ctx, userID, domain.UserStatusSelfExcluded)

    // Cancel pending deposits
    s.paymentClient.CancelPendingDeposits(ctx, userID)

    // Process pending withdrawals (don't block money)
    // (handled by payment service — they check user status)

    // Remove from marketing
    s.notificationClient.UnsubscribeAll(ctx, userID)

    // Store exclusion record for re-registration blocking
    s.repo.CreateExclusionRecord(ctx, &ExclusionRecord{
        UserID:      userID,
        EmailHash:   hashPII(user.Email),
        PhoneHash:   hashPII(user.Phone),
        DocumentHash: hashPII(user.DocumentNumber),
        DeviceFingerprints: user.KnownFingerprints,
        ExcludedAt:  time.Now(),
        ExcludedUntil: until,
        IsPermanent: isPermanent,
    })

    s.producer.Publish(ctx, "users.self_excluded", &SelfExcludedEvent{
        UserID:      userID,
        Period:      period,
        IsPermanent: isPermanent,
    })

    log.Info().Int64("user_id", userID).Str("period", period).Msg("User self-excluded")
    return nil
}
```

# ============================================================
# SECTION 5: REALITY CHECK
# ============================================================

```text
WHAT: Periodic popup during gambling showing session statistics.
WHEN: Every 30/60 minutes of continuous play (user-configurable).
SHOWS:
  - Time played in this session
  - Total amount wagered
  - Net win/loss
  - "Continue playing?" or "Take a break"

IMPLEMENTATION:
  Frontend: timer-based popup, cannot be dismissed without action
  Backend: provides session stats via GET /api/v1/responsible-gambling/session-stats
  
  Session start: recorded when first bet placed or game launched
  Session end: 30 minutes of inactivity OR explicit logout
```

# ============================================================
# SECTION 6: ANTI-PATTERNS
# ============================================================

```text
❌ NEVER skip self-exclusion check on ANY gambling action (bet, game, deposit)
❌ NEVER allow limit increase to take effect immediately (cooling period required)
❌ NEVER allow reversal of permanent self-exclusion
❌ NEVER reveal excluded account exists during re-registration
❌ NEVER block withdrawals for self-excluded users (they must get their money)
❌ NEVER suppress reality check popup (must be shown, cannot be auto-dismissed)
❌ NEVER set limit enforcement as optional/configurable feature flag
❌ NEVER log self-exclusion reason publicly (privacy-sensitive)
❌ NEVER allow admin to override self-exclusion (only regulator can)
❌ NEVER check limits from cache alone (always verify against DB for writes)
```

# ============================================================
# SECTION 7: TESTING
# ============================================================

```text
MUST TEST:
  ✅ Self-excluded user CANNOT: place bet, launch game, deposit
  ✅ Self-excluded user CAN: withdraw, view history
  ✅ Limit decrease: effective immediately
  ✅ Limit increase: effective only after cooling period (24-72h)
  ✅ Deposit over limit: rejected with remaining amount shown
  ✅ Loss limit reached: bets blocked, deposits blocked
  ✅ Session duration limit: forced logout at limit
  ✅ Reality check popup: fires at configured interval
  ✅ Re-registration with excluded email/phone: blocked
  ✅ Permanent exclusion: cannot be reversed via any API
  ✅ All active sessions killed on self-exclusion
```
```

---

