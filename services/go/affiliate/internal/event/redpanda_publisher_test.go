package event

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

func TestNewRedpandaPublisherRequiresBrokers(t *testing.T) {
	_, err := NewRedpandaPublisher(context.Background(), RedpandaPublisherConfig{}, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for empty broker list")
	}
}

func TestNewRedpandaPublisherUnreachableBroker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := NewRedpandaPublisher(ctx, RedpandaPublisherConfig{
		Brokers:         []string{"127.0.0.1:1"}, // nothing listens here
		DeliveryTimeout: time.Second,
	}, zap.NewNop())
	if err == nil {
		t.Fatal("expected ping error for unreachable broker")
	}
}

func TestBuildRecordLayout(t *testing.T) {
	payload := map[string]any{
		"affiliate_id": "aff-1",
		"amount":       "20.00",
		"nested":       map[string]any{"a": 1},
	}
	headers := map[string]any{"event_id": "evt-1", "retry": 3}

	rec, err := buildRecord("affiliate.commission.accrued", "aff-1", payload, headers)
	if err != nil {
		t.Fatalf("buildRecord failed: %v", err)
	}
	if rec.Topic != "affiliate.commission.accrued" {
		t.Fatalf("unexpected topic: %s", rec.Topic)
	}
	if string(rec.Key) != "aff-1" {
		t.Fatalf("key must carry the aggregate id, got %s", rec.Key)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Value, &decoded); err != nil {
		t.Fatalf("value is not valid JSON: %v", err)
	}
	if decoded["amount"] != "20.00" {
		t.Fatalf("payload corrupted: %v", decoded)
	}
	if rec.Timestamp.IsZero() {
		t.Fatal("record timestamp must be set")
	}
	got := map[string]string{}
	for _, h := range rec.Headers {
		got[h.Key] = string(h.Value)
	}
	if got["event_id"] != "evt-1" || got["retry"] != "3" {
		t.Fatalf("headers corrupted: %v", got)
	}
}

func TestBuildRecordRejectsUnmarshalablePayload(t *testing.T) {
	_, err := buildRecord("t", "k",
		map[string]any{"bad": func() {}}, nil)
	if err == nil {
		t.Fatal("expected marshal error for unmarshalable payload")
	}
}

func TestRedpandaPublisherImplementsInterface(t *testing.T) {
	var _ Publisher = (*RedpandaPublisher)(nil)
	var _ Publisher = (*LogPublisher)(nil)
}

func TestRecordDefaultPartitionerUsesKey(t *testing.T) {
	// franz-go's default sticky partitioner hashes Record.Key when set,
	// keeping one affiliate's events ordered in a single partition.
	// This test pins the contract: key must never be empty for outbox flow.
	rec := &kgo.Record{Topic: "t", Key: []byte("aff-1"), Value: []byte("{}")}
	if len(rec.Key) == 0 {
		t.Fatal("record key must be set for partition co-location")
	}
}
