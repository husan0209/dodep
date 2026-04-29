-- Migration 014: Payments Core (centralised from services/go/payment/migrations)
-- Phase 0.9: per-service → libs/migrations centralisation

-- ============================================================
-- ENUMS
-- ============================================================
CREATE TYPE payment_status AS ENUM (
    'pending', 'waiting', 'confirming', 'confirmed',
    'sending', 'partially_paid', 'finished', 'failed',
    'expired', 'refunded'
);

CREATE TYPE withdrawal_status AS ENUM (
    'processing', 'sending', 'sent', 'finished', 'failed', 'cancelled'
);

-- ============================================================
-- PAYMENTS
-- ============================================================
CREATE TABLE IF NOT EXISTS payments (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL UNIQUE,
    user_id         BIGINT NOT NULL,
    payment_id      VARCHAR(100) NOT NULL UNIQUE,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    requested_amount NUMERIC(18, 8) NOT NULL,
    actual_amount    NUMERIC(18, 8),
    fiat_amount      NUMERIC(18, 2) NOT NULL,
    fiat_currency    CHAR(3) NOT NULL DEFAULT 'USD',
    crypto_currency  VARCHAR(20) NOT NULL,
    pay_address      VARCHAR(255) NOT NULL,
    pay_amount       NUMERIC(18, 8),
    status           payment_status NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    ip_address       INET,
    user_agent       VARCHAR(500)
);

CREATE INDEX idx_payments_user_id_created ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_payment_id ON payments(payment_id);
CREATE INDEX idx_payments_idempotency_key ON payments(idempotency_key);
CREATE INDEX idx_payments_status_created ON payments(status, created_at)
    WHERE status NOT IN ('finished', 'failed', 'expired', 'refunded');
CREATE INDEX idx_payments_active ON payments(user_id, created_at DESC)
    WHERE status IN ('pending', 'waiting', 'confirming', 'confirmed', 'sending');

-- ============================================================
-- WITHDRAWALS
-- ============================================================
CREATE TABLE IF NOT EXISTS withdrawals (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL UNIQUE,
    user_id         BIGINT NOT NULL,
    withdrawal_id   VARCHAR(100) NOT NULL UNIQUE,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    amount          NUMERIC(18, 8) NOT NULL,
    fiat_amount     NUMERIC(18, 2) NOT NULL,
    fiat_currency   CHAR(3) NOT NULL DEFAULT 'USD',
    crypto_currency VARCHAR(20) NOT NULL,
    address         VARCHAR(255) NOT NULL,
    status          withdrawal_status NOT NULL DEFAULT 'processing',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    ip_address      INET,
    user_agent      VARCHAR(500)
);

CREATE INDEX idx_withdrawals_user_id_created ON withdrawals(user_id, created_at DESC);
CREATE INDEX idx_withdrawals_withdrawal_id ON withdrawals(withdrawal_id);
CREATE INDEX idx_withdrawals_idempotency_key ON withdrawals(idempotency_key);
CREATE INDEX idx_withdrawals_status ON withdrawals(status)
    WHERE status IN ('processing', 'sending');

-- ============================================================
-- TRIGGERS
-- ============================================================
CREATE OR REPLACE FUNCTION update_payments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_payments_updated_at();

CREATE OR REPLACE FUNCTION update_withdrawals_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_withdrawals_updated_at
    BEFORE UPDATE ON withdrawals
    FOR EACH ROW EXECUTE FUNCTION update_withdrawals_updated_at();
