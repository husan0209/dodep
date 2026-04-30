-- Migration: 009_outbox.sql
-- Description: Transactional outbox pattern for reliable event publishing
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- OUTBOX TABLE (transactional outbox pattern)
-- ============================================================

CREATE TABLE outbox (
    id              BIGSERIAL PRIMARY KEY,
    topic           VARCHAR(100) NOT NULL,
    key             VARCHAR(255) NOT NULL,
    payload         BYTEA NOT NULL,
    headers         JSONB DEFAULT '{}',
    status          outbox_status_enum NOT NULL DEFAULT 'pending',
    retries         INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ
);

-- ============================================================
-- INDEXES
-- ============================================================

-- Worker poll: find pending messages
CREATE INDEX idx_outbox_pending ON outbox (id)
    WHERE status = 'pending';

-- Retry poll: find messages ready for retry
CREATE INDEX idx_outbox_retry ON outbox (next_retry_at)
    WHERE status = 'failed' AND retries < 10;

-- Cleanup: find old sent messages
CREATE INDEX idx_outbox_sent ON outbox (sent_at)
    WHERE status = 'sent';

-- ============================================================
-- HELPER: write to outbox within transaction
-- ============================================================

CREATE OR REPLACE FUNCTION write_outbox(
    p_topic VARCHAR(100),
    p_key VARCHAR(255),
    p_payload BYTEA,
    p_headers JSONB DEFAULT '{}'
) RETURNS BIGINT AS $$
DECLARE
    v_id BIGINT;
BEGIN
    INSERT INTO outbox (topic, key, payload, headers)
    VALUES (p_topic, p_key, p_payload, p_headers)
    RETURNING id INTO v_id;

    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- CLEANUP: delete sent messages older than 7 days
-- Run via pg_partman or scheduled job
-- ============================================================

-- DELETE FROM outbox
-- WHERE status = 'sent' AND sent_at < NOW() - INTERVAL '7 days';
