// Package errors provides error types for Opus Casino platform.
package errors

import "fmt"

// AppError represents an application error
type AppError struct {
	Code    string
	Message string
	Err     error
}

// Error implements error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a validation error
func NewValidationError(message string) *AppError {
	return &AppError{
		Code:    "VALIDATION_ERROR",
		Message: message,
	}
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *AppError {
	return &AppError{
		Code:    "AUTH_ERROR",
		Message: message,
	}
}

// NewAuthzError creates an authorization error
func NewAuthzError(message string) *AppError {
	return &AppError{
		Code:    "AUTHZ_ERROR",
		Message: message,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(entity, id string) *AppError {
	return &AppError{
		Code:    "NOT_FOUND",
		Message: fmt.Sprintf("%s not found: %s", entity, id),
	}
}

// NewAlreadyExistsError creates an already exists error
func NewAlreadyExistsError(entity, id string) *AppError {
	return &AppError{
		Code:    "ALREADY_EXISTS",
		Message: fmt.Sprintf("%s already exists: %s", entity, id),
	}
}

// NewInvalidArgumentError creates an invalid argument error
func NewInvalidArgumentError(message string) *AppError {
	return &AppError{
		Code:    "INVALID_ARGUMENT",
		Message: message,
	}
}

// NewInsufficientBalanceError creates an insufficient balance error
func NewInsufficientBalanceError(message string) *AppError {
	return &AppError{
		Code:    "INSUFFICIENT_BALANCE",
		Message: message,
	}
}

// NewRateLimitExceededError creates a rate limit exceeded error
func NewRateLimitExceededError(message string) *AppError {
	return &AppError{
		Code:    "RATE_LIMIT_EXCEEDED",
		Message: message,
	}
}

// NewServiceUnavailableError creates a service unavailable error
func NewServiceUnavailableError(message string) *AppError {
	return &AppError{
		Code:    "SERVICE_UNAVAILABLE",
		Message: message,
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string) *AppError {
	return &AppError{
		Code:    "INTERNAL_ERROR",
		Message: message,
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsValidationError checks if error is a validation error
func IsValidationError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == "VALIDATION_ERROR"
	}
	return false
}

// IsNotFoundError checks if error is a not found error
func IsNotFoundError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == "NOT_FOUND"
	}
	return false
}

// IsInsufficientBalanceError checks if error is an insufficient balance error
func IsInsufficientBalanceError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == "INSUFFICIENT_BALANCE"
	}
	return false
}
