package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/repository"
	"github.com/opus-casino/kyc/internal/screening"
)

// KYCService holds all KYC business logic.
// It depends on repository/cache/provider interfaces — never on concrete
// transports — and never knows about HTTP or gRPC.
type KYCService struct {
	repo          repository.KYCRepository
	cache         cache.StatusCache
	screener      screening.ScreeningProvider
	crypt         *crypto.Service
	webhookSecret string
	log           *zap.Logger
	now           func() time.Time
	// events is the optional Redpanda publisher for kyc.verified/kyc.rejected.
	// Nil by default (no-op); wired via SetEventPublisher. Publishing is
	// best-effort and never fails a review decision.
	events EventPublisher
	// overrides fronts kyc_limit_overrides. In-memory by default (per-pod,
	// dev/test); production swaps a repo-backed store via SetOverrideStore.
	overrides LimitOverrideStore
}

// EventPublisher publishes review-outcome events (kyc.verified/kyc.rejected).
// The Redpanda producer implements this; tests use an in-memory fake.
type EventPublisher interface {
	PublishKYCEvent(ctx context.Context, topic string, event domain.KYCStatusEvent) error
}

// SetEventPublisher attaches the review-outcome publisher. Nil detaches.
func (s *KYCService) SetEventPublisher(p EventPublisher) {
	s.events = p
}

// publishKYCEvent emits best-effort: failures are logged, never returned.
func (s *KYCService) publishKYCEvent(ctx context.Context, topic string, userID int64, level domain.KYCLevel, status domain.VerificationStatus, reason string) {
	if s.events == nil {
		return
	}
	event := domain.NewKYCStatusEvent(userID, level, status, reason, s.now())
	if err := s.events.PublishKYCEvent(ctx, topic, event); err != nil {
		s.log.Warn("kyc: outcome event publish failed (best-effort)",
			zap.String("topic", topic), zap.Int64("user_id", userID), zap.Error(err))
	}
}

// LimitOverrideStore persists custom VIP terms (kyc_limit_overrides).
// The GORM repository gains a backed implementation when the table is cut
// over (see migrations/001_kyc.up.sql); until then memory serves dev/test.
type LimitOverrideStore interface {
	GetOverride(ctx context.Context, userID int64) (*domain.LimitOverride, error)
	UpsertOverride(ctx context.Context, o *domain.LimitOverride) error
}

// memoryOverrideStore is the per-pod default LimitOverrideStore.
type memoryOverrideStore struct {
	mu sync.Mutex
	m  map[int64]*domain.LimitOverride
}

func newMemoryOverrideStore() *memoryOverrideStore {
	return &memoryOverrideStore{m: map[int64]*domain.LimitOverride{}}
}

func (s *memoryOverrideStore) GetOverride(_ context.Context, userID int64) (*domain.LimitOverride, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.m[userID]; ok {
		cp := *o
		return &cp, nil
	}
	return nil, nil
}

func (s *memoryOverrideStore) UpsertOverride(_ context.Context, o *domain.LimitOverride) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *o
	cp.UpdatedAt = time.Now()
	s.m[o.UserID] = &cp
	return nil
}

// SetOverrideStore swaps the limit-override backend (e.g. repo-backed).
// Nil is ignored (keeps the current store).
func (s *KYCService) SetOverrideStore(st LimitOverrideStore) {
	if st != nil {
		s.overrides = st
	}
}

// UpsertLimitOverride stores custom VIP terms. Admin-only: callerRole must be
// "admin" (mirrors the JWT is_admin claim enforced in middleware.go).
// Every upsert is audit-logged and invalidates the status cache.
func (s *KYCService) UpsertLimitOverride(ctx context.Context, callerRole string, o *domain.LimitOverride) error {
	if !strings.EqualFold(strings.TrimSpace(callerRole), "admin") {
		return domain.ErrForbidden
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if err := s.overrides.UpsertOverride(ctx, o); err != nil {
		return fmt.Errorf("kyc: save limit override: %w", err)
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: o.UserID, Actor: "admin:" + strings.TrimSpace(o.UpdatedBy),
		Action: domain.AuditLimitOverride, NewStatus: "custom",
	})
	s.cacheInvalidate(ctx, o.UserID)
	s.log.Info("kyc: limit override upserted",
		zap.Int64("user_id", o.UserID), zap.String("by", o.UpdatedBy))
	return nil
}

