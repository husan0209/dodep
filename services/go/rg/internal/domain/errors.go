package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors — compared with errors.Is() in handlers.
var (
	ErrSelfExcluded       = errors.New("rg: user is self-excluded")
	ErrCoolOffActive      = errors.New("rg: cooling-off time-out is active")
	ErrDepositLimit       = errors.New("rg: deposit limit would be exceeded")
	ErrLossLimit          = errors.New("rg: loss limit reached")
	ErrWagerLimit         = errors.New("rg: wager limit would be exceeded")
	ErrSessionLimit       = errors.New("rg: session limit reached")
	ErrPermanentExclusion = errors.New("rg: permanent exclusion cannot be revoked")
	ErrExclusionExists    = errors.New("rg: an active exclusion already exists")
	ErrExclusionNotFound  = errors.New("rg: exclusion not found")
	ErrTimeoutExists      = errors.New("rg: an active time-out already exists")
	ErrInvalidPeriod      = errors.New("rg: invalid period")
	ErrInvalidChannel     = errors.New("rg: invalid channel")
	ErrInvalidLimitValue  = errors.New("rg: invalid limit value")
	ErrRevokeTooEarly     = errors.New("rg: exclusion has not expired yet")
	ErrRevokeCooling      = errors.New("rg: revocation cooling period has not elapsed")
	ErrConfirmRequired    = errors.New("rg: explicit confirmation is required")
	ErrNotFound           = errors.New("rg: resource not found")
	ErrConflict           = errors.New("rg: resource conflict")
	ErrForbidden          = errors.New("rg: action forbidden")
)

// ValidationError carries per-field validation failures.
type ValidationError struct {
	Fields []FieldError
}

// FieldError is a single field validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s — %s", e.Fields[0].Field, e.Fields[0].Message)
}

// NewValidationError builds a ValidationError from field errors.
func NewValidationError(fields ...FieldError) *ValidationError {
	return &ValidationError{Fields: fields}
}

// DetailedError wraps a sentinel with machine-readable context
// (limit, used, remaining) for 422 responses.
type DetailedError struct {
	Err     error
	Details map[string]interface{}
}

func (e *DetailedError) Error() string { return e.Err.Error() }

// Unwrap preserves the errors.Is/As chain.
func (e *DetailedError) Unwrap() error { return e.Err }

// WithDetails attaches context to a sentinel error.
func WithDetails(err error, details map[string]interface{}) *DetailedError {
	return &DetailedError{Err: err, Details: details}
}
