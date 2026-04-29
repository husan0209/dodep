package config

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	got := splitAndTrim(" broker1:9092, ,broker2:9092 , broker3:9092 ")
	want := []string{"broker1:9092", "broker2:9092", "broker3:9092"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected brokers: got %v want %v", got, want)
	}
}

func TestLoadRedpandaConfig(t *testing.T) {
	t.Setenv("REDPANDA_ENABLED", "true")
	t.Setenv("REDPANDA_BROKERS", "broker1:9092, broker2:9092")

	cfg := Load()
	if !cfg.RedpandaEnabled {
		t.Fatal("expected RedpandaEnabled to be true")
	}
	want := []string{"broker1:9092", "broker2:9092"}
	if !reflect.DeepEqual(cfg.RedpandaBrokers, want) {
		t.Fatalf("unexpected brokers: got %v want %v", cfg.RedpandaBrokers, want)
	}
}
