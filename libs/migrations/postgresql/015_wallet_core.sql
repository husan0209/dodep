-- Migration 015: Wallet Core (centralised from services/rust/wallet-core/migrations)
-- Double-entry bookkeeping, optimistic locking, outbox pattern
-- Phase 0.9: per-service → libs/migrations centralisation
-- NOTE: If shared DB with 003_wallets.sql, reconcile schemas first (separate DB recommended).

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- ENUMS
-- ============================================================================
CREATE TYPE wallet_type AS ENUM ('main', 'bonus', 'free_spins', 'cashback');
CREATE TYPE transaction_type AS ENUM (
    'deposit', 'withdrawal', 'bet_place', 'bet_win', 'bet_refund',
    'bonus_credit', 'bonus_debit', 'transfer', 'adjustment', 'fee'
);
CREATE TYPE transaction_status AS ENUM ('pending', 'processing', 'completed', 'failed', 'cancelled');
CREATE TYPE ledger_entry_type AS ENUM ('debit', 'credit');
CREATE TYPE account_type AS ENUM (
    'user_wallet', 'house_revenue', 'house_hold',
    'payment_gateway_transit', 'tax_reserve', 'bonus_pool'
);

-- ============================================================================
-- WALLETS
-- ============================================================================
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    wallet_type wallet_type NOT NULL,
    currency CHAR(3) NOT NULL,
    balance_available NUMERIC(19, 4) NOT NULL DEFAULT 0,
    balance_locked NUMERIC(19, 4) NOT NULL DEFAULT 0,
    balance_bonus NUMERIC(19, 4) NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallets_user_id_type_unique UNIQUE (user_id, wallet_type),
    CONSTRAINT wallets_balance_available_non_negative CHECK (balance_available >= 0),
    CONSTRAINT wallets_balance_locked_non_negative CHECK (balance_locked >= 0),
    CONSTRAINT wallets_balance_bonus_non_negative CHECK (balance_bonus >= 0),
    CONSTRAINT wallets_available_ge_locked CHECK (balance_available >= balance_locked)
);

CREATE INDEX idx_wallets_user_id ON wallets(user_id);
CREATE INDEX idx_wallets_user_active ON wallets(user_id, is_active);
CREATE INDEX idx_wallets_type ON wallets(wallet_type);

-- ============================================================================
-- TRANSACTIONS
-- ============================================================================
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    wallet_type wallet_type NOT NULL,
    transaction_type transaction_type NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    currency CHAR(3) NOT NULL,
    status transaction_status NOT NULL DEFAULT 'pending',
    reference_id UUID,
    reference_type VARCHAR(50),
    idempotency_key VARCHAR(255),
    description TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT transactions_amount_positive CHECK (amount > 0),
    CONSTRAINT transactions_idempotency_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_user_created ON transactions(user_id, created_at DESC);
CREATE INDEX idx_transactions_wallet_id ON transactions(wallet_id);
CREATE INDEX idx_transactions_reference ON transactions(reference_id, reference_type);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX idx_transactions_type ON transactions(transaction_type);

-- ============================================================================
-- LEDGER ENTRIES (Double-Entry Bookkeeping)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_type account_type NOT NULL,
    account_id VARCHAR(255) NOT NULL,
    entry_type ledger_entry_type NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    currency CHAR(3) NOT NULL,
    balance_after NUMERIC(19, 4),
    reference_type VARCHAR(50),
    reference_id UUID,
    idempotency_key VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ledger_entries_amount_positive CHECK (amount > 0),
    CONSTRAINT ledger_entries_idempotency_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_ledger_entries_transaction_id ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_type, account_id);
CREATE INDEX idx_ledger_entries_created ON ledger_entries(created_at DESC);
CREATE INDEX idx_ledger_entries_idempotency ON ledger_entries(idempotency_key);

-- ============================================================================
-- FUND LOCKS
-- ============================================================================
CREATE TABLE IF NOT EXISTS fund_locks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    reference_id UUID NOT NULL,
    reference_type VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    CONSTRAINT fund_locks_amount_positive CHECK (amount > 0),
    CONSTRAINT fund_locks_reference_unique UNIQUE (reference_id)
);

CREATE INDEX idx_fund_locks_wallet_id ON fund_locks(wallet_id);
CREATE INDEX idx_fund_locks_user_id ON fund_locks(user_id);
CREATE INDEX idx_fund_locks_reference ON fund_locks(reference_id);
CREATE INDEX idx_fund_locks_active ON fund_locks(is_active) WHERE is_active = TRUE;

-- ============================================================================
-- OUTBOX (Transactional Event Publishing)
-- ============================================================================
CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(100) NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    retries INT NOT NULL DEFAULT 0,
    last_error TEXT,
    CONSTRAINT outbox_not_sent CHECK (sent_at IS NULL OR retries > 0)
);

CREATE INDEX idx_outbox_not_sent ON outbox(sent_at) WHERE sent_at IS NULL;
CREATE INDEX idx_outbox_created ON outbox(created_at);

-- ============================================================================
-- RECONCILIATION VIEWS
-- ============================================================================
CREATE OR REPLACE VIEW wallet_reconciliation AS
SELECT
    w.id AS wallet_id,
    w.user_id,
    w.wallet_type,
    w.currency,
    (w.balance_available + w.balance_locked + w.balance_bonus) AS actual_balance,
    COALESCE(
        SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END) -
        SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END),
        0
    ) AS expected_balance,
    ABS(
        (w.balance_available + w.balance_locked + w.balance_bonus) -
        COALESCE(
            SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END) -
            SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END),
            0
        )
    ) AS discrepancy
FROM wallets w
LEFT JOIN ledger_entries le ON le.account_id = 'user_wallet:' || w.user_id::text || ':' || w.wallet_type::text || ':' || w.currency
GROUP BY w.id, w.user_id, w.wallet_type, w.currency;

CREATE OR REPLACE VIEW wallet_reconciliation_alerts AS
SELECT *
FROM wallet_reconciliation
WHERE discrepancy > 0.01;

-- ============================================================================
-- TRIGGERS
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