// EffectiveLimits returns the limits in force for a user: base tier from the
// record level merged with any custom override (override wins per field).
func (s *KYCService) EffectiveLimits(ctx context.Context, userID int64) (domain.Limits, error) {
	if userID <= 0 {
		return domain.Limits{}, domain.ErrInvalidUserID
	}
	level := domain.LevelNone
	if rec, err := s.repo.GetRecord(ctx, userID); err != nil {
		return domain.Limits{}, fmt.Errorf("kyc: get record for limits: %w", err)
	} else if rec != nil {
		level = rec.Level
	}
	var o *domain.LimitOverride
	if s.overrides != nil {
		var err error
		o, err = s.overrides.GetOverride(ctx, userID)
		if err != nil {
			return domain.Limits{}, fmt.Errorf("kyc: get limit override: %w", err)
		}
	}
	return domain.EffectiveLimits(level, o), nil
}

// Config wires optional collaborators.
type Config struct {
	WebhookSecret string
}

// NewKYCService creates the service. Nil collaborators are rejected fast,
// except cache (nil cache disables caching explicitly, used in tests).
func NewKYCService(repo repository.KYCRepository, c cache.StatusCache, screener screening.ScreeningProvider, crypt *crypto.Service, cfg Config, log *zap.Logger) *KYCService {
	if repo == nil {
		panic("kyc: repository is required")
	}
	if screener == nil {
		panic("kyc: screening provider is required")
	}
	if crypt == nil {
		panic("kyc: crypto service is required")
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &KYCService{
		repo: repo, cache: c, screener: screener, crypt: crypt,
		webhookSecret: cfg.WebhookSecret, log: log, now: time.Now,
		overrides: newMemoryOverrideStore(),
	}
}

// ReviewSubject carries PII for PEP/sanctions screening at promotion time.
// It is passed in memory only — full hit payloads stay with the provider.
type ReviewSubject struct {
	FullName    string
	BirthYear   int
	CountryCode string
}

// ─── Status / level reads ───

// Status aggregates record + documents for status endpoints.
type Status struct {
	Record    *domain.KycRecord
	Documents []*domain.Document
}

// GetStatus returns the user's verification snapshot.
// Cache-aside: 60s TTL; every mutation invalidates. Expiry is applied lazily
// so reads never serve a level whose backing document has expired.
func (s *KYCService) GetStatus(ctx context.Context, userID int64) (*Status, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	if _, err := s.applyExpiryLazy(ctx, userID); err != nil {
		s.log.Warn("kyc: lazy expiry sweep failed", zap.Error(err), zap.Int64("user_id", userID))
	}
	rec, err := s.repo.GetRecord(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("kyc: get record: %w", err)
	}
	if rec == nil {
		rec = &domain.KycRecord{UserID: userID, Status: domain.StatusNotStarted, ScreeningStatus: domain.ScreeningClear}
	}
	docs, _, _, err := s.repo.ListDocuments(ctx, userID, nil, nil, 100, "")
	if err != nil {
		return nil, fmt.Errorf("kyc: list documents: %w", err)
	}
	for _, d := range docs {
		s.decryptInPlace(d)
	}
	s.cacheSet(ctx, rec)
	return &Status{Record: rec, Documents: docs}, nil
}

// GetVerificationLevel returns the current tier plus pending requirements.
func (s *KYCService) GetVerificationLevel(ctx context.Context, userID int64) (*Status, []domain.DocumentType, error) {
	st, err := s.GetStatus(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	target := st.Record.Level + 1
	if target < domain.LevelBasic {
		target = domain.LevelBasic
	}
	var missing []domain.DocumentType
	if target > st.Record.Level && target <= domain.LevelVIP {
		missing = domain.RequiredDocTypes(target, st.Documents, s.now())
	}
	return st, missing, nil
}

// ─── Submission ───

// SubmitInput is a new document submission (user_id comes from JWT).
type SubmitInput struct {
	UserID         int64
	Type           domain.DocumentType
	Side           domain.DocumentSide
	FileID         string
	DocumentNumber string
	IssuingCountry string
	ExpiresAt      *time.Time
	Metadata       map[string]string
}

// SubmitDocument validates, duplicate-checks, encrypts PII, persists,
// advances the record state machine and invalidates the status cache.
func (s *KYCService) SubmitDocument(ctx context.Context, in SubmitInput) (*domain.Document, error) {
	now := s.now()
	doc := &domain.Document{
		ID:             uuid.New(),
		UserID:         in.UserID,
		Type:           in.Type,
		Side:           in.Side,
		Status:         domain.DocSubmitted,
		FileID:         strings.TrimSpace(in.FileID),
		DocumentNumber: strings.TrimSpace(in.DocumentNumber),
		IssuingCountry: strings.ToUpper(strings.TrimSpace(in.IssuingCountry)),
		ExpiresAt:      in.ExpiresAt,
		SubmittedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if doc.Side == "" {
		doc.Side = domain.SideSingle
	}
	if err := doc.ValidateSubmit(now); err != nil {
		return nil, err
	}

	// Cross-user duplicate detection on the search hash (same physical ID
	// used by two accounts = fraud signal). Per-user re-submission after
	// rejection is allowed — only other users conflict.
	if doc.DocumentNumber != "" {
		doc.NumberHash = crypto.HashForSearch(doc.DocumentNumber)
		if dup, err := s.repo.FindByNumberHash(ctx, doc.NumberHash, doc.UserID); err != nil {
			return nil, fmt.Errorf("kyc: duplicate check: %w", err)
		} else if dup != nil {
			s.log.Warn("kyc: document number reuse across users",
				zap.Int64("user_id", doc.UserID),
				zap.String("number_hash", doc.NumberHash))
			return nil, domain.ErrDocumentAlreadyUsed
		}
		enc, err := s.crypt.Encrypt(doc.DocumentNumber)
		if err != nil {
			return nil, fmt.Errorf("kyc: encrypt document number: %w", err)
		}
		doc.DocumentNumber = enc
	}

	if err := s.repo.CreateDocument(ctx, doc); err != nil {
		return nil, err
	}

	rec, err := s.ensureRecord(ctx, doc.UserID, now)
	if err != nil {
		return nil, err
	}
	old := rec.Status
	switch rec.Status {
	case domain.StatusNotStarted, domain.StatusExpired, domain.StatusRejected:
		rec.Status = domain.StatusInProgress
		rec.RejectionReason = ""
		rec.SubmittedAt = &now
		rec.UpdatedAt = now
	}
	// Any awaiting-review document moves the record to pending review.
	if rec.Status == domain.StatusInProgress {
		rec.Status = domain.StatusPending
	}
	if err := s.repo.UpsertRecord(ctx, rec); err != nil {
		return nil, fmt.Errorf("kyc: save record: %w", err)
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: doc.UserID, Actor: fmt.Sprintf("user:%d", doc.UserID),
		Action: domain.AuditSubmitDocument, DocumentID: doc.ID.String(),
		OldStatus: string(old), NewStatus: string(rec.Status),
	})
	s.cacheInvalidate(ctx, doc.UserID)

	s.log.Info("kyc: document submitted",
		zap.Int64("user_id", doc.UserID),
		zap.String("document_id", doc.ID.String()),
		zap.String("type", string(doc.Type)))
	out := *doc
	out.DocumentNumber = "" // NEVER return ciphertext or PII to the caller
	return &out, nil
}

// DeleteDocument removes an owner document that is not under review.
// Approved documents are immutable (audit trail) — delete is refused.
func (s *KYCService) DeleteDocument(ctx context.Context, userID int64, id uuid.UUID) error {
	doc, err := s.ownedDocument(ctx, userID, id)
	if err != nil {
		return err
	}
	switch doc.Status {
	case domain.DocSubmitted, domain.DocRejected, domain.DocDraft:
		// deletable
	default:
		return domain.ErrDocumentNotDeletable
	}
	if err := s.repo.DeleteDocument(ctx, id); err != nil {
		return err
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: userID, Actor: fmt.Sprintf("user:%d", userID),
		Action: domain.AuditDeleteDocument, DocumentID: id.String(),
		OldStatus: string(doc.Status), NewStatus: "deleted",
	})
	s.cacheInvalidate(ctx, userID)
	return nil
}

// ─── Review (admin) ───

// VerifyInput approves a document.
type VerifyInput struct {
	DocumentID    uuid.UUID
	VerifiedBy    string
	Notes         string
	ExtractedData map[string]string
	FullName      string // for PEP/sanctions screening on level change
	BirthYear     int
	CountryCode   string
}

// VerifyDocument approves a submitted document, recomputes the level and,
// on upgrade, runs PEP/sanctions screening before promoting the tier.
func (s *KYCService) VerifyDocument(ctx context.Context, in VerifyInput) (*domain.Document, error) {
	now := s.now()
	doc, err := s.repo.GetDocument(ctx, in.DocumentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, domain.ErrDocumentNotFound
	}
	if !doc.Status.CanTransitionTo(domain.DocApproved) {
		return nil, domain.ErrInvalidDocument
	}
	if strings.TrimSpace(in.VerifiedBy) == "" {
		return nil, domain.NewValidationError(domain.FieldError{Field: "verified_by", Message: "reviewer identity is required"})
	}
	doc.Status = domain.DocApproved
	reviewed := now
	doc.ReviewedAt = &reviewed
	doc.ReviewedBy = in.VerifiedBy
	doc.Notes = in.Notes
	if in.ExtractedData != nil {
		doc.ExtractedData = in.ExtractedData
	}
	if err := s.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: doc.UserID, Actor: "admin:" + in.VerifiedBy,
		Action: domain.AuditVerifyDocument, DocumentID: doc.ID.String(),
		OldStatus: "submitted", NewStatus: string(domain.DocApproved),
		Reason: in.Notes,
	})

	if err := s.recomputeAfterReview(ctx, doc.UserID, "admin:"+in.VerifiedBy, ReviewSubject{
		FullName: in.FullName, BirthYear: in.BirthYear, CountryCode: in.CountryCode,
	}); err != nil {
		return nil, err
	}
	// Outcome event: only a promotion that actually landed on verified is
	// published (screening holds stay silent until compliance resolves).
	if rec, err := s.repo.GetRecord(ctx, doc.UserID); err != nil {
		s.log.Warn("kyc: record reload for event failed", zap.Error(err), zap.Int64("user_id", doc.UserID))
	} else if rec != nil && rec.Status == domain.StatusVerified {
		s.publishKYCEvent(ctx, domain.TopicKYCVerified, doc.UserID, rec.Level, rec.Status, "")
	}
	s.decryptInPlace(doc)
	return doc, nil
}

