package telemetry

import (
	"os"
	"strconv"
	"strings"
)

// OTLP endpoint / sampler env contract (OpenTelemetry standard variables).
//
// The tracing package owns its own env contract on purpose: it keeps the OTel
// wiring self-contained and avoids spreading telemetry-specific knobs through
// every service config struct.
const (
	EnvEndpoint      = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvServiceName   = "OTEL_SERVICE_NAME"
	EnvSampleArg     = "OTEL_TRACES_SAMPLER_ARG"
	EnvInsecure      = "OTEL_EXPORTER_OTLP_INSECURE"
	EnvSampler       = "OTEL_TRACES_SAMPLER"
	defaultServiceNm = "bonus-service"
)

// TracingConfigFromEnv builds a TracingConfig from the environment.
//
// Tracing stays OFF unless OTEL_EXPORTER_OTLP_ENDPOINT is set, so local runs
// and unit tests never need a collector. Defaults follow the OTel spec:
//   - service name: OTEL_SERVICE_NAME, else the caller's default;
//   - sampler: parentbased_always_on, i.e. ratio 1 unless overridden;
//   - insecure transport: true (in-cluster collectors are usually plaintext
//     behind the mesh) — set OTEL_EXPORTER_OTLP_INSECURE=false for TLS.
func TracingConfigFromEnv(defaultServiceName string) TracingConfig {
	if strings.TrimSpace(defaultServiceName) == "" {
		defaultServiceName = defaultServiceNm
	}
	return TracingConfig{
		ServiceName: firstNonEmpty(os.Getenv(EnvServiceName), defaultServiceName),
		Endpoint:    strings.TrimSpace(os.Getenv(EnvEndpoint)),
		SampleRatio: sampleRatioFromEnv(),
		Insecure:    boolFromEnv(EnvInsecure, true),
	}
}

// sampleRatioFromEnv reads OTEL_TRACES_SAMPLER_ARG, honouring
// OTEL_TRACES_SAMPLER=always_on|always_off|traceidratio. Out-of-range or
// unparsable values fall back to 1.0 (record everything) because silently
// dropping traces is worse than a little overhead.
func sampleRatioFromEnv() float64 {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvSampler))) {
	case "always_off":
		return 0
	case "always_on":
		return 1
	}
	raw := strings.TrimSpace(os.Getenv(EnvSampleArg))
	if raw == "" {
		return 1
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > 1 {
		return 1
	}
	return v
}

func boolFromEnv(key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return def
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
