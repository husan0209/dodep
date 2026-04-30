-- Migration: 007_audit.sql
-- Description: Audit log — append-only, immutable
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- AUDIT LOG TABLE (append-only, partitioned)
-- ============================================================

CREATE TABLE audit_log (
    id              BIGSERIAL,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type      actor_type_enum NOT NULL,
    actor_id        BIGINT,
    action          VARCHAR(100) NOT NULL,   -- 'user.block', 'bet.void', 'wallet.credit'
    entity_type     VARCHAR(50) NOT NULL,    -- 'user', 'bet', 'wallet', 'payment'
    entity_id       BIGINT NOT NULL,
    old_value       JSONB,
    new_value       JSONB,
    ip_address      INET,
    user_agent      TEXT,
    metadata        JSONB DEFAULT '{}',
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- ============================================================
-- PARTITIONS
-- ============================================================

CREATE TABLE audit_log_2026_03
    PARTITION OF audit_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE audit_log_2026_04
    PARTITION OF audit_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- ============================================================
-- PROTECTION: No UPDATE or DELETE allowed
-- ============================================================

CREATE RULE audit_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING;
CREATE RULE audit_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING;

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX idx_audit_entity ON audit_log (entity_type, entity_id, timestamp DESC);
CREATE INDEX idx_audit_actor ON audit_log (actor_type, actor_id, timestamp DESC)
    WHERE actor_id IS NOT NULL;
CREATE INDEX idx_audit_action ON audit_log (action, timestamp DESC);

-- BRIN for time-series
CREATE INDEX idx_audit_timestamp_brin ON audit_log USING BRIN (timestamp);

-- ============================================================
-- AUDIT TRIGGER FUNCTION
-- ============================================================

CREATE OR REPLACE FUNCTION audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    v_actor_type actor_type_enum := 'system';
    v_actor_id BIGINT;
    v_old_value JSONB;
    v_new_value JSONB;
BEGIN
    -- Extract actor from session variable if available
    BEGIN
        v_actor_type := current_setting('app.actor_type', true)::actor_type_enum;
        v_actor_id := current_setting('app.actor_id', true)::BIGINT;
    EXCEPTION WHEN OTHERS THEN
        v_actor_type := 'system';
        v_actor_id := NULL;
    END;

    IF TG_OP = 'INSERT' THEN
        v_new_value := to_jsonb(NEW);
    ELSIF TG_OP = 'UPDATE' THEN
        v_old_value := to_jsonb(OLD);
        v_new_value := to_jsonb(NEW);
    ELSIF TG_OP = 'DELETE' THEN
        v_old_value := to_jsonb(OLD);
    END IF;

    INSERT INTO audit_log (
        actor_type, actor_id, action,
        entity_type, entity_id,
        old_value, new_value
    ) VALUES (
        v_actor_type,
        v_actor_id,
        TG_OP || '.' || TG_TABLE_NAME,
        TG_TABLE_NAME,
        COALESCE(NEW.id, OLD.id),
        v_old_value,
        v_new_value
    );

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;
