package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	grpcodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// installRecorder swaps the global tracer provider for an SDK provider backed
// by an in-memory span recorder, and returns the recorder for assertions.
func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})
	return rec
}

func TestInitTracing_DisabledWithoutEndpoint(t *testing.T) {
	shutdown, err := InitTracing(context.Background(), TracingConfig{ServiceName: "bonus-service"})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown(context.Background()))

	// With tracing disabled the tracer must be non-recording, and the app
	// must still work.
	_, span := StartServerSpan(context.Background(), "GET /health")
	assert.False(t, span.IsRecording())
	span.End()
}

func TestInitTracing_PropagatorAlwaysInstalled(t *testing.T) {
	_, err := InitTracing(context.Background(), TracingConfig{})
	require.NoError(t, err)
	assert.NotNil(t, otel.GetTextMapPropagator(), "propagator must be installed even when disabled")
}

func TestStartServerSpan_RecordsNameKindAndStatus(t *testing.T) {
	rec := installRecorder(t)

	ctx, span := StartServerSpan(context.Background(), "GET /api/v1/bonuses/:id",
		attribute.String("http.route", "/api/v1/bonuses/:id"))
	EndSpan(span, nil)
	span.End()
	_ = ctx

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "GET /api/v1/bonuses/:id", spans[0].Name())
	assert.Equal(t, oteltrace.SpanKindServer, spans[0].SpanKind())
	assert.Equal(t, otelcodes.Ok, spans[0].Status().Code)
	assert.Equal(t, "/api/v1/bonuses/:id", attrValue(spans[0].Attributes(), "http.route"))
}

func TestEndSpan_RecordsErrorAndStatus(t *testing.T) {
	rec := installRecorder(t)

	_, span := StartServerSpan(context.Background(), "GET /boom")
	EndSpan(span, assert.AnError)
	span.End()

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
	assert.NotEmpty(t, spans[0].Events(), "error must be recorded as a span event")
}

func TestStartConsumerSpan_MessagingAttributes(t *testing.T) {
	rec := installRecorder(t)

	_, span := StartConsumerSpan(context.Background(), "payments.completed process",
		"payments.completed", 3, 1234567)
	EndSpan(span, nil)
	span.End()

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, oteltrace.SpanKindConsumer, spans[0].SpanKind())
	attrs := spans[0].Attributes()
	assert.Equal(t, "payments.completed", attrValue(attrs, "messaging.destination.name"))
	assert.Equal(t, "1234567", attrValue(attrs, "messaging.kafka.offset"))
}

func TestStartClientSpan_KindClient(t *testing.T) {
	rec := installRecorder(t)

	_, span := StartClientSpan(context.Background(), "wallet.v1.WalletCoreService/Credit")
	EndSpan(span, nil)
	span.End()

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, oteltrace.SpanKindClient, spans[0].SpanKind())
}

func TestTracingMiddleware_SpanNameUsesRoutePattern(t *testing.T) {
	rec := installRecorder(t)

	app := fiber.New()
	app.Use(TracingMiddleware())
	app.Get("/api/v1/bonuses/:id", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// Three different IDs must produce ONE span name (no cardinality blow-up).
	for _, id := range []string{"a", "b", "c"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/bonuses/"+id, nil), -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	spans := rec.Ended()
	require.Len(t, spans, 3)
	for _, s := range spans {
		assert.Equal(t, "GET /api/v1/bonuses/:id", s.Name())
		assert.Equal(t, oteltrace.SpanKindServer, s.SpanKind())
	}
}

func TestTracingMiddleware_ContinuesUpstreamTrace(t *testing.T) {
	rec := installRecorder(t)

	app := fiber.New()
	app.Use(TracingMiddleware())
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// A W3C traceparent injected by the edge (Istio).
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", spans[0].SpanContext().TraceID().String(),
		"must continue the upstream trace, never start a new one")
}

func TestTracingMiddleware_ErrorStatusOnSpan(t *testing.T) {
	rec := installRecorder(t)

	app := fiber.New()
	app.Use(TracingMiddleware())
	app.Get("/boom", func(_ *fiber.Ctx) error { return fiber.ErrTeapot })

	_, err := app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil), -1)
	require.NoError(t, err)

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
	assert.Equal(t, int64(418), attrIntValue(spans[0].Attributes(), "http.response.status_code"))
}

func TestUnaryTracingInterceptor_RecordsCode(t *testing.T) {
	rec := installRecorder(t)

	interceptor := UnaryTracingInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/bonus.v1.BonusService/GetBonus"}
	_, err := interceptor(context.Background(), nil, info,
		func(context.Context, interface{}) (interface{}, error) {
			return nil, status.Error(grpcodes.NotFound, "bonus not found")
		})
	require.Error(t, err)

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "GetBonus", spans[0].Name(), "span name must be the short method name")
	assert.Equal(t, otelcodes.Error, spans[0].Status().Code)
	assert.Equal(t, "NotFound", attrValue(spans[0].Attributes(), "rpc.grpc.status_code"))
}

func TestRequestIDMiddleware_GeneratesAndEchoes(t *testing.T) {
	app := fiber.New()
	app.Use(RequestIDMiddleware())
	var seen string
	app.Get("/x", func(c *fiber.Ctx) error {
		seen = RequestID(c)
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.NotEmpty(t, seen, "handler must see a request id")
	assert.Equal(t, seen, resp.Header.Get(RequestIDHeader), "id must be echoed in the response header")
}

func TestRequestIDMiddleware_KeepsIncomingID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestIDMiddleware())
	var seen string
	app.Get("/x", func(c *fiber.Ctx) error {
		seen = RequestID(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "edge-req-42")
	_, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, "edge-req-42", seen, "incoming request id must be propagated")
}

func TestRequestIDMiddleware_ReplacesOversizedID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestIDMiddleware())
	var seen string
	app.Get("/x", func(c *fiber.Ctx) error {
		seen = RequestID(c)
		return c.SendString("ok")
	})

	// Client-supplied value above the 128-char limit must be replaced, so a
	// hostile header cannot flood the logs.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, strings.Repeat("a", 300))
	_, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(seen), 128, "unbounded client input must not be echoed into logs")
	assert.NotEqual(t, strings.Repeat("a", 300), seen)
}

func TestLogFieldsFor_IncludesCorrelation(t *testing.T) {
	rec := installRecorder(t)

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(TracingMiddleware())
	var fields []zap.Field
	app.Get("/x", func(c *fiber.Ctx) error {
		fields = LogFieldsFor(c)
		return c.SendString("ok")
	})
	_, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)

	keys := map[string]string{}
	for _, f := range fields {
		keys[f.Key] = f.String
	}
	assert.NotEmpty(t, keys["request_id"])
	assert.NotEmpty(t, keys["trace_id"], "traced request must expose trace_id for logs")
	assert.Len(t, rec.Ended(), 1)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// attrValue renders an attribute of any type as text.
func attrValue(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.String()
		}
	}
	return ""
}

func attrIntValue(attrs []attribute.KeyValue, key string) int64 {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsInt64()
		}
	}
	return 0
}
