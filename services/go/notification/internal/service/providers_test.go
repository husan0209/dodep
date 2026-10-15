package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ============ SendGrid ============

func TestSendGridEmailSender_SendEmail(t *testing.T) {
	t.Run("success returns nil on 202 and sends bearer auth", func(t *testing.T) {
		var gotAuth, gotContentType, gotBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			w.WriteHeader(http.StatusAccepted) // SendGrid returns 202 with empty body
		}))
		defer server.Close()

		sender := NewSendGridEmailSender(SendGridConfig{
			APIKey:  "sg-test-key",
			From:    "noreply@opuscasino.com",
			BaseURL: server.URL,
		}, nil, zap.NewNop())

		err := sender.SendEmail(context.Background(), EmailMessage{
			To:      "player@example.com",
			Subject: "Deposit confirmed",
			Body:    "Your deposit was confirmed",
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if gotAuth != "Bearer sg-test-key" {
			t.Fatalf("expected bearer auth header, got %q", gotAuth)
		}
		if gotContentType != "application/json" {
			t.Fatalf("expected json content type, got %q", gotContentType)
		}

		var payload sendGridMessage
		if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
			t.Fatalf("expected valid json payload, got: %v", err)
		}
		if payload.From.Email != "noreply@opuscasino.com" {
			t.Fatalf("unexpected from: %q", payload.From.Email)
		}
		if len(payload.Personalizations) != 1 || len(payload.Personalizations[0].To) != 1 ||
			payload.Personalizations[0].To[0].Email != "player@example.com" {
			t.Fatalf("unexpected personalizations: %+v", payload.Personalizations)
		}
		if payload.Subject != "Deposit confirmed" || len(payload.Content) != 1 ||
			payload.Content[0].Value != "Your deposit was confirmed" {
			t.Fatalf("unexpected subject/content: %+v", payload)
		}
	})

	t.Run("provider error surfaces status and body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"message":"invalid from address"}]}`))
		}))
		defer server.Close()

		sender := NewSendGridEmailSender(SendGridConfig{APIKey: "k", From: "a@b.c", BaseURL: server.URL}, nil, zap.NewNop())
		err := sender.SendEmail(context.Background(), EmailMessage{To: "x@y.z", Subject: "s", Body: "b"})
		if err == nil {
			t.Fatal("expected an error for a 4xx provider response")
		}
		if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid from address") {
			t.Fatalf("error should carry status and provider detail, got: %v", err)
		}
	})

	t.Run("missing recipient is rejected before any HTTP call", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		defer server.Close()

		sender := NewSendGridEmailSender(SendGridConfig{APIKey: "k", BaseURL: server.URL}, nil, zap.NewNop())
		if err := sender.SendEmail(context.Background(), EmailMessage{Subject: "s"}); err == nil {
			t.Fatal("expected an error for a missing recipient")
		}
		if called {
			t.Fatal("no HTTP call should be made when validation fails")
		}
	})

	t.Run("missing api key is rejected", func(t *testing.T) {
		sender := NewSendGridEmailSender(SendGridConfig{From: "a@b.c"}, nil, zap.NewNop())
		if err := sender.SendEmail(context.Background(), EmailMessage{To: "x@y.z"}); err == nil {
			t.Fatal("expected an error when the api key is empty")
		}
	})
}

// ============ Twilio ============

