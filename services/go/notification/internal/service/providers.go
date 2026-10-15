package service

// Real channel providers for the notification service.
//
// All three providers talk to the public REST APIs of the PSPs over plain
// net/http, so the service gains zero new module dependencies. Credentials
// come from environment variables only (NEVER-5: no secrets in code) and the
// factory below keeps the Log* stubs as the default whenever credentials are
// absent, which is why existing unit tests observe stub behavior.

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// providerHTTPTimeout bounds every outbound PSP call. Notification delivery is
// asynchronous work: a hung PSP must not pin a request goroutine forever.
const providerHTTPTimeout = 10 * time.Second

// fcmScopes is the OAuth2 scope set for FCM HTTP v1 sends.
const fcmScopes = "https://www.googleapis.com/auth/firebase.messaging https://www.googleapis.com/auth/cloud-platform"

// NewChannelSendersFromEnv builds the production channel senders from env.
//
// Selection rules (all env, no secrets defaulted):
//   - email: EMAIL_PROVIDER=sendgrid + key (SENDGRID_API_KEY, EMAIL_API_KEY alias)
//     -> SendGridEmailSender
//   - sms:   SMS_PROVIDER=twilio + TWILIO_ACCOUNT_SID/TWILIO_AUTH_TOKEN and
//     sender (TWILIO_FROM_NUMBER, TWILIO_FROM alias) -> TwilioSMSSender
//   - push:  PUSH_PROVIDER=firebase + FIREBASE_PROJECT_ID + service account
//     (FIREBASE_SERVICE_ACCOUNT_JSON inline or FIREBASE_SERVICE_ACCOUNT_FILE path)
//     -> FCMPushSender
//
// Anything else falls back to the Log* stubs, preserving the previous
// behavior for development and for environments without credentials.
// client may be nil (http.DefaultClient is used); tests inject their own.
func NewChannelSendersFromEnv(cfg ChannelConfig, log *zap.Logger, client *http.Client) (EmailSender, SMSSender, PushSender) {
	if log == nil {
		log = zap.NewNop()
	}
	if client == nil {
		client = &http.Client{Timeout: providerHTTPTimeout}
	}

	sendGridKey := firstNonEmpty(os.Getenv("SENDGRID_API_KEY"), os.Getenv("EMAIL_API_KEY"))
	email := EmailSender(NewLogEmailSender(cfg, log))
	if cfg.EmailProvider == "sendgrid" && sendGridKey != "" {
		email = NewSendGridEmailSender(SendGridConfig{
			APIKey:  sendGridKey,
			From:    cfg.EmailFrom,
			BaseURL: envOr("SENDGRID_BASE_URL", "https://api.sendgrid.com"),
		}, client, log)
	}

	twilioSID := os.Getenv("TWILIO_ACCOUNT_SID")
	twilioToken := os.Getenv("TWILIO_AUTH_TOKEN")
	twilioFrom := firstNonEmpty(os.Getenv("TWILIO_FROM_NUMBER"), os.Getenv("TWILIO_FROM"))
	sms := SMSSender(NewLogSMSSender(cfg, log))
	if cfg.SMSProvider == "twilio" && twilioSID != "" && twilioToken != "" && twilioFrom != "" {
		sms = NewTwilioSMSSender(TwilioConfig{
			AccountSID: twilioSID,
			AuthToken:  twilioToken,
			FromNumber: twilioFrom,
			BaseURL:    envOr("TWILIO_BASE_URL", "https://api.twilio.com"),
		}, client, log)
	}

	push := PushSender(NewLogPushSender(cfg, log))
	if cfg.PushProvider == "firebase" && os.Getenv("FIREBASE_PROJECT_ID") != "" {
		if sa, err := loadFirebaseServiceAccount(); err != nil {
			log.Error("Firebase push disabled: invalid service account", zap.Error(err))
		} else if sa != nil {
			push = NewFCMPushSender(FCMConfig{
				ProjectID:      os.Getenv("FIREBASE_PROJECT_ID"),
				BaseURL:        envOr("FCM_BASE_URL", "https://fcm.googleapis.com"),
				ServiceAccount: sa,
			}, client, log)
		}
	}

	return email, sms, push
}

// ============ SendGrid email ============

// SendGridConfig configures the SendGrid v3 mail send API client.
type SendGridConfig struct {
	APIKey  string
	From    string
	BaseURL string // default https://api.sendgrid.com
}

