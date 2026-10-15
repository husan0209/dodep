## #33 postgresql-schemas.skill.md

```markdown
# postgresql-schemas.skill.md

## РОЛЬ
Ты — DBA / Backend Developer, проектирующий и поддерживающий
PostgreSQL 16 + Citus схемы для гемблинг-платформы.

## КОНТЕКСТ
- PostgreSQL 16 + Citus (горизонтальный шардинг)
- PgBouncer (connection pooling, transaction mode)
- Миллиарды транзакций, сотни миллионов ставок
- ACID гарантии для финансовых операций
- Партиционирование по времени для больших таблиц

## ПРАВИЛА ПРОЕКТИРОВАНИЯ СХЕМЫ

### 1. Naming Conventions
```sql
-- Таблицы: snake_case, множественное число
CREATE TABLE users (...);
CREATE TABLE bets (...);
CREATE TABLE wallet_transactions (...);

-- Колонки: snake_case
user_id, created_at, currency_code

-- Индексы: idx_{table}_{columns}
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_bets_user_placed ON bets (user_id, placed_at DESC);

-- Constraints: {type}_{table}_{description}
CONSTRAINT pk_users PRIMARY KEY (id)
CONSTRAINT uq_users_email UNIQUE (email)
CONSTRAINT fk_bets_user FOREIGN KEY (user_id) REFERENCES users(id)
CONSTRAINT ck_wallets_balance_positive CHECK (balance >= 0)

-- Enum types: {entity}_{field}_enum
CREATE TYPE user_status_enum AS ENUM ('active', 'blocked', 'pending', 'self_excluded');
CREATE TYPE bet_status_enum AS ENUM ('pending', 'active', 'won', 'lost', 'void', 'cashout');
2. Обязательные колонки в каждой таблице
SQL

-- Каждая таблица ДОЛЖНА иметь:
id          BIGSERIAL PRIMARY KEY,           -- всегда bigint
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- Для таблиц с soft delete:
deleted_at  TIMESTAMPTZ  -- NULL = не удалён

-- Для таблиц с optimistic locking:
version     INTEGER NOT NULL DEFAULT 0
3. Auto-update updated_at
SQL

-- Триггер для автообновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Применять к каждой таблице:
CREATE TRIGGER trg_users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CITUS ШАРДИНГ
SQL

-- Distributed tables: шардируются по user_id
-- Все таблицы, связанные с пользователем, шардируются по user_id
-- для co-location (JOIN без network)
SELECT create_distributed_table('users', 'id');
SELECT create_distributed_table('wallets', 'user_id');
SELECT create_distributed_table('wallet_transactions', 'user_id');
SELECT create_distributed_table('bets', 'user_id');
SELECT create_distributed_table('bet_selections', 'user_id');
SELECT create_distributed_table('kyc_records', 'user_id');
SELECT create_distributed_table('user_sessions', 'user_id');
SELECT create_distributed_table('user_limits', 'user_id');

-- Reference tables: реплицируются на все ноды
-- Маленькие справочные таблицы
SELECT create_reference_table('currencies');
SELECT create_reference_table('countries');
SELECT create_reference_table('sports');
SELECT create_reference_table('game_configs');
SELECT create_reference_table('bonus_campaigns');

-- Co-location: wallets и bets на тех же шардах что users
SELECT mark_tables_colocated('users', 'wallets');
SELECT mark_tables_colocated('users', 'wallet_transactions');
SELECT mark_tables_colocated('users', 'bets');
ПАРТИЦИОНИРОВАНИЕ
SQL

-- Большие таблицы партиционируются по времени
-- wallet_transactions: по месяцам
CREATE TABLE wallet_transactions (
    id              BIGSERIAL,
    user_id         BIGINT NOT NULL,
    wallet_id       BIGINT NOT NULL,
    type            tx_type_enum NOT NULL,
    amount          NUMERIC(18,8) NOT NULL,
    balance_before  NUMERIC(18,8) NOT NULL,
    balance_after   NUMERIC(18,8) NOT NULL,
    reference_type  VARCHAR(50),
    reference_id    BIGINT,
    idempotency_key UUID UNIQUE NOT NULL,
    status          tx_status_enum NOT NULL DEFAULT 'completed',
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, id, created_at)
) PARTITION BY RANGE (created_at);

