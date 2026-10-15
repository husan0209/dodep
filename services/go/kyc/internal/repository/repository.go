package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opus-casino/kyc/internal/domain"
)

// KYCRepository abstracts all KYC persistence.
// The service layer depends on this interface, never on *gorm.DB.
type KYCRepository interface {
	// Records
	GetRecord(ctx context.Context, userID int64) (*domain.KycRecord, error)
	UpsertRecord(ctx context.Context, rec *domain.KycRecord) error

	// Documents
	CreateDocument(ctx context.Context, doc *domain.Document) error
	GetDocument(ctx context.Context, id uuid.UUID) (*domain.Document, error)
	ListDocuments(ctx context.Context, userID int64, docType *domain.DocumentType, status *domain.DocumentStatus, limit int, cursor string) ([]*domain.Document, string, bool, error)
	UpdateDocument(ctx context.Context, doc *domain.Document) error
	DeleteDocument(ctx context.Context, id uuid.UUID) error
	FindByNumberHash(ctx context.Context, numberHash string, excludeUserID int64) (*domain.Document, error)
	MarkExpiredBefore(ctx context.Context, now time.Time, limit int) (int64, error)

	// Screenings
	CreateScreening(ctx context.Context, s *domain.Screening) error
	GetScreening(ctx context.Context, id uuid.UUID) (*domain.Screening, error)
	ListPendingScreenings(ctx context.Context, limit int) ([]*domain.Screening, error)
	UpdateScreening(ctx context.Context, s *domain.Screening) error
	LatestScreening(ctx context.Context, userID int64) (*domain.Screening, error)

	// Audit (append-only; no update/delete methods by design)
	AppendAudit(ctx context.Context, e *domain.AuditEntry) error
	ListAudit(ctx context.Context, userID int64, limit int) ([]*domain.AuditEntry, error)
}

// ─── GORM models (repository layer only; converted at the boundary) ───