// SendGridEmailSender sends transactional email via the SendGrid REST API.
type SendGridEmailSender struct {
	cfg    SendGridConfig
	client *http.Client
	log    *zap.Logger
}

// NewSendGridEmailSender creates a SendGrid-backed email sender.
func NewSendGridEmailSender(cfg SendGridConfig, client *http.Client, log *zap.Logger) *SendGridEmailSender {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.sendgrid.com"
	}
	if client == nil {
		client = &http.Client{Timeout: providerHTTPTimeout}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &SendGridEmailSender{cfg: cfg, client: client, log: log}
}

type sendGridPersonalization struct {
	To []sendGridContact `json:"to"`
}

type sendGridContact struct {
	Email string `json:"email"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendGridMessage struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridContact            `json:"from"`
	Subject          string                     `json:"subject"`
	Content          []sendGridContent          `json:"content"`
}

// SendEmail posts one message to SendGrid. A 2xx response (SendGrid returns
// 202 Accepted with an empty body) is success; anything else is an error that
// carries the provider status code for classification upstream.
func (s *SendGridEmailSender) SendEmail(ctx context.Context, msg EmailMessage) error {
	if s.cfg.APIKey == "" {
		return errors.New("sendgrid: api key is not configured")
	}
	if msg.To == "" {
		return errors.New("sendgrid: recipient email is required")
	}

	payload := sendGridMessage{
		Personalizations: []sendGridPersonalization{{To: []sendGridContact{{Email: msg.To}}}},
		From:             sendGridContact{Email: s.cfg.From},
		Subject:          msg.Subject,
		Content:          []sendGridContent{{Type: "text/plain", Value: msg.Body}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sendgrid: marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sendgrid: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("sendgrid: provider rejected email with status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}

	s.log.Info("Email sent via SendGrid", zap.String("subject", msg.Subject))
	return nil
}

// ============ Twilio SMS ============

// TwilioConfig configures the Twilio Programmable SMS client.
type TwilioConfig struct {
	AccountSID string
	AuthToken  string
	FromNumber string
	BaseURL    string // default https://api.twilio.com
}

// TwilioSMSSender sends SMS via the Twilio Messages REST API.
type TwilioSMSSender struct {
	cfg    TwilioConfig
	client *http.Client
	log    *zap.Logger
}

// NewTwilioSMSSender creates a Twilio-backed SMS sender.
func NewTwilioSMSSender(cfg TwilioConfig, client *http.Client, log *zap.Logger) *TwilioSMSSender {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.twilio.com"
	}
	if client == nil {
		client = &http.Client{Timeout: providerHTTPTimeout}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &TwilioSMSSender{cfg: cfg, client: client, log: log}
}

type twilioErrorResponse struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
}

// SendSMS posts one message to Twilio. Success is 201 Created; Twilio reports
// failures as JSON {code,message} with a 4xx status.
func (s *TwilioSMSSender) SendSMS(ctx context.Context, msg SMSMessage) error {
	if msg.To == "" {
		return errors.New("twilio: destination phone is required")
	}

	form := url.Values{}
	form.Set("To", msg.To)
	form.Set("From", s.cfg.FromNumber)
	form.Set("Body", msg.Body)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", strings.TrimRight(s.cfg.BaseURL, "/"), s.cfg.AccountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(s.cfg.AccountSID, s.cfg.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var errResp twilioErrorResponse
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if json.Unmarshal(detail, &errResp) == nil && errResp.Message != "" {
			return fmt.Errorf("twilio: provider rejected sms with status %d: %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("twilio: provider rejected sms with status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}

	s.log.Info("SMS sent via Twilio")
	return nil
}

// ============ Firebase Cloud Messaging (HTTP v1) ============

// FCMConfig configures the FCM HTTP v1 client.
type FCMConfig struct {
	ProjectID      string
	BaseURL        string // default https://fcm.googleapis.com
	ServiceAccount *firebaseServiceAccount
}

// firebaseServiceAccount is the subset of the GCP service-account JSON used
// for OAuth2 JWT-bearer minting. Loaded from env, never from code.
type firebaseServiceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// loadFirebaseServiceAccount reads the service account from
// FIREBASE_SERVICE_ACCOUNT_JSON (inline JSON) or
// FIREBASE_SERVICE_ACCOUNT_FILE (path). Returns (nil, nil) when neither is
// configured so the caller can fall back to the stub.
func loadFirebaseServiceAccount() (*firebaseServiceAccount, error) {
	raw := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if raw == "" {
		path := os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE")
		if path == "" {
			return nil, nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read service account file: %w", err)
		}
		raw = string(content)
	}

	var sa firebaseServiceAccount
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return nil, fmt.Errorf("parse service account json: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("service account json missing client_email or private_key")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &sa, nil
}

// oauthTokenProvider mints and caches OAuth2 access tokens via the
// service-account JWT-bearer flow. Tokens are refreshed a minute before
// expiry; failures invalidate the cache so the next call retries.
type oauthTokenProvider struct {
	sa     *firebaseServiceAccount
	client *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

func (p *oauthTokenProvider) accessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Now().Add(time.Minute).Before(p.expires) {
		return p.token, nil
	}

	assertion, err := p.signedJWT(time.Now())
	if err != nil {
		p.token = ""
		return "", err
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.sa.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		p.token = ""
		return "", fmt.Errorf("fcm: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		p.token = ""
		return "", fmt.Errorf("fcm: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		p.token = ""
		return "", fmt.Errorf("fcm: token endpoint rejected request with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		p.token = ""
		return "", fmt.Errorf("fcm: parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		p.token = ""
		return "", errors.New("fcm: token response missing access_token")
	}

	p.token = tokenResp.AccessToken
	ttl := time.Hour
	if tokenResp.ExpiresIn > 0 {
		ttl = time.Duration(tokenResp.ExpiresIn) * time.Second
	}
	p.expires = time.Now().Add(ttl)
	return p.token, nil
}

// signedJWT builds the RS256 service-account assertion for the JWT-bearer
// flow: header+claims, base64url-encoded and signed with the account key.
func (p *oauthTokenProvider) signedJWT(now time.Time) (string, error) {
	key, err := parseRSAPrivateKey(p.sa.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("fcm: parse private key: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(
		`{"iss":%q,"scope":%q,"aud":%q,"iat":%d,"exp":%d}`,
		p.sa.ClientEmail, fcmScopes, p.sa.TokenURI,
		now.Unix(), now.Add(time.Hour).Unix(),
	)
	claimsEncoded := base64.RawURLEncoding.EncodeToString([]byte(claims))

	signingInput := header + "." + claimsEncoded
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("fcm: sign assertion: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseRSAPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("private key is not valid PEM")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unsupported private key format: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}
	return key, nil
}

// FCMPushSender sends push notifications via the FCM HTTP v1 API.
type FCMPushSender struct {
	cfg    FCMConfig
	client *http.Client
	tokens *oauthTokenProvider
	log    *zap.Logger
}

// NewFCMPushSender creates an FCM-backed push sender.
func NewFCMPushSender(cfg FCMConfig, client *http.Client, log *zap.Logger) *FCMPushSender {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://fcm.googleapis.com"
	}
	if client == nil {
		client = &http.Client{Timeout: providerHTTPTimeout}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &FCMPushSender{
		cfg:    cfg,
		client: client,
		tokens: &oauthTokenProvider{sa: cfg.ServiceAccount, client: client},
		log:    log,
	}
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification    `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmSendRequest struct {
	Message fcmMessage `json:"message"`
}

// SendPush posts one message to FCM. The device token must be supplied in
// msg.Metadata under "device_token" (or "fcm_token") — the notification
// service does not store device registrations itself. Success is 200 with a
// {"name": ...} envelope.
func (s *FCMPushSender) SendPush(ctx context.Context, msg PushMessage) error {
	deviceToken := firstNonEmpty(msg.Metadata["device_token"], msg.Metadata["fcm_token"])
	if deviceToken == "" {
		return errors.New("fcm: device token is required (metadata device_token)")
	}

	accessToken, err := s.tokens.accessToken(ctx)
	if err != nil {
		return err
	}

	payload := fcmSendRequest{
		Message: fcmMessage{
			Token:        deviceToken,
			Notification: fcmNotification{Title: msg.Title, Body: msg.Body},
			Data:         msg.Metadata,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("fcm: marshal message: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/projects/%s/messages:send", strings.TrimRight(s.cfg.BaseURL, "/"), s.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("fcm: provider rejected push with status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}

	s.log.Info("Push sent via FCM", zap.Uint64("user_id", msg.UserID))
	return nil
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