// RejectInput rejects a document with reason + remediation steps.
type RejectInput struct {
	DocumentID      uuid.UUID
	RejectedBy      string
	Reason          string
	RequiredActions []string
}

// RejectDocument rejects a submitted document and updates the record status.
func (s *KYCService) RejectDocument(ctx context.Context, in RejectInput) (*domain.Document, error) {
	now := s.now()
	doc, err := s.repo.GetDocument(ctx, in.DocumentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, domain.ErrDocumentNotFound
	}
	if !doc.Status.CanTransitionTo(domain.DocRejected) {
		return nil, domain.ErrInvalidDocument
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, domain.NewValidationError(domain.FieldError{Field: "rejection_reason", Message: "rejection reason is required"})
	}
	doc.Status = domain.DocRejected
	doc.ReviewedAt = &now
	doc.ReviewedBy = in.RejectedBy
	doc.RejectionReason = in.Reason
	doc.RequiredActions = in.RequiredActions
	if err := s.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: doc.UserID, Actor: "admin:" + in.RejectedBy,
		Action: domain.AuditRejectDocument, DocumentID: doc.ID.String(),
		OldStatus: "submitted", NewStatus: string(domain.DocRejected),
		Reason: in.Reason,
	})

	if err := s.recomputeAfterReview(ctx, doc.UserID, "admin:"+in.RejectedBy, ReviewSubject{}); err != nil {
		return nil, err
	}
	// Outcome event: every rejection is published (notification + analytics).
	level := domain.LevelNone
	if rec, err := s.repo.GetRecord(ctx, doc.UserID); err != nil {
		s.log.Warn("kyc: record reload for event failed", zap.Error(err), zap.Int64("user_id", doc.UserID))
	} else if rec != nil {
		level = rec.Level
	}
	s.publishKYCEvent(ctx, domain.TopicKYCRejected, doc.UserID, level, domain.StatusRejected, in.Reason)
	s.decryptInPlace(doc)
	return doc, nil
}

