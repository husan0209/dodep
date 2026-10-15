package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()
	assert.Equal(t, "8089", cfg.HTTPPort)
	assert.Equal(t, "50059", cfg.GRPCPort)
	assert.Equal(t, "development", cfg.Env)
}

func TestValidate_PortsMustDiffer(t *testing.T) {
	cfg := &Config{Env: "development", HTTPPort: "8089", GRPCPort: "8089"}
	assert.Error(t, cfg.Validate())
}

func TestValidate_ProductionRequiresSecrets(t *testing.T) {
	cfg := &Config{
		Env: "production", HTTPPort: "8089", GRPCPort: "50059",
		JWTSecretKey: "change-me-in-production",
	}
	assert.Error(t, cfg.Validate())

	cfg.JWTSecretKey = "0123456789abcdef0123456789abcdef"
	cfg.FieldKey = ""
	assert.Error(t, cfg.Validate(), "field key required in production")

	cfg.FieldKey = "0123456789abcdef0123456789abcdef"
	assert.NoError(t, cfg.Validate())
}
