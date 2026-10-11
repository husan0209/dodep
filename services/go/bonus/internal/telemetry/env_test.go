package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTracingConfigFromEnv_DisabledByDefault(t *testing.T) {
	t.Setenv(EnvEndpoint, "")
	t.Setenv(EnvServiceName, "")
	t.Setenv(EnvSampler, "")
	t.Setenv(EnvSampleArg, "")

	cfg := TracingConfigFromEnv("bonus-service")
	assert.Empty(t, cfg.Endpoint, "no endpoint → tracing disabled")
	assert.Equal(t, "bonus-service", cfg.ServiceName)
	assert.Equal(t, 1.0, cfg.SampleRatio)
	assert.True(t, cfg.Insecure, "in-cluster collectors default to plaintext")
}

func TestTracingConfigFromEnv_ReadsValues(t *testing.T) {
	t.Setenv(EnvEndpoint, "otel-collector.monitoring:4317")
	t.Setenv(EnvServiceName, "bonus-production")
	t.Setenv(EnvInsecure, "false")
	t.Setenv(EnvSampler, "traceidratio")
	t.Setenv(EnvSampleArg, "0.25")

	cfg := TracingConfigFromEnv("bonus-service")
	assert.Equal(t, "otel-collector.monitoring:4317", cfg.Endpoint)
	assert.Equal(t, "bonus-production", cfg.ServiceName)
	assert.False(t, cfg.Insecure)
	assert.InDelta(t, 0.25, cfg.SampleRatio, 1e-9)
}

func TestTracingConfigFromEnv_SamplerKeywords(t *testing.T) {
	t.Setenv(EnvEndpoint, "x:4317")

	t.Setenv(EnvSampler, "always_off")
	t.Setenv(EnvSampleArg, "0.9")
	assert.Equal(t, 0.0, TracingConfigFromEnv("d").SampleRatio, "always_off must win over the arg")

	t.Setenv(EnvSampler, "always_on")
	t.Setenv(EnvSampleArg, "0.01")
	assert.Equal(t, 1.0, TracingConfigFromEnv("d").SampleRatio, "always_on must win over the arg")
}

func TestTracingConfigFromEnv_InvalidSampleArgFallsBackToFull(t *testing.T) {
	t.Setenv(EnvEndpoint, "x:4317")
	t.Setenv(EnvSampler, "")

	for _, bad := range []string{"abc", "-1", "1.5", " "} {
		t.Setenv(EnvSampleArg, bad)
		assert.Equal(t, 1.0, TracingConfigFromEnv("d").SampleRatio,
			"invalid ratio %q must fall back to 1.0, not silently drop traces", bad)
	}
}

func TestTracingConfigFromEnv_DefaultServiceNameFallback(t *testing.T) {
	t.Setenv(EnvServiceName, "")
	assert.Equal(t, "bonus-service", TracingConfigFromEnv("").ServiceName,
		"empty default must fall back to the package default")
	assert.Equal(t, "custom", TracingConfigFromEnv("custom").ServiceName)
}

func TestBoolFromEnv(t *testing.T) {
	cases := map[string]bool{
		"1": true, "true": true, "YES": true, "on": true,
		"0": false, "false": false, "No": false, "off": false,
	}
	for raw, want := range cases {
		t.Setenv("TEST_BOOL", raw)
		assert.Equal(t, want, boolFromEnv("TEST_BOOL", !want), "raw %q", raw)
	}
	t.Setenv("TEST_BOOL", "nonsense")
	assert.True(t, boolFromEnv("TEST_BOOL", true), "unparsable value keeps the default")
	t.Setenv("TEST_BOOL", "")
	assert.False(t, boolFromEnv("TEST_BOOL", false))
}