// recomputeAfterReview refreshes the record after any review decision:
// level upgrades (screening-gated), rejection propagation, review stamping.
func (s *KYCService) recomputeAfterReview(ctx context.Context, userID int64, actor string, subject ReviewSubject) error {
	now := s.now()
	rec, err := s.ensureRecord(ctx, userID, now)
	if err != nil {
		return err
	}
	docs, _, _, err := s.repo.ListDocuments(ctx, userID, nil, nil, 100, "")
	if err != nil {
		return fmt.Errorf("kyc: list for recompute: %w", err)
	}
	pending := false
	for _, d := range docs {
		if d.Status == domain.DocSubmitted || d.Status == domain.DocInReview {
			pending = true
			break
		}
	}
	earned := domain.ComputeLevel(docs, now)
	oldLevel, oldStatus := rec.Level, rec.Status
	reviewed := now
	rec.LastReviewedAt = &reviewed
	rec.ReviewedBy = strings.TrimPrefix(actor, "admin:")
	rec.UpdatedAt = now

	switch {
	case earned > rec.Level:
		// Gate the promotion on PEP/sanctions screening for Identity+.
		if err := s.runScreeningGate(ctx, rec, subject, earned, actor); err != nil {
			return err
		}
	case earned < rec.Level:
		// Requirements lost (expiry handled elsewhere; here: admin rejected
		// a load-bearing document before promotion). Keep the achieved
		// level — NEVER silently downgrade on a single rejection — but
		// surface pending review while documents remain.
		if pending && rec.Status == domain.StatusVerified {
			rec.Status = domain.StatusPending
		}
	default:
		if pending && rec.Status != domain.StatusVerified {
			rec.Status = domain.StatusPending
		}
	}
	if rec.Level == domain.LevelNone && earned == domain.LevelNone {
		if hasRejection(docs) && !pending {
			rec.Status = domain.StatusRejected
			rec.RejectionReason = latestRejection(docs)
		} else if pending {
			rec.Status = domain.StatusPending
		}
	}
	if err := s.repo.UpsertRecord(ctx, rec); err != nil {
		return fmt.Errorf("kyc: save recomputed record: %w", err)
	}
	if rec.Level != oldLevel {
		action := domain.AuditLevelUpgraded
		if rec.Level < oldLevel {
			action = domain.AuditLevelDowngraded
		}
		_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
			UserID: userID, Actor: actor, Action: action,
			OldStatus: oldLevel.String(), NewStatus: rec.Level.String(),
		})
		s.log.Info("kyc: level changed",
			zap.Int64("user_id", userID),
			zap.String("from", oldLevel.String()),
			zap.String("to", rec.Level.String()))
	}
	_ = oldStatus
	s.cacheInvalidate(ctx, userID)
	return nil
}

