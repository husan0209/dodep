-- Migration: 003_wallets.sql
-- Description: Wallets with optimistic locking and CHECK constraints
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- WALLETS TABLE
-- ============================================================

CREATE TABLE wallets (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    currency_code   CHAR(3) NOT NULL,
    balance         NUMERIC(18,8) NOT NULL DEFAULT 0,
    locked_balance  NUMERIC(18,8) NOT NULL DEFAULT 0,
    bonus_balance   NUMERIC(18,8) NOT NULL DEFAULT 0,
    version         INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_wallets_user_currency UNIQUE (user_id, currency_code),
    CONSTRAINT ck_wallets_balance CHECK (balance >= 0),
    CONSTRAINT ck_wallets_locked CHECK (locked_balance >= 0),
    CONSTRAINT ck_wallets_bonus CHECK (bonus_balance >= 0),
    CONSTRAINT ck_wallets_locked_lte_balance CHECK (locked_balance <= balance)
);

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX idx_wallets_user ON wallets (user_id);

-- ============================================================
-- TRIGGER: auto-update updated_at
-- ============================================================

CREATE TRIGGER trg_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================
-- HOUSE ACCOUNT (platform balance tracking)
-- ============================================================

CREATE TABLE house_accounts (
    id              BIGSERIAL PRIMARY KEY,
    account_type    VARCHAR(50) NOT NULL,  -- 'house', 'bonus_pool', 'tax_reserve', 'revenue'
    currency_code   CHAR(3) NOT NULL,
    balance         NUMERIC(18,8) NOT NULL DEFAULT 0,
    version         INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_house_account_type_currency UNIQUE (account_type, currency_code),
    CONSTRAINT ck_house_balance CHECK (balance >= 0)
);

CREATE TRIGGER trg_house_accounts_updated_at
    BEFORE UPDATE ON house_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Seed house accounts
INSERT INTO house_accounts (account_type, currency_code, balance) VALUES
    ('house', 'USD', 0),
    ('house', 'EUR', 0),
    ('house', 'GBP', 0),
    ('house', 'BTC', 0),
    ('house', 'USDT', 0),
    ('bonus_pool', 'USD', 0),
    ('bonus_pool', 'EUR', 0),
    ('tax_reserve', 'USD', 0),
    ('revenue', 'USD', 0);
