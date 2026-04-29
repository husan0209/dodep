-- Migration 010: Seed reference data
-- Currencies, countries, KYC limits, game configs
-- Idempotent: uses INSERT ... ON CONFLICT DO NOTHING

BEGIN;

-- ── Currencies ───────────────────────────────────────────────────────
INSERT INTO currencies (code, name, symbol, type, decimals, min_deposit, min_withdrawal, is_active)
VALUES
  ('USD',  'US Dollar',          '$',    'fiat',   2, 10.00,  20.00,  true),
  ('EUR',  'Euro',               '€',    'fiat',   2, 10.00,  20.00,  true),
  ('GBP',  'British Pound',      '£',    'fiat',   2, 10.00,  20.00,  true),
  ('BTC',  'Bitcoin',            '₿',    'crypto', 8, 0.0001, 0.0002, true),
  ('ETH',  'Ethereum',           'Ξ',    'crypto', 8, 0.001,  0.002,  true),
  ('USDT', 'Tether (TRC-20)',    '₮',    'crypto', 6, 10.00,  20.00,  true),
  ('USDC', 'USD Coin',           '$',    'crypto', 6, 10.00,  20.00,  true),
  ('LTC',  'Litecoin',           'Ł',    'crypto', 8, 0.01,   0.02,   true),
  ('BCH',  'Bitcoin Cash',       'BCH',  'crypto', 8, 0.01,   0.02,   true),
  ('XRP',  'Ripple',             'XRP',  'crypto', 6, 10.00,  20.00,  false)
ON CONFLICT (code) DO UPDATE SET
  name             = EXCLUDED.name,
  min_deposit      = EXCLUDED.min_deposit,
  min_withdrawal   = EXCLUDED.min_withdrawal,
  is_active        = EXCLUDED.is_active,
  updated_at       = NOW();

-- ── Countries ────────────────────────────────────────────────────────
-- allowed=true means residents can play; blocked countries are hard-excluded
INSERT INTO countries (code, name, is_allowed, requires_kyc, is_high_risk)
VALUES
  -- Allowed
  ('DE', 'Germany',         true,  true,  false),
  ('AT', 'Austria',         true,  true,  false),
  ('CH', 'Switzerland',     true,  true,  false),
  ('CA', 'Canada',          true,  true,  false),
  ('AU', 'Australia',       true,  true,  false),
  ('NZ', 'New Zealand',     true,  false, false),
  ('BR', 'Brazil',          true,  false, false),
  ('MX', 'Mexico',          true,  false, false),
  ('ZA', 'South Africa',    true,  false, false),
  ('IN', 'India',           true,  false, true),
  ('KZ', 'Kazakhstan',      true,  false, false),
  ('UZ', 'Uzbekistan',      true,  false, false),
  ('TR', 'Turkey',          true,  false, true),
  ('UA', 'Ukraine',         true,  false, true),
  ('PL', 'Poland',          true,  true,  false),
  -- Blocked (regulatory / AML)
  ('US', 'United States',   false, true,  false),
  ('GB', 'United Kingdom',  false, true,  false),
  ('FR', 'France',          false, true,  false),
  ('IT', 'Italy',           false, true,  false),
  ('ES', 'Spain',           false, true,  false),
  ('NL', 'Netherlands',     false, true,  false),
  ('RU', 'Russia',          false, true,  true),
  ('CN', 'China',           false, true,  true),
  ('IL', 'Israel',          false, true,  true),
  ('SY', 'Syria',           false, true,  true),
  ('IR', 'Iran',            false, true,  true),
  ('KP', 'North Korea',     false, true,  true),
  ('CU', 'Cuba',            false, true,  true)
ON CONFLICT (code) DO UPDATE SET
  name        = EXCLUDED.name,
  is_allowed  = EXCLUDED.is_allowed,
  requires_kyc = EXCLUDED.requires_kyc,
  is_high_risk = EXCLUDED.is_high_risk,
  updated_at  = NOW();

