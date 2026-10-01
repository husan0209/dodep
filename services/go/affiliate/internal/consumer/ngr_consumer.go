package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
)

// NGRConsumer consumes GGR/NGR events from Redpanda and calculates affiliate commissions.
// Subscribes to topics:
//   - casino.rounds.settled
//   - betting.bets.settled
//   - affiliate.player.activity

// NGREvent represents a Net Gaming Revenue event
type NGREvent struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	UserID        int64             `json:"user_id"`
	AffiliateID   string            `json:"affiliate_id,omitempty"`
	GGRAmount     string            `json:"ggr_amount"`
	NGRAmount     string            `json:"ngr_amount"`
	Bonuses       string            `json:"bonuses"`
	PaymentFees   string            `json:"payment_fees"`
	Chargebacks   string            `json:"chargebacks"`
	SourceType    string            `json:"source_type"` // casino, sports, live
	SourceID      string            `json:"source_id"`   // round_id, bet_id
	PeriodStart   time.Time         `json:"period_start"`
	PeriodEnd     time.Time         `json:"period_end"`
	Currency      string            `json:"currency"`
	Metadata      map[string]string `json:"metadata"`
	Timestamp     time.Time         `json:"timestamp"`
	IdempotencyKey string           `json:"idempotency_key"`
}

// AttributionEvent represents a player attribution event
type AttributionEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"` // registration, ftd, first_bet
	UserID         int64     `json:"user_id"`
	AffiliateID    string    `json:"affiliate_id"`
	ClickID        string    `json:"click_id"`
	AttributedAt   time.Time `json:"attributed_at"`
	Timestamp      time.Time `json:"timestamp"`
}

// Message represents a generic Kafka message
type Message struct {
	Topic     string
	Key       []byte
	Value     []byte
	Partition int32
	Offset    int64
}

// MessageConsumer is the interface for consuming messages
type MessageConsumer interface {
	Consume(ctx context.Context, handler func(Message) error) error
}

// NGRConsumerConfig holds configuration
type NGRConsumerConfig struct {
	Brokers       []string
	GroupID       string
	Topics        []string
	PollTimeout   time.Duration
	RetryBackoff  time.Duration
	MaxRetries    int
	BatchSize     int
	FlushInterval time.Duration
}

// DefaultNGRConsumerConfig returns sensible defaults
func DefaultNGRConsumerConfig() NGRConsumerConfig {
	return NGRConsumerConfig{
		GroupID:       "affiliate-ngr-processor",
		Topics:        []string{"casino.rounds.settled", "betting.bets.settled", "affiliate.player.activity"},
		PollTimeout:   100 * time.Millisecond,
		RetryBackoff:  1 * time.Second,
		MaxRetries:    5,
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
	}
}

// NGRConsumer processes GGR/NGR events and triggers commission calculations
type NGRConsumer struct {
	config  NGRConsumerConfig
	svc     *service.AffiliateService
	logger  *zap.Logger
	metrics *ConsumerMetrics

	// Attribution cache (user_id -> affiliate_id)
	attributionCache sync.Map

	// Batch processor
	batchMu    sync.Mutex
	batch      []NGREvent
	lastFlush  time.Time
}

// ConsumerMetrics tracks processing metrics
type ConsumerMetrics struct {
	EventsProcessed int64
	EventsFailed    int64
	CommissionsCalc int64
	CommissionsFail int64
	LastProcessedAt time.Time
}

// NewNGRConsumer creates a new consumer
func NewNGRConsumer(svc *service.AffiliateService, logger *zap.Logger, config NGRConsumerConfig) *NGRConsumer {
	return &NGRConsumer{
		config:     config,
		svc:        svc,
		logger:     logger,
		metrics:    &ConsumerMetrics{},
		batch:      make([]NGREvent, 0, config.BatchSize),
		lastFlush:  time.Now(),
	}
}

// ProcessMessage handles incoming Kafka messages
func (c *NGRConsumer) ProcessMessage(ctx context.Context, msg Message) error {
	topic := msg.Topic

	switch topic {
	case "casino.rounds.settled", "betting.bets.settled":
		return c.processNGREvent(ctx, msg)
	case "affiliate.player.activity":
		return c.processAttributionEvent(ctx, msg)
	default:
		c.logger.Debug("ignoring message from unknown topic",
			zap.String("topic", topic),
			zap.Int64("offset", msg.Offset),
		)
		return nil
	}
}

// processNGREvent handles GGR/NGR events and calculates commission
func (c *NGRConsumer) processNGREvent(ctx context.Context, msg Message) error {
	var event NGREvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.logger.Error("failed to unmarshal NGR event",
			zap.Error(err),
			zap.String("topic", msg.Topic),
		)
		return nil // Skip malformed messages
	}

	// Skip if no affiliate attribution
	if event.AffiliateID == "" {
		// Try to resolve from cache
		if affiliateID, ok := c.attributionCache.Load(event.UserID); ok {
			event.AffiliateID = affiliateID.(string)
		} else {
			// No affiliate attribution, skip commission calculation
			c.logger.Debug("no affiliate attribution for user",
				zap.Int64("user_id", event.UserID),
			)
			return nil
		}
	}

	// Add to batch for processing
	c.addToBatch(event)

	return nil
}

