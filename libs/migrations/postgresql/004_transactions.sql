-- Migration: 004_transactions.sql
-- Description: Wallet transactions — append-only, partitioned by month
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- WALLET TRANSACTIONS TABLE (partitioned)
-- ============================================================

-- Parent table — PARTITION BY RANGE
CREATE TABLE wallet_transactions (
    id              BIGSERIAL,
    user_id         BIGINT NOT NULL,
    wallet_id       BIGINT NOT NULL,
    type            tx_type_enum NOT NULL,
    amount          NUMERIC(18,8) NOT NULL,
    balance_before  NUMERIC(18,8) NOT NULL,
    balance_after   NUMERIC(18,8) NOT NULL,
    reference_type  VARCHAR(50),       -- 'bet', 'payment', 'bonus', 'adjustment'
    reference_id    BIGINT,
    idempotency_key UUID UNIQUE NOT NULL,
    status          tx_status_enum NOT NULL DEFAULT 'completed',
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, id, created_at)
) PARTITION BY RANGE (created_at);

-- ============================================================
-- PARTITIONS (initial 3 months + current)
-- ============================================================

CREATE TABLE wallet_transactions_2026_01
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE wallet_transactions_2026_02
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE wallet_transactions_2026_03
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE wallet_transactions_2026_04
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- ============================================================
-- INDEXES
-- ============================================================

-- Fast lookup by wallet
CREATE INDEX idx_wtx_wallet ON wallet_transactions (wallet_id, created_at DESC);

-- Fast lookup by reference
CREATE INDEX idx_wtx_reference ON wallet_transactions (reference_type, reference_id)
    WHERE reference_type IS NOT NULL;

-- Idempotency lookup (already UNIQUE, but explicit index)
CREATE INDEX idx_wtx_idempotency ON wallet_transactions (idempotency_key);

-- BRIN index for time-series scans
CREATE INDEX idx_wtx_created_brin ON wallet_transactions USING BRIN (created_at);

-- ============================================================
-- pg_partman: auto-create future partitions
-- ============================================================

-- Will be configured after deployment:
-- SELECT partman.create_parent(
--     p_parent_table := 'public.wallet_transactions',
--     p_control := 'created_at',
--     p_type := 'native',
--     p_interval := 'monthly',
--     p_premake := 3
-- );

-- ============================================================
-- LEDGER ENTRIES (double-entry bookkeeping)
-- ============================================================

CREATE TABLE ledger_entries (
    id              BIGSERIAL,
    transaction_id  BIGINT NOT NULL,
    account_type    VARCHAR(50) NOT NULL,  -- 'user_wallet', 'house', 'bonus_pool', etc.
    account_id      BIGINT,                -- wallet_id for user_wallet, null for house
    direction       CHAR(3) NOT NULL CHECK (direction IN ('DR', 'CR')),
    amount          NUMERIC(18,8) NOT NULL,
    currency_code   CHAR(3) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE ledger_entries_2026_03
    PARTITION OF ledger_entries
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE ledger_entries_2026_04
    PARTITION OF ledger_entries
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE INDEX idx_ledger_tx ON ledger_entries (transaction_id);
CREATE INDEX idx_ledger_account ON ledger_entries (account_type, account_id);
