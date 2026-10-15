package domain

import (
	"time"

	"github.com/google/uuid"
)

// Redpanda topics emitted by the KYC service on review decisions.
// Consumers (notification-service, analytics) subscribe independently;
// the KYC service never consumes these topics itself.
const (
	TopicKYCVerified = "kyc.verified"
	TopicKYCRejected = "kyc.rejected"
)

// KYCStatusEvent is the payload published on TopicKYCVerified/TopicKYCRejected
// after an admin review decision. It carries outcome only — document bytes
// and PII stay in S3 and the encrypted columns, never in the event.
type KYCStatusEvent struct {
	EventID    string             `json:"event_id"`
	UserID     int64              `json:"user_id"`
	Level      KYCLevel           `json:"level"`
	Status     VerificationStatus `json:"status"`
	Reason     string             `json:"reason,omitempty"`
	OccurredAt time.Time          `json:"occurred_at"`
}

// NewKYCStatusEvent builds an event with a fresh idempotency key.
// Consumers dedupe on EventID (at-least-once delivery).
func NewKYCStatusEvent(userID int64, level KYCLevel, status VerificationStatus, reason string, now time.Time) KYCStatusEvent {
	return KYCStatusEvent{
		EventID:    uuid.NewString(),
		UserID:     userID,
		Level:      level,
		Status:     status,
		Reason:     reason,
		OccurredAt: now.UTC(),
	}
}
