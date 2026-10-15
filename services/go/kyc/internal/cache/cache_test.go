package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opus-casino/kyc/internal/domain"
)

func TestMemoryStatusCache_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemory()
	_, err := c.GetStatus(ctx, 1)
	require.NoError(t, err)

	s := CachedStatus{Level: domain.LevelIdentity, Status: domain.StatusVerified, UpdatedAt: time.Now()}
	require.NoError(t, c.SetStatus(ctx, 1, s))
	got, err := c.GetStatus(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.LevelIdentity, got.Level)

	require.NoError(t, c.Invalidate(ctx, 1))
	got, err = c.GetStatus(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryStatusCache_WebhookIdempotency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemory()
	seen, err := c.HasProcessedWebhook(ctx, "evt-1")
	require.NoError(t, err)
	assert.False(t, seen)
	require.NoError(t, c.MarkWebhookProcessed(ctx, "evt-1"))
	seen, err = c.HasProcessedWebhook(ctx, "evt-1")
	require.NoError(t, err)
	assert.True(t, seen)
	require.NoError(t, c.Close())
}
