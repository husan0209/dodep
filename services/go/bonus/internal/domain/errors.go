package domain

import "errors"

// Sentinel errors for the Bonus domain.
// Handlers map these to HTTP status codes / gRPC codes via errors.Is.
var (
	ErrBonusNotFound      = errors.New("bonus not found")
	ErrBonusNotActive     = errors.New("bonus is not active")
	ErrBonusAlreadyExists = errors.New("bonus already exists for user")
	ErrBonusExpired       = errors.New("bonus has expired")
	ErrInvalidBonusAmount = errors.New("invalid bonus amount")
	ErrWageringNotMet     = errors.New("wagering requirements not met")
	ErrInvalidPromoCode   = errors.New("invalid promo code")
	ErrBonusClaimLimit    = errors.New("bonus claim limit reached")
	ErrForbidden          = errors.New("action forbidden")
)
