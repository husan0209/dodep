package observability

import (
	"testing"
)

func TestMaskWalletAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "empty address",
			address:  "",
			expected: "",
		},
		{
			name:     "short address",
			address:  "short",
			expected: "***MASKED***",
		},
		{
			name:     "ethereum address",
			address:  "0x1234567890abcdef1234567890abcdef12345678",
			expected: "0x1234...5678",
		},
		{
			name:     "bitcoin address",
			address:  "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			expected: "bc1qar...5mdq",
		},
		{
			name:     "exactly 12 chars",
			address:  "123456789012",
			expected: "123456...9012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskWalletAddress(tt.address)
			if result != tt.expected {
				t.Errorf("MaskWalletAddress(%q) = %q, want %q", tt.address, result, tt.expected)
			}
		})
	}
}

func TestMaskPaymentID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "empty id",
			id:       "",
			expected: "",
		},
		{
			name:     "short id",
			id:       "short",
			expected: "***MASKED***",
		},
		{
			name:     "standard payment id",
			id:       "pay_1234567890abcdef",
			expected: "pay_1234...",
		},
		{
			name:     "exactly 10 chars",
			id:       "1234567890",
			expected: "12345678...",
		},
		{
			name:     "exactly 8 chars",
			id:       "12345678",
			expected: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskPaymentID(tt.id)
			if result != tt.expected {
				t.Errorf("MaskPaymentID(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

func TestMaskUserID(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		isDebug  bool
		expected interface{}
	}{
		{
			name:     "debug mode shows user id",
			userID:   12345,
			isDebug:  true,
			expected: int64(12345),
		},
		{
			name:     "production mode masks user id",
			userID:   12345,
			isDebug:  false,
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskUserID(tt.userID, tt.isDebug)
			if result != tt.expected {
				t.Errorf("MaskUserID(%d, %v) = %v, want %v", tt.userID, tt.isDebug, result, tt.expected)
			}
		})
	}
}

func TestMaskAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		isAdmin  bool
		expected interface{}
	}{
		{
			name:     "admin sees amount",
			amount:   100.50,
			isAdmin:  true,
			expected: 100.50,
		},
		{
			name:     "non-admin sees masked amount",
			amount:   100.50,
			isAdmin:  false,
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAmount(tt.amount, tt.isAdmin)
			if result != tt.expected {
				t.Errorf("MaskAmount(%f, %v) = %v, want %v", tt.amount, tt.isAdmin, result, tt.expected)
			}
		})
	}
}

func TestMaskSensitiveFields(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		isDebug  bool
		expected map[string]interface{}
	}{
		{
			name: "masks wallet address",
			data: map[string]interface{}{
				"wallet_address": "0x1234567890abcdef1234567890abcdef12345678",
			},
			isDebug: false,
			expected: map[string]interface{}{
				"wallet_address": "0x1234...5678",
			},
		},
		{
			name: "masks payment_id",
			data: map[string]interface{}{
				"payment_id": "pay_1234567890abcdef",
			},
			isDebug: false,
			expected: map[string]interface{}{
				"payment_id": "pay_1234...",
			},
		},
		{
			name: "masks user_id in production",
			data: map[string]interface{}{
				"user_id": int64(12345),
			},
			isDebug: false,
			expected: map[string]interface{}{
				"user_id": "***",
			},
		},
		{
			name: "shows user_id in debug mode",
			data: map[string]interface{}{
				"user_id": int64(12345),
			},
			isDebug: true,
			expected: map[string]interface{}{
				"user_id": int64(12345),
			},
		},
		{
			name: "preserves non-sensitive fields",
			data: map[string]interface{}{
				"status":  "completed",
				"amount":  100.50,
				"user_id": int64(12345),
			},
			isDebug: false,
			expected: map[string]interface{}{
				"status":  "completed",
				"amount":  100.50,
				"user_id": "***",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveFields(tt.data, tt.isDebug)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("MaskSensitiveFields()[%q] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config LogConfig
	}{
		{
			name: "production logger",
			config: LogConfig{
				Level:       "info",
				Development: false,
				JSON:        true,
			},
		},
		{
			name: "development logger",
			config: LogConfig{
				Level:       "debug",
				Development: true,
				JSON:        false,
			},
		},
		{
			name: "invalid level defaults to info",
			config: LogConfig{
				Level:       "invalid",
				Development: false,
				JSON:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)
			if err != nil {
				t.Errorf("NewLogger() error = %v", err)
				return
			}
			if logger == nil {
				t.Error("NewLogger() returned nil logger")
				return
			}
			if logger.Logger == nil {
				t.Error("NewLogger() returned nil zap.Logger")
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	logger, err := DefaultLogger()
	if err != nil {
		t.Errorf("DefaultLogger() error = %v", err)
		return
	}
	if logger == nil {
		t.Error("DefaultLogger() returned nil logger")
	}
}

func TestDevelopmentLogger(t *testing.T) {
	logger, err := DevelopmentLogger()
	if err != nil {
		t.Errorf("DevelopmentLogger() error = %v", err)
		return
	}
	if logger == nil {
		t.Error("DevelopmentLogger() returned nil logger")
	}
}

func TestLoggerWithRequestID(t *testing.T) {
	logger, _ := DefaultLogger()
	withRequestID := logger.WithRequestID("req-123")
	if withRequestID == nil {
		t.Error("WithRequestID() returned nil logger")
	}
}

func TestLoggerWithTraceID(t *testing.T) {
	logger, _ := DefaultLogger()
	withTraceID := logger.WithTraceID("trace-456")
	if withTraceID == nil {
		t.Error("WithTraceID() returned nil logger")
	}
}
