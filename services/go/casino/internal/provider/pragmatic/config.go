package pragmatic

import "time"

// Config holds Pragmatic Play API credentials and settings.
// All values must be populated from environment variables — never hardcode.
type Config struct {
	// AgentID is the secure_login value provided by Pragmatic Play.
	AgentID string

	// SecretKey is the HMAC secret for request signing.
	SecretKey string

	// APIURL is the base URL for the Integration Service API.
	// Production: https://api.prerelease-env.biz  (update when live)
	// Staging:    https://api.prerelease-env.biz
	APIURL string

	// Enabled gates all provider operations.
	Enabled bool

	// HTTPTimeout for outbound API calls.
	HTTPTimeout time.Duration

	// ReplayWindowSec is the max age (seconds) of an accepted callback.
	// Callbacks older than this are rejected as replays (default 180).
	ReplayWindowSec int
}

// DefaultConfig returns safe defaults (disabled).
func DefaultConfig() Config {
	return Config{
		APIURL:          "https://api.prerelease-env.biz",
		HTTPTimeout:     10 * time.Second,
		ReplayWindowSec: 180,
	}
}
