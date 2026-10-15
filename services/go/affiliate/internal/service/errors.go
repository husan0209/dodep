package service

import "errors"

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError carries structured input-validation failures. Handlers map
// it to 400; service code matches it with errors.As.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}
	return "validation failed: " + e.Fields[0].Field + " — " + e.Fields[0].Message
}

// NewValidationError builds a ValidationError from field failures.
func NewValidationError(fields ...FieldError) *ValidationError {
	return &ValidationError{Fields: fields}
}

var (
	// ErrIdempotencyKeyRequired is returned when a money-moving operation is
	// called without a client-generated idempotency key. Keys are mandatory
	// for all affiliate financial writes (accrual, payout).
	ErrIdempotencyKeyRequired = errors.New("idempotency_key is required")
	// ErrEarningNotFound is returned when an earning referenced by reversal
	// or release does not exist.
	ErrEarningNotFound = errors.New("affiliate earning not found")
	// ErrEarningNotReversible is returned when a reversal targets an earning
	// in a terminal state (paid or already reversed).
	ErrEarningNotReversible = errors.New("affiliate earning cannot be reversed in its current status")
	// ErrInvalidAdjustment is returned for zero/negative amounts, unknown
	// adjustment types, or missing audit fields (reason, created_by).
	ErrInvalidAdjustment = errors.New("invalid affiliate adjustment")
	// ErrPayoutMethodUnverified is returned when a payout is requested to a
	// method that is not verified or does not belong to the affiliate.
	ErrPayoutMethodUnverified = errors.New("affiliate payout method is not verified for this affiliate")
)
