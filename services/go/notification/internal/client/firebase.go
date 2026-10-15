package client

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
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

// fcmScope is the OAuth scope for Firebase Cloud Messaging.
const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// fcmEndpoint is the FCM v1 API root. Tests point FIREBASE_FCM_URL
// at an httptest server instead.
const fcmEndpoint = "https://fcm.googleapis.com"

// FirebaseSender delivers push via FCM HTTP v1 (plain HTTPS, no SDK).
// OAuth2 access tokens are minted from a service-account key with stdlib
// RSA signing and cached until expiry. It implements service.PushSender
// without changing the interface.
//
// NOTE: the retired FCM legacy server key (PUSH_FIREBASE_KEY) is
// deliberately NOT supported — Google shut the legacy endpoint down.
// Configure one of FIREBASE_SERVICE_ACCOUNT_JSON (inline JSON),
// GOOGLE_APPLICATION_CREDENTIALS or FIREBASE_SERVICE_ACCOUNT_PATH (file
// path). In Kubernetes the file arrives via a mounted Secret (Vault refs),
// never via code.
type FirebaseSender struct {
	projectID  string
	email      string
	privateKey *rsa.PrivateKey
	tokenURL   string
	fcmURL     string
	http       *http.Client
	log        *zap.Logger

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// Compile-time guarantee: the wave1 interface is implemented as-is.
var _ service.PushSender = (*FirebaseSender)(nil)

type serviceAccountJSON struct {
	ProjectID  string `json:"project_id"`
	PrivateKey string `json:"private_key"`
	ClientMail string `json:"client_email"`
	TokenURI   string `json:"token_uri"`
}

// NewFirebaseSenderFromEnv builds the sender from the environment.
// The second return value is false when no service-account material is
// configured — the caller must keep the log fallback in that case.
func NewFirebaseSenderFromEnv(log *zap.Logger) (*FirebaseSender, bool) {
	if log == nil {
		log = zap.NewNop()
	}
	raw := firstEnv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if raw == "" {
		if p := firstEnv("FIREBASE_SERVICE_ACCOUNT_PATH", "GOOGLE_APPLICATION_CREDENTIALS"); p != "" {
			data, err := os.ReadFile(p)
			if err != nil {
				log.Warn("firebase: cannot read service-account file", zap.Error(err))
				return nil, false
			}
			raw = string(data)
		}
	}
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	sender, err := newFirebaseSender(raw,
		endpointOr("FIREBASE_FCM_URL", fcmEndpoint), newHTTPClient(), log)
	if err != nil {
		log.Warn("firebase: invalid service-account material", zap.Error(err))
		return nil, false
	}
	return sender, true
}

func newFirebaseSender(saJSON, fcmURL string, httpClient *http.Client, log *zap.Logger) (*FirebaseSender, error) {
	var sa serviceAccountJSON
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return nil, fmt.Errorf("parse service account: %w", err)
	}
	projectID := firstEnv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		projectID = strings.TrimSpace(sa.ProjectID)
	}
	if projectID == "" || strings.TrimSpace(sa.ClientMail) == "" || strings.TrimSpace(sa.PrivateKey) == "" {
		return nil, fmt.Errorf("service account misses project_id, client_email or private_key")
	}
	key, err := parseRSAPrivateKey(sa.PrivateKey)
	if err != nil {
		return nil, err
	}
	tokenURL := strings.TrimSpace(sa.TokenURI)
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	return &FirebaseSender{
		projectID:  projectID,
		email:      strings.TrimSpace(sa.ClientMail),
		privateKey: key,
		tokenURL:   tokenURL,
		fcmURL:     strings.TrimRight(fcmURL, "/"),
		http:       httpClient,
		log:        log,
	}, nil
}

func parseRSAPrivateKey(pemText string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, fmt.Errorf("private key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

// accessToken returns a cached OAuth token or mints a fresh one.
func (s *FirebaseSender) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenExp.Add(-time.Minute)) {
		return s.token, nil
	}
	token, exp, err := s.mintTokenLocked(ctx)
	if err != nil {
		return "", err
	}
	s.token, s.tokenExp = token, exp
	return token, nil
}

// invalidateToken drops the cached token so the next call re-mints.
func (s *FirebaseSender) invalidateToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = ""
}

// mintTokenLocked performs the JWT-bearer OAuth2 exchange. Caller holds s.mu.
func (s *FirebaseSender) mintTokenLocked(ctx context.Context) (string, time.Time, error) {
	now := time.Now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]interface{}{
		"iss":   s.email,
		"scope": fcmScope,
		"aud":   s.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firebase: encode claims: %w", err)
	}
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firebase: sign assertion: %w", err)
	}
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + assertion
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firebase: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firebase: token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("firebase: token status %d: %s", resp.StatusCode, errorSnippet(resp.Body))
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("firebase: decode token: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("firebase: empty access token")
	}
	ttl := time.Duration(tokenResp.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	return tokenResp.AccessToken, now.Add(ttl), nil
}

type fcmSendRequest struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// SendPush delivers one push. The device token comes from message metadata
// (device_token / fcm_token); without it there is nothing to address.
func (s *FirebaseSender) SendPush(ctx context.Context, msg service.PushMessage) error {
	deviceToken := ""
	if msg.Metadata != nil {
		deviceToken = firstNonEmpty(msg.Metadata["device_token"], msg.Metadata["fcm_token"])
	}
	if strings.TrimSpace(deviceToken) == "" {
		return fmt.Errorf("firebase: device token is missing (metadata device_token)")
	}

	send := func(token string) (int, string, error) {
		payload, err := json.Marshal(fcmSendRequest{
			Message: fcmMessage{
				Token:        deviceToken,
				Notification: fcmNotification{Title: msg.Title, Body: msg.Body},
				Data:         msg.Metadata,
			},
		})
		if err != nil {
			return 0, "", fmt.Errorf("firebase: encode message: %w", err)
		}
		target := s.fcmURL + "/v1/projects/" + s.projectID + "/messages:send"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
		if err != nil {
			return 0, "", fmt.Errorf("firebase: build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.http.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("firebase: post: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusUnauthorized {
			return resp.StatusCode, "", errUnauthorized
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.StatusCode, errorSnippet(resp.Body), nil
		}
		return resp.StatusCode, "", nil
	}

	access, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	status, snippet, err := send(access)
	if err != nil {
		if err == errUnauthorized {
			// Token may have been revoked early: refresh once and retry.
			s.invalidateToken()
			access, err = s.accessToken(ctx)
			if err != nil {
				return err
			}
			status, snippet, err = send(access)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("firebase: unexpected status %d: %s", status, snippet)
	}
	s.log.Info("push sent via fcm")
	return nil
}

// errUnauthorized is a sentinel for the single 401 retry path.
var errUnauthorized = fmt.Errorf("unauthorized")

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
