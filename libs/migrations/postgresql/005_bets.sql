-- Migration: 005_bets.sql
-- Description: Bets table — partitioned by day for high volume
-- Author: DATA_ENGINEER
-- Date: 2026-03-24

-- ============================================================
-- BETS TABLE (partitioned by day)
-- ============================================================

CREATE TABLE bets (
    id              BIGSERIAL,
    user_id         BIGINT NOT NULL,
    bet_type        bet_type_enum NOT NULL,
    status          bet_status_enum NOT NULL DEFAULT 'pending',
    stake           NUMERIC(18,8) NOT NULL,
    potential_win   NUMERIC(18,8) NOT NULL,
    actual_win      NUMERIC(18,8) DEFAULT 0,
    odds            NUMERIC(12,6) NOT NULL,
    currency_code   CHAR(3) NOT NULL,
    sport_id        INTEGER,
    event_id        BIGINT,
    market_id       BIGINT,
    selection_id    BIGINT,
    idempotency_key UUID NOT NULL,
    ip_address      INET,
    device_fingerprint VARCHAR(64),
    placed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at      TIMESTAMPTZ,
    metadata        JSONB DEFAULT '{}',
    PRIMARY KEY (user_id, id, placed_at),
    CONSTRAINT ck_bets_stake CHECK (stake > 0),
    CONSTRAINT ck_bets_potential_win CHECK (potential_win >= 0),
    CONSTRAINT ck_bets_odds CHECK (odds > 1.0)
) PARTITION BY RANGE (placed_at);

-- ============================================================
-- PARTITIONS (initial 2 months — daily for high volume)
-- ============================================================

CREATE TABLE bets_2026_03_01 PARTITION OF bets
    FOR VALUES FROM ('2026-03-01') TO ('2026-03-02');
CREATE TABLE bets_2026_03_02 PARTITION OF bets
    FOR VALUES FROM ('2026-03-02') TO ('2026-03-03');
CREATE TABLE bets_2026_03_03 PARTITION OF bets
    FOR VALUES FROM ('2026-03-03') TO ('2026-03-04');
CREATE TABLE bets_2026_03_04 PARTITION OF bets
    FOR VALUES FROM ('2026-03-04') TO ('2026-03-05');
CREATE TABLE bets_2026_03_05 PARTITION OF bets
    FOR VALUES FROM ('2026-03-05') TO ('2026-03-06');
CREATE TABLE bets_2026_03_06 PARTITION OF bets
    FOR VALUES FROM ('2026-03-06') TO ('2026-03-07');
CREATE TABLE bets_2026_03_07 PARTITION OF bets
    FOR VALUES FROM ('2026-03-07') TO ('2026-03-08');
CREATE TABLE bets_2026_03_08 PARTITION OF bets
    FOR VALUES FROM ('2026-03-08') TO ('2026-03-09');
CREATE TABLE bets_2026_03_09 PARTITION OF bets
    FOR VALUES FROM ('2026-03-09') TO ('2026-03-10');
CREATE TABLE bets_2026_03_10 PARTITION OF bets
    FOR VALUES FROM ('2026-03-10') TO ('2026-03-11');
CREATE TABLE bets_2026_03_11 PARTITION OF bets
    FOR VALUES FROM ('2026-03-11') TO ('2026-03-12');
CREATE TABLE bets_2026_03_12 PARTITION OF bets
    FOR VALUES FROM ('2026-03-12') TO ('2026-03-13');
CREATE TABLE bets_2026_03_13 PARTITION OF bets
    FOR VALUES FROM ('2026-03-13') TO ('2026-03-14');
CREATE TABLE bets_2026_03_14 PARTITION OF bets
    FOR VALUES FROM ('2026-03-14') TO ('2026-03-15');
CREATE TABLE bets_2026_03_15 PARTITION OF bets
    FOR VALUES FROM ('2026-03-15') TO ('2026-03-16');
CREATE TABLE bets_2026_03_16 PARTITION OF bets
    FOR VALUES FROM ('2026-03-16') TO ('2026-03-17');
CREATE TABLE bets_2026_03_17 PARTITION OF bets
    FOR VALUES FROM ('2026-03-17') TO ('2026-03-18');
