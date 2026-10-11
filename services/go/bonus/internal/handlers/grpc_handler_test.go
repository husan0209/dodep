package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opus-casino/proto/gen/go/bonus/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/testutil"
)

func newFakeRepo() *testutil.FakeBonusRepository { return testutil.NewFakeBonusRepository() }

func testHandler(repo repository.BonusRepository) *BonusGRPCHandler {
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())
	return NewBonusGRPCHandler(svc, zap.NewNop())
}

func uid(v int64) *commonv1.UserId {
	return &commonv1.UserId{Value: itoa(v)}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestGetActiveBonuses_Empty(t *testing.T) {
	h := testHandler(newFakeRepo())
	resp, err := h.GetActiveBonuses(context.Background(), &pb.GetActiveBonusesRequest{UserId: uid(1)})
	require.NoError(t, err)
	assert.Empty(t, resp.Bonuses)
}

func TestGetActiveBonuses_InvalidUserID(t *testing.T) {
	h := testHandler(newFakeRepo())
	_, err := h.GetActiveBonuses(context.Background(), &pb.GetActiveBonusesRequest{UserId: uid(0)})
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetBonus_MapsMoneyAndProgress(t *testing.T) {
	repo := newFakeRepo()
	now := time.Now()
	id := uuid.New()
	repo.Bonuses[id] = &domain.Bonus{
		ID: id, UserID: 4, Type: domain.BonusTypeWelcome, Status: domain.BonusStatusActive,
		BonusAmount: decimal.NewFromInt(100), Currency: "USD",
		WageringRequired: decimal.NewFromInt(3000), WageringCompleted: decimal.NewFromInt(1500),
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	h := testHandler(repo)
	resp, err := h.GetBonus(context.Background(), &pb.GetBonusRequest{UserId: uid(4), BonusId: id.String()})
	require.NoError(t, err)
	assert.Equal(t, "100", resp.Bonus.BonusAmount.Amount)
	assert.Equal(t, "USD", resp.Bonus.BonusAmount.Currency)
	assert.InDelta(t, 50.0, resp.Bonus.Wagering.ProgressPercentage, 0.001)
	assert.False(t, resp.Bonus.Wagering.IsCompleted)
}

func TestGetBonus_NotFound_MapsToNotFound(t *testing.T) {
	h := testHandler(newFakeRepo())
	_, err := h.GetBonus(context.Background(), &pb.GetBonusRequest{UserId: uid(4), BonusId: uuid.NewString()})
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestActivateBonus_Success(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.Bonuses[id] = &domain.Bonus{ID: id, UserID: 6, Type: domain.BonusTypeReload, Status: domain.BonusStatusPending}
	h := testHandler(repo)
	resp, err := h.ActivateBonus(context.Background(), &pb.ActivateBonusRequest{UserId: uid(6), BonusId: id.String()})
	require.NoError(t, err)
	require.Nil(t, resp.Error)
	assert.Equal(t, pb.BonusStatus_BONUS_STATUS_ACTIVE, resp.Bonus.Status)
}
