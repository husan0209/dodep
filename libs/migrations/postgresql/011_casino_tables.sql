-- Migration 011: Casino service tables
-- game_providers, games, game_sessions, game_rounds, player_mappings, bonuses

BEGIN;

-- ── Game Providers ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS game_providers (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    slug                 TEXT NOT NULL UNIQUE,
    logo_url             TEXT DEFAULT '',
    description          TEXT DEFAULT '',
    is_active            BOOLEAN NOT NULL DEFAULT false,
    games_count          INT NOT NULL DEFAULT 0,
    supported_currencies TEXT[] NOT NULL DEFAULT '{}',
    restricted_countries TEXT[] NOT NULL DEFAULT '{}',
    metadata             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Games ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS games (
    id                   TEXT PRIMARY KEY,
    external_id          TEXT NOT NULL,
    provider_id          TEXT NOT NULL REFERENCES game_providers(id),
    provider_name        TEXT NOT NULL,
    name                 TEXT NOT NULL,
    category             TEXT NOT NULL DEFAULT 'slot',
    sub_category         TEXT DEFAULT '',
    tags                 TEXT[] NOT NULL DEFAULT '{}',
    description          TEXT DEFAULT '',
    image_url            TEXT DEFAULT '',
    thumbnail_url        TEXT DEFAULT '',
    supported_currencies TEXT[] NOT NULL DEFAULT '{}',
    min_bet              NUMERIC(18,8) NOT NULL DEFAULT 0,
    max_bet              NUMERIC(18,8) NOT NULL DEFAULT 0,
    rtp                  FLOAT NOT NULL DEFAULT 0,
    volatility           TEXT DEFAULT 'medium',
    is_active            BOOLEAN NOT NULL DEFAULT true,
    is_demo_available    BOOLEAN NOT NULL DEFAULT false,
    restricted_countries TEXT[] NOT NULL DEFAULT '{}',
    popularity_score     INT NOT NULL DEFAULT 0,
    released_at          TIMESTAMPTZ,
    metadata             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Game features as separate JSONB
ALTER TABLE games ADD COLUMN IF NOT EXISTS features JSONB NOT NULL DEFAULT '{"has_free_spins":false,"has_bonus_buy":false,"has_jackpot":false,"has_multiplayer":false,"has_live_dealer":false,"bonus_features":[]}';

CREATE INDEX IF NOT EXISTS idx_games_provider ON games(provider_id);
CREATE INDEX IF NOT EXISTS idx_games_category ON games(category);
CREATE INDEX IF NOT EXISTS idx_games_active ON games(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_games_popularity ON games(popularity_score DESC);
CREATE INDEX IF NOT EXISTS idx_games_name_search ON games USING GIN(to_tsvector('english', name));

-- ── Game Sessions ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS game_sessions (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL,
    game_id          TEXT NOT NULL REFERENCES games(id),
    provider_id      TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active', -- active, ended
    balance_at_start NUMERIC(18,8) NOT NULL DEFAULT 0,
    device_type      TEXT DEFAULT 'desktop',
    lobby_url        TEXT DEFAULT '',
    launch_url       TEXT NOT NULL,
    token            TEXT NOT NULL UNIQUE,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at         TIMESTAMPTZ,
    metadata         JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON game_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_game ON game_sessions(game_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON game_sessions(status) WHERE status = 'active';

-- ── Game Rounds ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS game_rounds (
    id             TEXT PRIMARY KEY,
    session_id     TEXT NOT NULL REFERENCES game_sessions(id),
    round_id       TEXT NOT NULL,
    bet_amount     NUMERIC(18,8) NOT NULL DEFAULT 0,
    win_amount     NUMERIC(18,8) NOT NULL DEFAULT 0,
    net_result     NUMERIC(18,8) NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'completed',
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    game_state     JSONB NOT NULL DEFAULT '{}',
    metadata       JSONB NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rounds_session_round ON game_rounds(session_id, round_id);

-- ── Player Mappings ───────────────────────────────────────────────────────
-- Maps opaque provider player_id → internal user_id
CREATE TABLE IF NOT EXISTS casino_player_mappings (
    id            BIGSERIAL PRIMARY KEY,
    provider_name TEXT NOT NULL,
    player_id     TEXT NOT NULL,
    user_id       BIGINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_player_mappings_unique
    ON casino_player_mappings(provider_name, player_id);
CREATE INDEX IF NOT EXISTS idx_player_mappings_user
    ON casino_player_mappings(provider_name, user_id);

-- ── Platform Config ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS platform_config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── KYC Limits ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS kyc_limits (
    id               BIGSERIAL PRIMARY KEY,
    kyc_level        INT NOT NULL,
    transaction_type TEXT NOT NULL,  -- 'deposit' | 'withdrawal'
    daily_limit      NUMERIC(18,2) NOT NULL,
    weekly_limit     NUMERIC(18,2) NOT NULL,
    monthly_limit    NUMERIC(18,2) NOT NULL,
    currency_code    TEXT NOT NULL DEFAULT 'USD',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_kyc_limits_unique
    ON kyc_limits(kyc_level, transaction_type);

-- ── Bonuses (for bonus service) ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS bonuses (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             BIGINT NOT NULL,
    type                TEXT NOT NULL,   -- welcome, reload, cashback, free_spins, referral
    status              TEXT NOT NULL DEFAULT 'pending',
    bonus_amount        NUMERIC(18,8) NOT NULL,
    real_amount         NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'USD',
    wagering_required   NUMERIC(18,8) NOT NULL,
    wagering_completed  NUMERIC(18,8) NOT NULL DEFAULT 0,
    wagering_multiplier INT NOT NULL DEFAULT 30,
    expires_at          TIMESTAMPTZ NOT NULL,
    activated_at        TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bonuses_user ON bonuses(user_id);
CREATE INDEX IF NOT EXISTS idx_bonuses_status ON bonuses(user_id, status) WHERE status IN ('pending', 'active');

COMMIT;
