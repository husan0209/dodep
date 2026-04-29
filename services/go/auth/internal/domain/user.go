package domain

import (
	"errors"
	"time"
)

// UserStatus represents the user account status
type UserStatus string

const (
	UserStatusPending     UserStatus = "pending"
	UserStatusActive      UserStatus = "active"
	UserStatusBlocked     UserStatus = "blocked"
	UserStatusSelfExcluded UserStatus = "self_excluded"
	UserStatusSuspended   UserStatus = "suspended"
	UserStatusClosed      UserStatus = "closed"
)

var (
	ErrValidation          = errors.New("validation failed")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountLocked       = errors.New("account locked")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidToken        = errors.New("invalid token")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrForbidden           = errors.New("forbidden")
	ErrDependencyUnavailable = errors.New("dependency unavailable")
	ErrInternal            = errors.New("internal error")
)

// User represents a user account
type User struct {
	ID            string      `json:"id" db:"id"`
	UUID          string      `json:"uuid" db:"uuid"`
	Email         string      `json:"email" db:"email"`
	Phone         *string     `json:"phone,omitempty" db:"phone"`
	PasswordHash  string      `json:"-" db:"password_hash"`
	Username      string      `json:"username" db:"username"`
	Status        UserStatus  `json:"status" db:"status"`
	KYCLevel      int         `json:"kyc_level" db:"kyc_level"`
	CountryCode   string      `json:"country_code" db:"country_code"`
	CurrencyCode  string      `json:"currency_code" db:"currency_code"`
	TwoFAEnabled  bool        `json:"two_fa_enabled" db:"two_fa_enabled"`
	TwoFASecret   *string     `json:"-" db:"two_fa_secret"`
	EmailVerified bool        `json:"email_verified" db:"email_verified"`
	PhoneVerified bool        `json:"phone_verified" db:"phone_verified"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`
	LastLoginAt   *time.Time  `json:"last_login_at,omitempty" db:"last_login_at"`
	Metadata      string      `json:"metadata,omitempty" db:"metadata"`
}

// RegisterRequest is the domain model for user registration
type RegisterRequest struct {
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	Username     string `json:"username" validate:"required,min=3,max=30"`
	CountryCode  string `json:"country_code" validate:"required,len=2"`
	CurrencyCode string `json:"currency_code" validate:"required,len=3"`
	DeviceID     string `json:"device_id"`
	IPAddress    string `json:"ip_address"`
}

// LoginRequest is the domain model for user login
type LoginRequest struct {
	Identifier  string  `json:"identifier"`
	Email       string  `json:"email"`
	Password    string  `json:"password" validate:"required"`
	DeviceID    string  `json:"device_id"`
	IPAddress   string  `json:"ip_address"`
	TOTPCode    *string `json:"totp_code,omitempty"`
	RememberMe  bool    `json:"remember_me"`
}

// ChangePasswordRequest is the domain model for password change
type ChangePasswordRequest struct {
	UserID          string  `json:"user_id"`
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ResetPasswordRequest is the domain model for password reset
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}
