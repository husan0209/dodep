package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

// GatewayPublisher delivers in-app notifications in real time through the
// Rust WebSocket gateway: the gateway fans the envelope out to the user's
// active connections. Transport is the gateway HTTP publish endpoint (plain
// HTTPS, shared-secret header). It implements service.InAppPublisher
// without changing the interface.
type GatewayPublisher struct {
	baseURL string
	path    string
	apiKey  string
	http    *http.Client
	log     *zap.Logger
}

// Compile-time guarantee: the wave1 interface is implemented as-is.
var _ service.InAppPublisher = (*GatewayPublisher)(nil)

// NewGatewayPublisherFromEnv builds the publisher from the environment.
// WS_GATEWAY_URL (e.g. http://websocket-gateway:8082) is required;
// WS_GATEWAY_API_KEY is the shared edge secret (optional, sent as
// X-API-Key); WS_GATEWAY_PUBLISH_PATH defaults to /api/v1/publish.
// The second return value is false when no URL is configured — the caller
// must keep the log fallback in that case.
func NewGatewayPublisherFromEnv(log *zap.Logger) (*GatewayPublisher, bool) {
	if log == nil {
		log = zap.NewNop()
	}
	base := firstEnv("WS_GATEWAY_URL")
	if base == "" {
		return nil, false
	}
	path := firstEnv("WS_GATEWAY_PUBLISH_PATH")
	if path == "" {
		path = "/api/v1/publish"
	}
	return &GatewayPublisher{
		baseURL: endpointOr("WS_GATEWAY_URL", ""),
		path:    path,
		apiKey:  firstEnv("WS_GATEWAY_API_KEY"),
		http:    newHTTPClient(),
		log:     log,
	}, true
}

type gatewayEnvelope struct {
	UserID  uint64            `json:"user_id"`
	Event   string            `json:"event"`
	Payload map[string]string `json:"payload"`
	SentAt  string            `json:"sent_at"`
}

// PublishInApp forwards the WS event envelope to the gateway.
func (p *GatewayPublisher) PublishInApp(ctx context.Context, event service.WSEvent) error {
	if event.UserID == 0 {
		return fmt.Errorf("gateway: user_id is required")
	}
	body, err := json.Marshal(gatewayEnvelope{
		UserID:  event.UserID,
		Event:   event.Event,
		Payload: event.Payload,
		SentAt:  event.SentAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("gateway: encode envelope: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+p.path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("X-API-Key", p.apiKey)
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("gateway: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gateway: unexpected status %d: %s", resp.StatusCode, errorSnippet(resp.Body))
	}
	p.log.Info("in-app event published to gateway",
		zap.String("event", event.Event),
		zap.Uint64("user_id", event.UserID))
	return nil
}