-- Создание партиций (автоматизировать через pg_partman)
CREATE TABLE wallet_transactions_2025_01
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE wallet_transactions_2025_02
    PARTITION OF wallet_transactions
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- pg_partman автоматизация
SELECT partman.create_parent(
    p_parent_table := 'public.wallet_transactions',
    p_control := 'created_at',
    p_type := 'native',
    p_interval := 'monthly',
    p_premake := 3       -- создавать 3 месяца вперёд
);

-- bets: по дням (больше данных)
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
    placed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at      TIMESTAMPTZ,
    ip_address      INET,
    device_fp       VARCHAR(64),
    metadata        JSONB DEFAULT '{}',
    PRIMARY KEY (user_id, id, placed_at)
) PARTITION BY RANGE (placed_at);
КЛЮЧЕВЫЕ СХЕМЫ
Users + Wallets
SQL

CREATE TYPE user_status_enum AS ENUM (
    'pending', 'active', 'blocked', 'self_excluded', 'closed'
);

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    phone           VARCHAR(20) UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    status          user_status_enum NOT NULL DEFAULT 'pending',
    kyc_level       SMALLINT NOT NULL DEFAULT 0 CHECK (kyc_level BETWEEN 0 AND 4),
    country_code    CHAR(2) NOT NULL,
    currency_code   CHAR(3) NOT NULL,
    language        CHAR(2) NOT NULL DEFAULT 'en',
    timezone        VARCHAR(50) DEFAULT 'UTC',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    phone_verified  BOOLEAN NOT NULL DEFAULT false,
    two_fa_enabled  BOOLEAN NOT NULL DEFAULT false,
    two_fa_secret   VARCHAR(255),
    metadata        JSONB DEFAULT '{}'
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_phone ON users (phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_users_uuid ON users (uuid);
CREATE INDEX idx_users_status ON users (status);
CREATE INDEX idx_users_country ON users (country_code);
CREATE INDEX idx_users_created ON users (created_at DESC);

CREATE TABLE wallets (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    currency_code   CHAR(3) NOT NULL,
    balance         NUMERIC(18,8) NOT NULL DEFAULT 0,
    locked_balance  NUMERIC(18,8) NOT NULL DEFAULT 0,
    bonus_balance   NUMERIC(18,8) NOT NULL DEFAULT 0,
    version         INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_wallets_user_currency UNIQUE (user_id, currency_code),
    CONSTRAINT ck_wallets_balance CHECK (balance >= 0),
    CONSTRAINT ck_wallets_locked CHECK (locked_balance >= 0),
    CONSTRAINT ck_wallets_bonus CHECK (bonus_balance >= 0),
    CONSTRAINT ck_wallets_locked_lte_balance CHECK (locked_balance <= balance)
);

CREATE INDEX idx_wallets_user ON wallets (user_id);
Audit Trail
SQL

-- Append-only, никогда не UPDATE/DELETE
CREATE TABLE audit_log (
    id              BIGSERIAL,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type      VARCHAR(20) NOT NULL,   -- 'user', 'admin', 'system'
    actor_id        BIGINT,
    action          VARCHAR(100) NOT NULL,  -- 'user.block', 'bet.void'
    entity_type     VARCHAR(50) NOT NULL,   -- 'user', 'bet', 'wallet'
    entity_id       BIGINT NOT NULL,
    old_value       JSONB,
    new_value       JSONB,
    ip_address      INET,
    user_agent      TEXT,
    metadata        JSONB DEFAULT '{}',
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Запретить UPDATE и DELETE
CREATE RULE audit_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING;
CREATE RULE audit_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING;
МИГРАЦИИ
SQL

-- Файл: migrations/V001__create_users.sql
-- Naming: V{NNN}__{description}.sql (Flyway format)

-- Каждая миграция:
-- 1. Идемпотентна (IF NOT EXISTS)
-- 2. Имеет комментарий с описанием
-- 3. Не содержит DROP без backup
-- 4. Не блокирует таблицу надолго

-- ✅ ПРАВИЛЬНО: создание индекса CONCURRENTLY
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bets_event 
    ON bets (event_id, placed_at DESC);

-- ❌ ПЛОХО: обычный CREATE INDEX на большой таблице
-- Заблокирует таблицу на минуты
CREATE INDEX idx_bets_event ON bets (event_id);

-- ✅ ПРАВИЛЬНО: добавление колонки с default
-- В PG 11+ это мгновенная операция
ALTER TABLE users ADD COLUMN IF NOT EXISTS vip_level SMALLINT DEFAULT 0;

-- ❌ ПЛОХО: NOT NULL без DEFAULT на существующей таблице
-- Перезапишет всю таблицу
ALTER TABLE users ADD COLUMN vip_level SMALLINT NOT NULL;
QUERY PATTERNS
Optimistic Locking для Wallet
SQL

-- ✅ ПРАВИЛЬНО: optimistic locking
UPDATE wallets
SET balance = balance - $1,
    locked_balance = locked_balance + $1,
    version = version + 1,
    updated_at = NOW()
WHERE user_id = $2
  AND currency_code = $3
  AND version = $4
  AND balance - locked_balance >= $1
RETURNING balance, locked_balance, version;
-- Если rows_affected = 0 → concurrent modification → retry
Idempotent Insert
SQL

-- ✅ ПРАВИЛЬНО: idempotent transaction
INSERT INTO wallet_transactions (
    user_id, wallet_id, type, amount,
    balance_before, balance_after,
    idempotency_key, reference_type, reference_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id, balance_after;
-- Если conflict → повторный запрос, нет дублей
АНТИПАТТЕРНЫ
SQL

-- ❌ ПЛОХО: SELECT * 
SELECT * FROM users WHERE id = $1;

-- ✅ ПРАВИЛЬНО: явные колонки
SELECT id, email, status, kyc_level, currency_code
FROM users WHERE id = $1;

-- ❌ ПЛОХО: N+1 запросы
-- В цикле: SELECT * FROM wallets WHERE user_id = $1;

-- ✅ ПРАВИЛЬНО: один запрос
SELECT * FROM wallets WHERE user_id = ANY($1::bigint[]);

-- ❌ ПЛОХО: OFFSET для пагинации больших таблиц
SELECT * FROM bets ORDER BY placed_at DESC LIMIT 20 OFFSET 10000;
-- Медленно: сканирует 10020 строк

-- ✅ ПРАВИЛЬНО: cursor-based pagination
SELECT * FROM bets 
WHERE placed_at < $1  -- курсор = placed_at последней записи
ORDER BY placed_at DESC 
LIMIT 20;

-- ❌ ПЛОХО: индексы на всё подряд
CREATE INDEX idx_users_metadata ON users (metadata);  -- GIN на JSONB всей таблицы

-- ✅ ПРАВИЛЬНО: частичные индексы
CREATE INDEX idx_users_blocked ON users (id) WHERE status = 'blocked';
CREATE INDEX idx_bets_unsettled ON bets (event_id) WHERE status IN ('pending', 'active');

-- ❌ ПЛОХО: хранить деньги как FLOAT
balance FLOAT;  -- 0.1 + 0.2 = 0.30000000000000004

-- ✅ ПРАВИЛЬНО: NUMERIC с фиксированной точностью
balance NUMERIC(18, 8);

-- ❌ ПЛОХО: DELETE для мягкого удаления
DELETE FROM users WHERE id = $1;

-- ✅ ПРАВИЛЬНО: soft delete
UPDATE users SET 
    status = 'closed', 
    deleted_at = NOW() 
WHERE id = $1;
PERFORMANCE
text

1. Prepared statements: ВСЕГДА (через SQLx/GORM параметризация)
2. Connection pool: PgBouncer transaction mode, 50 connections/pool
3. Индексы: EXPLAIN ANALYZE перед добавлением
4. Partial indexes: WHERE condition для уменьшения размера
5. BRIN indexes: для time-series колонок (created_at)
6. pg_stat_statements: мониторить slow queries
7. VACUUM: autovacuum настроен агрессивно для hot tables
8. Партиционирование: detach старые партиции без блокировки