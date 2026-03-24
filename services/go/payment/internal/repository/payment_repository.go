package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opus-casino/payment/internal/domain"
)

type PaymentRepository struct {
	pool pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: *pool}
}

func (r *PaymentRepository) CreateDeposit(ctx context.Context, req *domain.CreateDepositRequest) (*domain.Deposit, error) {
	deposit := &domain.Deposit{
		ID:                uuid.New().String(),
		UserID:            req.UserID,
		Amount:            req.Amount,
		Fee:               "0.00",
		NetAmount:         req.Amount,
		Currency:          req.Currency,
		PaymentMethodID:   req.PaymentMethodID,
		PaymentProvider:   req.PaymentProvider,
		Status:            domain.TransactionStatusPending,
		IdempotencyKey:    req.IdempotencyKey,
	}

	query := `
		INSERT INTO wallet_transactions (id, user_id, wallet_id, type, amount, balance_before, balance_after,
			reference_type, reference_id, idempotency_key, status, metadata, created_at)
		VALUES ($1, $2, (SELECT id FROM wallets WHERE user_id = $2 LIMIT 1), 'deposit',
			$3::numeric, 0, 0, 'payment', 0, $4, 'pending', '{}', NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query, deposit.ID, deposit.UserID, deposit.Amount, deposit.IdempotencyKey).
		Scan(&deposit.ID, &deposit.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create deposit: %w", err)
	}
	deposit.UpdatedAt = deposit.CreatedAt
	return deposit, nil
}

func (r *PaymentRepository) GetDeposit(ctx context.Context, userID int64, depositID string) (*domain.Deposit, error) {
	query := `
		SELECT id, user_id, amount::text, '0.00', amount::text, 'USD', '', '', '',
			COALESCE(metadata->>'provider_transaction_id', ''),
			status, COALESCE(metadata->>'idempotency_key', ''), created_at, created_at, NULL, NULL, metadata::text
		FROM wallet_transactions
		WHERE id = $1 AND user_id = $2 AND type = 'deposit'
	`
	d := &domain.Deposit{}
	err := r.pool.QueryRow(ctx, query, depositID, userID).Scan(
		&d.ID, &d.UserID, &d.Amount, &d.Fee, &d.NetAmount, &d.Currency,
		&d.PaymentMethodID, &d.PaymentMethodType, &d.PaymentProvider,
		&d.ProviderTransactionID, &d.Status, &d.IdempotencyKey,
		&d.CreatedAt, &d.UpdatedAt, &d.CompletedAt, &d.FailureReason, &d.Metadata,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get deposit: %w", err)
	}
	return d, nil
}

func (r *PaymentRepository) ListDeposits(ctx context.Context, userID int64, limit, offset int) ([]*domain.Deposit, int, error) {
	var total int
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM wallet_transactions WHERE user_id = $1 AND type = 'deposit'", userID).Scan(&total)

	query := `
		SELECT id, user_id, amount::text, '0.00', amount::text, 'USD', status, created_at, created_at
		FROM wallet_transactions WHERE user_id = $1 AND type = 'deposit'
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var deposits []*domain.Deposit
	for rows.Next() {
		d := &domain.Deposit{}
		rows.Scan(&d.ID, &d.UserID, &d.Amount, &d.Fee, &d.NetAmount, &d.Currency, &d.Status, &d.CreatedAt, &d.UpdatedAt)
		deposits = append(deposits, d)
	}
	return deposits, total, nil
}

func (r *PaymentRepository) UpdateDepositStatus(ctx context.Context, depositID string, status domain.TransactionStatus, providerTxID *string) error {
	query := `UPDATE wallet_transactions SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, depositID, status)
	return err
}

func (r *PaymentRepository) CreateWithdrawal(ctx context.Context, req *domain.RequestWithdrawalRequest) (*domain.Withdrawal, error) {
	w := &domain.Withdrawal{
		ID:              uuid.New().String(),
		UserID:          req.UserID,
		Amount:          req.Amount,
		Fee:             "0.00",
		NetAmount:       req.Amount,
		Currency:        req.Currency,
		PaymentMethodID: req.PaymentMethodID,
		PaymentProvider: req.PaymentProvider,
		Status:          domain.TransactionStatusPending,
		IdempotencyKey:  req.IdempotencyKey,
	}

	query := `
		INSERT INTO wallet_transactions (id, user_id, wallet_id, type, amount, balance_before, balance_after,
			reference_type, reference_id, idempotency_key, status, metadata, created_at)
		VALUES ($1, $2, (SELECT id FROM wallets WHERE user_id = $2 LIMIT 1), 'withdrawal',
			$3::numeric, 0, 0, 'payment', 0, $4, 'pending', '{}', NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query, w.ID, w.UserID, w.Amount, w.IdempotencyKey).
		Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}
	w.UpdatedAt = w.CreatedAt
	return w, nil
}

