// Package testutil provides in-memory fakes for bonus unit tests.
// It has no external dependencies beyond the module itself and must
// never be imported by production code.
package testutil

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
)

// FakeBonusRepository is an in-memory repository.BonusRepository.
// Safe for single-goroutine unit tests (no locking).
type FakeBonusRepository struct {
	Bonuses map[uuid.UUID]*domain.Bonus
}

// NewFakeBonusRepository creates an empty fake.
func NewFakeBonusRepository() *FakeBonusRepository {
	return &FakeBonusRepository{Bonuses: make(map[uuid.UUID]*domain.Bonus)}
}

var _ repository.BonusRepository = (*FakeBonusRepository)(nil)

func (f *FakeBonusRepository) FindWelcome(_ context.Context, userID int64) (*domain.Bonus, error) {
	for _, b := range f.Bonuses {
		if b.UserID == userID && b.Type == domain.BonusTypeWelcome &&
			(b.Status == domain.BonusStatusPending || b.Status == domain.BonusStatusActive || b.Status == domain.BonusStatusCompleted) {
			return b, nil
		}
	}
	return nil, nil
}

func (f *FakeBonusRepository) FindActive(_ context.Context, userID int64) (*domain.Bonus, error) {
	for _, b := range f.Bonuses {
		if b.UserID == userID && b.Status == domain.BonusStatusActive {
			return b, nil
		}
	}
	return nil, nil
}

func (f *FakeBonusRepository) FindByID(_ context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error) {
	b, ok := f.Bonuses[bonusID]
	if !ok || b.UserID != userID {
		return nil, nil
	}
	return b, nil
}

func (f *FakeBonusRepository) Create(_ context.Context, bonus *domain.Bonus) error {
	f.Bonuses[bonus.ID] = bonus
	return nil
}

func (f *FakeBonusRepository) UpdateWagering(_ context.Context, bonusID uuid.UUID, completed map[string]interface{}) error {
	b, ok := f.Bonuses[bonusID]
	if !ok {
		return domain.ErrBonusNotFound
	}
	if v, ok := completed["wagering_completed"].(string); ok {
		if d, err := decimal.NewFromString(v); err == nil {
			b.WageringCompleted = d
		}
	}
	if st, ok := completed["status"].(domain.BonusStatus); ok {
		b.Status = st
	}
	return nil
}

func (f *FakeBonusRepository) MarkExpired(_ context.Context, bonusID uuid.UUID) error {
	if b, ok := f.Bonuses[bonusID]; ok {
		b.Status = domain.BonusStatusExpired
	}
	return nil
}

func (f *FakeBonusRepository) ListByUser(_ context.Context, userID int64, _, _ int) ([]*domain.Bonus, int64, error) {
	var out []*domain.Bonus
	for _, b := range f.Bonuses {
		if b.UserID == userID {
			out = append(out, b)
		}
	}
	return out, int64(len(out)), nil
}

func (f *FakeBonusRepository) Activate(_ context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error) {
	b, ok := f.Bonuses[bonusID]
	if !ok || b.UserID != userID {
		return nil, domain.ErrBonusNotFound
	}
	b.Status = domain.BonusStatusActive
	return b, nil
}

func (f *FakeBonusRepository) Cancel(_ context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error) {
	b, ok := f.Bonuses[bonusID]
	if !ok || b.UserID != userID {
		return nil, domain.ErrBonusNotFound
	}
	b.Status = domain.BonusStatusCancelled
	return b, nil
}
