// Package types provides shared types for Opus Casino platform.
package types

import "errors"

var (
	// ErrCurrencyMismatch is returned when trying to operate on money with different currencies
	ErrCurrencyMismatch = errors.New("currency mismatch")
	
	// ErrNegativeAmount is returned when money amount is negative
	ErrNegativeAmount = errors.New("amount cannot be negative")
	
	// ErrInvalidUUID is returned when UUID parsing fails
	ErrInvalidUUID = errors.New("invalid UUID")
	
	// ErrInvalidFormat is returned when format validation fails
	ErrInvalidFormat = errors.New("invalid format")
)
