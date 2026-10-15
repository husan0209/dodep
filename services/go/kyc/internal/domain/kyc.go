package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// KYCLevel is the verification tier of a user.
// NEVER skip levels: upgrades move exactly one step at a time.
type KYCLevel int

const (
	LevelNone     KYCLevel = 0 // Registration only, no deposits
	LevelBasic    KYCLevel = 1 // Email + phone verified (external, via user/auth service)
	LevelIdentity KYCLevel = 2 // Government ID + selfie verified
	LevelEnhanced KYCLevel = 3 // Identity + proof of address verified
	LevelVIP      KYCLevel = 4 // Enhanced + source of funds + manual compliance review
)

// IsValid reports whether the level is a known tier.
func (l KYCLevel) IsValid() bool {
	return l >= LevelNone && l <= LevelVIP
}

// String returns the canonical level name.
func (l KYCLevel) String() string {
	switch l {
	case LevelNone:
		return "none"
	case LevelBasic:
		return "basic"
	case LevelIdentity:
		return "identity"
	case LevelEnhanced:
		return "enhanced"
	case LevelVIP:
		return "vip"
	default:
		return "unknown"
	}
}

// CanUpgradeTo enforces stepwise upgrades: exactly one level up, never down,
// never skipping. Basic is granted without documents (email+phone are verified
// externally); Identity and above require approved documents.
func (l KYCLevel) CanUpgradeTo(target KYCLevel) bool {
	if !l.IsValid() || !target.IsValid() {
		return false
	}
	return target == l+1
}

// VerificationStatus tracks the lifecycle of a user's KYC record.
type VerificationStatus string

const (
	StatusNotStarted VerificationStatus = "not_started"
	StatusInProgress VerificationStatus = "in_progress"
	StatusPending    VerificationStatus = "pending_review"
	StatusVerified   VerificationStatus = "verified"
	StatusRejected   VerificationStatus = "rejected"
	StatusExpired    VerificationStatus = "expired"
)

// IsValid reports whether the status is known.
func (s VerificationStatus) IsValid() bool {
	switch s {
	case StatusNotStarted, StatusInProgress, StatusPending,
		StatusVerified, StatusRejected, StatusExpired:
		return true
	}
	return false
}

// IsTerminal reports whether no further user action is expected
// (verified is terminal until a document expires; expired forces re-verification).
func (s VerificationStatus) IsTerminal() bool {
	return s == StatusVerified || s == StatusRejected || s == StatusExpired
}

// DocumentType classifies the KYC document.
type DocumentType string

const (
	DocPassport        DocumentType = "passport"
	DocDriversLicense  DocumentType = "drivers_license"
	DocNationalID      DocumentType = "national_id"
	DocResidencePermit DocumentType = "residence_permit"
	DocProofOfAddress  DocumentType = "proof_of_address"
	DocBankStatement   DocumentType = "bank_statement"
	DocSelfie          DocumentType = "selfie"
	DocSourceOfFunds   DocumentType = "source_of_funds"
)

// IsValid reports whether the document type is known.
func (t DocumentType) IsValid() bool {
	switch t {
	case DocPassport, DocDriversLicense, DocNationalID, DocResidencePermit,
		DocProofOfAddress, DocBankStatement, DocSelfie, DocSourceOfFunds:
		return true
	}
	return false
}

// IsIdentityDoc reports whether the type proves identity (photo ID).
func (t DocumentType) IsIdentityDoc() bool {
	switch t {
	case DocPassport, DocDriversLicense, DocNationalID, DocResidencePermit:
		return true
	}
	return false
}

// IsAddressDoc reports whether the type proves residence.
func (t DocumentType) IsAddressDoc() bool {
	return t == DocProofOfAddress || t == DocBankStatement
}

// RequiresNumber reports whether a document number is mandatory.
// Photo IDs and bank statements carry an identifier; selfies and utility
// bills used as address proof may not.
func (t DocumentType) RequiresNumber() bool {
	switch t {
	case DocPassport, DocDriversLicense, DocNationalID, DocResidencePermit, DocBankStatement:
		return true
	}
	return false
}

// RequiresExpiry reports whether the document itself expires
// (passport, licence, permits). Selfies and proofs of address rely on
// submission freshness instead (see AddressDocFreshnessDays).
func (t DocumentType) RequiresExpiry() bool {
	return t.IsIdentityDoc()
}

// DocumentSide identifies which side of a physical document was captured.
type DocumentSide string

