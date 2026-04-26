-- Withdrawal status enum
CREATE TYPE withdrawal_status AS ENUM (
    'processing', 'sending', 'sent', 'finished', 'failed', 'cancelled'
);

-- Withdrawals table
CREATE TABLE withdrawals (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL UNIQUE,
    user_id         BIGINT NOT NULL,
    withdrawal_id   VARCHAR(100) NOT NULL UNIQUE,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,

    -- Amounts
    amount          NUMERIC(18, 8) NOT NULL,
    fiat_amount     NUMERIC(18, 2) NOT NULL,
    fiat_currency   CHAR(3) NOT NULL DEFAULT 'USD',

    -- Crypto details
    crypto_currency VARCHAR(20) NOT NULL,
    address         VARCHAR(255) NOT NULL,

    -- Status
    status          withdrawal_status NOT NULL DEFAULT 'processing',

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,

    -- Metadata
    ip_address      INET,
    user_agent      VARCHAR(500)
);

-- Indexes
CREATE INDEX idx_withdrawals_user_id_created ON withdrawals(user_id, created_at DESC);
CREATE INDEX idx_withdrawals_withdrawal_id ON withdrawals(withdrawal_id);
CREATE INDEX idx_withdrawals_idempotency_key ON withdrawals(idempotency_key);
CREATE INDEX idx_withdrawals_status ON withdrawals(status)
    WHERE status IN ('processing', 'sending');

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_withdrawals_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_withdrawals_updated_at
    BEFORE UPDATE ON withdrawals
    FOR EACH ROW
    EXECUTE FUNCTION update_withdrawals_updated_at();