// runScreeningGate executes PEP/sanctions screening and applies the outcome:
// clear → promote; potential/true match → hold at pending/rejected for
// manual compliance review (fail secure, NEVER auto-approve flagged users).
func (s *KYCService) runScreeningGate(ctx context.Context, rec *domain.KycRecord, subject ReviewSubject, earned domain.KYCLevel, actor string) error {
	now := s.now()
	res, err := s.screener.Screen(ctx, screening.SearchRequest{
		UserID:      rec.UserID,
		FullName:    subject.FullName,
		BirthYear:   subject.BirthYear,
		CountryCode: subject.CountryCode,
	})
	if err != nil {
		// Provider failure fails closed: record stays, screening marked
		// pending, promotion deferred — NEVER promote on unknown risk.
		s.log.Error("kyc: screening provider failed", zap.Error(err), zap.Int64("user_id", rec.UserID))
		rec.Status = domain.StatusPending
		rec.ScreeningStatus = domain.ScreeningPending
		_ = s.repo.CreateScreening(ctx, &domain.Screening{
			ID: uuid.New(), UserID: rec.UserID,
			Provider: s.screener.Name(), Status: domain.ScreeningPending,
			Details: "provider error: " + err.Error(), ScreenedAt: now,
		})
		return nil
	}
	sc := &domain.Screening{
		ID: uuid.New(), UserID: rec.UserID,
		Provider: s.screener.Name(), Status: res.Status,
		Matches: res.Matches, Details: res.Summary, ScreenedAt: now,
	}
	_ = s.repo.CreateScreening(ctx, sc)
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: rec.UserID, Actor: actor, Action: domain.AuditScreeningRun,
		OldStatus: string(rec.ScreeningStatus), NewStatus: string(res.Status),
		Reason: res.Summary,
	})
	rec.ScreeningStatus = res.Status

	switch res.Status {
	case domain.ScreeningClear:
		rec.Level = earned
		if earned >= rec.TargetLevel && rec.TargetLevel > domain.LevelNone {
			rec.TargetLevel = earned
		}
		rec.Status = domain.StatusVerified
		verified := now
		rec.VerifiedAt = &verified
		rec.RejectionReason = ""
	case domain.ScreeningPotentialMatch:
		rec.Status = domain.StatusPending
		s.log.Warn("kyc: screening potential match — manual review required",
			zap.Int64("user_id", rec.UserID), zap.String("screening_id", sc.ID.String()))
	case domain.ScreeningTrueMatch:
		rec.Status = domain.StatusRejected
		rec.RejectionReason = "sanctions_match_pending_sar"
		s.log.Error("kyc: screening TRUE match — account blocked for SAR",
			zap.Int64("user_id", rec.UserID), zap.String("screening_id", sc.ID.String()))
	default:
		rec.Status = domain.StatusPending
	}
	return nil
}

