package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Nil-client fail-fast: repository must never panic on nil DB,
// following the casino/notification repository pattern.
func TestNilDB_FailFast(t *testing.T) {
	r := NewBonusRepository(nil)
	ctx := context.Background()

	_, err := r.FindWelcome(ctx, 1)
	assert.Error(t, err)

	_, err = r.FindActive(ctx, 1)
	assert.Error(t, err)

	_, err = r.FindByID(ctx, 1, uuid.New())
	assert.Error(t, err)

	assert.Error(t, r.Create(ctx, nil))
	assert.Error(t, r.UpdateWagering(ctx, uuid.New(), nil))
	assert.Error(t, r.MarkExpired(ctx, uuid.New()))

	_, _, err = r.ListByUser(ctx, 1, 20, 0)
	assert.Error(t, err)

	_, err = r.Activate(ctx, uuid.New(), 1)
	assert.Error(t, err)

	_, err = r.Cancel(ctx, uuid.New(), 1)
	assert.Error(t, err)
}

func TestListByUser_NormalizesPaginationOnNilDB(t *testing.T) {
	// Even with invalid pagination, nil DB must fail fast (not panic).
	r := NewBonusRepository(nil)
	_, _, err := r.ListByUser(context.Background(), 1, -5, -10)
	assert.Error(t, err)
}
