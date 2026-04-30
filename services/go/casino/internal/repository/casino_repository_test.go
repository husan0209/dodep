package repository

import (
	"context"
	"testing"
	"time"
)

func TestDatabaseMethods_ReturnErrorWhenDatabaseIsNil(t *testing.T) {
	repo := NewCasinoRepository(nil, nil)
	ctx := context.Background()

	session := &GameSession{
		ID:        "s-1",
		UserID:    "u-1",
		GameID:    "g-1",
		StartedAt: time.Now(),
	}

	round := &GameRound{
		ID:        "r-1",
		SessionID: "s-1",
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	if _, _, err := repo.GetGames(ctx, GetGamesOptions{Limit: 10, Offset: 0}); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetGame(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetProviders(ctx, nil); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetProvider(ctx, "p-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.CreateGameSession(ctx, session); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetGameSession(ctx, "s-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.UpdateGameSession(ctx, session); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.EndGameSession(ctx, "s-1", time.Now()); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, _, err := repo.GetGameHistory(ctx, "u-1", nil, nil, 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, _, err := repo.GetRoundHistory(ctx, "s-1", 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.CreateGameRound(ctx, round); err == nil {
		t.Fatalf("expected error when database is nil")
	}
}

func TestCacheMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewCasinoRepository(nil, nil)
	ctx := context.Background()

	game := &Game{
		ID:         "g-1",
		Name:       "Game",
		ReleasedAt: time.Now(),
	}

	if err := repo.CacheGame(ctx, game, time.Minute); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
	if _, err := repo.GetCachedGame(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
	if err := repo.InvalidateGameCache(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}
