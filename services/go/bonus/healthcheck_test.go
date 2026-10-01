package main

import (
	"net"
	"net/http"
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
