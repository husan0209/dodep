-- Audit log for all payment operations
CREATE TABLE payment_audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,
    operation_type  VARCHAR(50) NOT NULL,  -- 'deposit', 'withdrawal', 'webhook'
    operation_id    BIGINT,                -- payment_id or withdrawal_id
    reference_type  VARCHAR(50),           -- 'payment', 'withdrawal'
    reference_id    VARCHAR(100),          -- external ID (payment_id, withdrawal_id)

    -- Status change
    previous_status VARCHAR(50),
    new_status      VARCHAR(50),

    -- Amounts
    amount          NUMERIC(18, 8),
    currency        VARCHAR(20),

    -- Request/Response details (sanitized)
    request_details JSONB,
    response_details JSONB,

    -- Error info
    error_code      VARCHAR(50),
    error_message   TEXT,

    -- Tracing
    trace_id        VARCHAR(50),
    correlation_id  VARCHAR(50),

    -- Timestamp
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for audit queries
CREATE INDEX idx_audit_logs_user_id ON payment_audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_logs_operation_type ON payment_audit_logs(operation_type, created_at DESC);
CREATE INDEX idx_audit_logs_reference ON payment_audit_logs(reference_type, reference_id);
CREATE INDEX idx_audit_logs_trace_id ON payment_audit_logs(trace_id);
CREATE INDEX idx_audit_logs_created_at ON payment_audit_logs(created_at DESC);

-- Partial index for errors
CREATE INDEX idx_audit_logs_errors ON payment_audit_logs(error_code, created_at DESC)
    WHERE error_code IS NOT NULL;
