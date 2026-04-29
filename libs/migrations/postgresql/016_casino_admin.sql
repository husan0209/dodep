-- Migration 016: Casino Admin tables — game sessions and rounds
-- Phase 10: read-only monitoring tables for admin-bff

-- ============================================================
-- 1. CASINO GAME SESSIONS
-- ============================================================
CREATE TABLE casino_game_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         BIGINT NOT NULL,
    game_id         UUID NOT NULL REFERENCES casino_games(id) ON DELETE CASCADE,
    provider_id     VARCHAR(100) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    balance_at_start NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
    total_bet       NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_win       NUMERIC(18,2) NOT NULL DEFAULT 0,
    net_result      NUMERIC(18,2) NOT NULL DEFAULT 0,
    rounds_played   INT NOT NULL DEFAULT 0,
    device_type     VARCHAR(20),
    ip_address      INET,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_casino_sessions_user_id     ON casino_game_sessions(user_id);
CREATE INDEX idx_casino_sessions_game_id     ON casino_game_sessions(game_id);
CREATE INDEX idx_casino_sessions_provider_id ON casino_game_sessions(provider_id);
CREATE INDEX idx_casino_sessions_status      ON casino_game_sessions(status) WHERE status IN ('active','paused');
CREATE INDEX idx_casino_sessions_started_at  ON casino_game_sessions(started_at DESC);

-- ============================================================
-- 2. CASINO GAME ROUNDS (bets)
-- ============================================================
CREATE TABLE casino_game_rounds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES casino_game_sessions(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL,
    game_id         UUID NOT NULL REFERENCES casino_games(id) ON DELETE CASCADE,
    provider_id     VARCHAR(100) NOT NULL,
    round_id        VARCHAR(255) NOT NULL,
    bet_amount      NUMERIC(18,2) NOT NULL DEFAULT 0,
    win_amount      NUMERIC(18,2) NOT NULL DEFAULT 0,
    net_result      NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
    status          VARCHAR(20) NOT NULL DEFAULT 'completed',
    details         JSONB NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_casino_rounds_session_id   ON casino_game_rounds(session_id);
CREATE INDEX idx_casino_rounds_user_id       ON casino_game_rounds(user_id);
CREATE INDEX idx_casino_rounds_game_id       ON casino_game_rounds(game_id);
CREATE INDEX idx_casino_rounds_provider_id   ON casino_game_rounds(provider_id);
CREATE INDEX idx_casino_rounds_status        ON casino_game_rounds(status);
CREATE INDEX idx_casino_rounds_created_at    ON casino_game_rounds(created_at DESC);
