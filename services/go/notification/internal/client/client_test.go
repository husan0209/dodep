package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

func testLogger() *zap.Logger { return zap.NewNop() }

func clearChannelEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SENDGRID_API_KEY", "EMAIL_API_KEY", "SENDGRID_FROM", "EMAIL_FROM", "SENDGRID_API_URL",
		"TWILIO_ACCOUNT_SID", "SMS_API_KEY", "TWILIO_AUTH_TOKEN", "SMS_API_SECRET",
		"TWILIO_FROM_NUMBER", "SMS_FROM", "TWILIO_API_URL",
		"FIREBASE_SERVICE_ACCOUNT_JSON", "FIREBASE_SERVICE_ACCOUNT_PATH",
		"GOOGLE_APPLICATION_CREDENTIALS", "FIREBASE_PROJECT_ID", "FIREBASE_FCM_URL",
		"WS_GATEWAY_URL", "WS_GATEWAY_API_KEY", "WS_GATEWAY_PUBLISH_PATH",
	} {
		t.Setenv(k, "")
	}
}

func TestSendGrid_MissingKeyKeepsFallback(t *testing.T) {
	clearChannelEnv(t)
	_, ok := NewSendGridSenderFromEnv(testLogger())
	if ok {
		t.Fatal("no key must report not-configured so main keeps the log fallback")
	}
}

func TestSendGrid_SendUsesBearerKey(t *testing.T) {
	var gotAuth, gotFrom, gotSubject, gotTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var p sendGridPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		gotFrom = p.From.Email
		gotSubject = p.Subject
		if len(p.Personalizations) != 1 || len(p.Personalizations[0].To) != 1 {
			t.Errorf("unexpected personalizations: %+v", p.Personalizations)
		} else {
			gotTo = p.Personalizations[0].To[0].Email
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	clearChannelEnv(t)
	t.Setenv("SENDGRID_API_KEY", "SG.test-key-from-env")
	t.Setenv("SENDGRID_FROM", "sender@example.com")
	t.Setenv("SENDGRID_API_URL", srv.URL)

	sender, ok := NewSendGridSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured with key from env")
	}
	if err := sender.SendEmail(context.Background(), service.EmailMessage{
		To: "user@example.com", Subject: "hi", Body: "hello",
	}); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if gotAuth != "Bearer SG.test-key-from-env" {
		t.Errorf("auth header = %q, want bearer key from env", gotAuth)
	}
	if gotFrom != "sender@example.com" || gotSubject != "hi" || gotTo != "user@example.com" {
		t.Errorf("payload mismatch: from=%q subject=%q to=%q", gotFrom, gotSubject, gotTo)
	}
}

func TestSendGrid_UpstreamErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"message":"bad"}]}`))
	}))
	defer srv.Close()

	clearChannelEnv(t)
	t.Setenv("SENDGRID_API_KEY", "SG.k")
	t.Setenv("SENDGRID_API_URL", srv.URL)
	sender, ok := NewSendGridSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured")
	}
	err := sender.SendEmail(context.Background(), service.EmailMessage{To: "a@b.c", Subject: "s", Body: "b"})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected status 400 error, got %v", err)
	}
}

func TestSendGrid_EmptyRecipientRejected(t *testing.T) {
	clearChannelEnv(t)
	t.Setenv("SENDGRID_API_KEY", "SG.k")
	sender, ok := NewSendGridSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured")
	}
	if err := sender.SendEmail(context.Background(), service.EmailMessage{}); err == nil {
		t.Fatal("empty recipient must be rejected without network")
	}
}

func TestTwilio_MissingPairKeepsFallback(t *testing.T) {
	clearChannelEnv(t)
	t.Setenv("TWILIO_ACCOUNT_SID", "AC123")
	if _, ok := NewTwilioSenderFromEnv(testLogger()); ok {
		t.Fatal("half credentials must report not-configured")
	}
}

func TestTwilio_SendUsesBasicAuth(t *testing.T) {
	var gotUser, gotPass, gotForm, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		gotUser, gotPass, gotPath = u, p, r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotForm = r.Form.Get("To") + "|" + r.Form.Get("From") + "|" + r.Form.Get("Body")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM1"}`))
	}))
	defer srv.Close()

	clearChannelEnv(t)
	t.Setenv("TWILIO_ACCOUNT_SID", "ACsid-from-env")
	t.Setenv("TWILIO_AUTH_TOKEN", "token-from-env")
	t.Setenv("TWILIO_FROM_NUMBER", "+10000000000")
	t.Setenv("TWILIO_API_URL", srv.URL)

	sender, ok := NewTwilioSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured")
	}
	if err := sender.SendSMS(context.Background(), service.SMSMessage{
		To: "+79990000000", Body: "code 1234",
	}); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}
	if gotUser != "ACsid-from-env" || gotPass != "token-from-env" {
		t.Errorf("basic auth = %q/***, want env SID/token", gotUser)
	}
	if !strings.HasSuffix(gotPath, "/Messages.json") {
		t.Errorf("path = %q, want Messages.json", gotPath)
	}
	if gotForm != "+79990000000|+10000000000|code 1234" {
		t.Errorf("form = %q", gotForm)
	}
}

