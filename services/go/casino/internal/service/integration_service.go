package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
)

// ─── Prometheus metrics ───────────────────────────────────────────────────────

var (
	callbacksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "casino_callbacks_total",
		Help: "Total casino provider callbacks by provider, type, and status.",
	}, []string{"provider", "type", "status"})

	callbackLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "casino_provider_latency_seconds",
		Help:    "Casino provider callback processing latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "operation"})

	signatureInvalid = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "casino_signature_invalid_total",
		Help: "Total invalid provider callback signatures.",
	}, []string{"provider"})

	replayViolations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "casino_replay_window_violations_total",
		Help: "Total replay window violations per provider.",
	}, []string{"provider"})
)

// ─── WalletClient interface ───────────────────────────────────────────────────

// WalletClient is the gRPC-based wallet operations interface used by IntegrationService.
type WalletClient interface {
	GetBalance(ctx context.Context, userID int64, currency string) (decimal.Decimal, error)
	Debit(ctx context.Context, req DebitRequest) (DebitResult, error)
	Credit(ctx context.Context, req CreditRequest) (CreditResult, error)
	Rollback(ctx context.Context, req RollbackRequest) (RollbackResult, error)
}

// DebitRequest for wallet debit (casino bet).
type DebitRequest struct {
	UserID         int64
	Currency       string
	Amount         decimal.Decimal
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    string
}

// DebitResult from wallet debit.
type DebitResult struct {
	NewBalance    decimal.Decimal
	TransactionID string
}

// CreditRequest for wallet credit (casino win).
type CreditRequest struct {
	UserID         int64
	Currency       string
	Amount         decimal.Decimal
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    string
}

// CreditResult from wallet credit.
type CreditResult struct {
	NewBalance    decimal.Decimal
	TransactionID string
}

// RollbackRequest for reversing a transaction.
type RollbackRequest struct {
	UserID         int64
	RefTransID     string
	IdempotencyKey string
}

// RollbackResult from rollback.
type RollbackResult struct {
	NewBalance    decimal.Decimal
	TransactionID string
}

// ─── PlayerMapper interface ────────────────────────────────────────────────────

// PlayerMapper maps opaque provider player_id to internal user_id.
type PlayerMapper interface {
	// GetInternalUserID resolves a provider-side player_id to our user_id.
	GetInternalUserID(ctx context.Context, providerName, playerID string) (int64, error)
	// GetPlayerID resolves our user_id to a provider-side opaque player_id.
	GetPlayerID(ctx context.Context, providerName string, userID int64) (string, error)
}

// ─── IntegrationService ───────────────────────────────────────────────────────

// IntegrationService orchestrates inbound provider callbacks.
type IntegrationService struct {
	registry     *provider.Registry
	wallet       WalletClient
	playerMapper PlayerMapper
	idempotency  *redis.Client
	log          *zap.Logger
}

// NewIntegrationService creates the casino integration service.
func NewIntegrationService(
	registry *provider.Registry,
	wallet WalletClient,
	playerMapper PlayerMapper,
	rdb *redis.Client,
	log *zap.Logger,
) *IntegrationService {
	return &IntegrationService{
		registry:    registry,
		wallet:      wallet,
		playerMapper: playerMapper,
		idempotency: rdb,
		log:         log,
	}
}

// ─── Public API ───────────────────────────────────────────────────────────────