const (
	SideFront  DocumentSide = "front"
	SideBack   DocumentSide = "back"
	SideSingle DocumentSide = "single"
)

// IsValid reports whether the side is known.
func (s DocumentSide) IsValid() bool {
	return s == SideFront || s == SideBack || s == SideSingle
}

// DocumentStatus tracks the review lifecycle of a single document.
type DocumentStatus string

const (
	DocDraft     DocumentStatus = "draft"
	DocSubmitted DocumentStatus = "submitted"
	DocInReview  DocumentStatus = "in_review"
	DocApproved  DocumentStatus = "approved"
	DocRejected  DocumentStatus = "rejected"
	DocExpired   DocumentStatus = "expired"
)

// IsValid reports whether the document status is known.
func (s DocumentStatus) IsValid() bool {
	switch s {
	case DocDraft, DocSubmitted, DocInReview, DocApproved, DocRejected, DocExpired:
		return true
	}
	return false
}

// CanTransitionTo enforces the document state machine.
// Approved and expired are terminal; rejected documents must be re-submitted
// as new documents (immutable audit trail — NEVER rewrite history).
func (s DocumentStatus) CanTransitionTo(target DocumentStatus) bool {
	transitions := map[DocumentStatus][]DocumentStatus{
		DocDraft:     {DocSubmitted},
		DocSubmitted: {DocInReview, DocApproved, DocRejected},
		DocInReview:  {DocApproved, DocRejected},
		DocRejected:  {},
		DocApproved:  {DocExpired},
		DocExpired:   {},
	}
	allowed, ok := transitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// AddressDocFreshnessDays is how long a proof of address counts as fresh.
// Older address documents no longer satisfy Enhanced requirements.
const AddressDocFreshnessDays = 90

// KycRecord is the per-user verification state (one row per user).
// Money never appears here — only tier + status. Limits are derived
// from Level via LimitsForLevel (no float, decimal strings at the edge).
type KycRecord struct {
	UserID          int64
	Level           KYCLevel
	TargetLevel     KYCLevel
	Status          VerificationStatus
	RejectionReason string
	ScreeningStatus ScreeningStatus
	SubmittedAt     *time.Time
	VerifiedAt      *time.Time
	LastReviewedAt  *time.Time
	ReviewedBy      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Document is a single submitted KYC document.
// DocumentNumber is stored encrypted at rest (see internal/crypto);
// NumberHash is a SHA-256 search hash used for cross-user duplicate detection.
type Document struct {
	ID              uuid.UUID
	UserID          int64
	Type            DocumentType
	Side            DocumentSide
	Status          DocumentStatus
	FileID          string
	DocumentNumber  string // plaintext in memory only; encrypted before persistence
	NumberHash      string // SHA-256 hex for duplicate detection
	IssuingCountry  string // ISO 3166-1 alpha-2, uppercase
	ExpiresAt       *time.Time
	SubmittedAt     time.Time
	ReviewedAt      *time.Time
	ReviewedBy      string
	RejectionReason string
	RequiredActions []string
	ExtractedData   map[string]string
	Notes           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsExpired reports whether the document itself has expired as of now
// (passport expiry etc.). Proofs of address use IsFresh instead.
func (d *Document) IsExpired(now time.Time) bool {
	if d.ExpiresAt == nil {
		return false
	}
	return !now.Before(*d.ExpiresAt)
}

// IsFresh reports whether an address document was submitted recently enough
// to count towards Enhanced verification.
func (d *Document) IsFresh(now time.Time) bool {
	if !d.Type.IsAddressDoc() {
		return true
	}
	cutoff := now.AddDate(0, 0, -AddressDocFreshnessDays)
	return !d.SubmittedAt.Before(cutoff)
}

// CountsAsValid reports whether an approved document currently satisfies
// level requirements (approved, unexpired, and fresh for address docs).
func (d *Document) CountsAsValid(now time.Time) bool {
	if d.Status != DocApproved {
		return false
	}
	if d.IsExpired(now) {
		return false
	}
	return d.IsFresh(now)
}

// ValidateSubmit checks a new submission before persistence.
// user_id always comes from JWT (NEVER from body) — enforced in handlers.
func (d *Document) ValidateSubmit(now time.Time) error {
	var fields []FieldError

	if d.UserID <= 0 {
		fields = append(fields, FieldError{Field: "user_id", Message: "must be positive"})
	}
	if !d.Type.IsValid() {
		fields = append(fields, FieldError{Field: "document_type", Message: "unknown document type"})
	}
	if !d.Side.IsValid() {
		fields = append(fields, FieldError{Field: "document_side", Message: "must be front, back or single"})
	}
	if strings.TrimSpace(d.FileID) == "" {
		fields = append(fields, FieldError{Field: "file_id", Message: "file reference is required"})
	} else if len(d.FileID) > 255 {
		fields = append(fields, FieldError{Field: "file_id", Message: "must be at most 255 characters"})
	}
	if d.Type.RequiresNumber() && strings.TrimSpace(d.DocumentNumber) == "" {
		fields = append(fields, FieldError{Field: "document_number", Message: "document number is required for this document type"})
	}
	if len(d.DocumentNumber) > 64 {
		fields = append(fields, FieldError{Field: "document_number", Message: "must be at most 64 characters"})
	}
	if d.IssuingCountry != "" {
		if len(d.IssuingCountry) != 2 || d.IssuingCountry != strings.ToUpper(d.IssuingCountry) {
			fields = append(fields, FieldError{Field: "issuing_country", Message: "must be ISO 3166-1 alpha-2 uppercase"})
		}
	} else if d.Type.IsIdentityDoc() {
		fields = append(fields, FieldError{Field: "issuing_country", Message: "issuing country is required for identity documents"})
	}
	if d.Type.RequiresExpiry() {
		if d.ExpiresAt == nil {
			fields = append(fields, FieldError{Field: "expiry_date", Message: "expiry date is required for identity documents"})
		} else if !now.Before(*d.ExpiresAt) {
			fields = append(fields, FieldError{Field: "expiry_date", Message: "document is already expired"})
		}
	} else if d.ExpiresAt != nil && !now.Before(*d.ExpiresAt) {
		fields = append(fields, FieldError{Field: "expiry_date", Message: "expiry date must be in the future"})
	}

	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}

// ComputeLevel derives the highest level satisfied by valid documents.
// Levels are cumulative: Enhanced implies Identity requirements, etc.
func ComputeLevel(docs []*Document, now time.Time) KYCLevel {
	var hasID, hasSelfie, hasAddress, hasSoF bool
	for _, d := range docs {
		if d == nil || !d.CountsAsValid(now) {
			continue
		}
		switch {
		case d.Type.IsIdentityDoc():
			hasID = true
		case d.Type == DocSelfie:
			hasSelfie = true
		case d.Type.IsAddressDoc():
			hasAddress = true
		case d.Type == DocSourceOfFunds:
			hasSoF = true
		}
	}
	if hasID && hasSelfie && hasAddress && hasSoF {
		return LevelVIP
	}
	if hasID && hasSelfie && hasAddress {
		return LevelEnhanced
	}
	if hasID && hasSelfie {
		return LevelIdentity
	}
	return LevelNone
}

// RequiredDocTypes returns the document types still missing for the target level,
// given the currently valid documents.
func RequiredDocTypes(target KYCLevel, docs []*Document, now time.Time) []DocumentType {
	need := map[DocumentType]bool{}
	switch target {
	case LevelIdentity:
		need["identity"] = true
		need[DocSelfie] = true
	case LevelEnhanced:
		need["identity"] = true
		need[DocSelfie] = true
		need["address"] = true
	case LevelVIP:
		need["identity"] = true
		need[DocSelfie] = true
		need["address"] = true
		need[DocSourceOfFunds] = true
	default:
		return nil
	}
	for _, d := range docs {
		if d == nil || !d.CountsAsValid(now) {
			continue
		}
		switch {
		case d.Type.IsIdentityDoc():
			delete(need, "identity")
		case d.Type == DocSelfie:
			delete(need, DocSelfie)
		case d.Type.IsAddressDoc():
			delete(need, "address")
		case d.Type == DocSourceOfFunds:
			delete(need, DocSourceOfFunds)
		}
	}
	var out []DocumentType
	if need["identity"] {
		out = append(out, DocPassport)
	}
	if need[DocSelfie] {
		out = append(out, DocSelfie)
	}
	if need["address"] {
		out = append(out, DocProofOfAddress)
	}
	if need[DocSourceOfFunds] {
		out = append(out, DocSourceOfFunds)
	}
	return out
}

// ScreeningStatus is the outcome of a PEP/sanctions screening.
type ScreeningStatus string

const (
	ScreeningClear          ScreeningStatus = "clear"
	ScreeningPotentialMatch ScreeningStatus = "potential_match"
	ScreeningTrueMatch      ScreeningStatus = "true_match"
	ScreeningPending        ScreeningStatus = "pending"
)

// IsValid reports whether the screening status is known.
func (s ScreeningStatus) IsValid() bool {
	switch s {
	case ScreeningClear, ScreeningPotentialMatch, ScreeningTrueMatch, ScreeningPending:
		return true
	}
	return false
}

// BlocksUpgrade reports whether the screening outcome forbids automatic
// level upgrades (manual compliance review required).
func (s ScreeningStatus) BlocksUpgrade() bool {
	return s == ScreeningPotentialMatch || s == ScreeningTrueMatch
}

// Screening is one PEP/sanctions/adverse-media check for a user.
type Screening struct {
	ID         uuid.UUID
	UserID     int64
	Provider   string // e.g. "complyadvantage-stub"
	Status     ScreeningStatus
	Matches    int
	Details    string // summary; full hit payload stays with the provider
	ScreenedAt time.Time
	ResolvedAt *time.Time
	ResolvedBy string
	Resolution string
	CreatedAt  time.Time
}

// AuditEntry is an immutable record of every KYC state change.
type AuditEntry struct {
	ID         int64
	UserID     int64
	Actor      string // user id, admin id, or "system"/"webhook:sumsub"
	Action     string
	DocumentID string
	OldStatus  string
	NewStatus  string
	Reason     string
	CreatedAt  time.Time
}

// Audit actions recorded in kyc_audit_log.
const (
	AuditSubmitDocument   = "document.submitted"
	AuditDeleteDocument   = "document.deleted"
	AuditVerifyDocument   = "document.verified"
	AuditRejectDocument   = "document.rejected"
	AuditExpireDocument   = "document.expired"
	AuditLevelUpgraded    = "level.upgraded"
	AuditLevelDowngraded  = "level.downgraded"
	AuditUpgradeRequest   = "level.upgrade_requested"
	AuditScreeningRun     = "screening.completed"
	AuditScreeningResolve = "screening.resolved"
	AuditWebhook          = "webhook.processed"
)

// Limits describes per-level transaction limits as decimal strings.
// Decimal strings on the wire — NEVER float (CONVENTIONS NEVER-6).
type Limits struct {
	DailyDeposit      string
	DailyWithdrawal   string
	MonthlyDeposit    string
	MonthlyWithdrawal string
	MaxBet            string
	CanTradeCrypto    bool
	CanUseBonus       bool
}

// LimitsForLevel returns the transaction limits for a verification tier.
// VIP limits are high but finite; truly custom VIP terms are negotiated
// out of band (remaining work, see docs/api/kyc.md).
func LimitsForLevel(l KYCLevel) Limits {
	usd := func(v string) string { return v }
	switch l {
	case LevelBasic:
		return Limits{
			DailyDeposit: usd("200"), DailyWithdrawal: usd("0"),
			MonthlyDeposit: usd("1000"), MonthlyWithdrawal: usd("0"),
			MaxBet: usd("50"), CanTradeCrypto: false, CanUseBonus: false,
		}
	case LevelIdentity:
		return Limits{
			DailyDeposit: usd("5000"), DailyWithdrawal: usd("2000"),
			MonthlyDeposit: usd("20000"), MonthlyWithdrawal: usd("10000"),
			MaxBet: usd("500"), CanTradeCrypto: false, CanUseBonus: true,
		}
	case LevelEnhanced:
		return Limits{
			DailyDeposit: usd("50000"), DailyWithdrawal: usd("20000"),
			MonthlyDeposit: usd("500000"), MonthlyWithdrawal: usd("200000"),
			MaxBet: usd("5000"), CanTradeCrypto: true, CanUseBonus: true,
		}
	case LevelVIP:
		return Limits{
			DailyDeposit: usd("500000"), DailyWithdrawal: usd("200000"),
			MonthlyDeposit: usd("5000000"), MonthlyWithdrawal: usd("2000000"),
			MaxBet: usd("50000"), CanTradeCrypto: true, CanUseBonus: true,
		}
	default:
		return Limits{
			DailyDeposit: usd("0"), DailyWithdrawal: usd("0"),
			MonthlyDeposit: usd("0"), MonthlyWithdrawal: usd("0"),
			MaxBet: usd("0"), CanTradeCrypto: false, CanUseBonus: false,
		}
	}
}
