package main

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHealthProbe_Healthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strings.TrimPrefix(ln.Addr().String(), "127.0.0.1:")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Setenv("PORT", port)
	assert.Equal(t, 0, runHealthProbe())
}

func TestRunHealthProbe_Unreachable(t *testing.T) {
	t.Setenv("PORT", "1") // privileged/closed port: connection refused
	assert.Equal(t, 1, runHealthProbe())
}

func TestRunHealthProbe_Non200FailsClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strings.TrimPrefix(ln.Addr().String(), "127.0.0.1:")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Setenv("PORT", port)
	assert.Equal(t, 1, runHealthProbe())
}

// PORT is environment input, so a non-numeric or out-of-range value must fail
// closed instead of being interpolated into the probe URL (gosec G704).
func TestRunHealthProbe_RejectsNonNumericPort(t *testing.T) {
	for name, port := range map[string]string{
		"empty":            "",
		"not a number":     "http",
		"url injection":    "8088@evil.example.com",
		"path injection":   "8088/health",
		"zero":             "0",
		"out of range":     "70000",
		"negative":         "-1",
		"trailing garbage": "8088abc",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PORT", port)
			assert.Equal(t, 1, runHealthProbe())
		})
	}
}

// The fallback must stay the CONVENTIONS.md port and must not accidentally
// become a value that passes the numeric validation.
func TestDefaultHTTPPortIsValid(t *testing.T) {
	n, err := strconv.Atoi(defaultHTTPPort)
	require.NoError(t, err)
	assert.True(t, n > 0 && n <= 65535, "defaultHTTPPort must be a usable port")
}
