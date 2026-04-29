package service

import (
	"context"
	"errors"
	"testing"
)

func TestTOTPSecretFallbackWithoutVault(t *testing.T) {
	svc := &AdminAuthService{}
	ctx := context.Background()
	secret := "JBSWY3DPEHPK3PXP"

	encrypted, err := svc.encryptTOTPSecret(ctx, secret)
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if encrypted != secret {
		t.Fatalf("expected plaintext passthrough, got %q", encrypted)
	}

	decrypted, err := svc.decryptTOTPSecret(ctx, secret)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if decrypted != secret {
		t.Fatalf("expected plaintext passthrough, got %q", decrypted)
	}
}

func TestEncryptTOTPSecretFailsWhenVaultMisconfigured(t *testing.T) {
	svc := &AdminAuthService{totpVaultErr: errors.New("vault down")}

	if _, err := svc.encryptTOTPSecret(context.Background(), "JBSWY3DPEHPK3PXP"); err == nil {
		t.Fatal("expected error when vault is misconfigured")
	}
}
