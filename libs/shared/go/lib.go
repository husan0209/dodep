// Package shared provides shared libraries and utilities for Opus Casino platform.
//
// This package includes:
//   - Types: Strongly-typed identifiers, Money with decimal precision
//   - Validators: Email, UUID, password, odds, phone validation
//   - Constants: Currency codes, rate limits, betting limits
//   - Helpers: Money arithmetic, UUID generation, retry logic
//   - Errors: Comprehensive error types
//
// Example usage:
//
//	package main
//
//	import (
//	    "fmt"
//	    "github.com/opus-casino/shared/go"
//	)
//
//	func main() {
//	    userID := shared.GenerateUUID()
//	    balance := shared.NewMoney("100.00", "USD")
//
//	    if shared.IsValidEmail("user@example.com") {
//	        fmt.Println("Valid email")
//	    }
//	}
package shared

import (
	"github.com/opus-casino/shared/go/errors"
	"github.com/opus-casino/shared/go/types"
	"github.com/opus-casino/shared/go/validators"
	"github.com/opus-casino/shared/go/constants"
	"github.com/opus-casino/shared/go/helpers"
)

// Re-export types
type (
	UserId         = types.UserId
	BetId          = types.BetId
	TransactionId  = types.TransactionId
	GameId         = types.GameId
	SessionId      = types.SessionId
	Money          = types.Money
	PaginationParams = types.PaginationParams
	PaginationResult[T any] = types.PaginationResult[T]
	DateRange      = types.DateRange
	ErrorDetails   = types.ErrorDetails
	FieldError     = types.FieldError
	ApiResponse[T any] = types.ApiResponse[T]
	HealthStatus   = types.HealthStatus
	DeviceType     = types.DeviceType
	WalletType     = types.WalletType
	TransactionType = types.TransactionType
	BetType        = types.BetType
	BetStatus      = types.BetStatus
	KycLevel       = types.KycLevel
)

// Re-export validators
var (
	IsValidEmail       = validators.IsValidEmail
	IsValidUUID        = validators.IsValidUUID
	IsValidCountryCode = validators.IsValidCountryCode
	IsValidCurrencyCode = validators.IsValidCurrencyCode
	IsValidPassword    = validators.IsValidPassword
	IsValidPhone       = validators.IsValidPhone
	IsValidOdds        = validators.IsValidOdds
	IsValidPercentage  = validators.IsValidPercentage
	IsValidIP          = validators.IsValidIP
	IsValidDate        = validators.IsValidDate
	IsValidUsername    = validators.IsValidUsername
)

// Re-export constants
var (
	Currencies          = constants.Currencies
	RestrictedCountries = constants.RestrictedCountries
	RateLimits          = constants.RateLimits
	BetLimits           = constants.BetLimits
	PaymentLimits       = constants.PaymentLimits
	Session             = constants.Session
	ErrorCodes          = constants.ErrorCodes
)

// Re-export helpers
var (
	FormatMoney    = helpers.FormatMoney
	ParseMoney     = helpers.ParseMoney
	AddMoney       = helpers.AddMoney
	SubtractMoney  = helpers.SubtractMoney
	MultiplyMoney  = helpers.MultiplyMoney
	CompareMoney   = helpers.CompareMoney
	GenerateUUID   = helpers.GenerateUUID
	NowMs          = helpers.NowMs
	NowISO         = helpers.NowISO
	DeepClone      = helpers.DeepClone
	CalculatePercentage = helpers.CalculatePercentage
)

// Re-export errors
type (
	AppError = errors.AppError
)

var (
	NewValidationError     = errors.NewValidationError
	NewAuthError          = errors.NewAuthError
	NewAuthzError         = errors.NewAuthzError
	NewNotFoundError      = errors.NewNotFoundError
	NewAlreadyExistsError = errors.NewAlreadyExistsError
	NewInvalidArgumentError = errors.NewInvalidArgumentError
	NewInsufficientBalanceError = errors.NewInsufficientBalanceError
	NewRateLimitExceededError = errors.NewRateLimitExceededError
	NewServiceUnavailableError = errors.NewServiceUnavailableError
	NewInternalError      = errors.NewInternalError
)
