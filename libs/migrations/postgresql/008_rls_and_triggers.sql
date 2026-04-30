-- Migration: 008_rls_and_triggers.sql
-- Description: RLS policies, audit triggers, Citus sharding
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================

-- Enable RLS on user-facing tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE wallet_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE bets ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_limits ENABLE ROW LEVEL SECURITY;

-- Users: can only see own record
CREATE POLICY users_own_data ON users
    FOR ALL
    USING (id = current_setting('app.current_user_id', true)::BIGINT);

-- Wallets: user can see own wallets
CREATE POLICY wallets_own_data ON wallets
    FOR SELECT
    USING (user_id = current_setting('app.current_user_id', true)::BIGINT);

-- Transactions: user can see own transactions
CREATE POLICY wtx_own_data ON wallet_transactions
    FOR SELECT
    USING (user_id = current_setting('app.current_user_id', true)::BIGINT);

-- Bets: user can see own bets
CREATE POLICY bets_own_data ON bets
    FOR SELECT
    USING (user_id = current_setting('app.current_user_id', true)::BIGINT);

-- Preferences: user can modify own preferences
CREATE POLICY prefs_own_data ON user_preferences
    FOR ALL
    USING (user_id = current_setting('app.current_user_id', true)::BIGINT);

-- Limits: user can see own limits
CREATE POLICY limits_own_data ON user_limits
    FOR ALL
    USING (user_id = current_setting('app.current_user_id', true)::BIGINT);

-- Bypass RLS for service accounts
-- Run as superuser or with BYPASSRLS role

-- ============================================================
-- AUDIT TRIGGERS ON CRITICAL TABLES
-- ============================================================

CREATE TRIGGER trg_audit_users
    AFTER INSERT OR UPDATE OR DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();

CREATE TRIGGER trg_audit_wallets
    AFTER INSERT OR UPDATE OR DELETE ON wallets
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();

CREATE TRIGGER trg_audit_bets
    AFTER INSERT OR UPDATE OR DELETE ON bets
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();

CREATE TRIGGER trg_audit_user_limits
    AFTER INSERT OR UPDATE OR DELETE ON user_limits
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();

-- ============================================================
-- CITUS SHARDING
-- ============================================================

-- Distributed tables: shard by user_id for co-location
-- All user-related data on same shard = no network JOINs

SELECT create_distributed_table('users', 'id');
SELECT create_distributed_table('wallets', 'user_id');
SELECT create_distributed_table('wallet_transactions', 'user_id');
SELECT create_distributed_table('bets', 'user_id');
SELECT create_distributed_table('bet_selections', 'user_id');
SELECT create_distributed_table('user_preferences', 'user_id');
SELECT create_distributed_table('user_limits', 'user_id');

-- Reference tables: replicated to ALL nodes (small, read-heavy)
SELECT create_reference_table('currencies');
SELECT create_reference_table('countries');
SELECT create_reference_table('sports');
SELECT create_reference_table('game_configs');

-- House accounts: reference table (one per cluster)
SELECT create_reference_table('house_accounts');

-- Audit log: distributed by entity_id for locality
-- (not by user_id since system events may not have user_id)
-- Keep as local for now — can distribute later if needed

-- ============================================================
-- HELPER: set current user for RLS
-- ============================================================

CREATE OR REPLACE FUNCTION set_current_user(p_user_id BIGINT)
RETURNS VOID AS $$
BEGIN
    PERFORM set_config('app.current_user_id', p_user_id::TEXT, true);
END;
$$ LANGUAGE plpgsql;