// ResolveScreening settles a flagged screening by compliance review.
func (s *KYCService) ResolveScreening(ctx context.Context, adminID string, screeningID uuid.UUID, approve bool, note string) (*domain.Screening, error) {
	now := s.now()
	sc, err := s.repo.GetScreening(ctx, screeningID)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return nil, domain.ErrScreeningNotFound
	}
	if sc.ResolvedAt != nil {
		return nil, domain.ErrScreeningNotPending
	}
	if strings.TrimSpace(adminID) == "" {
		return nil, domain.NewValidationError(domain.FieldError{Field: "resolved_by", Message: "reviewer identity is required"})
	}
	resolved := now
	sc.ResolvedAt = &resolved
	sc.ResolvedBy = adminID
	if approve {
		sc.Status = domain.ScreeningClear
		sc.Resolution = note
	} else {
		sc.Status = domain.ScreeningTrueMatch
		sc.Resolution = note
	}
	if err := s.repo.UpdateScreening(ctx, sc); err != nil {
		return nil, err
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: sc.UserID, Actor: "admin:" + adminID,
		Action:    domain.AuditScreeningResolve,
		OldStatus: "flagged", NewStatus: string(sc.Status), Reason: note,
	})

	// On clearance, retry the earned promotion immediately.
	rec, err := s.ensureRecord(ctx, sc.UserID, now)
	if err != nil {
		return sc, err
	}
	if approve {
		docs, _, _, err := s.repo.ListDocuments(ctx, sc.UserID, nil, nil, 100, "")
		if err == nil {
			if earned := domain.ComputeLevel(docs, now); earned > rec.Level {
				old := rec.Level
				rec.Level = earned
				rec.TargetLevel = earned
				rec.Status = domain.StatusVerified
				rec.ScreeningStatus = domain.ScreeningClear
				rec.VerifiedAt = &now
				rec.RejectionReason = ""
				_ = s.repo.UpsertRecord(ctx, rec)
				_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
					UserID: sc.UserID, Actor: "admin:" + adminID,
					Action:    domain.AuditLevelUpgraded,
					OldStatus: old.String(), NewStatus: earned.String(),
					Reason: "screening cleared on manual review",
				})
			} else {
				rec.ScreeningStatus = domain.ScreeningClear
				_ = s.repo.UpsertRecord(ctx, rec)
			}
		}
	} else {
		rec.Status = domain.StatusRejected
		rec.RejectionReason = "sanctions_match_confirmed"
		rec.ScreeningStatus = domain.ScreeningTrueMatch
		_ = s.repo.UpsertRecord(ctx, rec)
	}
	s.cacheInvalidate(ctx, sc.UserID)
	return sc, nil
}

// ─── Upgrade requests ───

// RequestUpgrade registers intent to reach a higher tier.
// Exactly one step up; Identity+ returns the missing documents — the level
// itself changes only when approvals satisfy the requirements.
func (s *KYCService) RequestUpgrade(ctx context.Context, userID int64, target domain.KYCLevel) ([]domain.DocumentType, error) {
	now := s.now()
	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	if !target.IsValid() || target == domain.LevelNone {
		return nil, domain.NewValidationError(domain.FieldError{Field: "target_level", Message: "target level is invalid"})
	}
	rec, err := s.ensureRecord(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if target <= rec.Level {
		return nil, domain.ErrKYCAlreadyVerified
	}
	if !rec.Level.CanUpgradeTo(target) {
		return nil, domain.ErrInvalidLevelTransition
	}
	if rec.ScreeningStatus.BlocksUpgrade() {
		return nil, domain.ErrSanctionsMatch
	}
	if target == domain.LevelBasic {
		// Email+phone are verified externally (auth/user service); this call
		// records the externally-verified fact from a trusted internal caller.
		old := rec.Level
		rec.Level = domain.LevelBasic
		rec.TargetLevel = domain.LevelBasic
		rec.Status = domain.StatusVerified
		verified := now
		rec.VerifiedAt = &verified
		rec.UpdatedAt = now
		if err := s.repo.UpsertRecord(ctx, rec); err != nil {
			return nil, fmt.Errorf("kyc: save basic upgrade: %w", err)
		}
		_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
			UserID: userID, Actor: fmt.Sprintf("user:%d", userID),
			Action:    domain.AuditLevelUpgraded,
			OldStatus: old.String(), NewStatus: rec.Level.String(),
		})
		s.cacheInvalidate(ctx, userID)
		return nil, nil
	}
	rec.TargetLevel = target
	if rec.Status == domain.StatusVerified || rec.Status == domain.StatusNotStarted {
		rec.Status = domain.StatusInProgress
	}
	rec.UpdatedAt = now
	if err := s.repo.UpsertRecord(ctx, rec); err != nil {
		return nil, fmt.Errorf("kyc: save upgrade request: %w", err)
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: userID, Actor: fmt.Sprintf("user:%d", userID),
		Action: domain.AuditUpgradeRequest, NewStatus: target.String(),
	})
	s.cacheInvalidate(ctx, userID)

	docs, _, _, err := s.repo.ListDocuments(ctx, userID, nil, nil, 100, "")
	if err != nil {
		return nil, fmt.Errorf("kyc: list for requirements: %w", err)
	}
	return domain.RequiredDocTypes(target, docs, now), nil
}