func (r *PaymentRepository) GetWithdrawal(ctx context.Context, userID int64, withdrawalID string) (*domain.Withdrawal, error) {
	query := `
		SELECT id, user_id, amount::text, '0.00', amount::text, 'USD', '', '', '',
			COALESCE(metadata->>'provider_transaction_id', ''),
			status, COALESCE(metadata->>'idempotency_key', ''), created_at, created_at
		FROM wallet_transactions
		WHERE id = $1 AND user_id = $2 AND type = 'withdrawal'
	`
	w := &domain.Withdrawal{}
	err := r.pool.QueryRow(ctx, query, withdrawalID, userID).Scan(
		&w.ID, &w.UserID, &w.Amount, &w.Fee, &w.NetAmount, &w.Currency,
		&w.PaymentMethodID, &w.PaymentMethodType, &w.PaymentProvider,
		&w.ProviderTransactionID, &w.Status, &w.IdempotencyKey,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get withdrawal: %w", err)
	}
	return w, nil
}

func (r *PaymentRepository) ListWithdrawals(ctx context.Context, userID int64, limit, offset int) ([]*domain.Withdrawal, int, error) {
	var total int
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM wallet_transactions WHERE user_id = $1 AND type = 'withdrawal'", userID).Scan(&total)

	query := `
		SELECT id, user_id, amount::text, '0.00', amount::text, 'USD', status, created_at, created_at
		FROM wallet_transactions WHERE user_id = $1 AND type = 'withdrawal'
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var withdrawals []*domain.Withdrawal
	for rows.Next() {
		w := &domain.Withdrawal{}
		rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Fee, &w.NetAmount, &w.Currency, &w.Status, &w.CreatedAt, &w.UpdatedAt)
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, total, nil
}

func (r *PaymentRepository) CancelWithdrawal(ctx context.Context, userID int64, withdrawalID string) error {
	query := `
		UPDATE wallet_transactions SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND type = 'withdrawal' AND status = 'pending'
	`
	cmd, err := r.pool.Exec(ctx, query, withdrawalID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("withdrawal not found or cannot be cancelled")
	}
	return nil
}

func (r *PaymentRepository) GetPaymentMethods(ctx context.Context, userID int64) ([]*domain.PaymentMethodInfo, error) {
	return []*domain.PaymentMethodInfo{
		{ID: "card", Type: domain.PaymentMethodCreditCard, Provider: "stripe", DisplayName: "Credit/Debit Card", SupportedCurrencies: []string{"USD", "EUR", "GBP"}, MinAmount: "10.00", MaxAmount: "10000.00", ProcessingTime: "Instant"},
		{ID: "bank", Type: domain.PaymentMethodBankTransfer, Provider: "stripe", DisplayName: "Bank Transfer", SupportedCurrencies: []string{"USD", "EUR"}, MinAmount: "50.00", MaxAmount: "50000.00", ProcessingTime: "1-3 business days"},
		{ID: "crypto", Type: domain.PaymentMethodCrypto, Provider: "coinbase", DisplayName: "Cryptocurrency", SupportedCurrencies: []string{"BTC", "ETH", "USDT"}, MinAmount: "10.00", MaxAmount: "100000.00", ProcessingTime: "10-60 minutes"},
	}, nil
}

func (r *PaymentRepository) SavePaymentMethod(ctx context.Context, method *domain.PaymentMethod) error {
	query := `
		INSERT INTO payment_methods (id, user_id, type, provider, nickname, display_value, is_default, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW())
	`
	_, err := r.pool.Exec(ctx, query, method.ID, method.UserID, method.Type, method.Provider, method.Nickname, method.DisplayValue, method.IsDefault)
	return err
}

func (r *PaymentRepository) DeletePaymentMethod(ctx context.Context, userID int64, methodID string) error {
	query := `UPDATE payment_methods SET is_active = false WHERE id = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, query, methodID, userID)
	return err
}

func (r *PaymentRepository) RecordWebhookEvent(ctx context.Context, event *domain.WebhookEvent) error {
	query := `
		INSERT INTO audit_log (table_name, record_id, action, new_data, created_at)
		VALUES ('webhooks', 0, $1, $2, NOW())
	`
	_, err := r.pool.Exec(ctx, query, event.EventType, fmt.Sprintf(`{"provider":"%s","tx_id":"%s","status":"%s"}`, event.Provider, event.TransactionID, event.Status))
	return err
}

func (r *PaymentRepository) CheckIdempotency(ctx context.Context, key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM wallet_transactions WHERE idempotency_key = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, key).Scan(&exists)
	return exists, err
}
