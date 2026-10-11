package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
	"github.com/opus-casino/bonus/internal/service"
)

// fakeCrediter records conversion credits without calling wallet-core.
type fakeCrediter struct {
	calls []service.BonusCreditInput
}

func (f *fakeCrediter) CreditBonusConversion(_ context.Context, in service.BonusCreditInput) error {
	f.calls = append(f.calls, in)
	return nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("bonus_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to database")
	require.NoError(t, db.AutoMigrate(&domain.Bonus{}), "Failed to migrate bonus schema")
	return db
}

func setupTestService(t *testing.T, db *gorm.DB) (*service.BonusService, *fakeCrediter) {
	t.Helper()
	repo := repository.NewBonusRepository(db)
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())
	crediter := &fakeCrediter{}
	svc.SetWalletCrediter(crediter)
	return svc, crediter
}

func TestBonusFlow_WelcomeIdempotent(t *testing.T) {
	db := setupTestDB(t)
	svc, _ := setupTestService(t, db)
	ctx := context.Background()

	first, err := svc.AwardWelcomeBonus(ctx, 1001, decimal.NewFromInt(100))
	require.NoError(t, err)
	require.Equal(t, "100", first.BonusAmount.String())

	second, err := svc.AwardWelcomeBonus(ctx, 1001, decimal.NewFromInt(100))
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "second award must return the same bonus (idempotent)")

	var count int64
	require.NoError(t, db.Model(&domain.Bonus{}).Where("user_id = ?", 1001).Count(&count).Error)
	assert.Equal(t, int64(1), count, "exactly one welcome bonus row must exist")
}

func TestBonusFlow_WageringToConversion(t *testing.T) {
	db := setupTestDB(t)
	svc, crediter := setupTestService(t, db)
	ctx := context.Background()

	bonus, err := svc.AwardWelcomeBonus(ctx, 1002, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.Equal(t, "300", bonus.WageringRequired.String())

	// Partial wager: no credit, still active.
	require.NoError(t, svc.RecordWager(ctx, 1002, decimal.NewFromInt(100)))
	assert.Empty(t, crediter.calls)
	active, err := svc.GetActiveBonus(ctx, 1002)
	require.NoError(t, err)
	require.NotNil(t, active)

	// Completing wager: exactly one credit, then completed.
	require.NoError(t, svc.RecordWager(ctx, 1002, decimal.NewFromInt(200)))
	require.Len(t, crediter.calls, 1)
	assert.Equal(t, bonus.ID.String(), crediter.calls[0].ReferenceID)
	assert.Equal(t, "bonus-wagering-"+bonus.ID.String(), crediter.calls[0].IdempotencyKey)

	completed, err := svc.GetBonus(ctx, 1002, bonus.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusCompleted, completed.Status)

	// Post-completion wager: no-op, no double credit.
	require.NoError(t, svc.RecordWager(ctx, 1002, decimal.NewFromInt(50)))
	assert.Len(t, crediter.calls, 1, "must never double-credit")
}

func TestBonusFlow_ActivateCancelTransitions(t *testing.T) {
	db := setupTestDB(t)
	svc, _ := setupTestService(t, db)
	ctx := context.Background()

	pending := &domain.Bonus{
		ID:               uuid.New(),
		UserID:           1003,
		Type:             domain.BonusTypeReload,
		Status:           domain.BonusStatusPending,
		BonusAmount:      decimal.NewFromInt(25),
		Currency:         "USD",
		WageringRequired: decimal.NewFromInt(750),
	}
	require.NoError(t, db.WithContext(ctx).Create(pending).Error)

	activated, err := svc.ActivateBonus(ctx, 1003, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusActive, activated.Status)

	cancelled, err := svc.CancelBonus(ctx, 1003, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusCancelled, cancelled.Status)

	_, err = svc.ActivateBonus(ctx, 1003, pending.ID)
	assert.ErrorIs(t, err, domain.ErrBonusNotActive, "cancelled bonus must not reactivate")
}
