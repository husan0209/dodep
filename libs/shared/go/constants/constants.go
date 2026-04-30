// Package constants provides constants for Opus Casino platform.
package constants

// Currency codes (ISO 4217)
var Currencies = map[string]string{
	"USD": "USD",
	"EUR": "EUR",
	"GBP": "GBP",
	"RUB": "RUB",
	"BRL": "BRL",
	"INR": "INR",
	"JPY": "JPY",
	"CNY": "CNY",
	"KRW": "KRW",
	"CAD": "CAD",
	"AUD": "AUD",
	"CHF": "CHF",
	"BTC": "BTC",
	"ETH": "ETH",
	"USDT": "USDT",
}

// RestrictedCountries lists gambling-restricted jurisdictions
var RestrictedCountries = []string{
	"US", // United States (restricted states)
	"FR", // France (requires license)
	"IT", // Italy (requires license)
	"ES", // Spain (requires license)
	"NL", // Netherlands (requires license)
	"BE", // Belgium (requires license)
	"CH", // Switzerland (requires license)
	"AU", // Australia (interactive gambling act)
	"SG", // Singapore
	"HK", // Hong Kong
	"JP", // Japan (restrictions apply)
	"KR", // South Korea
	"CN", // China
	"RU", // Russia (restrictions apply)
}

// RateLimits defines rate limiting constants
var RateLimits = struct {
	// Authentication
	LoginAttempts       uint32
	LoginWindowMs       uint64
	// API
	APIRequestsPerMinute uint32
	APIRequestsPerHour   uint32
	// Betting
	BetsPerSecond       uint32
	// Withdrawal
	WithdrawalRequestsPerDay uint32
	// Password reset
	PasswordResetPerHour   uint32
	// 2FA
	TOTPWindowSeconds     uint64
	TOTPMaxAttempts       uint32
}{
	LoginAttempts:              5,
	LoginWindowMs:              15 * 60 * 1000,
	APIRequestsPerMinute:       100,
	APIRequestsPerHour:         1000,
	BetsPerSecond:              10,
	WithdrawalRequestsPerDay:   5,
	PasswordResetPerHour:       3,
	TOTPWindowSeconds:          30,
	TOTPMaxAttempts:            5,
}

// BetLimits defines betting limit constants
var BetLimits = struct {
	MinStake        string
	MaxStake        string
	MaxWinMultiplier uint32
	MaxOdds         string
	MinOdds         string
}{
	MinStake:         "0.10",
	MaxStake:         "10000.00",
	MaxWinMultiplier: 10000,
	MaxOdds:          "1000.00",
	MinOdds:          "1.01",
}

// PaymentLimits defines payment limit constants
var PaymentLimits = struct {
	MinDeposit          string
	MaxDepositDaily     string
	MinWithdrawal       string
	MaxWithdrawalDaily  string
	MaxWithdrawalMonthly string
}{
	MinDeposit:           "1.00",
	MaxDepositDaily:      "10000.00",
	MinWithdrawal:        "10.00",
	MaxWithdrawalDaily:   "50000.00",
	MaxWithdrawalMonthly: "500000.00",
}

// Session defines session-related constants
var Session = struct {
	AccessTokenTTLSeconds  uint64
	RefreshTokenTTLSeconds uint64
	SessionTTLSeconds      uint64
	MaxSessionsPerUser     int
}{
	AccessTokenTTLSeconds:  900,     // 15 minutes
	RefreshTokenTTLSeconds: 604800,  // 7 days
	SessionTTLSeconds:      2592000, // 30 days
	MaxSessionsPerUser:     5,
}

// ErrorCodes defines error code constants
var ErrorCodes = struct {
	// Authentication (1000-1999)
	AuthInvalidCredentials string
	AuthTokenExpired       string
	AuthTokenInvalid       string
	Auth2FARequired        string
	Auth2FAInvalid         string
	AuthAccountLocked      string
	// Wallet (5000-5999)
	WalletNotFound            string
	InsufficientBalance       string
	InsufficientAvailableBalance string
	// Bet (7000-7999)
	BetNotFound        string
	BetInvalid         string
	BetAlreadySettled  string
	BetLimitExceeded   string
	BetOddsChanged     string
	// System (11000-11999)
	InternalError         string
	ServiceUnavailable    string
	RateLimitExceeded     string
}{
	AuthInvalidCredentials:       "AUTH_1001",
	AuthTokenExpired:             "AUTH_1002",
	AuthTokenInvalid:             "AUTH_1003",
	Auth2FARequired:              "AUTH_1006",
	Auth2FAInvalid:               "AUTH_1007",
	AuthAccountLocked:            "AUTH_1008",
	WalletNotFound:               "WALLET_5001",
	InsufficientBalance:          "WALLET_5002",
	InsufficientAvailableBalance: "WALLET_5003",
	BetNotFound:                  "BET_7001",
	BetInvalid:                   "BET_7002",
	BetAlreadySettled:            "BET_7003",
	BetLimitExceeded:             "BET_7005",
	BetOddsChanged:               "BET_7007",
	InternalError:                "SYS_11001",
	ServiceUnavailable:           "SYS_11002",
	RateLimitExceeded:            "SYS_11005",
}
