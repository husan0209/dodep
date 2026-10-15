---

# SKILL #59 — fraud-detection.skill.md

```markdown
# fraud-detection.skill.md
# GAMBLING PLATFORM — FRAUD DETECTION & PREVENTION
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent, Python ML Agent, Go Business Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Fraud in gambling: bonus abuse, multi-accounting, match fixing,
money laundering, collusion, arbitrage, bot betting.

Real-time engine: Rust (< 10ms per event scoring).
ML models: Python (trained weekly, served via ONNX in Rust).
Investigation tools: Go (admin dashboard backend).

# ============================================================
# SECTION 2: ARCHITECTURE
# ============================================================

```text
                    ┌───────────────┐
  Events from ────▶│  Rule Engine  │──▶ Score 0-100
  Redpanda         │  (Rust)       │      │
                    └───────┬───────┘      │
                            │              ▼
                    ┌───────▼───────┐   ┌──────────┐
                    │  ML Scoring   │   │  Actions  │
                    │  (ONNX/Rust)  │   │  Engine   │
                    └───────────────┘   └──────────┘
                                           │
                              ┌─────────────┼──────────────┐
                              ▼             ▼              ▼
                         0-30: LOG    30-70: REVIEW   70-100: BLOCK
============================================================
SECTION 3: RULE ENGINE
============================================================
VELOCITY RULES
Rust

pub struct VelocityRules;

impl VelocityRules {
    pub fn check(&self, event: &UserEvent, history: &UserHistory) -> Vec<Signal> {
        let mut signals = vec![];
        
        // Betting velocity
        if history.bets_last_1_min > 10 {
            signals.push(Signal::new("VELOCITY_BETS_HIGH", 40,
                "More than 10 bets in 1 minute"));
        }
        
        // Deposit velocity
        if history.deposits_last_1_hour > 5 {
            signals.push(Signal::new("VELOCITY_DEPOSITS_HIGH", 50,
                "More than 5 deposits in 1 hour"));
        }
        
        // Login velocity (credential stuffing)
        if history.failed_logins_last_5_min > 3 
           && history.successful_login_after_failures {
            signals.push(Signal::new("VELOCITY_LOGIN_SUSPICIOUS", 60,
                "Multiple failures then success"));
        }
        
        // Registration velocity from same IP
        if history.registrations_from_ip_last_24h > 2 {
            signals.push(Signal::new("VELOCITY_MULTI_REG", 70,
                "Multiple registrations from same IP"));
        }
        
        signals
    }
}
AMOUNT ANOMALY RULES
Rust

pub struct AmountRules;

impl AmountRules {
    pub fn check(&self, event: &UserEvent, profile: &UserProfile) -> Vec<Signal> {
        let mut signals = vec![];
        
        // Bet significantly above average
        if let Some(avg_bet) = profile.avg_bet_amount {
            if event.amount > avg_bet * dec!(10) {
                signals.push(Signal::new("AMOUNT_BET_ANOMALY", 45,
                    &format!("Bet {}x above average", event.amount / avg_bet)));
            }
        }
        
        // First deposit unusually large
        if profile.total_deposits == 0 && event.amount > dec!(500) {
            signals.push(Signal::new("AMOUNT_FIRST_DEPOSIT_LARGE", 35,
                "First deposit > $500"));
        }
        
        // Structuring: multiple deposits just below reporting threshold
        if profile.deposits_last_24h_just_below_threshold >= 3 {
            signals.push(Signal::new("AML_STRUCTURING", 80,
                "Possible structuring detected"));
        }
        
        signals
    }
}
PATTERN RULES
Rust

pub struct PatternRules;

impl PatternRules {
    pub fn check(&self, event: &UserEvent, profile: &UserProfile) -> Vec<Signal> {
        let mut signals = vec![];
        
        // Deposit → minimal play → withdraw (money laundering indicator)
        if profile.total_wagered < profile.total_deposited * dec!(0.5)
           && profile.withdrawal_requested
           && profile.account_age_days < 7 {
            signals.push(Signal::new("PATTERN_WASH", 75,
                "Deposit-minimal play-withdraw pattern"));
        }
        
        // Bonus abuse: min-odds bets to clear wagering
        if profile.avg_bet_odds < dec!(1.20) 
           && profile.has_active_bonus
           && profile.wagering_progress > dec!(0.5) {
            signals.push(Signal::new("PATTERN_BONUS_ABUSE", 55,
                "Low-odds bets during wagering"));
        }
        
        // Geo mismatch: registration country ≠ IP country
        if event.ip_country != profile.registration_country {
            signals.push(Signal::new("GEO_MISMATCH", 30,
                "IP country differs from registration"));
        }
        
        // VPN/Proxy detected
        if event.is_proxy || event.is_vpn {
            signals.push(Signal::new("GEO_VPN_DETECTED", 40,
                "VPN or proxy detected"));
        }
        
        signals
    }
}
MULTI-ACCOUNTING DETECTION
text

