-- Migration 013: Admin BFF tables (centralised from services/go/admin-bff/migrations)
-- Domain: admin auth, bonus engine, risk, sportsbook, casino, regulatory, affiliates, settings
-- Phase 0.9: per-service → libs/migrations centralisation

-- ============================================================
-- 1. ADMIN AUTH
-- ============================================================
CREATE TYPE admin_role AS ENUM ('super_admin', 'admin', 'support_manager', 'finance_manager', 'viewer');
CREATE TYPE admin_status AS ENUM ('active', 'suspended', 'inactive');

CREATE TABLE admin_users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    role admin_role NOT NULL DEFAULT 'viewer',
    status admin_status NOT NULL DEFAULT 'active',
    permissions TEXT[] DEFAULT '{}',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id BIGINT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT REFERENCES admin_users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_sessions_admin_id ON admin_sessions(admin_id);
CREATE INDEX idx_admin_sessions_refresh_hash ON admin_sessions(refresh_token_hash);
CREATE INDEX idx_audit_logs_admin_id ON audit_logs(admin_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- ============================================================
-- 2. BONUS ENGINE
-- ============================================================
CREATE TYPE bonus_type AS ENUM ('deposit_match', 'free_spins', 'cashback', 'freebet', 'express_boost', 'tournament');
CREATE TYPE bonus_engine_status AS ENUM ('draft', 'active', 'paused', 'expired');
CREATE TYPE player_bonus_status AS ENUM ('active', 'wagered', 'voided', 'expired', 'completed');
CREATE TYPE bonus_trigger AS ENUM ('on_ftd', 'on_redeposit', 'manual', 'scheduled');
CREATE TYPE bonus_wagering_target AS ENUM ('bonus_only', 'deposit_and_bonus');

CREATE TABLE bonuses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status bonus_engine_status NOT NULL DEFAULT 'draft',
    bonus_type bonus_type NOT NULL,
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    max_uses_global INT NOT NULL DEFAULT 0,
    max_uses_per_player INT NOT NULL DEFAULT 1,
    match_percentage NUMERIC(5,2) DEFAULT 0,
    max_bonus_amount NUMERIC(18,2) DEFAULT 0,
    min_deposit NUMERIC(18,2) DEFAULT 0,
    free_spins_count INT DEFAULT 0,
    free_spins_game VARCHAR(255),
    spin_value NUMERIC(18,2) DEFAULT 0,
    cashback_percentage NUMERIC(5,2) DEFAULT 0,
    cashback_calculation VARCHAR(20) DEFAULT 'net_loss',
    cashback_period VARCHAR(20) DEFAULT 'weekly',
    freebet_amount NUMERIC(18,2) DEFAULT 0,
    freebet_min_odds NUMERIC(5,2) DEFAULT 0,
    freebet_allowed VARCHAR(20) DEFAULT 'both',
    return_stake_on_win BOOLEAN DEFAULT false,
    wagering_multiplier NUMERIC(5,2) NOT NULL DEFAULT 1.0,
    wagering_target bonus_wagering_target NOT NULL DEFAULT 'deposit_and_bonus',
    wagering_timeframe_days INT NOT NULL DEFAULT 7,
    max_bet_while_active NUMERIC(18,2),
    max_win_from_bonus NUMERIC(18,2),
    game_weights JSONB DEFAULT '{"slots":100,"live_casino":0,"table_games":0,"sports":0}'::jsonb,
    excluded_games JSONB DEFAULT '[]'::jsonb,
    sticky BOOLEAN NOT NULL DEFAULT false,
    eligible_countries JSONB DEFAULT '[]'::jsonb,
    excluded_tags JSONB DEFAULT '[]'::jsonb,
    player_groups JSONB DEFAULT '["all"]'::jsonb,
    promo_code VARCHAR(100),
    auto_assign_trigger bonus_trigger NOT NULL DEFAULT 'manual',
    can_combine BOOLEAN NOT NULL DEFAULT false,
    total_issued INT NOT NULL DEFAULT 0,
    total_cost NUMERIC(18,2) NOT NULL DEFAULT 0,
    conversion_rate_pct NUMERIC(5,2) DEFAULT 0,
    created_by VARCHAR(36) NOT NULL,
    created_by_name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bonuses_status ON bonuses(status);
CREATE INDEX idx_bonuses_type ON bonuses(bonus_type);
CREATE INDEX idx_bonuses_valid_to ON bonuses(valid_to);

CREATE TABLE player_bonuses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id BIGINT NOT NULL,
    player_email VARCHAR(255) NOT NULL,
    bonus_id UUID NOT NULL REFERENCES bonuses(id) ON DELETE CASCADE,
    bonus_name VARCHAR(255) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bonus_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    target_wager_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    wagered_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    status player_bonus_status NOT NULL DEFAULT 'active',
    progress_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    voided_by VARCHAR(36),
    void_reason TEXT,
    completed_at TIMESTAMPTZ,
    max_bet_violation BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_player_bonuses_player ON player_bonuses(player_id);
CREATE INDEX idx_player_bonuses_bonus ON player_bonuses(bonus_id);
CREATE INDEX idx_player_bonuses_status ON player_bonuses(status);
CREATE INDEX idx_player_bonuses_expires ON player_bonuses(expires_at) WHERE status = 'active';

CREATE TABLE bonus_activations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bonus_id UUID NOT NULL REFERENCES bonuses(id) ON DELETE CASCADE,
    player_id BIGINT NOT NULL,
    player_email VARCHAR(255) NOT NULL,
    triggered_by VARCHAR(50) NOT NULL DEFAULT 'manual',
    bonus_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bonus_activations_bonus ON bonus_activations(bonus_id);
CREATE INDEX idx_bonus_activations_player ON bonus_activations(player_id);

CREATE TABLE wagering_monitor (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id BIGINT NOT NULL,
    player_email VARCHAR(255) NOT NULL,
    bonus_id UUID NOT NULL REFERENCES bonuses(id) ON DELETE CASCADE,
    bonus_name VARCHAR(255) NOT NULL,
    wagered_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    target_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    progress_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    hours_remaining INT NOT NULL DEFAULT 0,
    abnormally_fast BOOLEAN NOT NULL DEFAULT false,
    near_completion BOOLEAN NOT NULL DEFAULT false,
    expires_soon BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wagering_monitor_player ON wagering_monitor(player_id);
CREATE INDEX idx_wagering_monitor_bonus ON wagering_monitor(bonus_id);
CREATE INDEX idx_wagering_monitor_abnormal ON wagering_monitor(abnormally_fast) WHERE abnormally_fast = true;
CREATE INDEX idx_wagering_monitor_near ON wagering_monitor(near_completion) WHERE near_completion = true;

-- ============================================================
-- 3. RISK
-- ============================================================
CREATE TYPE risk_alert_severity AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE risk_alert_status AS ENUM ('open', 'under_review', 'resolved', 'false_positive');
CREATE TYPE risk_alert_category AS ENUM ('velocity', 'threshold', 'pattern', 'mule', 'geo', 'bonus_abuse', 'account_takeover', 'multi_account', 'chargeback', 'collusion');
CREATE TYPE risk_rule_type AS ENUM ('velocity', 'threshold', 'pattern', 'mule', 'geo');
CREATE TYPE risk_rule_action AS ENUM ('flag', 'block', 'review', 'notify');
CREATE TYPE watchlist_list_type AS ENUM ('blacklist', 'greylist', 'whitelist');

CREATE TABLE risk_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    category risk_alert_category NOT NULL DEFAULT 'velocity',
    severity risk_alert_severity NOT NULL DEFAULT 'medium',
    status risk_alert_status NOT NULL DEFAULT 'open',
    risk_score INT NOT NULL DEFAULT 0,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}',
    assigned_to VARCHAR(36),
    resolution TEXT,
    dismiss_reason VARCHAR(100),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_alerts_user_id ON risk_alerts(user_id);
CREATE INDEX idx_risk_alerts_status ON risk_alerts(status);
CREATE INDEX idx_risk_alerts_severity ON risk_alerts(severity);
CREATE INDEX idx_risk_alerts_category ON risk_alerts(category);
CREATE INDEX idx_risk_alerts_created_at ON risk_alerts(created_at DESC);

CREATE TABLE risk_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    rule_type risk_rule_type NOT NULL,
    action risk_rule_action NOT NULL,
    priority INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT true,
    condition JSONB NOT NULL DEFAULT '{}',
    hit_count INT NOT NULL DEFAULT 0,
    created_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_rules_type ON risk_rules(rule_type);
CREATE INDEX idx_risk_rules_enabled ON risk_rules(enabled);

CREATE TABLE risk_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id VARCHAR(36) NOT NULL,
    admin_email VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(36) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    ip_address VARCHAR(45) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_audit_admin ON risk_audit_log(admin_id);
CREATE INDEX idx_risk_audit_resource ON risk_audit_log(resource_type, resource_id);
CREATE INDEX idx_risk_audit_created_at ON risk_audit_log(created_at DESC);

CREATE TABLE risk_watchlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_type watchlist_list_type NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    added_by VARCHAR(36) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_watchlist_type ON risk_watchlist(list_type);
CREATE INDEX idx_risk_watchlist_entity ON risk_watchlist(entity_type, entity_id);

CREATE TABLE risk_rule_whitelist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES risk_rules(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    added_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_whitelist_rule ON risk_rule_whitelist(rule_id);
CREATE INDEX idx_risk_whitelist_user ON risk_rule_whitelist(user_id);

-- ============================================================
-- 4. SPORTSBOOK (admin)
-- ============================================================
CREATE TYPE sports_event_status AS ENUM ('upcoming', 'live', 'completed', 'cancelled', 'postponed');

CREATE TABLE sports_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(100) NOT NULL UNIQUE,
    sport VARCHAR(50) NOT NULL,
    league VARCHAR(100) NOT NULL,
    home_team VARCHAR(255) NOT NULL,
    away_team VARCHAR(255) NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    status sports_event_status NOT NULL DEFAULT 'upcoming',
    score_home INT,
    score_away INT,
    is_suspended BOOLEAN NOT NULL DEFAULT false,
    suspend_reason TEXT,
    custom_margin NUMERIC(5,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sports_events_sport ON sports_events(sport);
CREATE INDEX idx_sports_events_league ON sports_events(league);
CREATE INDEX idx_sports_events_status ON sports_events(status);
CREATE INDEX idx_sports_events_start_time ON sports_events(start_time);
CREATE INDEX idx_sports_events_external_id ON sports_events(external_id);

CREATE TABLE sports_markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES sports_events(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sports_markets_event ON sports_markets(event_id);

CREATE TABLE odds_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id UUID NOT NULL REFERENCES sports_markets(id) ON DELETE CASCADE,
    selection VARCHAR(100) NOT NULL,
    odds NUMERIC(10,4) NOT NULL,
    reason TEXT NOT NULL,
    set_by VARCHAR(36) NOT NULL,
    reverted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_odds_overrides_market ON odds_overrides(market_id);

CREATE TABLE margin_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type VARCHAR(20) NOT NULL,
    scope_id VARCHAR(100),
    sport VARCHAR(50),
    league VARCHAR(100),
    margin_value NUMERIC(5,2) NOT NULL,
    updated_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_margin_scope ON margin_settings(scope_type, scope_id);

CREATE TABLE stake_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type VARCHAR(20) NOT NULL,
    scope_id VARCHAR(100),
    max_stake NUMERIC(18,2) NOT NULL,
    max_win NUMERIC(18,2) NOT NULL,
    max_liability NUMERIC(18,2) NOT NULL,
    updated_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stake_limits_scope ON stake_limits(scope_type, scope_id);

CREATE TABLE liability_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(100) NOT NULL,
    market_id VARCHAR(100) NOT NULL,
    selection VARCHAR(100) NOT NULL,
    total_stake NUMERIC(18,2) NOT NULL,
    total_bets INT NOT NULL DEFAULT 0,
    liability NUMERIC(18,2) NOT NULL,
    limit_value NUMERIC(18,2) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_liability_event ON liability_snapshots(event_id);
CREATE INDEX idx_liability_recorded_at ON liability_snapshots(recorded_at DESC);

-- ============================================================
-- 5. CASINO
-- ============================================================
CREATE TYPE game_category AS ENUM ('slots', 'table', 'live_dealer', 'poker', 'scratch', 'virtual_sports');
CREATE TYPE game_volatility AS ENUM ('low', 'medium', 'high', 'extreme');
CREATE TYPE jackpot_type AS ENUM ('progressive', 'fixed', 'daily_drop', 'hourly_drop');
CREATE TYPE settlement_status AS ENUM ('pending', 'invoiced', 'paid', 'disputed');

CREATE TABLE casino_games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    provider_id VARCHAR(100) NOT NULL,
    provider_name VARCHAR(255) NOT NULL,
    category game_category NOT NULL DEFAULT 'slots',
    tags TEXT[] DEFAULT '{}',
    description TEXT,
    image_url VARCHAR(512),
    thumbnail_url VARCHAR(512),
    supported_currencies TEXT[] DEFAULT '{}',
    min_bet NUMERIC(18,2) NOT NULL DEFAULT 0,
    max_bet NUMERIC(18,2) NOT NULL DEFAULT 0,
    rtp NUMERIC(5,2) NOT NULL DEFAULT 96.0,
    volatility game_volatility NOT NULL DEFAULT 'medium',
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_demo_available BOOLEAN NOT NULL DEFAULT true,
    restricted_countries TEXT[] DEFAULT '{}',
    popularity_score INT NOT NULL DEFAULT 0,
    released_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_casino_games_provider ON casino_games(provider_id);
CREATE INDEX idx_casino_games_category ON casino_games(category);
CREATE INDEX idx_casino_games_active ON casino_games(is_active);

CREATE TABLE casino_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    logo_url VARCHAR(512),
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    games_count INT NOT NULL DEFAULT 0,
    supported_currencies TEXT[] DEFAULT '{}',
    restricted_countries TEXT[] DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_casino_providers_active ON casino_providers(is_active);

CREATE TABLE rtp_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id VARCHAR(100),
    provider_id VARCHAR(100),
    player_group VARCHAR(50) NOT NULL DEFAULT 'default',
    target_rtp NUMERIC(5,2) NOT NULL DEFAULT 96.0,
    override_by VARCHAR(36),
    override_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rtp_game ON rtp_configs(game_id);
CREATE INDEX idx_rtp_provider ON rtp_configs(provider_id);

CREATE TABLE jackpot_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type jackpot_type NOT NULL,
    game_ids TEXT[] DEFAULT '{}',
    seed_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    current_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE provider_settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id VARCHAR(100) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    ggr NUMERIC(18,2) NOT NULL DEFAULT 0,
    revenue_share NUMERIC(18,2) NOT NULL DEFAULT 0,
    status settlement_status NOT NULL DEFAULT 'pending',
    invoice_number VARCHAR(100),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provider_settlements_provider ON provider_settlements(provider_id);
CREATE INDEX idx_provider_settlements_period ON provider_settlements(period_end DESC);

-- Casino enhancements (addendum v1.1)
ALTER TABLE casino_games
    ADD COLUMN display_name VARCHAR(255),
    ADD COLUMN badge VARCHAR(50),
    ADD COLUMN sort_weight INT NOT NULL DEFAULT 0,
    ADD COLUMN country_restrictions TEXT[] DEFAULT '{}';

ALTER TABLE casino_providers
    ADD COLUMN integration_type VARCHAR(50) NOT NULL DEFAULT 'direct',
    ADD COLUMN revenue_share_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN settlement_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    ADD COLUMN api_credentials_encrypted TEXT;

ALTER TABLE rtp_configs
    ADD COLUMN impact_estimate NUMERIC(5,2),
    ADD COLUMN confirmed_by VARCHAR(36),
    ADD COLUMN confirmed_at TIMESTAMPTZ;

ALTER TABLE jackpot_pools
    ADD COLUMN eligible_games TEXT[] DEFAULT '{}',
    ADD COLUMN contribution_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN seed_value NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN daily_drops_config JSONB NOT NULL DEFAULT '{}';

CREATE TABLE rtp_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id VARCHAR(100),
    provider_id VARCHAR(100),
    player_group VARCHAR(50) NOT NULL DEFAULT 'default',
    before_rtp NUMERIC(5,2) NOT NULL,
    after_rtp NUMERIC(5,2) NOT NULL,
    impact_estimate NUMERIC(5,2),
    changed_by VARCHAR(36) NOT NULL,
    confirmed_by VARCHAR(36),
    confirmed_at TIMESTAMPTZ,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rtp_audit_game ON rtp_audit_logs(game_id);
CREATE INDEX idx_rtp_audit_provider ON rtp_audit_logs(provider_id);
CREATE INDEX idx_rtp_audit_created ON rtp_audit_logs(created_at DESC);

-- ============================================================
-- 6. REGULATORY
-- ============================================================
CREATE TYPE regulatory_report_status AS ENUM ('draft', 'generated', 'submitted', 'accepted', 'rejected');
CREATE TYPE sar_status AS ENUM ('draft', 'internal_review', 'submitted', 'acknowledged');
CREATE TYPE complaint_status AS ENUM ('open', 'investigating', 'resolved', 'escalated_to_adr');

CREATE TABLE regulatory_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction VARCHAR(20) NOT NULL,
    report_type VARCHAR(50) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status regulatory_report_status NOT NULL DEFAULT 'draft',
    generated_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    submitted_by VARCHAR(36),
    regulator_ref VARCHAR(100),
    file_url VARCHAR(512),
    data_snapshot JSONB NOT NULL DEFAULT '{}',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_regulatory_reports_status ON regulatory_reports(status);
CREATE INDEX idx_regulatory_reports_jurisdiction ON regulatory_reports(jurisdiction);

CREATE TABLE sar_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction VARCHAR(20) NOT NULL,
    player_id BIGINT NOT NULL,
    trigger_type VARCHAR(20) NOT NULL,
    trigger_alert_id UUID,
    status sar_status NOT NULL DEFAULT 'draft',
    amount_involved NUMERIC(18,2),
    currency CHAR(3),
    description TEXT NOT NULL,
    supporting_data JSONB NOT NULL DEFAULT '{}',
    assigned_to VARCHAR(36),
    internal_notes TEXT,
    submitted_at TIMESTAMPTZ,
    submitted_by VARCHAR(36),
    regulator_ref VARCHAR(100),
    tipping_off_lock BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sar_reports_player ON sar_reports(player_id);
CREATE INDEX idx_sar_reports_status ON sar_reports(status);

CREATE TABLE player_complaints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id BIGINT NOT NULL,
    ticket_id UUID,
    category VARCHAR(30) NOT NULL,
    description TEXT NOT NULL,
    status complaint_status NOT NULL DEFAULT 'open',
    adr_ref VARCHAR(100),
    resolution TEXT,
    resolved_at TIMESTAMPTZ,
    assigned_to VARCHAR(36),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_player_complaints_player ON player_complaints(player_id);
CREATE INDEX idx_player_complaints_status ON player_complaints(status);

CREATE TABLE jurisdiction_ggr (
    period DATE NOT NULL,
    jurisdiction VARCHAR(20) NOT NULL,
    currency CHAR(3) NOT NULL,
    casino_ggr NUMERIC(18,2) NOT NULL DEFAULT 0,
    sports_ggr NUMERIC(18,2) NOT NULL DEFAULT 0,
    live_ggr NUMERIC(18,2) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(5,4),
    tax_amount NUMERIC(18,2),
    PRIMARY KEY (period, jurisdiction, currency)
);

CREATE TABLE tax_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction VARCHAR(20) NOT NULL,
    tax_type VARCHAR(20) NOT NULL,
    tax_base VARCHAR(20) NOT NULL,
    rate NUMERIC(5,4) NOT NULL,
    currency CHAR(3) NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tax_configs_jurisdiction ON tax_configs(jurisdiction);

-- ============================================================
-- 7. AFFILIATES
-- ============================================================
CREATE TYPE affiliate_status AS ENUM ('pending', 'active', 'suspended', 'rejected');
CREATE TYPE affiliate_deal_type AS ENUM ('revenue_share', 'cpa', 'hybrid');
CREATE TYPE payout_status AS ENUM ('pending', 'approved', 'rejected', 'paid');
CREATE TYPE fraud_flag_status AS ENUM ('open', 'under_review', 'resolved', 'dismissed');

CREATE TABLE affiliates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL,
    status affiliate_status NOT NULL DEFAULT 'pending',
    deal_type affiliate_deal_type NOT NULL DEFAULT 'revenue_share',
    revenue_share_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    cpa_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    hold_period_days INT NOT NULL DEFAULT 0,
    min_payout_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    sub_affiliate_enabled BOOLEAN NOT NULL DEFAULT false,
    sub_affiliate_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    postback_configs JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_affiliates_user_id ON affiliates(user_id);
CREATE INDEX idx_affiliates_status ON affiliates(status);

CREATE TABLE affiliate_payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id VARCHAR(36) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status payout_status NOT NULL DEFAULT 'pending',
    provider_reference VARCHAR(255),
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_affiliate_payouts_affiliate ON affiliate_payouts(affiliate_id);
CREATE INDEX idx_affiliate_payouts_status ON affiliate_payouts(status);

CREATE TABLE fraud_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id VARCHAR(36) NOT NULL,
    player_id VARCHAR(36) NOT NULL,
    flag_type VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL,
    status fraud_flag_status NOT NULL DEFAULT 'open',
    resolved_by VARCHAR(36),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_flags_affiliate ON fraud_flags(affiliate_id);
CREATE INDEX idx_fraud_flags_status ON fraud_flags(status);

CREATE TABLE postback_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id VARCHAR(36) NOT NULL,
    event VARCHAR(50) NOT NULL,
    player_id VARCHAR(36),
    url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL,
    attempt INT NOT NULL DEFAULT 0,
    response_body TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_postback_logs_affiliate ON postback_logs(affiliate_id);
CREATE INDEX idx_postback_logs_created ON postback_logs(created_at DESC);

-- ============================================================
-- 8. SETTINGS
-- ============================================================
CREATE TABLE system_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ip_whitelist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address INET NOT NULL,
    label VARCHAR(255),
    is_global BOOLEAN NOT NULL DEFAULT false,
    admin_id BIGINT,
    created_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ip_whitelist_global ON ip_whitelist(is_global);
CREATE INDEX idx_ip_whitelist_admin ON ip_whitelist(admin_id);
