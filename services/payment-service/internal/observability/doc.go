// Package observability provides metrics, logging, and tracing capabilities
// for the payment service.
//
// This package implements:
//
// # Prometheus Metrics
//
//   - Deposit and withdrawal counters with status, currency, and KYC level labels
//   - Amount histograms for deposit and withdrawal values
//   - Provider latency histogram for NOWPayments API calls
//   - Error counters categorized by error type and operation
//
// All metrics are registered with the default Prometheus registry and follow
// the OpenMetrics naming conventions.
//
// # Structured Logging
//
// The logging package provides structured logging using zap.Logger with:
//   - JSON output format for production
//   - Console output with colors for development
//   - Request ID and Trace ID context propagation
//   - Sensitive data masking for wallet addresses, payment IDs, and user IDs
//
// Example usage:
//
//	logger, _ := observability.NewLogger(observability.LogConfig{
//	    Level:       "info",
//	    Development: false,
//	    JSON:        true,
//	})
//
//	// Add request context
//	ctxLogger := logger.WithRequestID("req-123").WithTraceID("trace-456")
//	ctxLogger.Info("payment created",
//	    zap.String("payment_id", observability.MaskPaymentID("pay_1234567890")),
//	    zap.String("wallet", observability.MaskWalletAddress("0x1234567890abcdef")),
//	)
//
// # OpenTelemetry Tracing
//
// The tracing package provides distributed tracing using OpenTelemetry with:
//   - OTLP exporter for OpenTelemetry Collector integration
//   - gRPC and HTTP protocol support
//   - Configurable sampling rates
//   - Trace context propagation for gRPC and HTTP
//   - Helper functions for span creation and management
//
// Example usage:
//
//	// Initialize tracer provider
//	tp, err := observability.NewTracerProvider(observability.TracerConfig{
//	    ServiceName:  "payment-service",
//	    Environment:  "production",
//	    OTLPEndpoint: "localhost:4317",
//	    OTLPProtocol: "grpc",
//	    SampleRate:   1.0,
//	    Insecure:     true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
//
//	// Create a span for an operation
//	ctx, span := observability.StartSpan(ctx, observability.SpanPaymentInitiateDeposit)
//	defer observability.EndSpan(span)
//
//	// Add attributes to the span
//	observability.SetSpanAttributes(span,
//	    attribute.Int64("user.id", userID),
//	    attribute.String("payment.currency", "BTC"),
//	)
//
//	// Record an error if one occurs
//	if err != nil {
//	    observability.RecordError(span, err)
//	}
//
// # Span Names
//
// Predefined span names are available for consistent tracing:
//   - NOWPayments API: nowpayments.create_payment, nowpayments.create_payout, nowpayments.get_rate
//   - Wallet Service: wallet.GetBalance, wallet.CreditWallet, wallet.LockFunds
//   - Database: database.query, database.insert, database.update
//   - Payment operations: payment.initiate_deposit, payment.initiate_withdrawal
//   - Webhooks: webhook.process
package observability
