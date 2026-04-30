-- Migration: 002_users.sql
-- Description: Users table with BIGSERIAL for Citus sharding
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- USERS TABLE
-- ============================================================

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    phone           VARCHAR(20) UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,  -- Argon2id
    status          user_status_enum NOT NULL DEFAULT 'pending',
    kyc_level       SMALLINT NOT NULL DEFAULT 0 CHECK (kyc_level BETWEEN 0 AND 4),
    country_code    CHAR(2) NOT NULL,
    currency_code   CHAR(3) NOT NULL,
    language        CHAR(2) NOT NULL DEFAULT 'en',
    timezone        VARCHAR(50) DEFAULT 'UTC',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    phone_verified  BOOLEAN NOT NULL DEFAULT false,
    two_fa_enabled  BOOLEAN NOT NULL DEFAULT false,
    two_fa_secret   VARCHAR(255),
    deleted_at      TIMESTAMPTZ,
    metadata        JSONB DEFAULT '{}'
);

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_phone ON users (phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_users_uuid ON users (uuid);
CREATE INDEX idx_users_status ON users (status);
CREATE INDEX idx_users_country ON users (country_code);
CREATE INDEX idx_users_created ON users (created_at DESC);

-- Partial index: active users only (most queries)
CREATE INDEX idx_users_active ON users (id, email, country_code)
    WHERE status = 'active' AND deleted_at IS NULL;

-- ============================================================
-- TRIGGER: auto-update updated_at
-- ============================================================

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================
-- USER PREFERENCES TABLE
-- ============================================================

CREATE TABLE user_preferences (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    key             VARCHAR(100) NOT NULL,
    value           JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_preferences_user_key UNIQUE (user_id, key)
);

CREATE INDEX idx_user_preferences_user ON user_preferences (user_id);

CREATE TRIGGER trg_user_preferences_updated_at
    BEFORE UPDATE ON user_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================
-- USER LIMITS (Responsible Gambling)
-- ============================================================

CREATE TABLE user_limits (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id),
    deposit_limit_daily     NUMERIC(18,8),
    deposit_limit_weekly    NUMERIC(18,8),
    deposit_limit_monthly   NUMERIC(18,8),
    loss_limit              NUMERIC(18,8),
    session_time_limit      INTERVAL,
    self_exclusion_until    TIMESTAMPTZ,
    cooling_period_ends_at  TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_limits_user UNIQUE (user_id),
    CONSTRAINT ck_limits_positive CHECK (
        deposit_limit_daily IS NULL OR deposit_limit_daily > 0
    )
);

CREATE INDEX idx_user_limits_user ON user_limits (user_id);
CREATE INDEX idx_user_limits_excluded ON user_limits (user_id)
    WHERE self_exclusion_until IS NOT NULL;

CREATE TRIGGER trg_user_limits_updated_at
    BEFORE UPDATE ON user_limits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