type kycRecordModel struct {
	UserID          int64  `gorm:"primaryKey"`
	Level           int    `gorm:"not null;default:0"`
	TargetLevel     int    `gorm:"not null;default:0"`
	Status          string `gorm:"not null;default:'not_started'"`
	RejectionReason string `gorm:"type:text;not null;default:''"`
	ScreeningStatus string `gorm:"not null;default:'clear'"`
	SubmittedAt     *time.Time
	VerifiedAt      *time.Time
	LastReviewedAt  *time.Time
	ReviewedBy      string `gorm:"type:varchar(128);not null;default:''"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName pins the table name (GORM pluralisation is not relied upon).
func (kycRecordModel) TableName() string { return "kyc_records" }

type documentModel struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID          int64     `gorm:"not null;index:idx_documents_user,priority:1"`
	Type            string    `gorm:"type:varchar(32);not null"`
	Side            string    `gorm:"type:varchar(16);not null"`
	Status          string    `gorm:"type:varchar(16);not null;index"`
	FileID          string    `gorm:"type:varchar(255);not null"`
	DocumentNumber  string    `gorm:"type:text;not null;default:''"` // AES-256-GCM ciphertext
	NumberHash      string    `gorm:"type:varchar(64);index"`
	IssuingCountry  string    `gorm:"type:char(2);not null;default:''"`
	ExpiresAt       *time.Time
	SubmittedAt     time.Time `gorm:"not null;index"`
	ReviewedAt      *time.Time
	ReviewedBy      string `gorm:"type:varchar(128);not null;default:''"`
	RejectionReason string `gorm:"type:text;not null;default:''"`
	RequiredActions string `gorm:"type:text;not null;default:'[]'"`
	ExtractedData   string `gorm:"type:text;not null;default:'{}'"`
	Notes           string `gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (documentModel) TableName() string { return "kyc_documents" }

type screeningModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     int64     `gorm:"not null;index"`
	Provider   string    `gorm:"type:varchar(64);not null"`
	Status     string    `gorm:"type:varchar(32);not null"`
	Matches    int       `gorm:"not null;default:0"`
	Details    string    `gorm:"type:text;not null;default:''"`
	ScreenedAt time.Time `gorm:"not null"`
	ResolvedAt *time.Time
	ResolvedBy string `gorm:"type:varchar(128);not null;default:''"`
	Resolution string `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time
}

func (screeningModel) TableName() string { return "kyc_screenings" }

type auditModel struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	UserID     int64     `gorm:"not null;index:idx_audit_user,priority:1"`
	Actor      string    `gorm:"type:varchar(128);not null"`
	Action     string    `gorm:"type:varchar(64);not null"`
	DocumentID string    `gorm:"type:varchar(64);not null;default:''"`
	OldStatus  string    `gorm:"type:varchar(64);not null;default:''"`
	NewStatus  string    `gorm:"type:varchar(64);not null;default:''"`
	Reason     string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time `gorm:"not null;index:idx_audit_user,priority:2"`
}

func (auditModel) TableName() string { return "kyc_audit_log" }

// AutoMigrate creates tables for a fresh database (dev/test).
// Production schema is owned by libs/migrations; this stays idempotent.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&kycRecordModel{}, &documentModel{}, &screeningModel{}, &auditModel{})
}

// ─── Conversions ───

func recordToDomain(m *kycRecordModel) *domain.KycRecord {
	return &domain.KycRecord{
		UserID:          m.UserID,
		Level:           domain.KYCLevel(m.Level),
		TargetLevel:     domain.KYCLevel(m.TargetLevel),
		Status:          domain.VerificationStatus(m.Status),
		RejectionReason: m.RejectionReason,
		ScreeningStatus: domain.ScreeningStatus(emptyDefault(m.ScreeningStatus, "clear")),
		SubmittedAt:     m.SubmittedAt,
		VerifiedAt:      m.VerifiedAt,
		LastReviewedAt:  m.LastReviewedAt,
		ReviewedBy:      m.ReviewedBy,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func recordToModel(r *domain.KycRecord) *kycRecordModel {
	return &kycRecordModel{
		UserID:          r.UserID,
		Level:           int(r.Level),
		TargetLevel:     int(r.TargetLevel),
		Status:          string(r.Status),
		RejectionReason: r.RejectionReason,
		ScreeningStatus: string(r.ScreeningStatus),
		SubmittedAt:     r.SubmittedAt,
		VerifiedAt:      r.VerifiedAt,
		LastReviewedAt:  r.LastReviewedAt,
		ReviewedBy:      r.ReviewedBy,
	}
}

func emptyDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func docToDomain(m *documentModel) *domain.Document {
	d := &domain.Document{
		ID:              m.ID,
		UserID:          m.UserID,
		Type:            domain.DocumentType(m.Type),
		Side:            domain.DocumentSide(m.Side),
		Status:          domain.DocumentStatus(m.Status),
		FileID:          m.FileID,
		DocumentNumber:  m.DocumentNumber, // ciphertext; service decrypts after load
		NumberHash:      m.NumberHash,
		IssuingCountry:  m.IssuingCountry,
		ExpiresAt:       m.ExpiresAt,
		SubmittedAt:     m.SubmittedAt,
		ReviewedAt:      m.ReviewedAt,
		ReviewedBy:      m.ReviewedBy,
		RejectionReason: m.RejectionReason,
		Notes:           m.Notes,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(emptyDefault(m.RequiredActions, "[]")), &d.RequiredActions)
	_ = json.Unmarshal([]byte(emptyDefault(m.ExtractedData, "{}")), &d.ExtractedData)
	if d.ExtractedData == nil {
		d.ExtractedData = map[string]string{}
	}
	return d
}

func docToModel(d *domain.Document) *documentModel {
	actions, _ := json.Marshal(d.RequiredActions)
	if len(actions) == 0 {
		actions = []byte("[]")
	}
	extracted, _ := json.Marshal(d.ExtractedData)
	if len(extracted) == 0 {
		extracted = []byte("{}")
	}
	return &documentModel{
		ID:              d.ID,
		UserID:          d.UserID,
		Type:            string(d.Type),
		Side:            string(d.Side),
		Status:          string(d.Status),
		FileID:          d.FileID,
		DocumentNumber:  d.DocumentNumber,
		NumberHash:      d.NumberHash,
		IssuingCountry:  d.IssuingCountry,
		ExpiresAt:       d.ExpiresAt,
		SubmittedAt:     d.SubmittedAt,
		ReviewedAt:      d.ReviewedAt,
		ReviewedBy:      d.ReviewedBy,
		RejectionReason: d.RejectionReason,
		RequiredActions: string(actions),
		ExtractedData:   string(extracted),
		Notes:           d.Notes,
	}
}

func screeningToDomain(m *screeningModel) *domain.Screening {
	return &domain.Screening{
		ID:         m.ID,
		UserID:     m.UserID,
		Provider:   m.Provider,
		Status:     domain.ScreeningStatus(m.Status),
		Matches:    m.Matches,
		Details:    m.Details,
		ScreenedAt: m.ScreenedAt,
		ResolvedAt: m.ResolvedAt,
		ResolvedBy: m.ResolvedBy,
		Resolution: m.Resolution,
		CreatedAt:  m.CreatedAt,
	}
}

func screeningToModel(s *domain.Screening) *screeningModel {
	return &screeningModel{
		ID:         s.ID,
		UserID:     s.UserID,
		Provider:   s.Provider,
		Status:     string(s.Status),
		Matches:    s.Matches,
		Details:    s.Details,
		ScreenedAt: s.ScreenedAt,
		ResolvedAt: s.ResolvedAt,
		ResolvedBy: s.ResolvedBy,
		Resolution: s.Resolution,
	}
}

// ─── Implementation ───

type gormKYCRepository struct {
	db *gorm.DB
}

// NewKYCRepository creates a GORM-backed repository.
func NewKYCRepository(db *gorm.DB) KYCRepository {
	return &gormKYCRepository{db: db}
}

func (r *gormKYCRepository) failFast() error {
	if r == nil || r.db == nil {
		return errors.New("kyc repository: nil database client")
	}
	return nil
}

func (r *gormKYCRepository) GetRecord(ctx context.Context, userID int64) (*domain.KycRecord, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var m kycRecordModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kyc: get record: %w", err)
	}
	return recordToDomain(&m), nil
}

func (r *gormKYCRepository) UpsertRecord(ctx context.Context, rec *domain.KycRecord) error {
	if err := r.failFast(); err != nil {
		return err
	}
	// Update-or-create: keeps exactly one row per user, race-safe enough
	// for per-user serialised flows (unique PK on user_id guards the rest).
	return r.saveRecord(ctx, rec)
}

// SaveRecord overwrites the row; used after read-modify-write on known rows.
func (r *gormKYCRepository) saveRecord(ctx context.Context, rec *domain.KycRecord) error {
	m := recordToModel(rec)
	res := r.db.WithContext(ctx).Model(&kycRecordModel{}).
		Where("user_id = ?", rec.UserID).
		Updates(map[string]interface{}{
			"level": rec.Level, "target_level": rec.TargetLevel,
			"status": string(rec.Status), "rejection_reason": rec.RejectionReason,
			"screening_status": string(rec.ScreeningStatus),
			"submitted_at":     rec.SubmittedAt, "verified_at": rec.VerifiedAt,
			"last_reviewed_at": rec.LastReviewedAt, "reviewed_by": rec.ReviewedBy,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("kyc: save record: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return r.db.WithContext(ctx).Create(m).Error
	}
	return nil
}

func (r *gormKYCRepository) CreateDocument(ctx context.Context, doc *domain.Document) error {
	if err := r.failFast(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(docToModel(doc)).Error; err != nil {
		return fmt.Errorf("kyc: create document: %w", err)
	}
	return nil
}

func (r *gormKYCRepository) GetDocument(ctx context.Context, id uuid.UUID) (*domain.Document, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var m documentModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kyc: get document: %w", err)
	}
	return docToDomain(&m), nil
}

func (r *gormKYCRepository) ListDocuments(ctx context.Context, userID int64, docType *domain.DocumentType, status *domain.DocumentStatus, limit int, cursor string) ([]*domain.Document, string, bool, error) {
	if err := r.failFast(); err != nil {
		return nil, "", false, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Model(&documentModel{}).Where("user_id = ?", userID)
	if docType != nil {
		q = q.Where("type = ?", string(*docType))
	}
	if status != nil {
		q = q.Where("status = ?", string(*status))
	}
	if cursor != "" {
		if id, err := uuid.Parse(cursor); err == nil {
			// Keyset on (submitted_at, id): submitted_at is not unique,
			// so tie-break with id for stable pagination.
			var anchor documentModel
			if err := r.db.WithContext(ctx).Where("id = ?", id).First(&anchor).Error; err == nil {
				q = q.Where("(submitted_at, id) < (?, ?)", anchor.SubmittedAt, anchor.ID)
			}
		}
	}
	var models []documentModel
	if err := q.Order("submitted_at DESC, id DESC").Limit(limit + 1).Find(&models).Error; err != nil {
		return nil, "", false, fmt.Errorf("kyc: list documents: %w", err)
	}
	hasMore := len(models) > limit
	if hasMore {
		models = models[:limit]
	}
	out := make([]*domain.Document, len(models))
	for i := range models {
		out[i] = docToDomain(&models[i])
	}
	next := ""
	if hasMore && len(models) > 0 {
		next = models[len(models)-1].ID.String()
	}
	return out, next, hasMore, nil
}

func (r *gormKYCRepository) UpdateDocument(ctx context.Context, doc *domain.Document) error {
	if err := r.failFast(); err != nil {
		return err
	}
	m := docToModel(doc)
	res := r.db.WithContext(ctx).Model(&documentModel{}).
		Where("id = ?", doc.ID).
		Updates(map[string]interface{}{
			"status": m.Status, "reviewed_at": doc.ReviewedAt, "reviewed_by": doc.ReviewedBy,
			"rejection_reason": doc.RejectionReason, "required_actions": m.RequiredActions,
			"extracted_data": m.ExtractedData, "notes": doc.Notes,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("kyc: update document: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *gormKYCRepository) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	if err := r.failFast(); err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&documentModel{})
	if res.Error != nil {
		return fmt.Errorf("kyc: delete document: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *gormKYCRepository) FindByNumberHash(ctx context.Context, numberHash string, excludeUserID int64) (*domain.Document, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(numberHash) == "" {
		return nil, nil
	}
	var m documentModel
	err := r.db.WithContext(ctx).
		Where("number_hash = ? AND user_id <> ? AND status IN ('submitted','in_review','approved')", numberHash, excludeUserID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kyc: find by number hash: %w", err)
	}
	return docToDomain(&m), nil
}

func (r *gormKYCRepository) MarkExpiredBefore(ctx context.Context, now time.Time, limit int) (int64, error) {
	if err := r.failFast(); err != nil {
		return 0, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	// Bounded batch: the sweep endpoint pages until affected == 0.
	res := r.db.WithContext(ctx).Model(&documentModel{}).
		Where("status = 'approved' AND expires_at IS NOT NULL AND expires_at <= ?", now).
		Limit(limit).
		Updates(map[string]interface{}{"status": string(domain.DocExpired), "updated_at": now})
	if res.Error != nil {
		return 0, fmt.Errorf("kyc: mark expired: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *gormKYCRepository) CreateScreening(ctx context.Context, s *domain.Screening) error {
	if err := r.failFast(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(screeningToModel(s)).Error; err != nil {
		return fmt.Errorf("kyc: create screening: %w", err)
	}
	return nil
}

func (r *gormKYCRepository) GetScreening(ctx context.Context, id uuid.UUID) (*domain.Screening, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var m screeningModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kyc: get screening: %w", err)
	}
	return screeningToDomain(&m), nil
}

func (r *gormKYCRepository) ListPendingScreenings(ctx context.Context, limit int) ([]*domain.Screening, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var models []screeningModel
	if err := r.db.WithContext(ctx).
		Where("status IN ('potential_match','true_match') AND resolved_at IS NULL").
		Order("screened_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("kyc: list screenings: %w", err)
	}
	out := make([]*domain.Screening, len(models))
	for i := range models {
		out[i] = screeningToDomain(&models[i])
	}
	return out, nil
}

func (r *gormKYCRepository) UpdateScreening(ctx context.Context, s *domain.Screening) error {
	if err := r.failFast(); err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&screeningModel{}).
		Where("id = ?", s.ID).
		Updates(map[string]interface{}{
			"status": string(s.Status), "resolved_at": s.ResolvedAt,
			"resolved_by": s.ResolvedBy, "resolution": s.Resolution,
		})
	if res.Error != nil {
		return fmt.Errorf("kyc: update screening: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrScreeningNotFound
	}
	return nil
}

func (r *gormKYCRepository) LatestScreening(ctx context.Context, userID int64) (*domain.Screening, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var m screeningModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("screened_at DESC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kyc: latest screening: %w", err)
	}
	return screeningToDomain(&m), nil
}

func (r *gormKYCRepository) AppendAudit(ctx context.Context, e *domain.AuditEntry) error {
	if err := r.failFast(); err != nil {
		return err
	}
	m := auditModel{
		UserID: e.UserID, Actor: e.Actor, Action: e.Action,
		DocumentID: e.DocumentID, OldStatus: e.OldStatus, NewStatus: e.NewStatus,
		Reason: e.Reason, CreatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("kyc: append audit: %w", err)
	}
	e.ID = m.ID
	e.CreatedAt = m.CreatedAt
	return nil
}

func (r *gormKYCRepository) ListAudit(ctx context.Context, userID int64, limit int) ([]*domain.AuditEntry, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var models []auditModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("kyc: list audit: %w", err)
	}
	out := make([]*domain.AuditEntry, len(models))
	for i, m := range models {
		out[i] = &domain.AuditEntry{
			ID: m.ID, UserID: m.UserID, Actor: m.Actor, Action: m.Action,
			DocumentID: m.DocumentID, OldStatus: m.OldStatus, NewStatus: m.NewStatus,
			Reason: m.Reason, CreatedAt: m.CreatedAt,
		}
	}
	return out, nil
}