// ─── Provider webhook (Sumsub-style) ───

// WebhookInput is a verified provider callback.
type WebhookInput struct {
	Provider       string
	EventID        string
	ExternalUserID int64
	ReviewAnswer   string // GREEN | RED
	Level          int
	RejectLabels   []string
	RawBody        []byte
	Signature      string
}

// ProcessWebhook verifies signature, enforces idempotency, and applies the
// provider decision to the record. Document-level detail arrives via the
// review API; the webhook settles the overall verification outcome.
func (s *KYCService) ProcessWebhook(ctx context.Context, in WebhookInput) error {
	now := s.now()
	if in.Provider != "sumsub" {
		return domain.ErrUnsupportedProvider
	}
	if !screening.VerifyWebhookSignature(in.RawBody, in.Signature, s.webhookSecret) {
		return domain.ErrInvalidWebhookSig
	}
	eventID := strings.TrimSpace(in.EventID)
	if eventID == "" {
		eventID = screening.WebhookEventID(in.Provider, in.RawBody)
	}
	if s.cache != nil {
		seen, err := s.cache.HasProcessedWebhook(ctx, eventID)
		if err != nil {
			return fmt.Errorf("kyc: webhook idempotency check: %w", err)
		}
		if seen {
			return domain.ErrDuplicateWebhook
		}
	}
	if in.ExternalUserID <= 0 {
		return domain.ErrInvalidUserID
	}
	rec, err := s.ensureRecord(ctx, in.ExternalUserID, now)
	if err != nil {
		return err
	}
	old := rec.Status
	reviewed := now
	rec.LastReviewedAt = &reviewed
	rec.ReviewedBy = "webhook:" + in.Provider
	rec.UpdatedAt = now

	switch strings.ToUpper(strings.TrimSpace(in.ReviewAnswer)) {
	case "GREEN":
		level := domain.KYCLevel(in.Level)
		if level.IsValid() && level > rec.Level {
			rec.Level = level
			rec.TargetLevel = level
		}
		rec.Status = domain.StatusVerified
		rec.VerifiedAt = &now
		rec.RejectionReason = ""
	case "RED":
		rec.Status = domain.StatusRejected
		rec.RejectionReason = strings.Join(in.RejectLabels, ", ")
	default:
		return domain.NewValidationError(domain.FieldError{Field: "review_answer", Message: "must be GREEN or RED"})
	}
	if err := s.repo.UpsertRecord(ctx, rec); err != nil {
		return fmt.Errorf("kyc: save webhook outcome: %w", err)
	}
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: in.ExternalUserID, Actor: "webhook:" + in.Provider,
		Action:    domain.AuditWebhook,
		OldStatus: string(old), NewStatus: string(rec.Status),
		Reason: string(mustJSON(in.RejectLabels)),
	})
	if s.cache != nil {
		_ = s.cache.MarkWebhookProcessed(ctx, eventID)
	}
	s.cacheInvalidate(ctx, in.ExternalUserID)
	s.log.Info("kyc: provider webhook applied",
		zap.Int64("user_id", in.ExternalUserID),
		zap.String("answer", in.ReviewAnswer))
	return nil
}

// ─── Expiry tracking ───

// ListPendingScreenings returns unresolved flagged screenings for compliance.
func (s *KYCService) ListPendingScreenings(ctx context.Context, limit int) ([]*domain.Screening, error) {
	return s.repo.ListPendingScreenings(ctx, limit)
}

// SweepExpired marks approved documents past their expiry and downgrades
// records whose tier no longer holds. Bounded batches; the admin endpoint
// (or scheduler) pages until no rows are affected.
func (s *KYCService) SweepExpired(ctx context.Context) (int64, error) {
	now := s.now()
	affected, err := s.repo.MarkExpiredBefore(ctx, now, 200)
	if err != nil {
		return 0, err
	}
	// Downgrade pass is record-driven; without a cheap "users with expired
	// docs" index we rely on lazy downgrade at read time (applyExpiryLazy).
	// The batch mark above is the cron-critical part.
	_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: 0, Actor: "system",
		Action: domain.AuditExpireDocument, NewStatus: string(domain.DocExpired),
		Reason: fmt.Sprintf("sweep marked %d documents expired", affected),
	})
	return affected, nil
}

