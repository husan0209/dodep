# Gambling Compliance Documentation
# Opus Casino — Regulatory Requirements & Compliance

## 📋 Overview

Этот документ описывает compliance требования для онлайн гемблинг платформы Opus Casino в соответствии с международными регуляторными стандартами.

## 🎯 Юрисдикции и Лицензии

### Target Markets

| Регион | Лицензия | Регулятор | Требования |
|--------|----------|-----------|------------|
| **EU (Malta)** | MGA B2B/B2C | Malta Gaming Authority | MGA/CRP/123/2024 |
| **UK** | UKGC | UK Gambling Commission | Remote Operating License |
| **Curacao** | Master License | Curacao eGaming | Sub-license available |
| **Gibraltar** | Remote Gambling | Gibraltar Regulatory Authority | Class 4 License |
| **Isle of Man** | OGRA | Isle of Man Gambling Supervision | Full/Restricted License |

---

## 🔐 AML (Anti-Money Laundering) Compliance

### 1. KYC (Know Your Customer)

**Требования:**

```
✅ Identity Verification (обязательно до первого withdrawal)
  - Government-issued ID (passport, driver's license, national ID)
  - Proof of Address (utility bill, bank statement < 3 months)
  - Selfie with ID (liveness check)

✅ Verification Timeline
  - Standard: Within 24 hours
  - Expedited: Within 2 hours (VIP)
  - Maximum: 72 hours (additional documents required)

✅ Document Retention
  - Keep records for 5 years after account closure
  - Encrypted storage (AES-256-GCM)
  - Access logging (who accessed what and when)
```

**Implementation:**

```go
// KYC Verification Flow
type KYCVerification struct {
    UserID           int64     `json:"user_id"`
    Status           string    `json:"status"` // pending, verified, rejected
    VerificationLevel int      `json:"level"`  // 1, 2, 3
    Documents        []Document `json:"documents"`
    VerifiedAt       time.Time `json:"verified_at"`
    VerifiedBy       string    `json:"verified_by"`
}

type Document struct {
    Type       string    `json:"type"` // id, address, selfie
    URL        string    `json:"url"`  // Encrypted S3 URL
    Status     string    `json:"status"`
    RejectionReason string `json:"rejection_reason,omitempty"`
    UploadedAt   time.Time `json:"uploaded_at"`
}

// AML Checks
func PerformAMLCheck(userID int64) (*AMLResult, error) {
    // 1. Sanctions list screening (OFAC, UN, EU)
    sanctionsMatch := checkSanctionsLists(userID)
    
    // 2. PEP (Politically Exposed Person) check
    pepMatch := checkPEPDatabase(userID)
    
    // 3. Adverse media screening
    adverseMedia := screenAdverseMedia(userID)
    
    // 4. Transaction monitoring
    suspiciousPatterns := detectSuspiciousPatterns(userID)
    
    return &AMLResult{
        RiskScore: calculateRiskScore(sanctionsMatch, pepMatch, adverseMedia, suspiciousPatterns),
        RequiresEnhancedDueDiligence: pepMatch || sanctionsMatch,
        RecommendedAction: determineAction(sanctionsMatch, pepMatch),
    }, nil
}
```

### 2. Transaction Monitoring

**Thresholds for Reporting:**

```
Suspicious Activity Report (SAR) triggers:
┌─────────────────────────────────────────────────────────┐
│ Transaction Type          │ Threshold    │ Time Window │
├─────────────────────────────────────────────────────────┤
│ Single Deposit           | > €10,000    | Immediate   │
│ Single Withdrawal        | > €10,000    | Immediate   │
│ Total Deposits (24h)     | > €5,000     | 24 hours    │
│ Total Withdrawals (24h)  | > €5,000     | 24 hours    │
│ Rapid Deposit/Withdraw   | 3+ cycles    | 1 hour      │
│ Structuring Detection    | Multiple <€3K| 24 hours    │
│ Unusual Betting Pattern  | >€5K on longshot | 1 hour  │
└─────────────────────────────────────────────────────────┘
```

**Implementation:**

```sql
-- Transaction monitoring query
CREATE MATERIALIZED VIEW mv_suspicious_transactions AS
SELECT 
    user_id,
    COUNT(*) as transaction_count,
    SUM(amount) as total_amount,
    MAX(amount) as max_single_transaction,
    STDDEV(amount) as amount_variance,
    COUNT(DISTINCT payment_method) as method_count
FROM transactions
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY user_id
HAVING 
    SUM(amount) > 5000 OR
    COUNT(*) > 10 OR
    MAX(amount) > 3000 OR
    COUNT(DISTINCT payment_method) > 3;

-- Alert generation
INSERT INTO compliance_alerts (user_id, alert_type, risk_score, status)
SELECT 
    user_id,
    'SUSPICIOUS_ACTIVITY',
    calculate_risk_score(total_amount, transaction_count, max_single_transaction),
    'PENDING_REVIEW'
FROM mv_suspicious_transactions
WHERE total_amount > 5000 OR transaction_count > 10;
```

