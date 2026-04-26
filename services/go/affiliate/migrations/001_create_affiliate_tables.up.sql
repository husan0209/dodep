CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_status') THEN
        CREATE TYPE affiliate_status AS ENUM (
            'pending_review',
            'active',
            'suspended',
            'rejected',
            'closed'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_enrollment_status') THEN
        CREATE TYPE affiliate_enrollment_status AS ENUM (
            'pending_review',
            'approved',
            'rejected'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_earning_status') THEN
        CREATE TYPE affiliate_earning_status AS ENUM (
            'accrued',
            'pending',
            'available',
            'paid',
            'reversed'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_payout_status') THEN
        CREATE TYPE affiliate_payout_status AS ENUM (
            'requested',
            'reviewing',
            'approved',
            'processing',
            'paid',
            'rejected',
            'failed'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_payout_method_type') THEN
        CREATE TYPE affiliate_payout_method_type AS ENUM (
            'bank_transfer',
            'crypto',
            'ewallet'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_adjustment_type') THEN
        CREATE TYPE affiliate_adjustment_type AS ENUM (
            'credit',
            'debit'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_approval_mode') THEN
        CREATE TYPE affiliate_approval_mode AS ENUM (
            'manual',
            'automatic'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_payout_schedule') THEN
        CREATE TYPE affiliate_payout_schedule AS ENUM (
            'manual',
            'weekly',
            'monthly'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_fraud_severity') THEN
        CREATE TYPE affiliate_fraud_severity AS ENUM (
            'low',
            'medium',
            'high',
            'critical'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_fraud_status') THEN
        CREATE TYPE affiliate_fraud_status AS ENUM (
            'open',
            'in_review',
            'resolved',
            'dismissed'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_ledger_account_type') THEN
        CREATE TYPE affiliate_ledger_account_type AS ENUM (
            'pending',
            'available',
            'paid',
            'reversed',
            'adjusted'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_ledger_entry_direction') THEN
        CREATE TYPE affiliate_ledger_entry_direction AS ENUM (
            'debit',
            'credit'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS affiliate_enrollment_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    status affiliate_enrollment_status NOT NULL DEFAULT 'pending_review',
    reason TEXT,
    review_notes TEXT,
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS affiliate_commission_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    commission_type VARCHAR(32) NOT NULL DEFAULT 'revshare',
    commission_rate NUMERIC(10,4) NOT NULL,
    hold_period_days INT NOT NULL DEFAULT 14,
    min_payout_amount NUMERIC(18,8) NOT NULL DEFAULT 100.00000000,
    approval_mode affiliate_approval_mode NOT NULL DEFAULT 'manual',
    payout_schedule affiliate_payout_schedule NOT NULL DEFAULT 'monthly',
    negative_carryover_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_plan_rate CHECK (commission_rate >= 0 AND commission_rate <= 1),
    CONSTRAINT chk_affiliate_plan_hold_period CHECK (hold_period_days >= 0),
    CONSTRAINT chk_affiliate_plan_min_payout CHECK (min_payout_amount >= 0)
);

CREATE TABLE IF NOT EXISTS affiliate_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    status affiliate_status NOT NULL DEFAULT 'pending_review',
    affiliate_code VARCHAR(32) NOT NULL,
    commission_plan_id UUID REFERENCES affiliate_commission_plans(id),
    commission_rate NUMERIC(10,4) NOT NULL,
    hold_period_days INT NOT NULL DEFAULT 14,
    min_payout_amount NUMERIC(18,8) NOT NULL DEFAULT 100.00000000,
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    kyc_required BOOLEAN NOT NULL DEFAULT TRUE,
    approval_mode affiliate_approval_mode NOT NULL DEFAULT 'manual',
    payout_schedule affiliate_payout_schedule NOT NULL DEFAULT 'monthly',
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_profiles_user UNIQUE (user_id),
    CONSTRAINT uq_affiliate_profiles_code UNIQUE (affiliate_code),
    CONSTRAINT chk_affiliate_commission_rate CHECK (commission_rate >= 0 AND commission_rate <= 1),
    CONSTRAINT chk_affiliate_hold_period CHECK (hold_period_days >= 0),
    CONSTRAINT chk_affiliate_min_payout CHECK (min_payout_amount >= 0)
);

CREATE TABLE IF NOT EXISTS affiliate_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    campaign_name VARCHAR(100) NOT NULL,
    landing_page TEXT NOT NULL,
    referral_code VARCHAR(32) NOT NULL,
    utm_source VARCHAR(64),
    utm_medium VARCHAR(64),
    utm_campaign VARCHAR(64),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS affiliate_clicks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    link_id UUID REFERENCES affiliate_links(id),
    click_id VARCHAR(64) NOT NULL,
    ip_hash VARCHAR(128) NOT NULL,
    user_agent_hash VARCHAR(128) NOT NULL,
    device_fingerprint VARCHAR(255),
    country_code CHAR(2),
    landing_page TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_clicks_click_id UNIQUE (click_id)
);

CREATE TABLE IF NOT EXISTS affiliate_attributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    referred_user_id BIGINT NOT NULL,
    click_id VARCHAR(64),
    attribution_model VARCHAR(32) NOT NULL DEFAULT 'last_click',
    attributed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ftd_at TIMESTAMPTZ,
    is_ftd_qualified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_attribution_referred_user UNIQUE (referred_user_id)
);

CREATE TABLE IF NOT EXISTS affiliate_daily_aggregates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    report_date DATE NOT NULL,
    clicks BIGINT NOT NULL DEFAULT 0,
    registrations BIGINT NOT NULL DEFAULT 0,
    ftd_count BIGINT NOT NULL DEFAULT 0,
    active_players BIGINT NOT NULL DEFAULT 0,
    ggr_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ngr_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    commission_accrued NUMERIC(18,8) NOT NULL DEFAULT 0,
    commission_reversed NUMERIC(18,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_daily_aggregate UNIQUE (affiliate_id, report_date)
);

CREATE TABLE IF NOT EXISTS affiliate_earnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    referred_user_id BIGINT NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id VARCHAR(64) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    ggr_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ngr_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    commission_rate NUMERIC(10,4) NOT NULL,
    commission_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    status affiliate_earning_status NOT NULL DEFAULT 'accrued',
    hold_until TIMESTAMPTZ NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_earnings_idempotency UNIQUE (idempotency_key),
    CONSTRAINT chk_affiliate_earnings_commission_rate CHECK (commission_rate >= 0 AND commission_rate <= 1)
);

CREATE TABLE IF NOT EXISTS affiliate_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    type affiliate_adjustment_type NOT NULL,
    amount NUMERIC(18,8) NOT NULL,
    reason TEXT NOT NULL,
    reference_id VARCHAR(64),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_adjustments_amount CHECK (amount > 0)
);

CREATE TABLE IF NOT EXISTS affiliate_payout_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    method_type affiliate_payout_method_type NOT NULL,
    details_encrypted TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS affiliate_payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    amount NUMERIC(18,8) NOT NULL,
    currency CHAR(3) NOT NULL,
    method_id UUID NOT NULL REFERENCES affiliate_payout_methods(id),
    idempotency_key VARCHAR(128) NOT NULL,
    status affiliate_payout_status NOT NULL DEFAULT 'requested',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    provider_reference VARCHAR(128),
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_payouts_idempotency UNIQUE (idempotency_key),
    CONSTRAINT chk_affiliate_payout_amount CHECK (amount > 0)
);

CREATE TABLE IF NOT EXISTS affiliate_fraud_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    referred_user_id BIGINT,
    flag_type VARCHAR(64) NOT NULL,
    severity affiliate_fraud_severity NOT NULL,
    status affiliate_fraud_status NOT NULL DEFAULT 'open',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT
);

CREATE TABLE IF NOT EXISTS affiliate_ledger_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    account_type affiliate_ledger_account_type NOT NULL,
    currency CHAR(3) NOT NULL,
    balance NUMERIC(18,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_ledger_account UNIQUE (affiliate_id, account_type, currency)
);

CREATE TABLE IF NOT EXISTS affiliate_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    affiliate_id UUID NOT NULL REFERENCES affiliate_profiles(id),
    account_id UUID NOT NULL REFERENCES affiliate_ledger_accounts(id),
    direction affiliate_ledger_entry_direction NOT NULL,
    amount NUMERIC(18,8) NOT NULL,
    balance_after NUMERIC(18,8) NOT NULL,
    reference_type VARCHAR(32) NOT NULL,
    reference_id VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_ledger_entry UNIQUE (transaction_id, account_id, direction),
    CONSTRAINT chk_affiliate_ledger_entry_amount CHECK (amount > 0)
);

CREATE TABLE IF NOT EXISTS affiliate_outbox (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    topic VARCHAR(128) NOT NULL,
    event_key VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_affiliate_profiles_status ON affiliate_profiles(status, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_affiliate_enrollment_pending_user
    ON affiliate_enrollment_requests(user_id)
    WHERE status = 'pending_review';
CREATE UNIQUE INDEX IF NOT EXISTS idx_affiliate_commission_plans_single_default
    ON affiliate_commission_plans((is_default))
    WHERE is_default = TRUE;
CREATE INDEX IF NOT EXISTS idx_affiliate_attributions_affiliate_id ON affiliate_attributions(affiliate_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_affiliate_status ON affiliate_earnings(affiliate_id, status, hold_until);
CREATE INDEX IF NOT EXISTS idx_affiliate_payouts_affiliate_status ON affiliate_payouts(affiliate_id, status, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_flags_affiliate_status ON affiliate_fraud_flags(affiliate_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_ledger_entries_affiliate_created ON affiliate_ledger_entries(affiliate_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_ledger_entries_idempotency ON affiliate_ledger_entries(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_affiliate_outbox_unpublished ON affiliate_outbox(created_at) WHERE published_at IS NULL;

INSERT INTO affiliate_commission_plans (
    id,
    name,
    commission_type,
    commission_rate,
    hold_period_days,
    min_payout_amount,
    approval_mode,
    payout_schedule,
    negative_carryover_enabled,
    is_default,
    is_active
) VALUES (
    gen_random_uuid(),
    'Default RevShare 20',
    'revshare',
    0.20,
    14,
    100.00000000,
    'manual',
    'monthly',
    FALSE,
    TRUE,
    TRUE
)
ON CONFLICT DO NOTHING;