func TestTwilioSMSSender_SendSMS(t *testing.T) {
	t.Run("success posts form with basic auth", func(t *testing.T) {
		var gotUser, gotPass, gotContentType, gotBody string
		var gotOK bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser, gotPass, gotOK = r.BasicAuth()
			gotContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		sender := NewTwilioSMSSender(TwilioConfig{
			AccountSID: "AC123",
			AuthToken:  "token123",
			FromNumber: "+10000000001",
			BaseURL:    server.URL,
		}, nil, zap.NewNop())

		err := sender.SendSMS(context.Background(), SMSMessage{To: "+20000000002", Body: "Your code is 123456"})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if !gotOK || gotUser != "AC123" || gotPass != "token123" {
			t.Fatalf("expected twilio basic auth AC123/token123, got %q/%q ok=%v", gotUser, gotPass, gotOK)
		}
		if gotContentType != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content type: %q", gotContentType)
		}
		for _, want := range []string{"To=%2B20000000002", "From=%2B10000000001", "Body=Your+code+is+123456"} {
			if !strings.Contains(gotBody, want) {
				t.Fatalf("form body %q missing %q", gotBody, want)
			}
		}
	})

	t.Run("provider error surfaces twilio message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":21211,"message":"Invalid 'To' phone number"}`))
		}))
		defer server.Close()

		sender := NewTwilioSMSSender(TwilioConfig{AccountSID: "AC", AuthToken: "t", FromNumber: "+1", BaseURL: server.URL}, nil, zap.NewNop())
		err := sender.SendSMS(context.Background(), SMSMessage{To: "not-a-phone", Body: "x"})
		if err == nil {
			t.Fatal("expected an error for a 4xx provider response")
		}
		if !strings.Contains(err.Error(), "Invalid 'To' phone number") {
			t.Fatalf("error should carry the twilio detail, got: %v", err)
		}
	})

	t.Run("missing destination is rejected before any HTTP call", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		defer server.Close()

		sender := NewTwilioSMSSender(TwilioConfig{AccountSID: "AC", AuthToken: "t", FromNumber: "+1", BaseURL: server.URL}, nil, zap.NewNop())
		if err := sender.SendSMS(context.Background(), SMSMessage{Body: "x"}); err == nil {
			t.Fatal("expected an error for a missing destination")
		}
		if called {
			t.Fatal("no HTTP call should be made when validation fails")
		}
	})
}

// ============ Firebase FCM v1 ============

func TestFCMPushSender_SendPush(t *testing.T) {
	t.Run("success sends message with oauth token and caches it", func(t *testing.T) {
		var tokenCalls int
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			tokenCalls++
			_, _ = w.Write([]byte(`{"access_token":"ya29.test-token","expires_in":3600}`))
		}))
		defer tokenServer.Close()

		var gotAuth, gotPath, gotBody string
		fcmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			_, _ = w.Write([]byte(`{"name":"projects/p/messages/1"}`))
		}))
		defer fcmServer.Close()

		sa := &firebaseServiceAccount{
			ClientEmail: "sa@project.iam.gserviceaccount.com",
			TokenURI:    tokenServer.URL,
			PrivateKey:  testRSAPrivateKeyPEM(t),
		}
		sender := NewFCMPushSender(FCMConfig{ProjectID: "p", BaseURL: fcmServer.URL, ServiceAccount: sa}, nil, zap.NewNop())

		msg := PushMessage{
			UserID:   42,
			Title:    "Bet settled",
			Body:     "You won 10 USD",
			Metadata: map[string]string{"device_token": "fcm-device-token-1"},
		}
		if err := sender.SendPush(context.Background(), msg); err != nil {
			t.Fatalf("first send: %v", err)
		}
		if err := sender.SendPush(context.Background(), msg); err != nil {
			t.Fatalf("second send: %v", err)
		}

		if gotAuth != "Bearer ya29.test-token" {
			t.Fatalf("expected bearer oauth token, got %q", gotAuth)
		}
		if gotPath != "/v1/projects/p/messages:send" {
			t.Fatalf("unexpected fcm path: %q", gotPath)
		}
		if tokenCalls != 1 {
			t.Fatalf("expected the oauth token to be minted once and cached, got %d calls", tokenCalls)
		}

		var payload fcmSendRequest
		if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
			t.Fatalf("invalid fcm payload: %v", err)
		}
		if payload.Message.Token != "fcm-device-token-1" {
			t.Fatalf("unexpected device token: %q", payload.Message.Token)
		}
		if payload.Message.Notification.Title != "Bet settled" || payload.Message.Notification.Body != "You won 10 USD" {
			t.Fatalf("unexpected notification body: %+v", payload.Message.Notification)
		}
	})

	t.Run("fcm_token metadata key is accepted", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
		}))
		defer tokenServer.Close()

		var gotBody string
		fcmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
		}))
		defer fcmServer.Close()

		sender := NewFCMPushSender(FCMConfig{
			ProjectID: "p",
			BaseURL:   fcmServer.URL,
			ServiceAccount: &firebaseServiceAccount{
				ClientEmail: "sa@p.iam.gserviceaccount.com",
				TokenURI:    tokenServer.URL,
				PrivateKey:  testRSAPrivateKeyPEM(t),
			},
		}, nil, zap.NewNop())

		if err := sender.SendPush(context.Background(), PushMessage{
			Metadata: map[string]string{"fcm_token": "tok-2"},
		}); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if !strings.Contains(gotBody, "tok-2") {
			t.Fatalf("expected device token in payload, got: %s", gotBody)
		}
	})

	t.Run("missing device token is rejected", func(t *testing.T) {
		sender := NewFCMPushSender(FCMConfig{
			ProjectID: "p",
			ServiceAccount: &firebaseServiceAccount{
				ClientEmail: "sa@p.iam.gserviceaccount.com",
				TokenURI:    "http://127.0.0.1:1/unused",
				PrivateKey:  testRSAPrivateKeyPEM(t),
			},
		}, nil, zap.NewNop())

		if err := sender.SendPush(context.Background(), PushMessage{Metadata: map[string]string{}}); err == nil {
			t.Fatal("expected an error when no device token is present")
		}
	})

	t.Run("provider error surfaces status", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
		}))
		defer tokenServer.Close()

		fcmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"status":"UNAUTHENTICATED"}}`))
		}))
		defer fcmServer.Close()

		sender := NewFCMPushSender(FCMConfig{
			ProjectID: "p",
			BaseURL:   fcmServer.URL,
			ServiceAccount: &firebaseServiceAccount{
				ClientEmail: "sa@p.iam.gserviceaccount.com",
				TokenURI:    tokenServer.URL,
				PrivateKey:  testRSAPrivateKeyPEM(t),
			},
		}, nil, zap.NewNop())

		err := sender.SendPush(context.Background(), PushMessage{Metadata: map[string]string{"device_token": "tok"}})
		if err == nil {
			t.Fatal("expected an error for a 4xx provider response")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Fatalf("error should carry the provider status, got: %v", err)
		}
	})

	t.Run("token endpoint failure propagates", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		defer tokenServer.Close()

		sender := NewFCMPushSender(FCMConfig{
			ProjectID: "p",
			BaseURL:   "http://127.0.0.1:1/unused",
			ServiceAccount: &firebaseServiceAccount{
				ClientEmail: "sa@p.iam.gserviceaccount.com",
				TokenURI:    tokenServer.URL,
				PrivateKey:  testRSAPrivateKeyPEM(t),
			},
		}, nil, zap.NewNop())

		err := sender.SendPush(context.Background(), PushMessage{Metadata: map[string]string{"device_token": "tok"}})
		if err == nil {
			t.Fatal("expected an error when the token endpoint fails")
		}
		if !strings.Contains(err.Error(), "token endpoint") {
			t.Fatalf("error should mention the token endpoint, got: %v", err)
		}
	})
}

