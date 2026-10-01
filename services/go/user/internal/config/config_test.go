package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.GRPCPort != 9092 {
		t.Errorf("expected default grpc 9092, got %d", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8082" {
		t.Errorf("expected default http 8082, got %s", cfg.HTTPPort)
	}
	if cfg.RedisAddr != "localhost:6379" || cfg.RedisDB != 0 {
		t.Errorf("bad redis defaults: %+v", cfg)
	}
	if cfg.Env != "development" {
		t.Errorf("expected development default, got %s", cfg.Env)
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecretKey == "" {
		t.Error("database url and jwt secret must have defaults")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "50052")
	t.Setenv("PORT", "8085")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x?sslmode=require")
	t.Setenv("REDIS_ADDR", "cache:6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	t.Setenv("APP_ENV", "staging")

	cfg := Load()
	if cfg.GRPCPort != 50052 {
		t.Errorf("expected 50052, got %d", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8085" {
		t.Errorf("expected 8085, got %s", cfg.HTTPPort)
	}
	if cfg.DatabaseURL != "postgres://u:p@db:5432/x?sslmode=require" {
		t.Errorf("bad db url: %s", cfg.DatabaseURL)
	}
	if cfg.RedisAddr != "cache:6380" || cfg.RedisPassword != "s3cret" || cfg.RedisDB != 2 {
		t.Errorf("bad redis env: %+v", cfg)
	}
	if cfg.JWTSecretKey != "test-secret" || cfg.Env != "staging" {
		t.Errorf("bad misc env: %+v", cfg)
	}
}

func TestLoadBadIntFallsBack(t *testing.T) {
	t.Setenv("GRPC_PORT", "not-a-port")
	t.Setenv("REDIS_DB", "nan")
	cfg := Load()
	if cfg.GRPCPort != 0 {
		t.Errorf("expected 0 on bad grpc port (strconv.Atoi zero), got %d", cfg.GRPCPort)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("expected 0 fallback, got %d", cfg.RedisDB)
	}
}