// ProcessCallback is the unified entry point for all provider callbacks.
// It validates signature, checks idempotency, resolves player_id → user_id,
// dispatches to wallet, and returns the response for the provider.
func (s *IntegrationService) ProcessCallback(
	ctx context.Context,
	providerName string,
	body []byte,
	headers map[string]string,
) (*provider.CallbackResponse, error) {
	start := time.Now()

	adapter, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("unknown provider %q", providerName)
	}

	// 1. Verify signature
	if !adapter.VerifyCallbackSignature(body, headers) {
		signatureInvalid.WithLabelValues(providerName).Inc()
		return nil, errors.New("invalid signature")
	}

	// 2. Parse callback
	event, err := adapter.ParseCallback(body)
	if err != nil {
		if isReplayError(err) {
			replayViolations.WithLabelValues(providerName).Inc()
		}
		return nil, fmt.Errorf("parse callback: %w", err)
	}

	callbackType := string(event.Type)

	// 3. Handle balance query (no wallet mutation needed)
	if event.Type == provider.CallbackBalance {
		resp, err := s.handleBalance(ctx, providerName, event)
		if err != nil {
			callbacksTotal.WithLabelValues(providerName, callbackType, "error").Inc()
			return nil, err
		}
		callbacksTotal.WithLabelValues(providerName, callbackType, "ok").Inc()
		callbackLatency.WithLabelValues(providerName, callbackType).Observe(time.Since(start).Seconds())
		return resp, nil
	}

	// 4. Idempotency check (fast path: DragonflyDB)
	idempotencyKey := fmt.Sprintf("casino:callback:%s:%s", providerName, event.TransactionID)
	if cached, err := s.idempotency.Get(ctx, idempotencyKey).Bytes(); err == nil && len(cached) > 0 {
		var resp provider.CallbackResponse
		if jsonErr := json.Unmarshal(cached, &resp); jsonErr == nil {
			s.log.Debug("Casino: idempotent response",
				zap.String("provider", providerName),
				zap.String("transaction_id", event.TransactionID))
			callbacksTotal.WithLabelValues(providerName, callbackType, "duplicate").Inc()
			return &resp, nil
		}
	}

	// 5. Resolve player_id → internal user_id
	userID, err := s.playerMapper.GetInternalUserID(ctx, providerName, event.PlayerID)
	if err != nil {
		callbacksTotal.WithLabelValues(providerName, callbackType, "error").Inc()
		return nil, fmt.Errorf("resolve player: %w", err)
	}

	// 6. Dispatch to wallet
	var resp *provider.CallbackResponse
	switch event.Type {
	case provider.CallbackBet, provider.CallbackFreeSpins:
		resp, err = s.handleBet(ctx, userID, event)
	case provider.CallbackWin, provider.CallbackJackpot:
		resp, err = s.handleWin(ctx, userID, event)
	case provider.CallbackRollback:
		resp, err = s.handleRollback(ctx, userID, event)
	default:
		err = fmt.Errorf("unknown callback type: %s", event.Type)
	}

	if err != nil {
		callbacksTotal.WithLabelValues(providerName, callbackType, "error").Inc()
		return nil, err
	}

	// 7. Cache the response for idempotency (TTL = replay window + buffer)
	if cached, jsonErr := json.Marshal(resp); jsonErr == nil {
		ttl := 24 * time.Hour
		s.idempotency.Set(ctx, idempotencyKey, cached, ttl)
	}

	callbacksTotal.WithLabelValues(providerName, callbackType, "ok").Inc()
	callbackLatency.WithLabelValues(providerName, callbackType).Observe(time.Since(start).Seconds())

	s.log.Info("Casino: callback processed",
		zap.String("provider", providerName),
		zap.String("type", callbackType),
		zap.String("tx_id", event.TransactionID),
		zap.Int64("user_id", userID),
		zap.String("amount", event.Amount.String()))

	return resp, nil
}

// ─── Wallet dispatchers ──────────────────────────────────────────────────────

func (s *IntegrationService) handleBalance(ctx context.Context, providerName string, event *provider.CallbackEvent) (*provider.CallbackResponse, error) {
	userID, err := s.playerMapper.GetInternalUserID(ctx, providerName, event.PlayerID)
	if err != nil {
		return nil, fmt.Errorf("resolve player for balance: %w", err)
	}

	balance, err := s.wallet.GetBalance(ctx, userID, event.Currency)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	return &provider.CallbackResponse{Balance: balance}, nil
}

func (s *IntegrationService) handleBet(ctx context.Context, userID int64, event *provider.CallbackEvent) (*provider.CallbackResponse, error) {
	result, err := s.wallet.Debit(ctx, DebitRequest{
		UserID:         userID,
		Currency:       event.Currency,
		Amount:         event.Amount,
		IdempotencyKey: "casino:bet:" + event.TransactionID,
		ReferenceType:  "casino_bet",
		ReferenceID:    event.RoundID,
	})
	if err != nil {
		return nil, fmt.Errorf("casino bet: %w", err)
	}

	return &provider.CallbackResponse{
		Balance:       result.NewBalance,
		TransactionID: result.TransactionID,
	}, nil
}

func (s *IntegrationService) handleWin(ctx context.Context, userID int64, event *provider.CallbackEvent) (*provider.CallbackResponse, error) {
	// Zero-amount wins are valid (no win this round)
	if event.Amount.IsZero() {
		balance, err := s.wallet.GetBalance(ctx, userID, event.Currency)
		if err != nil {
			return nil, err
		}
		return &provider.CallbackResponse{Balance: balance}, nil
	}

	result, err := s.wallet.Credit(ctx, CreditRequest{
		UserID:         userID,
		Currency:       event.Currency,
		Amount:         event.Amount,
		IdempotencyKey: "casino:win:" + event.TransactionID,
		ReferenceType:  "casino_win",
		ReferenceID:    event.RoundID,
	})
	if err != nil {
		return nil, fmt.Errorf("casino win: %w", err)
	}

	return &provider.CallbackResponse{
		Balance:       result.NewBalance,
		TransactionID: result.TransactionID,
	}, nil
}

func (s *IntegrationService) handleRollback(ctx context.Context, userID int64, event *provider.CallbackEvent) (*provider.CallbackResponse, error) {
	result, err := s.wallet.Rollback(ctx, RollbackRequest{
		UserID:         userID,
		RefTransID:     event.RefTransID,
		IdempotencyKey: "casino:rollback:" + event.TransactionID,
	})
	if err != nil {
		return nil, fmt.Errorf("casino rollback: %w", err)
	}

	return &provider.CallbackResponse{
		Balance:       result.NewBalance,
		TransactionID: result.TransactionID,
	}, nil
}

func isReplayError(err error) bool {
	return err != nil && (contains(err.Error(), "replay") || contains(err.Error(), "age="))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
