package client

import (
	"errors"
	"testing"
)

func TestNewVaultTransitClientWithoutEnvIsNotConfigured(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")
	t.Setenv("VAULT_TRANSIT_KEY", "")

	got, err := NewVaultTransitClient()
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil client, got %#v", got)
	}
}

func TestNewClickHouseClientWithoutDSNIsNotConfigured(t *testing.T) {
	t.Setenv("CLICKHOUSE_DSN", "")

	got, err := NewClickHouseClient()
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil client, got %#v", got)
	}
}

func TestIsClickHouseNotConfigured(t *testing.T) {
	if !IsClickHouseNotConfigured(ErrNotConfigured) {
		t.Fatal("expected ErrNotConfigured to be recognized")
	}
}
