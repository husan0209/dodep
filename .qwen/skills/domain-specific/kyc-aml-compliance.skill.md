

4 файла, формат B (400-600 строк).

---

# SKILL #56 — kyc-aml-compliance.skill.md

```markdown
# kyc-aml-compliance.skill.md
# GAMBLING PLATFORM — KYC/AML COMPLIANCE
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent, Security Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

KYC (Know Your Customer) and AML (Anti-Money Laundering) are
LEGAL REQUIREMENTS. Non-compliance = license revocation + fines.

KYC Service: Go (integrates with Sumsub).
AML Service: Go (integrates with ComplyAdvantage).
Document Storage: S3 (encrypted, restricted access).

# ============================================================
# SECTION 2: KYC LEVELS
# ============================================================

```text
LEVEL 0 — Registration only
  Requirements: email + password + age confirmation
  Limits:       deposit $0 (cannot deposit yet)
  Triggers:     account creation

LEVEL 1 — Email + Phone verified
  Requirements: email verification + SMS verification
  Limits:       deposit $200/day, withdrawal $0
  Triggers:     automatic after verification

LEVEL 2 — Identity verified
  Requirements: government ID (passport/driving license) + selfie
  Limits:       deposit $5,000/day, withdrawal $2,000/day
  Triggers:     first deposit > $50 OR first withdrawal request
  Provider:     Sumsub SDK (frontend) + webhook (backend)

LEVEL 3 — Address + Source of Funds
  Requirements: proof of address + source of funds declaration
  Limits:       deposit $50,000/day, withdrawal $20,000/day
  Triggers:     cumulative deposits > $10,000 OR withdrawal > $2,000

LEVEL 4 — Enhanced Due Diligence
  Requirements: manual review by compliance team
  Limits:       custom (agreed individually)
  Triggers:     VIP status OR suspicious activity flag
```

# ============================================================
# SECTION 3: SUMSUB INTEGRATION
# ============================================================

```text
FLOW:
  1. User triggers KYC → POST /api/v1/kyc/start
  2. Backend creates Sumsub applicant → gets SDK token
  3. Frontend opens Sumsub SDK widget (iframe/native)
  4. User uploads document + takes selfie in SDK
  5. Sumsub processes verification (30 sec - 24 hours)
  6. Sumsub sends webhook → POST /api/v1/kyc/webhook
  7. Backend verifies webhook signature
  8. Update user KYC level + limits
  9. Publish event + notify user
```

```go
// Start KYC verification
func (s *KYCService) StartVerification(
    ctx context.Context,
    userID int64,
    level int,
) (*StartVerificationResult, error) {
    user, err := s.userClient.GetUser(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("get user: %w", err)
    }

    // Check if already at or above requested level
    if user.KYCLevel >= level {
        return nil, domain.ErrKYCAlreadyVerified
    }

    // Create or get Sumsub applicant
    applicantID, err := s.sumsub.CreateApplicant(ctx, SumsubApplicant{
        ExternalUserID: fmt.Sprintf("%d", userID),
        Email:          user.Email,
        Phone:          user.Phone,
        Country:        user.CountryCode,
    })
    if err != nil {
        return nil, fmt.Errorf("create applicant: %w", err)
    }

    // Get SDK access token
    token, err := s.sumsub.GenerateAccessToken(ctx, applicantID, levelToFlow(level))
    if err != nil {
        return nil, fmt.Errorf("generate token: %w", err)
    }

    // Record verification attempt
    s.repo.CreateVerificationAttempt(ctx, &VerificationAttempt{
        UserID:      userID,
        Level:       level,
        ApplicantID: applicantID,
        Status:      "pending",
        StartedAt:   time.Now(),
    })

    return &StartVerificationResult{
        ApplicantID: applicantID,
        SDKToken:    token,
        FlowName:    levelToFlow(level),
    }, nil
}

// Process Sumsub webhook
func (s *KYCService) ProcessWebhook(ctx context.Context, payload []byte, signature string) error {
    // 1. Verify signature
    if !s.sumsub.VerifyWebhookSignature(payload, signature) {
        return domain.ErrInvalidWebhookSignature
    }

    // 2. Parse webhook
    var event SumsubWebhookEvent
    if err := json.Unmarshal(payload, &event); err != nil {
        return fmt.Errorf("parse webhook: %w", err)
    }

    // 3. Idempotency check
    if s.cache.HasProcessedWebhook(ctx, event.EventID) {
        return nil // already processed
    }

    // 4. Process based on type
    switch event.Type {
    case "applicantReviewed":
        return s.handleApplicantReviewed(ctx, &event)
    case "applicantPending":
        return s.handleApplicantPending(ctx, &event)
    default:
        log.Warn().Str("type", event.Type).Msg("Unknown webhook type")
        return nil
    }
}

