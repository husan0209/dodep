package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// WSMessage is the envelope pushed to browser clients.
type WSMessage struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
	Ts      int64           `json:"ts"`
}

// wsClient represents a connected admin browser session.
type wsClient struct {
	adminID   string
	adminRole string
	send      chan []byte
	done      chan struct{}
}

// WSHub manages all connected WebSocket clients and dispatches messages by role.
type WSHub struct {
	mu              sync.RWMutex
	clients         map[string]*wsClient // keyed by adminID
	rdb             *redis.Client
	log             *zap.Logger
	redpandaBrokers []string
	redpandaGroupID string
}

// roleTopics maps an admin role to the Redpanda/Redis pub-sub topics it may receive.
var roleTopics = map[string][]string{
	"SUPER_ADMIN":       {"admin.metrics.live", "admin.risk.alerts", "admin.withdrawals.new", "admin.kyc.submitted", "admin.provider.health", "admin.bets.large"},
	"FINANCE_MANAGER":   {"admin.metrics.live", "admin.withdrawals.new", "admin.provider.health"},
	"RISK_MANAGER":      {"admin.risk.alerts", "admin.withdrawals.new", "admin.kyc.submitted", "admin.bets.large"},
	"KYC_OFFICER":       {"admin.kyc.submitted"},
	"SPORTS_TRADER":     {"admin.metrics.live", "admin.bets.large"},
	"SUPPORT_AGENT":     {},
	"CRM_MANAGER":       {"admin.metrics.live"},
	"AFFILIATE_MANAGER": {},
	"CONTENT_MANAGER":   {},
	"VIEWER":            {"admin.metrics.live"},
}

var adminAlertTopics = []string{
	"admin.metrics.live",
	"admin.risk.alerts",
	"admin.withdrawals.new",
	"admin.kyc.submitted",
	"admin.provider.health",
	"admin.bets.large",
}

func NewWSHub(rdb *redis.Client, log *zap.Logger, redpandaBrokers []string) *WSHub {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "local"
	}
	return &WSHub{
		clients:         make(map[string]*wsClient),
		rdb:             rdb,
		log:             log,
		redpandaBrokers: redpandaBrokers,
		redpandaGroupID: "admin-bff-alerts-" + hostname,
	}
}

// Start launches background goroutines that poll the configured real-time
// source before pushing messages to subscribed clients.
func (h *WSHub) Start(ctx context.Context) {
	go h.pumpMetrics(ctx)
	if len(h.redpandaBrokers) > 0 {
		go h.pumpRedpanda(ctx)
		h.log.Info("Redpanda configured for admin alerts", zap.Int("brokers", len(h.redpandaBrokers)))
	} else {
		go h.pumpPubSub(ctx)
		h.log.Info("Redpanda not configured; using Redis pub-sub fallback for admin alerts")
	}
}

// pumpMetrics pushes a live-metrics heartbeat every 5 seconds to SUPER_ADMIN + VIEWER roles.
func (h *WSHub) pumpMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload, _ := json.Marshal(map[string]any{
				"heartbeat": true,
				"ts":        time.Now().Unix(),
			})
			msg := WSMessage{Topic: "admin.metrics.live", Payload: payload, Ts: time.Now().Unix()}
			h.broadcast("admin.metrics.live", msg)
		}
	}
}

// pumpPubSub subscribes to Redis pub-sub channels and dispatches to connected clients.
func (h *WSHub) pumpPubSub(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, adminAlertTopics...)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			wsMsg := WSMessage{
				Topic:   msg.Channel,
				Payload: json.RawMessage(msg.Payload),
				Ts:      time.Now().Unix(),
			}
			h.broadcast(msg.Channel, wsMsg)
		}
	}
}

func (h *WSHub) pumpRedpanda(ctx context.Context) {
	for _, topic := range adminAlertTopics {
		go h.consumeRedpandaTopic(ctx, topic)
	}
}

func (h *WSHub) consumeRedpandaTopic(ctx context.Context, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     h.redpandaBrokers,
		Topic:       topic,
		GroupID:     h.redpandaGroupID,
		StartOffset: kafka.LastOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer reader.Close()

	h.log.Info("Starting Redpanda consumer", zap.String("topic", topic))

	for {
		if ctx.Err() != nil {
			return
		}

		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			h.log.Warn("Redpanda fetch failed", zap.String("topic", topic), zap.Error(err))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		payload := msg.Value
		if len(payload) == 0 {
			payload = []byte("{}")
		}

		h.broadcast(topic, WSMessage{
			Topic:   topic,
			Payload: json.RawMessage(payload),
			Ts:      time.Now().Unix(),
		})

		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			h.log.Warn("Redpanda commit failed", zap.String("topic", topic), zap.Error(err))
		}
	}
}

// broadcast sends a message to all clients subscribed to the given topic.
func (h *WSHub) broadcast(topic string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		if topicAllowed(client.adminRole, topic) {
			select {
			case client.send <- data:
			default:
				// Client buffer full — skip this message to avoid blocking
			}
		}
	}
}

func topicAllowed(role, topic string) bool {
	topics, ok := roleTopics[role]
	if !ok {
		return false
	}
	for _, t := range topics {
		if t == topic {
			return true
		}
	}
	return false
}

// RegisterWSRoutes registers the /admin/ws endpoint using Server-Sent Events (SSE).
// SSE is fully supported by all modern browsers and is functionally equivalent for
// server→client push. For bidirectional WS, add gofiber/websocket/v2 in a follow-up.
func RegisterWSRoutes(router fiber.Router, hub *WSHub, log *zap.Logger) {
	router.Get("/ws", func(c *fiber.Ctx) error {
		adminID := normalizeID(c.Locals("admin_id"))
		adminRole, _ := c.Locals("admin_role").(string)

		client := &wsClient{
			adminID:   adminID,
			adminRole: adminRole,
			send:      make(chan []byte, 64),
			done:      make(chan struct{}),
		}

		hub.mu.Lock()
		hub.clients[adminID] = client
		hub.mu.Unlock()

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			defer func() {
				hub.mu.Lock()
				delete(hub.clients, adminID)
				hub.mu.Unlock()
				close(client.done)
			}()

			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case data, ok := <-client.send:
					if !ok {
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", data)
					if err := w.Flush(); err != nil {
						return
					}
				case <-ticker.C:
					fmt.Fprint(w, ": keepalive\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		}))
		return nil
	})
}

func normalizeID(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
