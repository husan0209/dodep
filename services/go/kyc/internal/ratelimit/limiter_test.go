package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter_AllowsThenBlocks(t *testing.T) {
	t.Parallel()
	l := New(Config{Name: "test", Limit: 2, Window: time.Minute})
	now := time.Now()
	assert.True(t, l.Allow("k", now).Allowed)
	assert.True(t, l.Allow("k", now).Allowed)
	res := l.Allow("k", now)
	assert.False(t, res.Allowed)
	assert.Equal(t, 2, res.Limit)
	assert.Equal(t, 0, res.Remaining)
}

func TestLimiter_WindowResets(t *testing.T) {
	t.Parallel()
	l := New(Config{Name: "test", Limit: 1, Window: time.Minute})
	now := time.Now()
	assert.True(t, l.Allow("k", now).Allowed)
	assert.False(t, l.Allow("k", now).Allowed)
	assert.True(t, l.Allow("k", now.Add(2*time.Minute)).Allowed)
}

func TestLimiter_KeysIsolated(t *testing.T) {
	t.Parallel()
	l := New(Config{Name: "test", Limit: 1, Window: time.Minute})
	now := time.Now()
	assert.True(t, l.Allow("a", now).Allowed)
	assert.True(t, l.Allow("b", now).Allowed)
	assert.False(t, l.Allow("a", now).Allowed)
}
