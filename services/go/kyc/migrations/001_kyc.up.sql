-- 001_kyc.up.sql — KYC service schema mirror.
--
-- Mirrors repository.AutoMigrate (internal/repository/repository.go) for
-- environments where schema is owned by SQL migrations instead of GORM:
--   kyc_records      <- kycRecordModel
--   kyc_documents    <- documentModel
--   kyc_screenings   <- screeningModel
--   kyc_audit_log    <- auditModel (append-only by contract)
-- Plus the additive Wave2 table:
--   kyc_limit_overrides <- domain.LimitOverride (custom VIP terms)
--
-- Rules: IF NOT EXISTS everywhere (idempotent, safe to re-apply);
-- additive only (no DROP/ALTER of existing tables); money never appears
-- here (limits are TEXT whole-unit strings, NEVER float — NEVER-6);
-- document_number stores AES-256-GCM ciphertext, number_hash the SHA-256
-- search hash for cross-user duplicate detection.

-- ─── Per-user verification state (one row per user) ───
CREATE TABLE IF NOT EXISTS kyc_records (
    user_id          BIGINT        PRIMARY KEY,
    level            INTEGER       NOT NULL DEFAULT 0,
    target_level     INTEGER       NOT NULL DEFAULT 0,
    status           VARCHAR(32)   NOT NULL DEFAULT 'not_started',
    rejection_reason TEXT          NOT NULL DEFAULT '',
    screening_status VARCHAR(32)   NOT NULL DEFAULT 'clear',
    submitted_at     TIMESTAMPTZ   NULL,
    verified_at      TIMESTAMPTZ   NULL,
    last_reviewed_at TIMESTAMPTZ   NULL,
    reviewed_by      VARCHAR(128)  NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ─── Submitted identity documents (FileID -> S3 object, see domain/storage.go) ───
CREATE TABLE IF NOT EXISTS kyc_documents (
    id               UUID          PRIMARY KEY,
    user_id          BIGINT        NOT NULL,
    type             VARCHAR(32)   NOT NULL,
    side             VARCHAR(16)   NOT NULL,
    status           VARCHAR(16)   NOT NULL,
    file_id          VARCHAR(255)  NOT NULL,
    document_number  TEXT          NOT NULL DEFAULT '',
    number_hash      VARCHAR(64)   NOT NULL DEFAULT '',
    issuing_country  CHAR(2)       NOT NULL DEFAULT '',
    expires_at       TIMESTAMPTZ   NULL,
    submitted_at     TIMESTAMPTZ   NOT NULL,
    reviewed_at      TIMESTAMPTZ   NULL,
    reviewed_by      VARCHAR(128)  NOT NULL DEFAULT '',
    rejection_reason TEXT          NOT NULL DEFAULT '',
    required_actions TEXT          NOT NULL DEFAULT '[]',
    extracted_data   TEXT          NOT NULL DEFAULT '{}',
    notes            TEXT          NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_user ON kyc_documents (user_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON kyc_documents (status);
CREATE INDEX IF NOT EXISTS idx_documents_number_hash ON kyc_documents (number_hash);
CREATE INDEX IF NOT EXISTS idx_documents_submitted ON kyc_documents (submitted_at);

-- ─── PEP / sanctions / adverse-media screenings ───
CREATE TABLE IF NOT EXISTS kyc_screenings (
    id          UUID          PRIMARY KEY,
    user_id     BIGINT        NOT NULL,
    provider    VARCHAR(64)   NOT NULL,
    status      VARCHAR(32)   NOT NULL,
    matches     INTEGER       NOT NULL DEFAULT 0,
    details     TEXT          NOT NULL DEFAULT '',
    screened_at TIMESTAMPTZ   NOT NULL,
    resolved_at TIMESTAMPTZ   NULL,
    resolved_by VARCHAR(128)  NOT NULL DEFAULT '',
    resolution  TEXT          NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_screenings_user ON kyc_screenings (user_id);

-- ─── Immutable audit trail (INSERT only; no UPDATE/DELETE by contract) ───
CREATE TABLE IF NOT EXISTS kyc_audit_log (
    id          BIGSERIAL     PRIMARY KEY,
    user_id     BIGINT        NOT NULL,
    actor       VARCHAR(128)  NOT NULL,
    action      VARCHAR(64)   NOT NULL,
    document_id VARCHAR(64)   NOT NULL DEFAULT '',
    old_status  VARCHAR(64)   NOT NULL DEFAULT '',
    new_status  VARCHAR(64)   NOT NULL DEFAULT '',
    reason      TEXT          NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_user ON kyc_audit_log (user_id, created_at);

-- ─── Custom VIP terms (additive, Wave2) ───
-- One row per user; NULL = inherit the base tier from LimitsForLevel.
-- Amounts are TEXT whole-unit strings (NEVER float). Writes are admin-only
-- (service.UpsertLimitOverride enforces the admin role + audit entry).
CREATE TABLE IF NOT EXISTS kyc_limit_overrides (
    user_id            BIGINT        PRIMARY KEY,
    daily_deposit      TEXT          NULL,
    daily_withdrawal   TEXT          NULL,
    monthly_deposit    TEXT          NULL,
    monthly_withdrawal TEXT          NULL,
    max_bet            TEXT          NULL,
    can_trade_crypto   BOOLEAN       NULL,
    can_use_bonus      BOOLEAN       NULL,
    updated_by         VARCHAR(128)  NOT NULL,
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