func TestOAuthTokenProvider_JWTAssertionIsRS256(t *testing.T) {
	var gotGrantType, gotAssertion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotGrantType = r.FormValue("grant_type")
		gotAssertion = r.FormValue("assertion")
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	}))
	defer server.Close()

	provider := &oauthTokenProvider{
		sa: &firebaseServiceAccount{
			ClientEmail: "sa@p.iam.gserviceaccount.com",
			TokenURI:    server.URL,
			PrivateKey:  testRSAPrivateKeyPEM(t),
		},
		client: &http.Client{Timeout: time.Second},
	}

	token, err := provider.accessToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok" {
		t.Fatalf("unexpected token: %q", token)
	}
	if gotGrantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		t.Fatalf("unexpected grant type: %q", gotGrantType)
	}

	parts := strings.Split(gotAssertion, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion must have 3 parts, got %d", len(parts))
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("invalid header encoding: %v", err)
	}
	if !strings.Contains(string(headerRaw), `"alg":"RS256"`) {
		t.Fatalf("unexpected header: %s", headerRaw)
	}
	claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("invalid claims encoding: %v", err)
	}
	for _, want := range []string{`"iss":"sa@p.iam.gserviceaccount.com"`, "firebase.messaging", `"aud"`} {
		if !strings.Contains(string(claimsRaw), want) {
			t.Fatalf("claims %s missing %q", claimsRaw, want)
		}
	}
}