SIGNALS:
  Same device_fingerprint on multiple accounts       → score 80
  Same payment method (card hash) on multiple accounts → score 90
  Same email pattern (john1@, john2@, john3@)         → score 50
  Same IP + same user agent on multiple accounts      → score 60
  
DEVICE FINGERPRINT includes:
  Browser: canvas hash, WebGL hash, fonts, screen, timezone, plugins
  Mobile: device ID, advertising ID, hardware info
  
IMPLEMENTATION:
  On registration: check fingerprint against all existing users
  On login: update fingerprint, check for links
  Store fingerprint hashes in PostgreSQL with GIN index
  Graph-based analysis for complex multi-account networks
============================================================
SECTION 4: ML MODEL
============================================================
text

MODEL: XGBoost / LightGBM, served as ONNX in Rust

FEATURES (per user per event):
  bet_frequency_1h, bet_frequency_24h
  bet_amount_mean, bet_amount_std, bet_amount_max
  deposit_frequency_24h, deposit_amount_mean
  win_rate_7d, win_rate_30d
  cashout_frequency
  session_duration_mean
  device_count, ip_count, country_count
  time_since_registration_days
  kyc_level
  avg_odds, odds_variance
  
TRAINING:
  Weekly retrain on labeled data (fraud confirmed by risk team)
  Features computed from ClickHouse aggregations
  Train/test split: 80/20 with time-based split
  Metrics: AUC > 0.95, precision@90recall > 0.85
  
SERVING:
  ONNX model loaded into Rust service at startup
  Inference: < 5ms per user event
  Model hot-reload: new model file → reload without restart
  
FALLBACK:
  If ML model fails → use rule engine score only
  Never block a user based solely on ML (rules must agree)
============================================================
SECTION 5: SCORING AND ACTIONS
============================================================
Rust

pub struct FraudDecision {
    pub score: u8,           // 0-100
    pub signals: Vec<Signal>,
    pub action: FraudAction,
}

pub enum FraudAction {
    Allow,                    // 0-29: normal operation
    EnhancedMonitoring,       // 30-59: log extra details, watch
    ManualReview,             // 60-79: hold for risk team review
    Block,                    // 80-100: block + alert risk team
}

impl FraudAction {
    pub fn from_score(score: u8) -> Self {
        match score {
            0..=29 => Self::Allow,
            30..=59 => Self::EnhancedMonitoring,
            60..=79 => Self::ManualReview,
            80..=100 => Self::Block,
            _ => Self::Block,
        }
    }
}

// Score aggregation: take max of rule score + ML score, capped at 100
pub fn aggregate_score(rule_score: u8, ml_score: u8) -> u8 {
    rule_score.max(ml_score).min(100)
}
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER block user without audit trail (log reason, signals, scores)
❌ NEVER expose fraud rules to client (security through obscurity helps here)
❌ NEVER use ML score alone to block (rules must corroborate)
❌ NEVER skip fraud check on "trusted" users (VIPs get checked too)
❌ NEVER hard-code thresholds (use config, adjustable by risk team)
❌ NEVER ignore false positives (track and tune to keep FPR < 5%)
❌ NEVER check fraud AFTER money movement (check BEFORE)
❌ NEVER log PII in fraud signals (use user_id, not email/name)
============================================================
SECTION 7: TESTING
============================================================
text

MUST TEST:
  ✅ Velocity rules trigger at correct thresholds
  ✅ Multi-accounting detected on same device fingerprint
  ✅ ML model inference < 5ms
  ✅ Score aggregation correct (max of rule + ML)
  ✅ Block action prevents bet placement / withdrawal
  ✅ Audit log created for every fraud decision
  ✅ False positive: legitimate user not blocked (test with clean profiles)
  ✅ Model fallback works when ONNX file is corrupted