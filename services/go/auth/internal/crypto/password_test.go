package crypto

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("Str0ng!Passw0rd")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}

	ok, err := VerifyPassword("Str0ng!Passw0rd", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("correct password must verify")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword with wrong password: %v", err)
	}
	if ok {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashPasswordUsesUniqueSalts(t *testing.T) {
	first, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if first == second {
		t.Fatal("identical passwords must produce different hashes (unique salt)")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	valid, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	cases := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"too few parts", "$argon2id$v=19$m=65536"},
		{"no leading dollar", strings.TrimPrefix(valid, "$")},
		{"wrong algorithm", strings.Replace(valid, "$argon2id$", "$bcrypt$", 1)},
		{"wrong version", strings.Replace(valid, "$v=19$", "$v=16$", 1)},
		{"memory out of range (OOM)", "$argon2id$v=19$m=4294967295,t=3,p=4$c2FsdA$aGFzaA"},
		{"memory too small", "$argon2id$v=19$m=1,t=3,p=4$c2FsdA$aGFzaA"},
		{"iterations out of range", "$argon2id$v=19$m=65536,t=4294967295,p=4$c2FsdA$aGFzaA"},
		{"parallelism zero (panic risk)", "$argon2id$v=19$m=65536,t=3,p=0$c2FsdA$aGFzaA"},
		{"parallelism overflow", "$argon2id$v=19$m=65536,t=3,p=256$c2FsdA$aGFzaA"},
		{"bad salt encoding", "$argon2id$v=19$m=65536,t=3,p=4$not-base64!$aGFzaA"},
		{"bad hash encoding", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$not-base64!"},
		{"short salt", "$argon2id$v=19$m=65536,t=3,p=4$YQ$aGFzaGFzaGhhZ2hhZ2hhZ2g"},
		{"short hash", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ok, err := VerifyPassword("password123", tc.hash); err == nil && ok {
				t.Fatal("malformed hash must not verify")
			}
		})
	}
}

func TestGenerateRandomHelpers(t *testing.T) {
	b, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("GenerateRandomBytes: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}

	s, err := GenerateRandomString(24)
	if err != nil {
		t.Fatalf("GenerateRandomString: %v", err)
	}
	if s == "" {
		t.Fatal("random string must not be empty")
	}

	other, err := GenerateRandomString(24)
	if err != nil {
		t.Fatalf("GenerateRandomString: %v", err)
	}
	if s == other {
		t.Fatal("random strings must differ")
	}
}

func TestTOTPRoundTrip(t *testing.T) {
	cfg := DefaultTOTPConfig("player@example.com")
	secret, qrURI, err := cfg.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("secret must not be empty")
	}
	if !strings.HasPrefix(qrURI, "otpauth://totp/") {
		t.Fatalf("unexpected otpauth URI: %s", qrURI)
	}

	code, err := totp.GenerateCode(secret, nowForTest())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !ValidateTOTP(secret, code) {
		t.Fatal("freshly generated code must validate")
	}
	if code != "000000" && ValidateTOTP(secret, "000000") {
		t.Fatal("wrong code must not validate")
	}
	if ValidateTOTP(secret, "not-a-code") {
		t.Fatal("malformed code must not validate")
	}
}

func TestGenerateBackupCodesAreUnique(t *testing.T) {
	codes, err := GenerateBackupCodes(10)
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("expected 10 codes, got %d", len(codes))
	}

	seen := map[string]struct{}{}
	for _, code := range codes {
		if len(code) != 8 {
			t.Fatalf("expected 8-character code, got %q", code)
		}
		if _, dup := seen[code]; dup {
			t.Fatalf("duplicate backup code generated: %s", code)
		}
		seen[code] = struct{}{}
	}
}

func TestGenerateBackupCodesRejectsNonPositiveCount(t *testing.T) {
	if _, err := GenerateBackupCodes(0); err == nil {
		t.Fatal("count 0 must be rejected")
	}
	if _, err := GenerateBackupCodes(-1); err == nil {
		t.Fatal("negative count must be rejected")
	}
}

func TestGenerateTempToken(t *testing.T) {
	token, err := GenerateTempToken()
	if err != nil {
		t.Fatalf("GenerateTempToken: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("temp token too short: %d", len(token))
	}
	other, err := GenerateTempToken()
	if err != nil {
		t.Fatalf("GenerateTempToken: %v", err)
	}
	if token == other {
		t.Fatal("temp tokens must be unique")
	}
}

func TestTOTPExpiryIsInTheFuture(t *testing.T) {
	expiry := GetTOTPExpiry()
	if !expiry.After(nowForTest()) {
		t.Fatal("TOTP temp token must expire in the future")
	}
}

// nowForTest keeps time.Now() in one place for assertions.
func nowForTest() time.Time { return time.Now() }