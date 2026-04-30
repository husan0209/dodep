-- Migration: 006_reference_data.sql
-- Description: Reference tables (replicated to all Citus nodes)
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- CURRENCIES
-- ============================================================

CREATE TABLE currencies (
    id              SERIAL PRIMARY KEY,
    code            CHAR(3) UNIQUE NOT NULL,  -- ISO 4217
    name            VARCHAR(100) NOT NULL,
    type            currency_type_enum NOT NULL DEFAULT 'fiat',
    decimal_places  SMALLINT NOT NULL DEFAULT 2,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    min_deposit     NUMERIC(18,8) DEFAULT 0,
    max_deposit     NUMERIC(18,8),
    min_withdrawal  NUMERIC(18,8) DEFAULT 0,
    max_withdrawal  NUMERIC(18,8),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO currencies (code, name, type, decimal_places, min_deposit, max_deposit, min_withdrawal) VALUES
    ('USD', 'US Dollar', 'fiat', 2, 10, 50000, 20),
    ('EUR', 'Euro', 'fiat', 2, 10, 50000, 20),
    ('GBP', 'British Pound', 'fiat', 2, 10, 50000, 20),
    ('BTC', 'Bitcoin', 'crypto', 8, 0.0001, 10, 0.0002),
    ('ETH', 'Ethereum', 'crypto', 8, 0.005, 500, 0.01),
    ('USDT', 'Tether', 'crypto', 6, 10, 100000, 20),
    ('RUB', 'Russian Ruble', 'fiat', 2, 500, 3000000, 1000),
    ('BRL', 'Brazilian Real', 'fiat', 2, 50, 250000, 100),
    ('UAH', 'Ukrainian Hryvnia', 'fiat', 2, 300, 1500000, 500),
    ('INR', 'Indian Rupee', 'fiat', 2, 500, 3000000, 1000);

-- ============================================================
-- COUNTRIES
-- ============================================================

CREATE TABLE countries (
    id              SERIAL PRIMARY KEY,
    code            CHAR(2) UNIQUE NOT NULL,  -- ISO 3166-1 alpha-2
    name            VARCHAR(100) NOT NULL,
    is_blocked      BOOLEAN NOT NULL DEFAULT false,
    default_currency CHAR(3) NOT NULL DEFAULT 'USD',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO countries (code, name, is_blocked, default_currency) VALUES
    ('US', 'United States', true, 'USD'),   -- blocked for gambling
    ('GB', 'United Kingdom', false, 'GBP'),
    ('DE', 'Germany', false, 'EUR'),
    ('FR', 'France', false, 'EUR'),
    ('BR', 'Brazil', false, 'BRL'),
    ('RU', 'Russia', false, 'RUB'),
    ('UA', 'Ukraine', false, 'UAH'),
    ('IN', 'India', false, 'INR'),
    ('AU', 'Australia', false, 'USD'),
    ('CA', 'Canada', false, 'USD'),
    ('NL', 'Netherlands', true, 'EUR'),    -- blocked
    ('KP', 'North Korea', true, 'USD'),    -- blocked
    ('IR', 'Iran', true, 'USD'),           -- blocked
    ('SY', 'Syria', true, 'USD'),          -- blocked
    ('CU', 'Cuba', true, 'USD');           -- blocked

-- ============================================================
-- SPORTS
-- ============================================================

CREATE TABLE sports (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    slug            VARCHAR(50) UNIQUE NOT NULL,
    status          sport_status_enum NOT NULL DEFAULT 'active',
    sort_order      SMALLINT NOT NULL DEFAULT 0,
    icon_url        VARCHAR(500),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO sports (name, slug, sort_order) VALUES
    ('Football', 'football', 1),
    ('Basketball', 'basketball', 2),
    ('Tennis', 'tennis', 3),
    ('Ice Hockey', 'ice-hockey', 4),
    ('Baseball', 'baseball', 5),
    ('MMA', 'mma', 6),
    ('Boxing', 'boxing', 7),
    ('Cricket', 'cricket', 8),
    ('Rugby', 'rugby', 9),
    ('Esports', 'esports', 10),
    ('Table Tennis', 'table-tennis', 11),
    ('Volleyball', 'volleyball', 12),
    ('Handball', 'handball', 13),
    ('Golf', 'golf', 14),
    ('Darts', 'darts', 15);

-- ============================================================
-- GAME CONFIGS (Casino games catalog)
-- ============================================================

CREATE TABLE game_configs (
    id              SERIAL PRIMARY KEY,
    provider        VARCHAR(100) NOT NULL,
    provider_game_id VARCHAR(100) NOT NULL,
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(100) UNIQUE NOT NULL,
    category        VARCHAR(50) NOT NULL,  -- 'slots', 'table', 'live', 'instant'
    subcategory     VARCHAR(50),
    rtp             NUMERIC(5,2),          -- Return to Player %
    volatility      VARCHAR(20),           -- 'low', 'medium', 'high'
    min_bet         NUMERIC(18,8) NOT NULL DEFAULT 0.10,
    max_bet         NUMERIC(18,8) NOT NULL DEFAULT 10000,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    is_mobile       BOOLEAN NOT NULL DEFAULT true,
    thumbnail_url   VARCHAR(500),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_provider UNIQUE (provider, provider_game_id)
);

CREATE INDEX idx_game_configs_category ON game_configs (category, is_active);
CREATE INDEX idx_game_configs_provider ON game_configs (provider, is_active);

CREATE TRIGGER trg_game_configs_updated_at
    BEFORE UPDATE ON game_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