// applyExpiryLazy downgrades a single user's record at read time, but ONLY
// when a load-bearing document has actually expired since the last write.
// Levels granted without local documents (Basic via external verification,
// provider webhooks) are never touched here — there is nothing to expire.
func (s *KYCService) applyExpiryLazy(ctx context.Context, userID int64) (bool, error) {
	now := s.now()
	rec, err := s.repo.GetRecord(ctx, userID)
	if err != nil || rec == nil || rec.Level == domain.LevelNone {
		return false, err
	}
	docs, _, _, err := s.repo.ListDocuments(ctx, userID, nil, nil, 100, "")
	if err != nil {
		return false, err
	}
	expiredAny := false
	for _, d := range docs {
		if d.Status == domain.DocApproved && d.IsExpired(now) {
			d.Status = domain.DocExpired
			if err := s.repo.UpdateDocument(ctx, d); err != nil {
				return expiredAny, err
			}
			expiredAny = true
			_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
				UserID: userID, Actor: "system",
				Action: domain.AuditExpireDocument, DocumentID: d.ID.String(),
				OldStatus: string(domain.DocApproved), NewStatus: string(domain.DocExpired),
			})
		}
	}
	if !expiredAny {
		return false, nil
	}
	if earned := domain.ComputeLevel(validDocs(docs), now); earned < rec.Level {
		old := rec.Level
		rec.Level = earned
		rec.Status = domain.StatusExpired
		rec.UpdatedAt = now
		if err := s.repo.UpsertRecord(ctx, rec); err != nil {
			return true, err
		}
		_ = s.repo.AppendAudit(ctx, &domain.AuditEntry{
			UserID: userID, Actor: "system", Action: domain.AuditLevelDowngraded,
			OldStatus: old.String(), NewStatus: earned.String(),
			Reason: "load-bearing document expired",
		})
		s.cacheInvalidate(ctx, userID)
		return true, nil
	}
	s.cacheInvalidate(ctx, userID)
	return true, nil
}

// ─── Helpers ───

func (s *KYCService) ensureRecord(ctx context.Context, userID int64, now time.Time) (*domain.KycRecord, error) {
	rec, err := s.repo.GetRecord(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("kyc: get record: %w", err)
	}
	if rec == nil {
		rec = &domain.KycRecord{
			UserID: userID, Status: domain.StatusNotStarted,
			ScreeningStatus: domain.ScreeningClear,
			CreatedAt:       now, UpdatedAt: now,
		}
		if err := s.repo.UpsertRecord(ctx, rec); err != nil {
			return nil, fmt.Errorf("kyc: create record: %w", err)
		}
	}
	if !rec.ScreeningStatus.IsValid() {
		rec.ScreeningStatus = domain.ScreeningClear
	}
	return rec, nil
}

func (s *KYCService) ownedDocument(ctx context.Context, userID int64, id uuid.UUID) (*domain.Document, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.UserID != userID {
		return nil, domain.ErrDocumentNotFound // owner mismatch → 404, never 403 oracle
	}
	s.decryptInPlace(doc)
	return doc, nil
}

// decryptInPlace replaces ciphertext with plaintext for service callers.
// Failures degrade to empty (never leak ciphertext upstream).
func (s *KYCService) decryptInPlace(d *domain.Document) {
	if d == nil || d.DocumentNumber == "" {
		return
	}
	if open, err := s.crypt.Decrypt(d.DocumentNumber); err == nil {
		d.DocumentNumber = open
	} else {
		d.DocumentNumber = ""
	}
}

func (s *KYCService) cacheSet(ctx context.Context, rec *domain.KycRecord) {
	if s.cache == nil {
		return
	}
	_ = s.cache.SetStatus(ctx, rec.UserID, cache.CachedStatus{
		Level: rec.Level, Status: rec.Status,
		ScreeningStatus: rec.ScreeningStatus, UpdatedAt: s.now(),
	})
}

func (s *KYCService) cacheInvalidate(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Invalidate(ctx, userID); err != nil {
		s.log.Warn("kyc: cache invalidate failed", zap.Error(err))
	}
}

func validDocs(docs []*domain.Document) []*domain.Document { return docs }

func hasRejection(docs []*domain.Document) bool {
	for _, d := range docs {
		if d.Status == domain.DocRejected {
			return true
		}
	}
	return false
}

func latestRejection(docs []*domain.Document) string {
	var latest *domain.Document
	for _, d := range docs {
		if d.Status != domain.DocRejected {
			continue
		}
		if latest == nil || d.UpdatedAt.After(latest.UpdatedAt) {
			latest = d
		}
	}
	if latest == nil {
		return ""
	}
	return latest.RejectionReason
}

func mustJSON(v []string) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
