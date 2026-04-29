-- Migration 011: Payment Management Extensions
-- Phase 5: Chargebacks, Balance Sheet, Crypto Wallets, P2P, Reconciliation

-- ============================================================
-- Chargebacks
-- ============================================================
CREATE TABLE IF NOT EXISTS chargebacks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    amount          NUMERIC(18,2) NOT NULL,
    currency        CHAR(3) NOT NULL,
    gateway         TEXT NOT NULL,
    gateway_cb_id   TEXT,
    reason_code     TEXT,
    reason_text     TEXT,
    status          TEXT NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'under_review', 'accepted', 'fighting', 'won', 'lost')),
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deadline_at     TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    assigned_to     UUID REFERENCES admin_users(id),
    fight_evidence  JSONB DEFAULT '[]',
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_chargebacks_player ON chargebacks(player_id);
CREATE INDEX idx_chargebacks_status ON chargebacks(status) WHERE status NOT IN ('won', 'lost');
CREATE INDEX idx_chargebacks_assigned ON chargebacks(assigned_to) WHERE status = 'under_review';

-- ============================================================
-- Balance Sheet Snapshots
-- ============================================================
CREATE TABLE IF NOT EXISTS balance_sheet_snapshots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    as_of               TIMESTAMPTZ NOT NULL DEFAULT now(),
    player_balances     NUMERIC(18,2) NOT NULL DEFAULT 0,
    bonus_balances      NUMERIC(18,2) NOT NULL DEFAULT 0,
    pending_withdrawals NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_liabilities   NUMERIC(18,2) NOT NULL DEFAULT 0,
    gateway_balances    JSONB DEFAULT '[]',
    crypto_hot_balances JSONB DEFAULT '[]',
    crypto_cold_balances JSONB DEFAULT '[]',
    bank_account_balance NUMERIC(18,2) DEFAULT 0,
    total_assets        NUMERIC(18,2) NOT NULL DEFAULT 0,
    coverage_ratio      NUMERIC(5,2),
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_balance_sheet_as_of ON balance_sheet_snapshots(as_of DESC);

-- ============================================================
-- Crypto Wallets (platform managed)
-- ============================================================
CREATE TABLE IF NOT EXISTS crypto_wallets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coin                TEXT NOT NULL,
    wallet_type         TEXT NOT NULL CHECK (wallet_type IN ('hot', 'cold')),
    balance             NUMERIC(18,8) NOT NULL DEFAULT 0,
    address             TEXT NOT NULL,
    daily_withdrawal_avg NUMERIC(18,8) DEFAULT 0,
    threshold_amount    NUMERIC(18,8) DEFAULT 0,
    is_low              BOOLEAN DEFAULT false,
    pending_deposits    INT DEFAULT 0,
    pending_withdrawals INT DEFAULT 0,
    last_updated        TIMESTAMPTZ DEFAULT now(),
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX idx_crypto_wallets_coin_type ON crypto_wallets(coin, wallet_type);

-- ============================================================
-- P2P Transactions (gray market deposits/withdrawals)
-- ============================================================
CREATE TABLE IF NOT EXISTS p2p_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL CHECK (type IN ('deposit', 'withdrawal')),
    amount          NUMERIC(18,2) NOT NULL,
    currency        CHAR(3) NOT NULL,
    method          TEXT NOT NULL CHECK (method IN ('papara', 'bank_transfer', 'crypto_p2p')),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'rejected', 'sent', 'completed')),
    receipt_url     TEXT,
    confirmed_by    UUID REFERENCES admin_users(id),
    confirmed_at    TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_p2p_status ON p2p_transactions(status) WHERE status IN ('pending', 'sent');
CREATE INDEX idx_p2p_player ON p2p_transactions(player_id);
CREATE INDEX idx_p2p_created ON p2p_transactions(created_at DESC);

-- ============================================================
-- Payment Gateway Configuration (per country)
-- ============================================================
CREATE TABLE IF NOT EXISTS payment_method_configs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code        CHAR(2) NOT NULL,
    method              TEXT NOT NULL,
    gateway             TEXT NOT NULL,
    enabled_deposit     BOOLEAN DEFAULT true,
    enabled_withdrawal  BOOLEAN DEFAULT true,
    min_deposit         NUMERIC(18,2) DEFAULT 0,
    max_deposit         NUMERIC(18,2),
    min_withdrawal      NUMERIC(18,2) DEFAULT 0,
    max_withdrawal      NUMERIC(18,2),
    fee_percent         NUMERIC(5,2) DEFAULT 0,
    fee_fixed           NUMERIC(18,2) DEFAULT 0,
    priority            INT DEFAULT 0,
    temporary_disabled_until TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now(),
    updated_at          TIMESTAMPTZ DEFAULT now(),
    UNIQUE(country_code, method, gateway)
);

CREATE INDEX idx_payment_configs_country ON payment_method_configs(country_code);

-- ============================================================
-- Daily Reconciliation Records
-- ============================================================
CREATE TABLE IF NOT EXISTS reconciliation_records (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recon_date          DATE NOT NULL,
    gateway             TEXT NOT NULL,
    expected_balance    NUMERIC(18,2) NOT NULL,
    actual_balance      NUMERIC(18,2) NOT NULL,
    difference          NUMERIC(18,2) NOT NULL,
    pending_tx_count    INT DEFAULT 0,
    failed_callbacks    INT DEFAULT 0,
    chargeback_amount   NUMERIC(18,2) DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'resolved', 'investigating')),
    notes               TEXT,
    created_at          TIMESTAMPTZ DEFAULT now(),
    updated_at          TIMESTAMPTZ DEFAULT now(),
    UNIQUE(recon_date, gateway)
);

CREATE INDEX idx_recon_date ON reconciliation_records(recon_date DESC);

-- ============================================================
-- Triggers
-- ============================================================
CREATE TRIGGER trg_chargebacks_updated
    BEFORE UPDATE ON chargebacks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_p2p_transactions_updated
    BEFORE UPDATE ON p2p_transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_payment_configs_updated
    BEFORE UPDATE ON payment_method_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_reconciliation_updated
    BEFORE UPDATE ON reconciliation_records
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
