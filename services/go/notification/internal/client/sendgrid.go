package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

// sendGridEndpoint is the SendGrid v3 mail API. Tests point
// SENDGRID_API_URL at an httptest server instead.
const sendGridEndpoint = "https://api.sendgrid.com/v3/mail/send"

// SendGridSender delivers email via the SendGrid v3 API (plain HTTPS, no SDK).
// It implements service.EmailSender without changing the interface.
type SendGridSender struct {
	apiKey   string
	from     string
	endpoint string
	http     *http.Client
	log      *zap.Logger
}

// Compile-time guarantee: the wave1 interface is implemented as-is.
var _ service.EmailSender = (*SendGridSender)(nil)

// NewSendGridSenderFromEnv builds the sender from the environment.
// SENDGRID_API_KEY (fallback EMAIL_API_KEY) is required; SENDGRID_FROM
// (fallback EMAIL_FROM) defaults to the platform no-reply address.
// The second return value is false when no key is configured — the caller
// must keep the log fallback in that case.
func NewSendGridSenderFromEnv(log *zap.Logger) (*SendGridSender, bool) {
	if log == nil {
		log = zap.NewNop()
	}
	key := firstEnv("SENDGRID_API_KEY", "EMAIL_API_KEY")
	if key == "" {
		return nil, false
	}
	from := firstEnv("SENDGRID_FROM", "EMAIL_FROM")
	if from == "" {
		from = "noreply@opuscasino.com"
	}
	return &SendGridSender{
		apiKey:   key,
		from:     from,
		endpoint: endpointOr("SENDGRID_API_URL", sendGridEndpoint),
		http:     newHTTPClient(),
		log:      log,
	}, true
}

type sendGridPayload struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridAddress           `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

type sendGridPersonalization struct {
	To []sendGridAddress `json:"to"`
}

type sendGridAddress struct {
	Email string `json:"email"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SendEmail posts one transactional email. The key travels only in the
// Authorization header; it is never logged.
func (s *SendGridSender) SendEmail(ctx context.Context, msg service.EmailMessage) error {
	to := strings.TrimSpace(msg.To)
	if to == "" {
		return fmt.Errorf("sendgrid: recipient email is empty")
	}
	body, err := json.Marshal(sendGridPayload{
		Personalizations: []sendGridPersonalization{{To: []sendGridAddress{{Email: to}}}},
		From:             sendGridAddress{Email: s.from},
		Subject:          msg.Subject,
		Content:          []sendGridContent{{Type: "text/plain", Value: msg.Body}},
	})
	if err != nil {
		return fmt.Errorf("sendgrid: encode payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sendgrid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid: unexpected status %d: %s", resp.StatusCode, errorSnippet(resp.Body))
	}
	s.log.Info("email sent via sendgrid", zap.String("subject", msg.Subject))
	return nil
}