// processAttributionEvent handles player attribution events
func (c *NGRConsumer) processAttributionEvent(ctx context.Context, msg Message) error {
	var event AttributionEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.logger.Error("failed to unmarshal attribution event",
			zap.Error(err),
		)
		return nil
	}

	// Cache the attribution
	c.attributionCache.Store(event.UserID, event.AffiliateID)

	// If this is a registration or FTD, bind the referred user
	if event.EventType == "registration" || event.EventType == "ftd" {
		affiliateID, err := uuid.Parse(event.AffiliateID)
		if err != nil {
			c.logger.Error("invalid affiliate_id in attribution event",
				zap.Error(err),
				zap.String("affiliate_id", event.AffiliateID),
			)
			return nil
		}

		_, err = c.svc.BindReferredUser(ctx, service.BindReferredUserInput{
			AffiliateID:    affiliateID,
			ReferredUserID: event.UserID,
			ClickID:        event.ClickID,
		})
		if err != nil {
			// Log but don't fail - attribution might already exist
			c.logger.Debug("bind referred user result",
				zap.Error(err),
				zap.Int64("user_id", event.UserID),
			)
		}
	}

	return nil
}

// addToBatch adds an event to the processing batch
func (c *NGRConsumer) addToBatch(event NGREvent) {
	c.batchMu.Lock()
	defer c.batchMu.Unlock()

	c.batch = append(c.batch, event)

	// Flush if batch is full
	if len(c.batch) >= c.config.BatchSize {
		go c.flushBatch(context.Background())
	}
}

// flushBatch processes accumulated events
func (c *NGRConsumer) flushBatch(ctx context.Context) {
	c.batchMu.Lock()
	if len(c.batch) == 0 {
		c.batchMu.Unlock()
		return
	}

	// Swap out the batch
	events := c.batch
	c.batch = make([]NGREvent, 0, c.config.BatchSize)
	c.batchMu.Unlock()

	// Process each event
	for _, event := range events {
		if err := c.calculateCommission(ctx, event); err != nil {
			c.logger.Error("failed to calculate commission",
				zap.Error(err),
				zap.String("event_id", event.EventID),
				zap.String("affiliate_id", event.AffiliateID),
			)
			c.metrics.CommissionsFail++
		} else {
			c.metrics.CommissionsCalc++
		}
	}
}

// calculateCommission triggers commission calculation for an NGR event
func (c *NGRConsumer) calculateCommission(ctx context.Context, event NGREvent) error {
	affiliateID, err := uuid.Parse(event.AffiliateID)
	if err != nil {
		return fmt.Errorf("invalid affiliate_id: %w", err)
	}

	ngrAmount, err := decimal.NewFromString(event.NGRAmount)
	if err != nil {
		return fmt.Errorf("invalid ngr_amount: %w", err)
	}

	ggrAmount, _ := decimal.NewFromString(event.GGRAmount)

	// Calculate commission
	earning, err := c.svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    affiliateID,
		ReferredUserID: event.UserID,
		SourceType:     event.SourceType,
		SourceID:       event.SourceID,
		PeriodStart:    event.PeriodStart,
		PeriodEnd:      event.PeriodEnd,
		GGRAmount:      ggrAmount,
		NGRAmount:      ngrAmount,
		IdempotencyKey: event.IdempotencyKey,
	})
	if err != nil {
		// Check if it's a duplicate (idempotency)
		if err == domain.ErrAffiliateNotFound {
			c.logger.Debug("affiliate not found for commission calculation",
				zap.String("affiliate_id", event.AffiliateID),
			)
			return nil
		}
		return err
	}

	c.logger.Info("commission calculated",
		zap.String("earning_id", earning.ID.String()),
		zap.String("affiliate_id", event.AffiliateID),
		zap.String("ngr_amount", ngrAmount.String()),
		zap.String("commission", earning.CommissionAmount.String()),
	)

	return nil
}

// Run starts the consumer loop
func (c *NGRConsumer) Run(ctx context.Context, consumer MessageConsumer) error {
	c.logger.Info("starting NGR consumer",
		zap.Strings("topics", c.config.Topics),
	)

	// Flush ticker
	flushTicker := time.NewTicker(c.config.FlushInterval)
	defer flushTicker.Stop()

	// Create error channel
	errCh := make(chan error, 1)

	// Start consumer in goroutine
	go func() {
		errCh <- consumer.Consume(ctx, func(msg Message) error {
			c.metrics.EventsProcessed++
			c.metrics.LastProcessedAt = time.Now()
			return c.ProcessMessage(ctx, msg)
		})
	}()

	// Main loop
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("shutting down NGR consumer")
			// Final flush
			c.flushBatch(context.Background())
			return ctx.Err()

		case err := <-errCh:
			if err != nil {
				c.logger.Error("consumer error", zap.Error(err))
				c.metrics.EventsFailed++
			}
			return err

		case <-flushTicker.C:
			// Periodic flush
			c.flushBatch(ctx)
		}
	}
}

// GetMetrics returns current consumer metrics
func (c *NGRConsumer) GetMetrics() ConsumerMetrics {
	return *c.metrics
}