### 3. Reporting Requirements

**Regulatory Reports:**

| Report | Frequency | Recipient | Deadline |
|--------|-----------|-----------|----------|
| **SAR** (Suspicious Activity Report) | On detection | FIU (Financial Intelligence Unit) | Within 24h |
| **CTR** (Cash Transaction Report) | Monthly | Regulator | 15th of month |
| **Annual Compliance Report** | Yearly | Regulator | Jan 31 |
| **Player Protection Report** | Quarterly | Regulator | 15 days after quarter |

---

## 🛡 Responsible Gambling

### 1. Player Protection Tools

**Self-Imposed Limits:**

```go
type PlayerLimits struct {
    UserID int64 `json:"user_id"`
    
    // Deposit limits (rolling windows)
    DailyDepositLimit   decimal.Decimal `json:"daily_deposit_limit"`
    WeeklyDepositLimit  decimal.Decimal `json:"weekly_deposit_limit"`
    MonthlyDepositLimit decimal.Decimal `json:"monthly_deposit_limit"`
    
    // Loss limits
    DailyLossLimit   decimal.Decimal `json:"daily_loss_limit"`
    WeeklyLossLimit  decimal.Decimal `json:"weekly_loss_limit"`
    MonthlyLossLimit decimal.Decimal `json:"monthly_loss_limit"`
    
    // Wager limits
    DailyWagerLimit   decimal.Decimal `json:"daily_wager_limit"`
    WeeklyWagerLimit  decimal.Decimal `json:"weekly_wager_limit"`
    
    // Session limits
    MaxSessionDuration time.Duration `json:"max_session_duration"` // in minutes
    SessionReminder    time.Duration `json:"session_reminder"`     // in minutes
    
    // Cool-off period
    CoolOffUntil *time.Time `json:"cool_off_until"`
    
    // Self-exclusion
    SelfExcluded   bool       `json:"self_excluded"`
    ExclusionUntil *time.Time `json:"exclusion_until"`
}

// Limit change rules
func ApplyLimitChange(limits *PlayerLimits, newLimit LimitChange) error {
    // INCREASE: requires cooling-off period (24-72 hours)
    if newLimit.IsIncrease() {
        limits.PendingChanges = append(limits.PendingChanges, PendingChange{
            Change: newLimit,
            EffectiveAfter: time.Now().Add(24 * time.Hour),
            Status: "PENDING_COOLING_OFF",
        })
        return nil
    }
    
    // DECREASE: takes effect immediately
    if newLimit.IsDecrease() {
        ApplyLimitImmediately(limits, newLimit)
        return nil
    }
    
    return nil
}
```

### 2. Self-Exclusion

**Implementation:**

```go
type SelfExclusion struct {
    UserID       int64     `json:"user_id"`
    StartDate    time.Time `json:"start_date"`
    EndDate      time.Time `json:"end_date"`
    Duration     string    `json:"duration"` // 6months, 1year, 2years, 5years, permanent
    Reason       string    `json:"reason"`
    RequestedVia string    `json:"requested_via"` // web, mobile, support, api
    
    // Multi-operator self-exclusion (if available in jurisdiction)
    MultiOperatorExcluded bool `json:"multi_operator_excluded"`
    
    // Cooling-off period (24h before activation)
    CoolingOffEnd time.Time `json:"cooling_off_end"`
}

// Enforce self-exclusion
func CheckSelfExclusion(userID int64) error {
    exclusion, err := getSelfExclusion(userID)
    if err != nil {
        return nil // No exclusion
    }
    
    if time.Now().Before(exclusion.CoolingOffEnd) {
        return fmt.Errorf("cooling_off_period_active_until: %s", exclusion.CoolingOffEnd)
    }
    
    if time.Now().Between(exclusion.StartDate, exclusion.EndDate) {
        // Block all gambling activities
        return fmt.Errorf("self_excluded_until: %s", exclusion.EndDate)
    }
    
    return nil
}
```

### 3. Reality Checks

**Session Monitoring:**

