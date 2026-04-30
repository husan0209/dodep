// Package validators provides validation functions for Opus Casino platform.
package validators

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var (
	uuidRegex       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	emailRegex      = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phoneRegex      = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	amountRegex     = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)
	oddsRegex       = regexp.MustCompile(`^\d+(\.\d+)?$`)
	usernameRegex   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,19}$`)
	isoDateRegex    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// IsValidUUID validates UUID v4
func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// IsValidEmail validates email address
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// IsValidCountryCode validates ISO 3166-1 alpha-2 country code
func IsValidCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if !unicode.IsUpper(r) || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// IsValidCurrencyCode validates ISO 4217 currency code
func IsValidCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, r := range code {
		if !unicode.IsUpper(r) || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// IsValidMoneyAmount validates money amount string
func IsValidMoneyAmount(amount string) bool {
	return amountRegex.MatchString(amount)
}

// IsValidPassword validates password strength
// Requirements:
// - At least 8 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one number
// - At least one special character
func IsValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSpecial
}

// IsValidPhone validates E.164 phone number
func IsValidPhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// IsValidOdds validates decimal odds (1.01 - 1000.00)
func IsValidOdds(odds string) bool {
	if !oddsRegex.MatchString(odds) {
		return false
	}

	value, err := strconv.ParseFloat(odds, 64)
	if err != nil {
		return false
	}

	return value >= 1.01 && value <= 1000.0
}

// IsValidPercentage validates percentage (0-100)
func IsValidPercentage(value float64) bool {
	return value >= 0 && value <= 100
}

// IsValidIP validates IPv4 or IPv6 address
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsValidDate validates ISO 8601 date (YYYY-MM-DD)
func IsValidDate(date string) bool {
	if !isoDateRegex.MatchString(date) {
		return false
	}

	parts := strings.Split(date, "-")
	if len(parts) != 3 {
		return false
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	if year < 1900 || year > 2100 {
		return false
	}

	if month < 1 || month > 12 {
		return false
	}

	if day < 1 || day > 31 {
		return false
	}

	// Validate actual date
	_, err = time.Parse("2006-01-02", date)
	return err == nil
}

// IsValidUsername validates username
// Requirements:
// - 3-20 characters
// - Only alphanumeric characters and underscores
// - Must start with a letter
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}
