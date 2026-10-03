package errors

import (
	"errors"
	"testing"
)

func TestConstructors(t *testing.T) {
	cases := []struct {
		name string
		err  *AppError
		code string
	}{
		{"validation", NewValidationError("bad field"), "VALIDATION_ERROR"},
		{"auth", NewAuthError("no token"), "AUTH_ERROR"},
		{"authz", NewAuthzError("denied"), "AUTHZ_ERROR"},
		{"not found", NewNotFoundError("user", "42"), "NOT_FOUND"},
		{"exists", NewAlreadyExistsError("user", "a@b.c"), "ALREADY_EXISTS"},
		{"invalid arg", NewInvalidArgumentError("bad"), "INVALID_ARGUMENT"},
		{"balance", NewInsufficientBalanceError("empty"), "INSUFFICIENT_BALANCE"},
		{"rate limit", NewRateLimitExceededError("slow down"), "RATE_LIMIT_EXCEEDED"},
		{"unavailable", NewServiceUnavailableError("down"), "SERVICE_UNAVAILABLE"},
		{"internal", NewInternalError("boom"), "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.code {
				t.Errorf("expected code %s, got %s", tc.code, tc.err.Code)
			}
			if tc.err.Error() == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestErrorChaining(t *testing.T) {
	cause := errors.New("connection refused")
	wrapped := WrapError(cause, "SERVICE_UNAVAILABLE", "wallet down")
	if wrapped.Code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("bad code: %s", wrapped.Code)
	}
	if !errors.Is(wrapped, cause) {
		t.Fatal("expected errors.Is to find cause via Unwrap")
	}
	var target *AppError
	if !errors.As(wrapped, &target) {
		t.Fatal("expected errors.As to match *AppError")
	}

	plain := NewNotFoundError("user", "7")
	if errors.Is(plain, cause) {
		t.Error("unrelated error must not match")
	}
	msg := plain.Error()
	if msg != "NOT_FOUND: user not found: 7" {
		t.Errorf("unexpected format: %q", msg)
	}
	chainedMsg := wrapped.Error()
	if chainedMsg != "SERVICE_UNAVAILABLE: wallet down: connection refused" {
		t.Errorf("unexpected chained format: %q", chainedMsg)
	}
}

func TestPredicates(t *testing.T) {
	if !IsValidationError(NewValidationError("x")) {
		t.Error("expected validation predicate true")
	}
	if IsValidationError(NewNotFoundError("u", "1")) {
		t.Error("validation predicate must be false for not-found")
	}
	if IsValidationError(errors.New("plain")) {
		t.Error("validation predicate must be false for plain error")
	}
	if !IsNotFoundError(NewNotFoundError("u", "1")) {
		t.Error("expected not-found predicate true")
	}
	if IsNotFoundError(NewAuthError("x")) {
		t.Error("not-found predicate must be false for auth error")
	}
	if !IsInsufficientBalanceError(NewInsufficientBalanceError("x")) {
		t.Error("expected balance predicate true")
	}
	if IsInsufficientBalanceError(nil) {
		t.Error("predicates must be false for nil")
	}
}