-- ── KYC Deposit / Withdrawal Limits per level ─────────────────────
-- kyc_limits (level, transaction_type, daily_limit, weekly_limit, monthly_limit, currency)
INSERT INTO kyc_limits (kyc_level, transaction_type, daily_limit, weekly_limit, monthly_limit, currency_code)
VALUES
  -- Level 0 (unverified): small crypto-only deposits, no withdrawals
  (0, 'deposit',    500.00,   1500.00,   3000.00,  'USD'),
  (0, 'withdrawal',   0.00,      0.00,      0.00,  'USD'),
  -- Level 1 (email + name + DoB verified)
  (1, 'deposit',   2000.00,   5000.00,  10000.00,  'USD'),
  (1, 'withdrawal',  500.00,  2000.00,   5000.00,  'USD'),
  -- Level 2 (document verified)
  (2, 'deposit',  10000.00,  30000.00,  50000.00,  'USD'),
  (2, 'withdrawal', 5000.00, 20000.00,  30000.00,  'USD'),
  -- Level 3 (enhanced due diligence)
  (3, 'deposit',  50000.00, 150000.00, 300000.00,  'USD'),
  (3, 'withdrawal',25000.00,100000.00, 200000.00,  'USD')
ON CONFLICT (kyc_level, transaction_type) DO UPDATE SET
  daily_limit   = EXCLUDED.daily_limit,
  weekly_limit  = EXCLUDED.weekly_limit,
  monthly_limit = EXCLUDED.monthly_limit,
  updated_at    = NOW();

-- ── Casino Providers ─────────────────────────────────────────────────
INSERT INTO game_providers (id, name, slug, logo_url, is_active, supported_currencies, metadata)
VALUES
  ('pragmatic',  'Pragmatic Play',      'pragmatic',  '/providers/pragmatic.png',  false, ARRAY['USD','EUR','GBP','BTC','ETH','USDT'], '{}'),
  ('pgsoft',     'PG Soft',             'pgsoft',     '/providers/pgsoft.png',     false, ARRAY['USD','EUR','GBP','USDT'],              '{}'),
  ('amatic',     'Amatic Industries',   'amatic',     '/providers/amatic.png',     false, ARRAY['USD','EUR'],                           '{}'),
  ('amusnet',    'Amusnet (EGT)',        'amusnet',    '/providers/amusnet.png',    false, ARRAY['USD','EUR'],                           '{}')
ON CONFLICT (id) DO UPDATE SET
  name                 = EXCLUDED.name,
  supported_currencies = EXCLUDED.supported_currencies,
  updated_at           = NOW();

-- ── Responsible Gambling defaults ─────────────────────────────────
-- Default session reality-check interval: 30 minutes
INSERT INTO platform_config (key, value, description)
VALUES
  ('rg.reality_check_interval_min', '30',  'Reality check popup interval in minutes'),
  ('rg.session_max_duration_min',   '240', 'Maximum single session duration in minutes'),
  ('rg.self_exclusion_min_days',    '1',   'Minimum self-exclusion period in days'),
  ('rg.self_exclusion_max_days',    '3650','Maximum self-exclusion period in days (10 years)'),
  ('aml.high_risk_threshold_usd',   '5000','AML manual review threshold in USD (single transaction)'),
  ('aml.monthly_threshold_usd',     '10000','AML monthly cumulative threshold for SAR'),
  ('bonus.welcome_pct',             '100', 'Welcome bonus percentage'),
  ('bonus.welcome_max_usd',         '200', 'Welcome bonus max amount USD'),
  ('bonus.welcome_wagering',        '30',  'Welcome bonus wagering requirement multiplier'),
  ('bonus.welcome_expiry_days',     '30',  'Welcome bonus expiry in days')
ON CONFLICT (key) DO UPDATE SET
  value      = EXCLUDED.value,
  updated_at = NOW();

COMMIT;