func TestLoadFirebaseServiceAccount(t *testing.T) {
	t.Run("inline json", func(t *testing.T) {
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", `{"client_email":"sa@p","private_key":"k","token_uri":"https://oauth2.googleapis.com/token"}`)
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "")

		sa, err := loadFirebaseServiceAccount()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sa == nil || sa.ClientEmail != "sa@p" {
			t.Fatalf("unexpected account: %+v", sa)
		}
	})

	t.Run("neither configured returns nil", func(t *testing.T) {
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "")

		sa, err := loadFirebaseServiceAccount()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sa != nil {
			t.Fatalf("expected nil account, got %+v", sa)
		}
	})

	t.Run("invalid json errors", func(t *testing.T) {
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", `{not json`)
		t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "")

		if _, err := loadFirebaseServiceAccount(); err == nil {
			t.Fatal("expected an error for malformed json")
		}
	})
}

func TestNewChannelSendersFromEnv_FallbacksToStubs(t *testing.T) {
	for _, key := range []string{
		"SENDGRID_API_KEY", "EMAIL_API_KEY",
		"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_FROM_NUMBER", "TWILIO_FROM",
		"FIREBASE_PROJECT_ID", "FIREBASE_SERVICE_ACCOUNT_JSON", "FIREBASE_SERVICE_ACCOUNT_FILE",
	} {
		t.Setenv(key, "")
	}

	email, sms, push := NewChannelSendersFromEnv(DefaultChannelConfigFromEnv(), zap.NewNop(), nil)

	if _, ok := email.(*LogEmailSender); !ok {
		t.Fatalf("expected LogEmailSender fallback, got %T", email)
	}
	if _, ok := sms.(*LogSMSSender); !ok {
		t.Fatalf("expected LogSMSSender fallback, got %T", sms)
	}
	if _, ok := push.(*LogPushSender); !ok {
		t.Fatalf("expected LogPushSender fallback, got %T", push)
	}
}

func TestNewChannelSendersFromEnv_SelectsRealProviders(t *testing.T) {
	saJSON := `{"client_email":"sa@p.iam.gserviceaccount.com","private_key":"` +
		strings.ReplaceAll(testRSAPrivateKeyPEM(t), "\n", `\n`) + `"}`

	t.Setenv("EMAIL_PROVIDER", "sendgrid")
	t.Setenv("SMS_PROVIDER", "twilio")
	t.Setenv("PUSH_PROVIDER", "firebase")
	t.Setenv("SENDGRID_API_KEY", "sg-key")
	t.Setenv("EMAIL_API_KEY", "")
	t.Setenv("TWILIO_ACCOUNT_SID", "AC")
	t.Setenv("TWILIO_AUTH_TOKEN", "tok")
	t.Setenv("TWILIO_FROM_NUMBER", "+1")
	t.Setenv("TWILIO_FROM", "")
	t.Setenv("FIREBASE_PROJECT_ID", "proj")
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", saJSON)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "")

	email, sms, push := NewChannelSendersFromEnv(DefaultChannelConfigFromEnv(), zap.NewNop(), nil)

	if _, ok := email.(*SendGridEmailSender); !ok {
		t.Fatalf("expected SendGridEmailSender, got %T", email)
	}
	if _, ok := sms.(*TwilioSMSSender); !ok {
		t.Fatalf("expected TwilioSMSSender, got %T", sms)
	}
	if _, ok := push.(*FCMPushSender); !ok {
		t.Fatalf("expected FCMPushSender, got %T", push)
	}
}

// testRSAPrivateKeyPEM generates a throwaway RSA key for JWT signing tests.
func testRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}
