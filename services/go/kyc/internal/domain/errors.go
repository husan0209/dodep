package domain

import "errors"

// Sentinel errors for the KYC domain.
// Handlers map these to HTTP status codes / gRPC codes via errors.Is.
var (
	ErrKYCNotFound            = errors.New("kyc record not found")
	ErrDocumentNotFound       = errors.New("kyc document not found")
	ErrDocumentNotDeletable   = errors.New("document cannot be deleted in its current status")
	ErrDocumentAlreadyUsed    = errors.New("document number is already registered to another user")
	ErrDocumentExpired        = errors.New("document has expired")
	ErrInvalidDocument        = errors.New("invalid document data")
	ErrInvalidLevelTransition = errors.New("invalid verification level transition")
	ErrKYCAlreadyVerified     = errors.New("kyc level is already reached or exceeded")
	ErrKYCPendingReview       = errors.New("kyc verification is pending review")
	ErrKYCRejected            = errors.New("kyc verification was rejected")
	ErrSanctionsMatch         = errors.New("sanctions or pep match requires manual review")
	ErrScreeningNotFound      = errors.New("screening record not found")
	ErrScreeningNotPending    = errors.New("screening is not pending review")
	ErrInvalidWebhookSig      = errors.New("invalid webhook signature")
	ErrDuplicateWebhook       = errors.New("webhook event already processed")
	ErrUnsupportedProvider    = errors.New("unsupported verification provider")
	ErrForbidden              = errors.New("action forbidden")
	ErrInvalidUserID          = errors.New("invalid user id")
)

// ValidationError carries field-level validation failures.
type ValidationError struct {
	Fields []FieldError
}

// FieldError describes a single invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}
	return "validation failed: " + e.Fields[0].Field + " — " + e.Fields[0].Message
}

// NewValidationError builds a ValidationError from field errors.
func NewValidationError(fields ...FieldError) *ValidationError {
	return &ValidationError{Fields: fields}
}
