package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Тест проверяет именно дефолты Load(), поэтому не должен наследовать
	// окружение раннера (в CI выставляется APP_ENV=test).
	// Пустое значение getEnv/getIntEnv трактуется как "переменной нет".
	t.Setenv("GRPC_PORT", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", "")

	cfg := Load().Validate()
	if cfg.GRPCPort != 50062 {
		t.Errorf("expected grpc 50062, got %d", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8091" {
		t.Errorf("expected http 8091, got %s", cfg.HTTPPort)
	}
	if cfg.CoolingHours != 24 || cfg.RevokeCoolingHours != 24 {
		t.Errorf("bad cooling defaults: %+v", cfg)
	}
	if cfg.Env != "development" {
		t.Errorf("expected development, got %s", cfg.Env)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "50062")
	t.Setenv("PORT", "8091")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x?sslmode=require")
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	t.Setenv("RG_ADMIN_TOKEN", "admin-token")
	t.Setenv("RG_COOLING_HOURS", "48")
	t.Setenv("RG_REVOKE_COOLING_HOURS", "72")
	t.Setenv("APP_ENV", "staging")

	cfg := Load().Validate()
	if cfg.CoolingHours != 48 || cfg.RevokeCoolingHours != 72 {
		t.Errorf("bad cooling env: %+v", cfg)
	}
	if cfg.AdminToken != "admin-token" || cfg.JWTSecretKey != "test-secret" {
		t.Errorf("bad secrets env: %+v", cfg)
	}
}

func TestValidateClampsToRegulatoryRange(t *testing.T) {
	// Limit increases cool down 24-72h; the lower bound is the hard rule.
	cfg := &Config{CoolingHours: 1, RevokeCoolingHours: 1}
	cfg.Validate()
	if cfg.CoolingHours != 24 {
		t.Errorf("cooling below 24h must clamp to 24, got %d", cfg.CoolingHours)
	}
	if cfg.RevokeCoolingHours != 24 {
		t.Errorf("revoke cooling below 24h must clamp to 24, got %d", cfg.RevokeCoolingHours)
	}
	cfg = &Config{CoolingHours: 200, RevokeCoolingHours: 24}
	cfg.Validate()
	if cfg.CoolingHours != 72 {
		t.Errorf("cooling above 72h must clamp to 72, got %d", cfg.CoolingHours)
	}
	// Revoke cooling has no regulatory upper bound — long waits stay allowed.
	cfg = &Config{CoolingHours: 24, RevokeCoolingHours: 24 * 90}
	cfg.Validate()
	if cfg.RevokeCoolingHours != 24*90 {
		t.Errorf("revoke cooling must not be capped, got %d", cfg.RevokeCoolingHours)
	}
}