```go
type RealityCheck struct {
    UserID           int64         `json:"user_id"`
    SessionStart     time.Time     `json:"session_start"`
    SessionDuration  time.Duration `json:"session_duration"`
    TotalWagered     decimal.Decimal `json:"total_wagered"`
    TotalWon         decimal.Decimal `json:"total_won"`
    NetResult        decimal.Decimal `json:"net_result"`
    BetsPlaced       int           `json:"bets_placed"`
    
    // Reminder intervals
    ReminderInterval time.Duration `json:"reminder_interval"` // 30min, 60min, etc.
    LastReminder     time.Time     `json:"last_reminder"`
}

// Send reality check notification
func SendRealityCheck(ctx context.Context, userID int64) error {
    check, err := getSessionSummary(userID)
    if err != nil {
        return err
    }
    
    notification := Notification{
        UserID: userID,
        Type:   "REALITY_CHECK",
        Title:  "Session Summary",
        Message: fmt.Sprintf(
            "You've been playing for %s. Total wagered: €%.2f, Won: €%.2f, Net: €%.2f",
            check.SessionDuration,
            check.TotalWagered,
            check.TotalWon,
            check.NetResult,
        ),
        Actions: []NotificationAction{
            { Label: "Continue", Action: "dismiss" },
            { Label: "Take a Break", Action: "cool_off_24h" },
            { Label: "Self-Exclude", Action: "self_exclusion" },
        },
    }
    
    return sendNotification(ctx, notification)
}
```

---

## 📊 Data Protection (GDPR Compliance)

### 1. Data Classification

```
PERSONAL DATA (GDPR protected):
├─ Identity Data (name, DOB, nationality)
├─ Contact Data (email, phone, address)
├─ Financial Data (bank account, payment methods)
├─ KYC Data (ID documents, selfies)
├─ Technical Data (IP, device fingerprint, location)
└─ Behavioral Data (betting history, session data)

SPECIAL CATEGORY DATA (requires explicit consent):
└─ None collected (gambling platform does not process health, political, religious data)
```

### 2. Data Subject Rights

**Implementation:**

```go
type DataSubjectRequest struct {
    ID            string    `json:"id"`
    UserID        int64     `json:"user_id"`
    RequestType   string    `json:"type"` // access, rectification, erasure, portability, restriction
    Status        string    `json:"status"`
    RequestedAt   time.Time `json:"requested_at"`
    CompletedAt   *time.Time `json:"completed_at"`
    Response      string    `json:"response,omitempty"` // Encrypted response URL
}

// Handle GDPR requests
func HandleDataSubjectRequest(req *DataSubjectRequest) error {
    switch req.RequestType {
    case "access":
        return handleAccessRequest(req.UserID)
    
    case "rectification":
        return handleRectificationRequest(req.UserID)
    
    case "erasure":
        // Check legal obligations before deletion
        if hasLegalHold(req.UserID) {
            return fmt.Errorf("legal_hold_active")
        }
        return handleErasureRequest(req.UserID)
    
    case "portability":
        return handlePortabilityRequest(req.UserID)
    
    case "restriction":
        return handleRestrictionRequest(req.UserID)
    
    default:
        return fmt.Errorf("unknown_request_type")
    }
}

// Data retention policy
var DataRetentionSchedule = map[string]time.Duration{
    "kyc_documents":      5 * 365 * 24 * time.Hour,  // 5 years after account closure
    "transaction_history": 5 * 365 * 24 * time.Hour, // 5 years (AML requirement)
    "bet_history":        5 * 365 * 24 * time.Hour, // 5 years
    "session_logs":       1 * 365 * 24 * time.Hour, // 1 year
    "analytics_data":     2 * 365 * 24 * time.Hour, // 2 years (anonymized after)
    "marketing_data":     2 * 365 * 24 * time.Hour, // 2 years or until opt-out
}
```

### 3. Privacy by Design

```
DATA FLOW WITH PRIVACY CONTROLS:
┌─────────────────────────────────────────────────────────┐
│ User Registration                                        │
│ → Collect minimum required data                          │
│ → Explicit consent for each processing purpose           │
│ → Privacy notice displayed                               │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ Data Processing                                          │
│ → Pseudonymization (user_id instead of email)            │
│ → Encryption at rest (AES-256-GCM)                       │
│ → Encryption in transit (TLS 1.3)                        │
│ → Access logging (who accessed what)                     │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ Data Retention                                           │
│ → Automated deletion after retention period              │
│ → Anonymization for analytics                            │
│ → Regular data quality audits                            │
└─────────────────────────────────────────────────────────┘
```

---

## 🔒 Technical Security Measures

### 1. Encryption Standards

