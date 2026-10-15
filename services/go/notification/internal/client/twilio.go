package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

// twilioEndpoint is the Twilio Programmable Messaging API root. Tests point
// TWILIO_API_URL at an httptest server instead.
const twilioEndpoint = "https://api.twilio.com"

// TwilioSender delivers SMS via the Twilio Messages API (plain HTTPS, no SDK).
// It implements service.SMSSender without changing the interface.
type TwilioSender struct {
	accountSID string
	authToken  string
	from       string
	endpoint   string
	http       *http.Client
	log        *zap.Logger
}

// Compile-time guarantee: the wave1 interface is implemented as-is.
var _ service.SMSSender = (*TwilioSender)(nil)

// NewTwilioSenderFromEnv builds the sender from the environment.
// TWILIO_ACCOUNT_SID (fallback SMS_API_KEY) and TWILIO_AUTH_TOKEN (fallback
// SMS_API_SECRET) are required; TWILIO_FROM_NUMBER (fallback SMS_FROM) is the
// provisioned sender number. The second return value is false when the pair
// is not configured — the caller must keep the log fallback in that case.
func NewTwilioSenderFromEnv(log *zap.Logger) (*TwilioSender, bool) {
	if log == nil {
		log = zap.NewNop()
	}
	sid := firstEnv("TWILIO_ACCOUNT_SID", "SMS_API_KEY")
	token := firstEnv("TWILIO_AUTH_TOKEN", "SMS_API_SECRET")
	if sid == "" || token == "" {
		return nil, false
	}
	return &TwilioSender{
		accountSID: sid,
		authToken:  token,
		from:       firstEnv("TWILIO_FROM_NUMBER", "SMS_FROM"),
		endpoint:   endpointOr("TWILIO_API_URL", twilioEndpoint),
		http:       newHTTPClient(),
		log:        log,
	}, true
}

// SendSMS posts one message. Credentials travel only via HTTP Basic Auth;
// they are never logged.
func (s *TwilioSender) SendSMS(ctx context.Context, msg service.SMSMessage) error {
	to := strings.TrimSpace(msg.To)
	if to == "" {
		return fmt.Errorf("twilio: recipient phone is empty")
	}
	if s.from == "" {
		return fmt.Errorf("twilio: sender number is not configured (TWILIO_FROM_NUMBER)")
	}

	form := url.Values{
		"To":   {to},
		"From": {s.from},
		"Body": {msg.Body},
	}
	target := s.endpoint + "/2010-04-01/Accounts/" + url.PathEscape(s.accountSID) + "/Messages.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio: unexpected status %d: %s", resp.StatusCode, errorSnippet(resp.Body))
	}
	s.log.Info("sms sent via twilio")
	return nil
}