func (s *KYCService) handleApplicantReviewed(ctx context.Context, event *SumsubWebhookEvent) error {
    userID, _ := strconv.ParseInt(event.ExternalUserID, 10, 64)

    if event.ReviewResult.ReviewAnswer == "GREEN" {
        // Approved
        level := flowToLevel(event.FlowName)
        
        // Update KYC level
        s.userClient.UpdateKYCLevel(ctx, userID, level)
        
        // Update limits
        s.updateUserLimits(ctx, userID, level)
        
        // Publish event
        s.producer.Publish(ctx, "kyc.verified", &KYCVerifiedEvent{
            UserID: userID,
            Level:  level,
        })
        
        log.Info().Int64("user_id", userID).Int("level", level).Msg("KYC approved")
    } else {
        // Rejected
        reason := event.ReviewResult.RejectLabels
        
        s.repo.UpdateVerificationStatus(ctx, userID, "rejected", reason)
        
        s.producer.Publish(ctx, "kyc.rejected", &KYCRejectedEvent{
            UserID: userID,
            Reason: strings.Join(reason, ", "),
        })
        
        log.Info().Int64("user_id", userID).Strs("reasons", reason).Msg("KYC rejected")
    }

    s.cache.MarkWebhookProcessed(ctx, event.EventID)
    return nil
}
```

# ============================================================
# SECTION 4: AML SCREENING
# ============================================================

```text
CHECKS:
  PEP — Politically Exposed Persons (government officials, family)
  Sanctions — OFAC, EU, UN sanctions lists
  Adverse Media — negative news articles
  
WHEN:
  On KYC Level 2 approval (initial screening)
  Daily batch re-screening (all active users)
  On suspicious activity flag (triggered by fraud engine)

PROVIDER: ComplyAdvantage API

RESULTS:
  no_match:     clear, allow normal operation
  potential_match: requires manual review by compliance
  true_match:   block account, file SAR (Suspicious Activity Report)
```

```go
func (s *AMLService) ScreenUser(ctx context.Context, userID int64) (*ScreeningResult, error) {
    user, _ := s.userClient.GetUser(ctx, userID)

    result, err := s.complyAdvantage.Search(ctx, ComplyAdvantageSearch{
        SearchTerm: user.FullName,
        Filters: ComplyAdvantageFilters{
            BirthYear:   user.BirthYear,
            CountryCodes: []string{user.CountryCode},
            Types:       []string{"pep", "sanction", "adverse-media"},
        },
    })
    if err != nil {
        return nil, fmt.Errorf("comply advantage search: %w", err)
    }

    screening := &ScreeningResult{
        UserID:    userID,
        Status:    "clear",
        Matches:   len(result.Hits),
        ScreenedAt: time.Now(),
    }

    if len(result.Hits) > 0 {
        screening.Status = "potential_match"
        screening.Details = result.Hits
        
        // Create compliance alert
        s.repo.CreateAlert(ctx, &ComplianceAlert{
            UserID:   userID,
            Type:     "aml_match",
            Severity: classifySeverity(result.Hits),
            Details:  result.Hits,
            Status:   "pending_review",
        })
    }

    s.repo.SaveScreening(ctx, screening)
    return screening, nil
}
```

# ============================================================
# SECTION 5: DOCUMENT STORAGE
# ============================================================

```text
STORAGE: S3 bucket "platform-kyc-documents"
ENCRYPTION: SSE-KMS (AWS managed key)
ACCESS: Only KYC service + compliance team (IAM policy)
RETENTION: 5-7 years per jurisdiction requirement
DELETION: Automated after retention period via S3 lifecycle

KEY STRUCTURE: kyc/{user_id}/{document_type}/{timestamp}_{hash}.ext
EXAMPLE: kyc/12345/passport/2025-01-15_abc123.jpg

RULES:
  - Documents NEVER served to frontend after upload
  - Compliance team views via admin panel (pre-signed URL, 5 min expiry)
  - All document access is audit-logged
  - GDPR: user can request copy of their documents (data portability)
  - GDPR: documents deleted on account closure (after retention period)
```

# ============================================================
# SECTION 6: ANTI-PATTERNS
# ============================================================

```text
❌ NEVER store KYC documents in PostgreSQL (use S3 with encryption)
❌ NEVER skip webhook signature verification (spoofing risk)
❌ NEVER allow deposit/withdrawal above KYC level limits
❌ NEVER expose Sumsub applicant ID to end user
❌ NEVER cache KYC level longer than 1 minute (level can change)
❌ NEVER log document contents or full ID numbers
❌ NEVER allow re-verification bypass (user must go through Sumsub again)
❌ NEVER skip AML screening on KYC Level 2+ users
❌ NEVER auto-approve Level 4 (always manual compliance review)
❌ NEVER delete screening results (immutable audit trail)
```

# ============================================================
# SECTION 7: TESTING
# ============================================================

```text
MUST TEST:
  ✅ KYC flow: start → Sumsub SDK → webhook → level updated
  ✅ Rejected KYC: webhook with RED result → user notified, limits unchanged
  ✅ Limits enforced per KYC level (deposit + withdrawal)
  ✅ AML screening returns matches → compliance alert created
  ✅ Webhook idempotency: duplicate webhook processed once
  ✅ Document upload → S3 encrypted, accessible only via pre-signed URL
  ✅ Re-verification: user can retry after rejection
  ✅ Daily batch re-screening runs without errors
  ✅ GDPR data export includes KYC documents
```
```

---