```
DATA AT REST:
├─ Database: AES-256-GCM (field-level for sensitive data)
├─ Files: AES-256-GCM + envelope encryption
├─ Backups: AES-256-GCM + KMS managed keys
└─ Keys: HashiCorp Vault (HSM-backed for root keys)

DATA IN TRANSIT:
├─ External: TLS 1.3 (minimum TLS 1.2)
├─ Internal: mTLS (Istio service mesh)
├─ API: HTTPS with HSTS
└─ WebSocket: WSS (WebSocket Secure)

KEY MANAGEMENT:
├─ Root Keys: HSM (Hardware Security Module)
├─ Data Keys: Vault Transit Engine
├─ Key Rotation: Annual (or on compromise)
└─ Key Escrow: Multi-party computation (MPC)
```

### 2. Access Controls

```
ROLE-BASED ACCESS CONTROL (RBAC):
┌─────────────────────────────────────────────────────────┐
│ Role              │ Permissions                         │
├─────────────────────────────────────────────────────────┤
│ support_l1        │ View user profile, basic info       │
│ support_l2        │ + Edit limits, approve withdrawals  │
│ risk_manager      │ + Block users, void bets            │
│ finance           │ Transactions, payment approval      │
│ compliance        │ Full KYC/AML access, SAR filing     │
│ admin             │ All permissions                     │
│ super_admin       │ + System config, user delete        │
└─────────────────────────────────────────────────────────┘

PRINCIPLE OF LEAST PRIVILEGE:
- Service accounts have minimum required permissions
- Just-In-Time (JIT) access for sensitive operations
- All access logged and audited
```

### 3. Audit Logging

```
AUDIT LOG REQUIREMENTS:
┌─────────────────────────────────────────────────────────┐
│ Field            │ Description                          │
├─────────────────────────────────────────────────────────┤
│ timestamp        │ ISO 8601 with timezone               │
│ event_type       │ LOGIN, BET_PLACED, WITHDRAWAL, etc.  │
│ user_id          │ User identifier (pseudonymized)      │
│ actor_id         │ Who performed the action             │
│ action           │ CREATE, READ, UPDATE, DELETE         │
│ resource         │ Affected resource (bets, users, etc.)│
│ resource_id      │ Resource identifier                  │
│ ip_address       │ Client IP (anonymized last octet)    │
│ user_agent       │ Browser/client info                  │
│ result           │ SUCCESS, FAILURE, PARTIAL            │
│ reason           │ For failures/rejections              │
│ metadata         │ Additional context (JSON)            │
└─────────────────────────────────────────────────────────┘

RETENTION: 7 years minimum (regulatory requirement)
```

---

## 📝 Compliance Checklist

### Pre-Launch Requirements

- [ ] **License Application Submitted**
  - [ ] Business plan approved
  - [ ] Technical documentation submitted
  - [ ] Key personnel approved (fit & proper test)
  - [ ] Proof of funds (minimum capital requirement)

- [ ] **Technical Compliance**
  - [ ] RNG certification (eCOGRA, GLI, or iTech Labs)
  - [ ] Game fairness audit
  - [ ] Security penetration test
  - [ ] Disaster recovery plan tested

- [ ] **AML/CTF Compliance**
  - [ ] AML policy documented
  - [ ] MLRO (Money Laundering Reporting Officer) appointed
  - [ ] Staff AML training completed
  - [ ] Transaction monitoring system deployed

- [ ] **Player Protection**
  - [ ] Responsible gambling policy implemented
  - [ ] Self-exclusion system operational
  - [ ] Reality checks configured
  - [ ] Age verification system deployed (18+/21+)

### Ongoing Requirements

- [ ] **Monthly**
  - [ ] Transaction reports filed
  - [ ] Player fund segregation verified
  - [ ] System uptime report (>99.9%)

- [ ] **Quarterly**
  - [ ] Internal compliance audit
  - [ ] Player protection report
  - [ ] Security review

- [ ] **Annually**
  - [ ] External compliance audit
  - [ ] RNG re-certification
  - [ ] Financial statements audited
  - [ ] License renewal (if applicable)

---

## 📞 Regulatory Contacts

| Jurisdiction | Regulator | Contact | Emergency |
|--------------|-----------|---------|-----------|
| **Malta** | MGA | compliance@mga.org.mt | +356 2546 6400 |
| **UK** | UKGC | contact@ukgc.gov.uk | +44 121 230 6666 |
| **Curacao** | GCB | info@gcb.cw | +599 9 461 0244 |
| **Gibraltar** | GRA | gambling@gibraltar.gov.gi | +350 200 72800 |

---

**Document Owner:** Compliance Officer (MLRO)  
**Review Cycle:** Quarterly  
**Last Updated:** 2026-03-24  
**Next Review:** 2026-06-24
