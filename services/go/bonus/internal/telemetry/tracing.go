package telemetry

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracingConfig configures the OpenTelemetry tracing pipeline.
//
// Tracing is opt-in: when Endpoint is empty the service runs with the global
// no-op tracer, so local development and unit tests need no collector and
// produce zero overhead beyond the API calls.
type TracingConfig struct {
	// ServiceName is reported as service.name on every span.
	ServiceName string
	// Endpoint is the OTLP/gRPC collector address (e.g.
	// otel-collector.monitoring:4317). Empty disables tracing.
	Endpoint string
	// SampleRatio is the head sampling ratio in [0,1].
	SampleRatio float64
	// Insecure disables TLS for the collector connection (in-cluster mesh).
	Insecure bool
	// Environment is recorded as deployment.environment.
	Environment string
	// Version is recorded as service.version.
	Version string
}

// TracerName is the instrumentation scope for all bonus-service spans.
const TracerName = "github.com/opus-casino/bonus"

// InitTracing installs a global tracer provider and propagator.
//
// Returns a shutdown function that flushes pending spans; it is safe to call
// even when tracing is disabled. Errors are returned so the caller can decide
// between failing fast (production) and degrading gracefully (local dev).
func InitTracing(ctx context.Context, cfg TracingConfig) (func(context.Context) error, error) {
	noopShutdown := func(context.Context) error { return nil }

	// Always install the W3C propagator so that even a disabled tracer keeps
	// extracting/injecting an empty-but-valid context.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if strings.TrimSpace(cfg.Endpoint) == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return noopShutdown, nil
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: create OTLP exporter: %w", err)
	}

	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: build resource: %w", err)
	}

	// ParentBased(TraceIDRatioBased) keeps upstream decisions: if the caller
	// (Istio) already sampled, we keep the trace; we only decide for roots.
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(provider)

	return func(shutdownCtx context.Context) error {
		// Flush is best-effort: a collector outage must not block shutdown.
		if err := provider.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("telemetry: shutdown tracer provider: %w", err)
		}
		return nil
	}, nil
}

// Tracer returns the bonus-service tracer. Safe before InitTracing: it yields a
// no-op tracer, so instrumentation code never needs nil checks.
func Tracer() oteltrace.Tracer { return otel.Tracer(TracerName) }

// StartServerSpan starts a SERVER span for an inbound request.
// Propagator is the global one, so W3C headers from the mesh are honoured.
func StartServerSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	return Tracer().Start(ctx, name, oteltrace.WithSpanKind(oteltrace.SpanKindServer), oteltrace.WithAttributes(attrs...))
}

// StartClientSpan starts a CLIENT span for an outbound call (e.g. wallet-core).
func StartClientSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	return Tracer().Start(ctx, name, oteltrace.WithSpanKind(oteltrace.SpanKindClient), oteltrace.WithAttributes(attrs...))
}

// StartInternalSpan starts an INTERNAL span for background work.
func StartInternalSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	return Tracer().Start(ctx, name, oteltrace.WithSpanKind(oteltrace.SpanKindInternal), oteltrace.WithAttributes(attrs...))
}

// StartConsumerSpan starts a CONSUMER span for a message-broker record,
// recording the standard messaging attributes for correlation.
func StartConsumerSpan(ctx context.Context, name, topic string, partition int32, offset int64, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	base := []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", topic),
		attribute.Int("messaging.destination.partition", int(partition)),
		attribute.Int64("messaging.kafka.offset", offset),
		attribute.String("messaging.operation", "process"),
	}
	return Tracer().Start(ctx, name,
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
		oteltrace.WithAttributes(append(base, attrs...)...),
	)
}

// EndSpan records the outcome of a span and sets its status.
// Note: it does NOT close the span — the caller owns span.End() via defer,
// which keeps the `defer` discipline explicit at the call site.
func EndSpan(span oteltrace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	span.SetStatus(codes.Ok, "")
}
