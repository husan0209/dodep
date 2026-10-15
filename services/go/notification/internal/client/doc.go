// Package client holds the production channel senders for the notification
// service: SendGrid (email), Twilio (SMS), Firebase FCM v1 (push) and the
// WebSocket gateway HTTP client (in-app).
//
// Contract shared by every sender here:
//   - Credentials come ONLY from environment variables (CONVENTIONS NEVER-5).
//     There are no default keys, no embedded secrets, no config-file fallback.
//   - When the required key is absent the constructor reports ok=false and
//     main keeps the log-only fallback wired by NewNotificationService —
//     delivery degrades to structured logs, never to a crash.
//   - The wave1 interfaces (service.EmailSender / SMSSender / PushSender /
//     InAppPublisher) are implemented as-is and MUST NOT change.
//   - Outbound calls use plain net/http against the provider REST APIs, so no
//     vendor SDK is required in go.mod; time budget per send is 5 seconds
//     (notification gRPC deadline guideline).
package client

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// firstEnv returns the first non-blank value of the given variable names.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// endpointOr returns the override variable when set, otherwise def.
func endpointOr(overrideKey, def string) string {
	if v := strings.TrimSpace(os.Getenv(overrideKey)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return def
}

// newHTTPClient builds the shared short-deadline client for provider calls.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// errorSnippet caps upstream error bodies before they reach logs/errors.
func errorSnippet(r io.Reader) string {
	const maxSnippet = 512
	buf, _ := io.ReadAll(io.LimitReader(r, maxSnippet))
	return strings.TrimSpace(string(buf))
}
