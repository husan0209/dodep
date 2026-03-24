package domain

import (
	"time"
)

// Session represents an active user session
type Session struct {
	ID           string     `json:"id" db:"id"`
	UserID       int64      `json:"user_id" db:"user_id"`
	DeviceID     string     `json:"device_id" db:"device_id"`
	DeviceType   string     `json:"device_type" db:"device_type"`
	IPAddress    string     `json:"ip_address" db:"ip_address"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	Country      string     `json:"country" db:"country"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	LastActivity time.Time  `json:"last_activity" db:"last_activity"`
	IsActive     bool       `json:"is_active" db:"is_active"`
}

// TokenPair represents JWT token pair
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// AuthResult is the result of authentication
type AuthResult struct {
	UserID       int64   `json:"user_id"`
	Tokens       *TokenPair `json:"tokens"`
	Session      *Session   `json:"session"`
	Requires2FA  bool       `json:"requires_2fa"`
	TempToken    string     `json:"temp_token,omitempty"`
}

// RefreshTokenRequest is the domain model for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	DeviceID     string `json:"device_id"`
}

// LoginAttempt tracks failed login attempts
type LoginAttempt struct {
	IPAddress string    `json:"ip_address"`
	Email     string    `json:"email"`
	Attempts  int       `json:"attempts"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// Max login attempts before account lockout
const MaxLoginAttempts = 10
const LockoutDuration = 30 * time.Minute
const MaxSessionsPerUser = 5
