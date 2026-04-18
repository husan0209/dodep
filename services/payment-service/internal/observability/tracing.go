package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Span names for payment service operations.
// These follow OpenTelemetry naming conventions.
const (
	// NOWPayments API operations
	SpanNOWPaymentsCreatePayment = "nowpayments.create_payment"
	SpanNOWPaymentsCreatePayout  = "nowpayments.create_payout"
	SpanNOWPaymentsGetRate       = "nowpayments.get_rate"
	SpanNOWPaymentsGetCurrencies = "nowpayments.get_currencies"
	SpanNOWPaymentsVerifyWebhook = "nowpayments.verify_webhook"

	// Wallet Service gRPC operations
	SpanWalletGetBalance   = "wallet.GetBalance"
	SpanWalletCreditWallet = "wallet.CreditWallet"
	SpanWalletLockFunds    = "wallet.LockFunds"
	SpanWalletUnlockFunds  = "wallet.UnlockFunds"
	SpanWalletFinalizeDebit = "wallet.FinalizeDebit"

	// Database operations
	SpanDatabaseQuery    = "database.query"
	SpanDatabaseInsert   = "database.insert"
	SpanDatabaseUpdate   = "database.update"
	SpanDatabaseSelect   = "database.select"

	// Payment service operations
	SpanPaymentInitiateDeposit   = "payment.initiate_deposit"
	SpanPaymentInitiateWithdrawal = "payment.initiate_withdrawal"
	SpanPaymentGetPayment        = "payment.get_payment"
	SpanPaymentListPayments      = "payment.list_payments"

	// Webhook operations
	SpanWebhookProcess = "webhook.process"

	// KYC operations
	SpanKYCCheckLimits = "kyc.check_limits"
	SpanKYCGetLevel    = "kyc.get_level"

	// Exchange rate operations
	SpanExchangeRateGet = "exchange_rate.get"
)

// TracerConfig holds configuration for the OpenTelemetry tracer.
type TracerConfig struct {
	// ServiceName is the name of the service for trace identification.
	ServiceName string

	// Environment is the deployment environment (development, staging, production).
	Environment string

	// OTLPEndpoint is the OpenTelemetry Collector endpoint (e.g., "localhost:4317" for gRPC).
	OTLPEndpoint string

	// OTLPProtocol is the protocol to use: "grpc" or "http".
	OTLPProtocol string

	// SampleRate is the sampling rate (0.0 to 1.0).
	// 1.0 means sample all traces, 0.1 means sample 10%.
	SampleRate float64

	// Insecure indicates whether to use insecure connection (no TLS).
	Insecure bool
}

// TracerProvider wraps the OpenTelemetry TracerProvider with helper methods.
type TracerProvider struct {
	*sdktrace.TracerProvider
	tracer trace.Tracer
}

// tracerProvider is the global tracer provider instance.
var tracerProvider *TracerProvider

// NewTracerProvider creates a new OpenTelemetry tracer provider.
// It configures OTLP exporter, sampling, and resource attributes.
func NewTracerProvider(cfg TracerConfig) (*TracerProvider, error) {
	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	// Create OTLP exporter
	var exporter sdktrace.SpanExporter
	ctx := context.Background()

	switch cfg.OTLPProtocol {
	case "grpc":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	default:
		// Default to gRPC
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	}

	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	switch {
	case cfg.SampleRate >= 1.0:
		sampler = sdktrace.AlwaysSample()
	case cfg.SampleRate <= 0.0:
		sampler = sdktrace.NeverSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global text map propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracerProvider = &TracerProvider{
		TracerProvider: tp,
		tracer:         tp.Tracer("github.com/platform/services/payment-service"),
	}

	return tracerProvider, nil
}

// Tracer returns the tracer for creating spans.
func (tp *TracerProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// Shutdown gracefully shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.TracerProvider.Shutdown(ctx)
}

// StartSpan starts a new span with the given name and options.
// The span is automatically linked to the parent context if one exists.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tracerProvider == nil {
		// Return a no-op span if tracer is not initialized
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracerProvider.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanEvent adds an event to the span with optional attributes.
func AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	if span == nil || !span.IsRecording() {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records an error on the span.
// It also sets the span status to Error.
func RecordError(span trace.Span, err error, attrs ...attribute.KeyValue) {
	if span == nil || !span.IsRecording() || err == nil {
		return
	}
	span.RecordError(err, trace.WithAttributes(attrs...))
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanAttributes sets attributes on the span.
func SetSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(attrs...)
}

// SetSpanStatus sets the status of the span.
func SetSpanStatus(span trace.Span, code codes.Code, description string) {
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetStatus(code, description)
}

// EndSpan ends the span. Should be called in a defer statement.
func EndSpan(span trace.Span) {
	if span == nil {
		return
	}
	span.End()
}

// TraceIDFromContext extracts the trace ID from the context.
// Returns an empty string if no trace is found.
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts the span ID from the context.
// Returns an empty string if no span is found.
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// InjectTraceContext injects trace context into gRPC metadata.
// This enables trace propagation to downstream services.
func InjectTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	// Inject trace context into metadata
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, &metadataCarrier{md})

	return metadata.NewOutgoingContext(ctx, md)
}

