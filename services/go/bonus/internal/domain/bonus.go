package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// BonusType classifies the type of bonus.
type BonusType string

const (
	BonusTypeWelcome    BonusType = "welcome"
	BonusTypeReload     BonusType = "reload"
	BonusTypeCashback   BonusType = "cashback"
	BonusTypeFreeSpins  BonusType = "free_spins"
	BonusTypeReferral   BonusType = "referral"
)

// BonusStatus tracks bonus lifecycle.
type BonusStatus string

const (
	BonusStatusPending    BonusStatus = "pending"    // Awarded, not yet activated
	BonusStatusActive     BonusStatus = "active"     // In progress (wagering)
	BonusStatusCompleted  BonusStatus = "completed"  // Wagering complete, converted to real
	BonusStatusExpired    BonusStatus = "expired"    // TTL exceeded before wagering complete
	BonusStatusCancelled  BonusStatus = "cancelled"  // Manually or policy cancelled
)

// Bonus represents a user's bonus record.
type Bonus struct {
	ID                 uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID             int64           `gorm:"not null;index"`
	Type               BonusType       `gorm:"not null"`
	Status             BonusStatus     `gorm:"not null;default:'pending'"`
	BonusAmount        decimal.Decimal `gorm:"type:numeric(18,8);not null"`
	RealAmount         decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"` // credited on deposit
	Currency           string          `gorm:"not null;default:'USD'"`
	WageringRequired   decimal.Decimal `gorm:"type:numeric(18,8);not null"` // total wagering needed
	WageringCompleted  decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	WageringMultiplier int             `gorm:"not null;default:30"`
	ExpiresAt          time.Time       `gorm:"not null"`
	ActivatedAt        *time.Time
	CompletedAt        *time.Time
	CancelledAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsWageringComplete returns true if all wagering requirements are met.
func (b *Bonus) IsWageringComplete() bool {
	return b.WageringCompleted.GreaterThanOrEqual(b.WageringRequired)
}

// IsExpired returns true if the bonus has passed its expiry date.
func (b *Bonus) IsExpired() bool {
	return time.Now().After(b.ExpiresAt)
}

// RemainingWagering returns how much more needs to be wagered.
func (b *Bonus) RemainingWagering() decimal.Decimal {
	rem := b.WageringRequired.Sub(b.WageringCompleted)
	if rem.IsNegative() {
		return decimal.Zero
	}
	return rem
}
