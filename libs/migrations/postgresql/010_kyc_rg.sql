-- Migration 010: KYC & Responsible Gambling
-- Phase 4: KYC Queue, Document Expiry, PEP/Sanctions Screening, RG

-- ============================================================
-- KYC Documents (extended from base user schema)
-- ============================================================
CREATE TABLE IF NOT EXISTS kyc_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL CHECK (type IN ('identity', 'address', 'source_of_funds', 'selfie')),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected', 'expired')),
    file_url        TEXT NOT NULL,
    uploaded_at     TIMESTAMPTZ DEFAULT now(),
    reviewed_by     UUID REFERENCES admin_users(id),
    reviewed_at     TIMESTAMPTZ,
    rejection_reason TEXT,
    notes           TEXT,
    expires_at      DATE,
    expiry_reminder_30d_at TIMESTAMPTZ,
    expiry_reminder_7d_at  TIMESTAMPTZ,
    ocr_data        JSONB,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_kyc_documents_player ON kyc_documents(player_id);
CREATE INDEX idx_kyc_documents_status ON kyc_documents(status, created_at DESC);
CREATE INDEX idx_kyc_documents_expires ON kyc_documents(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================
-- KYC Reviews (queue tracking)
-- ============================================================
CREATE TABLE IF NOT EXISTS kyc_reviews (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES kyc_documents(id) ON DELETE CASCADE,
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    priority        TEXT NOT NULL DEFAULT 'low' CHECK (priority IN ('low', 'medium', 'high')),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_review', 'approved', 'rejected', 'resubmission_requested')),
    assigned_to     UUID REFERENCES admin_users(id),
    wait_time_minutes INT DEFAULT 0,
    reviewed_by     UUID REFERENCES admin_users(id),
    reviewed_at     TIMESTAMPTZ,
    decision_reason TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_kyc_reviews_status ON kyc_reviews(status) WHERE status NOT IN ('approved', 'rejected');
CREATE INDEX idx_kyc_reviews_assigned ON kyc_reviews(assigned_to) WHERE status = 'pending';
CREATE INDEX idx_kyc_reviews_player ON kyc_reviews(player_id);

-- ============================================================
-- PEP / Sanctions Screening
-- ============================================================
CREATE TABLE IF NOT EXISTS player_screenings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT NOT NULL CHECK (status IN ('clear', 'pep_match', 'sanctions_hit', 'review_required')),
    matched_lists   TEXT[],
    match_score     NUMERIC(5,2),
    screened_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_screen_at  TIMESTAMPTZ,
    screened_by     TEXT NOT NULL DEFAULT 'auto',
    raw_response    JSONB,
    reviewed_by     UUID REFERENCES admin_users(id),
    reviewed_at     TIMESTAMPTZ,
    review_notes    TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_screenings_player ON player_screenings(player_id, screened_at DESC);
CREATE INDEX idx_screenings_status ON player_screenings(status) WHERE status != 'clear';

-- ============================================================
-- AML / Source of Funds (SOF) Requests
-- ============================================================
CREATE TABLE IF NOT EXISTS sof_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trigger_type    TEXT NOT NULL CHECK (trigger_type IN ('threshold', 'single_tx', 'manual')),
    threshold_amount NUMERIC(18,2),
    period_days     INT,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'submitted', 'under_review', 'approved', 'rejected', 'expired')),
    deadline_at     TIMESTAMPTZ NOT NULL,
    documents       JSONB DEFAULT '[]',
    reviewed_by     UUID REFERENCES admin_users(id),
    reviewed_at     TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_sof_requests_status ON sof_requests(status) WHERE status NOT IN ('approved', 'rejected', 'expired');
CREATE INDEX idx_sof_requests_player ON sof_requests(player_id);

-- ============================================================
-- Responsible Gambling: Player Limits & Self-Exclusions
-- ============================================================
CREATE TABLE IF NOT EXISTS rg_player_limits (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deposit_limit_daily     NUMERIC(18,2),
    deposit_limit_weekly    NUMERIC(18,2),
    deposit_limit_monthly   NUMERIC(18,2),
    loss_limit              NUMERIC(18,2),
    wager_limit_daily       NUMERIC(18,2),
    session_time_limit_minutes INT,
    reality_check_frequency_minutes INT,
    self_exclusion_until    TIMESTAMPTZ,
    cool_off_until          TIMESTAMPTZ,
    created_at              TIMESTAMPTZ DEFAULT now(),
    updated_at              TIMESTAMPTZ DEFAULT now(),
    UNIQUE(player_id)
);

CREATE INDEX idx_rg_limits_self_exclusion ON rg_player_limits(self_exclusion_until) WHERE self_exclusion_until IS NOT NULL;

-- ============================================================
-- RG Alerts (automatic triggers from Risk Engine)
-- ============================================================
CREATE TABLE IF NOT EXISTS rg_alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_type      TEXT NOT NULL CHECK (alert_type IN ('chasing_losses', 'late_night_session', 'rapid_deposit_increase', 'long_session', 'limit_breach')),
    severity        TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    details         JSONB,
    acknowledged_by UUID REFERENCES admin_users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_rg_alerts_player ON rg_alerts(player_id, created_at DESC);
CREATE INDEX idx_rg_alerts_unack ON rg_alerts(created_at DESC) WHERE acknowledged_at IS NULL;

-- ============================================================
-- KYC Team Metrics (daily snapshots)
-- ============================================================
CREATE TABLE IF NOT EXISTS kyc_team_metrics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    officer_id      UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    metric_date     DATE NOT NULL,
    reviewed_count  INT DEFAULT 0,
    avg_review_time_minutes NUMERIC(8,2),
    approve_count   INT DEFAULT 0,
    reject_count    INT DEFAULT 0,
    sla_breach_count INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(officer_id, metric_date)
);

-- ============================================================
-- Update trigger for updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_kyc_documents_updated
    BEFORE UPDATE ON kyc_documents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_kyc_reviews_updated
    BEFORE UPDATE ON kyc_reviews
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_sof_requests_updated
    BEFORE UPDATE ON sof_requests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_rg_player_limits_updated
    BEFORE UPDATE ON rg_player_limits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