// ExtractTraceContext extracts trace context from gRPC metadata.
// This enables receiving trace context from upstream services.
func ExtractTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	// Extract trace context from metadata
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, &metadataCarrier{md})
}

// metadataCarrier implements propagation.TextMapCarrier for gRPC metadata.
type metadataCarrier struct {
	md metadata.MD
}

// Get returns the value for the given key.
func (c *metadataCarrier) Get(key string) string {
	values := c.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set sets the value for the given key.
func (c *metadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

// Keys returns all keys in the carrier.
func (c *metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.md))
	for k := range c.md {
		keys = append(keys, k)
	}
	return keys
}

// GRPCClientInterceptor returns a gRPC unary client interceptor for tracing.
// It creates spans for outgoing gRPC calls and propagates trace context.
func GRPCClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Start a span for the gRPC call
		ctx, span := StartSpan(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
		)
		defer EndSpan(span)

		// Add method attribute
		SetSpanAttributes(span,
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", method),
		)

		// Inject trace context into outgoing metadata
		ctx = InjectTraceContext(ctx)

		// Invoke the RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Record error if any
		if err != nil {
			RecordError(span, err)
		} else {
			SetSpanStatus(span, codes.Ok, "")
		}

		return err
	}
}

// GRPCStreamClientInterceptor returns a gRPC stream client interceptor for tracing.
func GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Start a span for the gRPC stream call
		ctx, span := StartSpan(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
		)

		// Add method attribute
		SetSpanAttributes(span,
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", method),
		)

		// Inject trace context into outgoing metadata
		ctx = InjectTraceContext(ctx)

		// Invoke the streamer
		stream, err := streamer(ctx, desc, cc, method, opts...)

		// Record error if any
		if err != nil {
			RecordError(span, err)
			EndSpan(span)
		} else {
			SetSpanStatus(span, codes.Ok, "")
			// For streams, we end the span when the stream ends
			// In a production implementation, we would wrap the stream
			// and end the span when the stream is closed
		}

		return stream, err
	}
}

// SpanOption is a functional option for configuring spans.
type SpanOption func(*spanConfig)

type spanConfig struct {
	attrs     []attribute.KeyValue
	kind      trace.SpanKind
	links     []trace.Link
	timestamp trace.Timestamp
}

// WithAttributes returns a SpanOption that sets attributes on the span.
func WithAttributes(attrs ...attribute.KeyValue) SpanOption {
	return func(c *spanConfig) {
		c.attrs = attrs
	}
}

// WithSpanKind returns a SpanOption that sets the span kind.
func WithSpanKind(kind trace.SpanKind) SpanOption {
	return func(c *spanConfig) {
		c.kind = kind
	}
}

// WithLinks returns a SpanOption that sets links to other spans.
func WithLinks(links ...trace.Link) SpanOption {
	return func(c *spanConfig) {
		c.links = links
	}
}

// StartSpanWithOptions starts a new span with functional options.
func StartSpanWithOptions(ctx context.Context, name string, opts ...SpanOption) (context.Context, trace.Span) {
	cfg := &spanConfig{
		kind: trace.SpanKindInternal,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var spanOpts []trace.SpanStartOption
	if len(cfg.attrs) > 0 {
		spanOpts = append(spanOpts, trace.WithAttributes(cfg.attrs...))
	}
	if cfg.kind != trace.SpanKindUnspecified {
		spanOpts = append(spanOpts, trace.WithSpanKind(cfg.kind))
	}
	if len(cfg.links) > 0 {
		spanOpts = append(spanOpts, trace.WithLinks(cfg.links...))
	}

	return StartSpan(ctx, name, spanOpts...)
}

// DatabaseSpanAttributes returns common attributes for database spans.
func DatabaseSpanAttributes(operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", operation),
		attribute.String("db.sql.table", table),
	}
}

// NOWPaymentsSpanAttributes returns common attributes for NOWPayments API spans.
func NOWPaymentsSpanAttributes(operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", "POST"),
		attribute.String("http.host", "api.nowpayments.io"),
		attribute.String("rpc.system", "http"),
		attribute.String("rpc.method", operation),
	}
}

// WalletSpanAttributes returns common attributes for Wallet Service gRPC spans.
func WalletSpanAttributes(method string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("rpc.system", "grpc"),
		attribute.String("rpc.service", "platform.wallet.v1.WalletService"),
		attribute.String("rpc.method", method),
	}
}

// PaymentSpanAttributes returns common attributes for payment operation spans.
func PaymentSpanAttributes(operation string, userID int64, currency string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("payment.operation", operation),
		attribute.Int64("user.id", userID),
		attribute.String("payment.currency", currency),
	}
}

// WebhookSpanAttributes returns common attributes for webhook spans.
func WebhookSpanAttributes(webhookType, status string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("webhook.type", webhookType),
		attribute.String("webhook.status", status),
		attribute.String("webhook.provider", "nowpayments"),
	}
}
