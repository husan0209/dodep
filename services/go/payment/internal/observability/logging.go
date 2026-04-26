package observability

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogConfig holds logging configuration
type LogConfig struct {
	Level       string // debug, info, warn, error
	Development bool   // Use development mode (console encoder)
	JSON        bool   // Use JSON encoder (default for production)
}

// Logger wraps zap.Logger with additional context helpers
type Logger struct {
	*zap.Logger
}

// NewLogger creates a new structured logger with proper defaults
func NewLogger(cfg LogConfig) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var encoder zapcore.Encoder
	if cfg.Development {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		// Production: JSON output
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{Logger: logger}, nil
}

// NewLoggerFromZap creates a Logger wrapper from an existing zap.Logger
func NewLoggerFromZap(logger *zap.Logger) *Logger {
	return &Logger{Logger: logger}
}

// parseLevel parses a log level string
func parseLevel(level string) (zapcore.Level, error) {
	var l zapcore.Level
	err := l.UnmarshalText([]byte(level))
	return l, err
}

// WithRequestID adds request ID to the logger context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{Logger: l.Logger.With(zap.String("request_id", requestID))}
}

// WithTraceID adds trace ID to the logger context (OpenTelemetry)
func (l *Logger) WithTraceID(traceID string) *Logger {
	return &Logger{Logger: l.Logger.With(zap.String("trace_id", traceID))}
}

// WithContext adds request ID and trace ID from context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Extract request ID from context
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		logger = logger.With(zap.String("request_id", requestID))
	}

	// Extract trace ID from context (OpenTelemetry)
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		logger = logger.With(zap.String("trace_id", traceID))
	}

	return &Logger{Logger: logger}
}

// Context key types for request and trace IDs
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	TraceIDKey   contextKey = "trace_id"
)

// MaskWalletAddress masks a wallet address for logging
// Shows first 6 and last 4 characters only
// Example: "0x1234567890abcdef1234567890abcdef12345678" -> "0x1234...5678"
func MaskWalletAddress(address string) string {
	if address == "" {
		return ""
	}

	// Minimum length to show meaningful masked value
	if len(address) < 12 {
		return "***MASKED***"
	}

	// Show first 6 and last 4 characters
	return address[:6] + "..." + address[len(address)-4:]
}

// MaskPaymentID masks a payment ID for logging
// Shows first 8 characters only
// Example: "pay_1234567890abcdef" -> "pay_1234..."
func MaskPaymentID(id string) string {
	if id == "" {
		return ""
	}

	if len(id) < 8 {
		return "***MASKED***"
	}
	if len(id) == 8 {
		return id
	}
	// Minimum length to show meaningful masked value
	if len(id) < 10 {
		return "***MASKED***"
	}
	return id[:8] + "..."
}

// MaskUserID masks a user ID for logging in non-debug mode
// Returns "***" in production, actual ID in debug mode
func MaskUserID(userID int64, isDebug bool) interface{} {
	if isDebug {
		return userID
	}
	return "***"
}

// MaskAmount masks financial amounts for non-admin users
// Returns "***" in production, actual amount for admins
func MaskAmount(amount float64, isAdmin bool) interface{} {
	if isAdmin {
		return amount
	}
	return "***"
}

// MaskSensitiveFields creates a map with sensitive fields masked
// Useful for logging request/response payloads
func MaskSensitiveFields(data map[string]interface{}, isDebug bool) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		lowerKey := strings.ToLower(k)
		switch {
		case strings.Contains(lowerKey, "wallet") || strings.Contains(lowerKey, "address"):
			if str, ok := v.(string); ok {
				result[k] = MaskWalletAddress(str)
			} else {
				result[k] = v
			}
		case strings.Contains(lowerKey, "payment_id"):
			if str, ok := v.(string); ok {
				result[k] = MaskPaymentID(str)
			} else {
				result[k] = v
			}
		case strings.Contains(lowerKey, "user_id"):
			if !isDebug {
				result[k] = "***"
			} else {
				result[k] = v
			}
		default:
			result[k] = v
		}
	}
	return result
}

// DefaultLogger creates a logger with sensible defaults
func DefaultLogger() (*Logger, error) {
	return NewLogger(LogConfig{
		Level:       "info",
		Development: false,
		JSON:        true,
	})
}

// DevelopmentLogger creates a logger for development
func DevelopmentLogger() (*Logger, error) {
	return NewLogger(LogConfig{
		Level:       "debug",
		Development: true,
		JSON:        false,
	})
}