func TestGateway_MissingURLKeepsFallback(t *testing.T) {
	clearChannelEnv(t)
	if _, ok := NewGatewayPublisherFromEnv(testLogger()); ok {
		t.Fatal("missing URL must report not-configured")
	}
}

func TestGateway_PublishesEnvelope(t *testing.T) {
	var gotKey, gotEvent, gotPath string
	var gotUser uint64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("X-API-Key"), r.URL.Path
		var env gatewayEnvelope
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			t.Errorf("decode envelope: %v", err)
		}
		gotEvent, gotUser = env.Event, env.UserID
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	clearChannelEnv(t)
	t.Setenv("WS_GATEWAY_URL", srv.URL)
	t.Setenv("WS_GATEWAY_API_KEY", "edge-secret")

	pub, ok := NewGatewayPublisherFromEnv(testLogger())
	if !ok {
		t.Fatal("publisher must be configured")
	}
	if err := pub.PublishInApp(context.Background(), service.WSEvent{
		Event: "notification.created", UserID: 42, Payload: map[string]string{"type": "bet_settled"},
	}); err != nil {
		t.Fatalf("PublishInApp: %v", err)
	}
	if gotKey != "edge-secret" || gotEvent != "notification.created" || gotUser != 42 {
		t.Errorf("envelope mismatch: key=%q event=%q user=%d", gotKey, gotEvent, gotUser)
	}
	if gotPath != "/api/v1/publish" {
		t.Errorf("path = %q", gotPath)
	}
}

// testServiceAccount builds a throwaway RSA service-account JSON for tests.
func testServiceAccount(t *testing.T, tokenURL, project string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	raw, err := json.Marshal(map[string]string{
		"project_id":   project,
		"private_key":  pemKey,
		"client_email": "svc@test.iam.gserviceaccount.com",
		"token_uri":    tokenURL,
	})
	if err != nil {
		t.Fatalf("marshal sa: %v", err)
	}
	return string(raw)
}

func TestFirebase_MissingMaterialKeepsFallback(t *testing.T) {
	clearChannelEnv(t)
	if _, ok := NewFirebaseSenderFromEnv(testLogger()); ok {
		t.Fatal("missing service account must report not-configured")
	}
}

func TestFirebase_SendViaFCMv1(t *testing.T) {
	var sawBearer, sawPath, sawToken string
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBearer, sawPath = r.Header.Get("Authorization"), r.URL.Path
		var req fcmSendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode fcm request: %v", err)
		}
		sawToken = req.Message.Token
		if req.Message.Notification.Title != "Title" {
			t.Errorf("title = %q", req.Message.Notification.Title)
		}
		_, _ = w.Write([]byte(`{"name":"projects/test-proj/messages/1"}`))
	}))
	defer fcm.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer tokenSrv.Close()

	clearChannelEnv(t)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", testServiceAccount(t, tokenSrv.URL, "test-proj"))
	t.Setenv("FIREBASE_FCM_URL", fcm.URL)

	sender, ok := NewFirebaseSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured")
	}
	if err := sender.SendPush(context.Background(), service.PushMessage{
		UserID: 7, Title: "Title", Body: "Body",
		Metadata: map[string]string{"device_token": "device-123"},
	}); err != nil {
		t.Fatalf("SendPush: %v", err)
	}
	if sawBearer != "Bearer ya29.test" {
		t.Errorf("bearer = %q", sawBearer)
	}
	if sawPath != "/v1/projects/test-proj/messages:send" {
		t.Errorf("path = %q", sawPath)
	}
	if sawToken != "device-123" {
		t.Errorf("token = %q", sawToken)
	}
}

func TestFirebase_MissingDeviceTokenRejected(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"x","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	clearChannelEnv(t)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", testServiceAccount(t, tokenSrv.URL, "p"))
	sender, ok := NewFirebaseSenderFromEnv(testLogger())
	if !ok {
		t.Fatal("sender must be configured")
	}
	if err := sender.SendPush(context.Background(), service.PushMessage{UserID: 1}); err == nil {
		t.Fatal("missing device token must be rejected without network")
	}
}
