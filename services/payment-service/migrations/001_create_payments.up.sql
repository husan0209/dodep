-- Payment status enum
CREATE TYPE payment_status AS ENUM (
    'pending', 'waiting', 'confirming', 'confirmed',
    'sending', 'partially_paid', 'finished', 'failed',
    'expired', 'refunded'
);

-- Payments table
CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL UNIQUE,
    user_id         BIGINT NOT NULL,
    payment_id      VARCHAR(100) NOT NULL UNIQUE,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,

    -- Amounts (NUMERIC for precision)
    requested_amount NUMERIC(18, 8) NOT NULL,
    actual_amount    NUMERIC(18, 8),
    fiat_amount      NUMERIC(18, 2) NOT NULL,
    fiat_currency    CHAR(3) NOT NULL DEFAULT 'USD',

    -- Crypto details
    crypto_currency  VARCHAR(20) NOT NULL,
    pay_address      VARCHAR(255) NOT NULL,
    pay_amount       NUMERIC(18, 8),

    -- Status
    status           payment_status NOT NULL DEFAULT 'pending',

    -- Timestamps
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,

    -- Metadata
    ip_address       INET,
    user_agent       VARCHAR(500)
);

-- Indexes for common queries
CREATE INDEX idx_payments_user_id_created ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_payment_id ON payments(payment_id);
CREATE INDEX idx_payments_idempotency_key ON payments(idempotency_key);
CREATE INDEX idx_payments_status_created ON payments(status, created_at)
    WHERE status NOT IN ('finished', 'failed', 'expired', 'refunded');

-- Partial index for active payments
CREATE INDEX idx_payments_active ON payments(user_id, created_at DESC)
    WHERE status IN ('pending', 'waiting', 'confirming', 'confirmed', 'sending');

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_payments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION update_payments_updated_at();