CREATE TABLE bets_2026_03_18 PARTITION OF bets
    FOR VALUES FROM ('2026-03-18') TO ('2026-03-19');
CREATE TABLE bets_2026_03_19 PARTITION OF bets
    FOR VALUES FROM ('2026-03-19') TO ('2026-03-20');
CREATE TABLE bets_2026_03_20 PARTITION OF bets
    FOR VALUES FROM ('2026-03-20') TO ('2026-03-21');
CREATE TABLE bets_2026_03_21 PARTITION OF bets
    FOR VALUES FROM ('2026-03-21') TO ('2026-03-22');
CREATE TABLE bets_2026_03_22 PARTITION OF bets
    FOR VALUES FROM ('2026-03-22') TO ('2026-03-23');
CREATE TABLE bets_2026_03_23 PARTITION OF bets
    FOR VALUES FROM ('2026-03-23') TO ('2026-03-24');
CREATE TABLE bets_2026_03_24 PARTITION OF bets
    FOR VALUES FROM ('2026-03-24') TO ('2026-03-25');
CREATE TABLE bets_2026_03_25 PARTITION OF bets
    FOR VALUES FROM ('2026-03-25') TO ('2026-03-26');
CREATE TABLE bets_2026_03_26 PARTITION OF bets
    FOR VALUES FROM ('2026-03-26') TO ('2026-03-27');
CREATE TABLE bets_2026_03_27 PARTITION OF bets
    FOR VALUES FROM ('2026-03-27') TO ('2026-03-28');
CREATE TABLE bets_2026_03_28 PARTITION OF bets
    FOR VALUES FROM ('2026-03-28') TO ('2026-03-29');
CREATE TABLE bets_2026_03_29 PARTITION OF bets
    FOR VALUES FROM ('2026-03-29') TO ('2026-03-30');
CREATE TABLE bets_2026_03_30 PARTITION OF bets
    FOR VALUES FROM ('2026-03-30') TO ('2026-03-31');
CREATE TABLE bets_2026_03_31 PARTITION OF bets
    FOR VALUES FROM ('2026-03-31') TO ('2026-04-01');

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX idx_bets_user_placed ON bets (user_id, placed_at DESC);
CREATE INDEX idx_bets_status ON bets (status) WHERE status IN ('pending', 'active');
CREATE INDEX idx_bets_event ON bets (event_id, placed_at DESC) WHERE event_id IS NOT NULL;
CREATE INDEX idx_bets_sport ON bets (sport_id, placed_at DESC) WHERE sport_id IS NOT NULL;
CREATE INDEX idx_bets_idempotency ON bets (idempotency_key);

-- BRIN for time-series
CREATE INDEX idx_bets_placed_brin ON bets USING BRIN (placed_at);

-- ============================================================
-- IDEMPOTENCY SAFETY NET (global unique constraint workaround
-- for partitioned bets table where PG < 15 does not allow
-- UNIQUE without partition key)
-- ============================================================

CREATE TABLE bet_idempotency_keys (
    idempotency_key UUID PRIMARY KEY,
    bet_id          BIGINT NOT NULL,
    user_id         BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_idempotency_keys_bet ON bet_idempotency_keys (bet_id);

-- ============================================================
-- BET SELECTIONS (accumulator/system bet legs)
-- ============================================================

CREATE TABLE bet_selections (
    id              BIGSERIAL PRIMARY KEY,
    bet_id          BIGINT NOT NULL,
    user_id         BIGINT NOT NULL,
    event_id        BIGINT NOT NULL,
    market_id       BIGINT NOT NULL,
    selection_id    BIGINT NOT NULL,
    odds            NUMERIC(12,6) NOT NULL,
    status          bet_status_enum NOT NULL DEFAULT 'active',
    result          VARCHAR(20),  -- 'won', 'lost', 'void'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at      TIMESTAMPTZ,
    CONSTRAINT ck_selections_odds CHECK (odds > 1.0)
);

CREATE INDEX idx_selections_bet ON bet_selections (bet_id, user_id);
CREATE INDEX idx_selections_event ON bet_selections (event_id)
    WHERE status = 'active';
