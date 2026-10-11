package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("GRPC_PORT", "")
	t.Setenv("WELCOME_BONUS_PERCENTAGE", "")
	t.Setenv("WELCOME_BONUS_MAX_AMOUNT", "")
	t.Setenv("WELCOME_BONUS_WAGERING_REQUIREMENT", "")
	t.Setenv("WELCOME_BONUS_EXPIRY_DAYS", "")
	t.Setenv("KAFKA_BROKERS", "")

	cfg := Load()
	assert.Equal(t, "8088", cfg.HTTPPort)
	assert.Equal(t, "50056", cfg.GRPCPort)
	assert.Equal(t, 100, cfg.WelcomePct)
	assert.Equal(t, "200", cfg.WelcomeMaxUSD.String())
	assert.Equal(t, 30, cfg.WelcomeWagering)
	assert.Equal(t, 30, cfg.WelcomeExpiryDays)
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "18088")
	t.Setenv("WELCOME_BONUS_PERCENTAGE", "50")
	t.Setenv("WELCOME_BONUS_MAX_AMOUNT", "150.50")
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092, kafka-2:9092")

	cfg := Load()
	assert.Equal(t, "18088", cfg.HTTPPort)
	assert.Equal(t, 50, cfg.WelcomePct)
	assert.Equal(t, "150.5", cfg.WelcomeMaxUSD.String())
	assert.Equal(t, []string{"kafka-1:9092", "kafka-2:9092"}, cfg.KafkaBrokers)
}

func TestGetDecimalEnv_RejectsInvalidAndNegative(t *testing.T) {
	t.Setenv("WELCOME_BONUS_MAX_AMOUNT", "not-a-number")
	cfg := Load()
	assert.Equal(t, "200", cfg.WelcomeMaxUSD.String(), "invalid env must fall back to default")

	t.Setenv("WELCOME_BONUS_MAX_AMOUNT", "-50")
	cfg = Load()
	assert.Equal(t, "200", cfg.WelcomeMaxUSD.String(), "negative money must fall back to default")
}

func TestGetIntEnv_RejectsInvalid(t *testing.T) {
	t.Setenv("WELCOME_BONUS_PERCENTAGE", "abc")
	cfg := Load()
	assert.Equal(t, 100, cfg.WelcomePct)
}

func TestSplitCSV_TrimsAndSkipsEmpty(t *testing.T) {
	got := splitCSV("a:1,  b:2 ,,c:3")
	require.Equal(t, []string{"a:1", "b:2", "c:3"}, got)
	assert.Empty(t, splitCSV(""))
}
