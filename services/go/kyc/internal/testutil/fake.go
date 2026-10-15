// Package testutil provides in-memory fakes for KYC tests.
// No testcontainers needed: the fake repository implements
// repository.KYCRepository with plain maps.
package testutil

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/repository"
)

// FakeRepo is an in-memory repository.KYCRepository.
type FakeRepo struct {
	mu         sync.Mutex
	Records    map[int64]*domain.KycRecord
	Docs       map[uuid.UUID]*domain.Document
	Screenings map[uuid.UUID]*domain.Screening
	Audits     []*domain.AuditEntry
}

// NewFakeRepo creates an empty fake repository.
func NewFakeRepo() *FakeRepo {
	return &FakeRepo{
		Records:    map[int64]*domain.KycRecord{},
		Docs:       map[uuid.UUID]*domain.Document{},
		Screenings: map[uuid.UUID]*domain.Screening{},
	}
}

var _ repository.KYCRepository = (*FakeRepo)(nil)

// Doc returns a copy of a stored document (nil when missing).
func (f *FakeRepo) Doc(id uuid.UUID) *domain.Document {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.Docs[id]; ok {
		cp := *d
		return &cp
	}
	return nil
}

// SetDocExpiry rewrites a stored document's expiry (expiry simulations).
func (f *FakeRepo) SetDocExpiry(id uuid.UUID, t *time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.Docs[id]; ok {
		d.ExpiresAt = t
	}
}

func (f *FakeRepo) GetRecord(_ context.Context, userID int64) (*domain.KycRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.Records[userID]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, nil
}

func (f *FakeRepo) UpsertRecord(_ context.Context, rec *domain.KycRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *rec
	f.Records[rec.UserID] = &cp
	return nil
}

func (f *FakeRepo) CreateDocument(_ context.Context, doc *domain.Document) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *doc
	f.Docs[doc.ID] = &cp
	return nil
}

func (f *FakeRepo) GetDocument(_ context.Context, id uuid.UUID) (*domain.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.Docs[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, nil
}

func (f *FakeRepo) ListDocuments(_ context.Context, userID int64, docType *domain.DocumentType, st *domain.DocumentStatus, _ int, _ string) ([]*domain.Document, string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Document
	for _, d := range f.Docs {
		if d.UserID != userID {
			continue
		}
		if docType != nil && d.Type != *docType {
			continue
		}
		if st != nil && d.Status != *st {
			continue
		}
		cp := *d
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out, "", false, nil
}

func (f *FakeRepo) UpdateDocument(_ context.Context, doc *domain.Document) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Docs[doc.ID]; !ok {
		return domain.ErrDocumentNotFound
	}
	cp := *doc
	f.Docs[doc.ID] = &cp
	return nil
}

func (f *FakeRepo) DeleteDocument(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Docs[id]; !ok {
		return domain.ErrDocumentNotFound
	}
	delete(f.Docs, id)
	return nil
}

func (f *FakeRepo) FindByNumberHash(_ context.Context, hash string, exclude int64) (*domain.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.Docs {
		if d.NumberHash == hash && d.UserID != exclude &&
			(d.Status == domain.DocSubmitted || d.Status == domain.DocInReview || d.Status == domain.DocApproved) {
			cp := *d
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *FakeRepo) MarkExpiredBefore(_ context.Context, now time.Time, _ int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, d := range f.Docs {
		if d.Status == domain.DocApproved && d.ExpiresAt != nil && !now.Before(*d.ExpiresAt) {
			d.Status = domain.DocExpired
			n++
		}
	}
	return n, nil
}

func (f *FakeRepo) CreateScreening(_ context.Context, s *domain.Screening) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *s
	f.Screenings[s.ID] = &cp
	return nil
}

func (f *FakeRepo) GetScreening(_ context.Context, id uuid.UUID) (*domain.Screening, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.Screenings[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (f *FakeRepo) ListPendingScreenings(_ context.Context, _ int) ([]*domain.Screening, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Screening
	for _, s := range f.Screenings {
		if s.ResolvedAt == nil && (s.Status == domain.ScreeningPotentialMatch || s.Status == domain.ScreeningTrueMatch) {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *FakeRepo) UpdateScreening(_ context.Context, s *domain.Screening) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Screenings[s.ID]; !ok {
		return domain.ErrScreeningNotFound
	}
	cp := *s
	f.Screenings[s.ID] = &cp
	return nil
}

func (f *FakeRepo) LatestScreening(_ context.Context, userID int64) (*domain.Screening, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var best *domain.Screening
	for _, s := range f.Screenings {
		if s.UserID != userID {
			continue
		}
		if best == nil || s.ScreenedAt.After(best.ScreenedAt) {
			cp := *s
			best = &cp
		}
	}
	return best, nil
}

func (f *FakeRepo) AppendAudit(_ context.Context, e *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e.ID = int64(len(f.Audits) + 1)
	f.Audits = append(f.Audits, e)
	return nil
}

func (f *FakeRepo) ListAudit(_ context.Context, userID int64, _ int) ([]*domain.AuditEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.AuditEntry
	for _, e := range f.Audits {
		if e.UserID == userID {
			out = append(out, e)
		}
	}
	return out, nil
}
