package domain

import (
	"errors"
	"fmt"
)

// Payment error codes (5000-5999 range)
const (
	ErrCodePaymentNotFound          = 5001
	ErrCodePaymentAlreadyProcessed  = 5002
	ErrCodePaymentExpired           = 5003
	ErrCodeKYCRequired              = 5004
	ErrCodeDailyLimitExceeded       = 5005
	ErrCodeInsufficientBalance      = 5006
	ErrCodeWalletLocked             = 5007
	ErrCodeInvalidCryptoAddress     = 5008
	ErrCodeCurrencyNotSupported     = 5009
	ErrCodeProviderUnavailable      = 5010
	ErrCodeWebhookSignatureInvalid  = 5011
	ErrCodeWithdrawalNotFound       = 5012
	ErrCodeWithdrawalAlreadyProcessed = 5013
	ErrCodeInvalidAmount            = 5014
	ErrCodeInvalidStatusTransition  = 5015
)

// Domain errors
var (
	ErrPaymentNotFound           = errors.New("payment not found")
	ErrPaymentAlreadyProcessed   = errors.New("payment already processed")
	ErrPaymentExpired            = errors.New("payment has expired")
	ErrWithdrawalNotFound        = errors.New("withdrawal not found")
	ErrWithdrawalAlreadyProcessed = errors.New("withdrawal already processed")

	ErrKYCRequired         = errors.New("KYC level 2 required for withdrawals")
	ErrDailyLimitExceeded  = errors.New("daily limit exceeded")

	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrWalletLocked        = errors.New("wallet is locked")

	ErrInvalidCryptoAddress = errors.New("invalid crypto address format")
	ErrCurrencyNotSupported = errors.New("cryptocurrency not supported")
	ErrProviderUnavailable  = errors.New("payment provider unavailable")

	ErrWebhookSignatureInvalid = errors.New("webhook signature verification failed")

	ErrInvalidAmount           = errors.New("invalid amount")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

// DetailedError provides additional context for errors
type DetailedError struct {
	Err     error
	Code    int
	Details map[string]interface{}
}

// Error implements the error interface
func (e *DetailedError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *DetailedError) Unwrap() error {
	return e.Err
}

// WithDetails creates a DetailedError with additional context
func WithDetails(err error, code int, details map[string]interface{}) *DetailedError {
	return &DetailedError{
		Err:     err,
		Code:    code,
		Details: details,
	}
}

// NewDetailedError creates a new DetailedError
func NewDetailedError(err error, code int) *DetailedError {
	return &DetailedError{
		Err:  err,
		Code: code,
	}
}

// IsPaymentError checks if error is a payment-related error
func IsPaymentError(err error) bool {
	var detailed *DetailedError
	if errors.As(err, &detailed) {
		return detailed.Code >= 5000 && detailed.Code < 6000
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) int {
	var detailed *DetailedError
	if errors.As(err, &detailed) {
		return detailed.Code
	}
	return 0
}

// PaymentError creates a payment error with code
func PaymentError(err error, code int) error {
	return NewDetailedError(err, code)
}

// Error messages with context
func ErrorPaymentNotFound(id int64) error {
	return WithDetails(ErrPaymentNotFound, ErrCodePaymentNotFound, map[string]interface{}{
		"payment_id": id,
	})
}

func ErrorPaymentNotFoundByPaymentID(paymentID string) error {
	return WithDetails(ErrPaymentNotFound, ErrCodePaymentNotFound, map[string]interface{}{
		"payment_id": paymentID,
	})
}

func ErrorWithdrawalNotFound(id int64) error {
	return WithDetails(ErrWithdrawalNotFound, ErrCodeWithdrawalNotFound, map[string]interface{}{
		"withdrawal_id": id,
	})
}

func ErrorKYCRequiredLevel(level int) error {
	return WithDetails(ErrKYCRequired, ErrCodeKYCRequired, map[string]interface{}{
		"current_level": level,
		"required_level": 2,
	})
}

func ErrorDailyLimitExceeded(limit, used, requested float64) error {
	return WithDetails(ErrDailyLimitExceeded, ErrCodeDailyLimitExceeded, map[string]interface{}{
		"limit":      limit,
		"used":       used,
		"requested":  requested,
		"available":  limit - used,
	})
}

func ErrorInsufficientBalance(available, requested float64) error {
	return WithDetails(ErrInsufficientBalance, ErrCodeInsufficientBalance, map[string]interface{}{
		"available":  available,
		"requested":  requested,
	})
}

func ErrorInvalidStatusTransition(from, to string) error {
	return WithDetails(ErrInvalidStatusTransition, ErrCodeInvalidStatusTransition, map[string]interface{}{
		"from_status": from,
		"to_status":   to,
	})
}

func ErrorCurrencyNotSupported(currency string) error {
	return WithDetails(ErrCurrencyNotSupported, ErrCodeCurrencyNotSupported, map[string]interface{}{
		"currency": currency,
	})
}

func ErrorProviderUnavailable(provider string, reason error) error {
	return WithDetails(ErrProviderUnavailable, ErrCodeProviderUnavailable, map[string]interface{}{
		"provider": provider,
		"reason":   reason.Error(),
	})
}

// HTTPStatus returns the appropriate HTTP status code for an error
func HTTPStatus(err error) int {
	code := GetErrorCode(err)
	switch {
	case code == ErrCodePaymentNotFound || code == ErrCodeWithdrawalNotFound:
		return 404
	case code == ErrCodeKYCRequired:
		return 403
	case code == ErrCodeInsufficientBalance || code == ErrCodeDailyLimitExceeded:
		return 422
	case code == ErrCodeWebhookSignatureInvalid:
		return 401
	case code == ErrCodeProviderUnavailable:
		return 502
	case code == ErrCodeInvalidCryptoAddress || code == ErrCodeInvalidAmount:
		return 400
	default:
		return 500
	}
}

// ErrorJSON represents an error in JSON format
type ErrorJSON struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ToJSON converts an error to JSON format
func ToJSON(err error) ErrorJSON {
	var detailed *DetailedError
	if errors.As(err, &detailed) {
		return ErrorJSON{
			Code:    detailed.Code,
			Message: detailed.Err.Error(),
			Details: detailed.Details,
		}
	}
	return ErrorJSON{
		Code:    5000,
		Message: err.Error(),
	}
}

// FormatError formats an error for logging
func FormatError(err error) string {
	var detailed *DetailedError
	if errors.As(err, &detailed) {
		if len(detailed.Details) > 0 {
			return fmt.Sprintf("%s (code=%d, details=%v)", detailed.Err.Error(), detailed.Code, detailed.Details)
		}
		return fmt.Sprintf("%s (code=%d)", detailed.Err.Error(), detailed.Code)
	}
	return err.Error()
}
