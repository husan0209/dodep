# PostgreSQL Schema Documentation

**Author:** DATA_ENGINEER
**Updated:** 2026-03-24
**Database:** PostgreSQL 16 + Citus

## Кластер

| Роль        | Pods | CPU     | RAM  | Storage   |
| ----------- | ---- | ------- | ---- | --------- |
| Coordinator | 1    | 4 cores | 16GB | 200GB gp3 |
| Workers     | 3    | 4 cores | 16GB | 200GB gp3 |
| PgBouncer   | 2    | 1 core  | 2GB  | —         |

**Connection pooling:** PgBouncer, transaction mode, 10K max connections

## Миграции

| #   | Файл                           | Описание                                                        |
| --- | ------------------------------ | --------------------------------------------------------------- |
| 001 | `001_extensions_and_enums.sql` | Расширения (citus, pgcrypto, pg_stat_statements), 13 ENUM типов |
| 002 | `002_users.sql`                | users, user_preferences, user_limits                            |
| 003 | `003_wallets.sql`              | wallets (optimistic locking), house_accounts                    |
| 004 | `004_transactions.sql`         | wallet_transactions (monthly), ledger_entries (double-entry)    |
| 005 | `005_bets.sql`                 | bets (daily), bet_selections                                    |
| 006 | `006_reference_data.sql`       | currencies, countries, sports, game_configs                     |
| 007 | `007_audit.sql`                | audit_log (append-only, RULE no update/delete)                  |
| 008 | `008_rls_and_triggers.sql`     | RLS policies, audit triggers, Citus sharding                    |
| 009 | `009_outbox.sql`               | outbox table (transactional outbox pattern)                     |
| 021 | `021_affiliates.sql`           | affiliate_* (15 таблиц RevShare from NGR, ledger, outbox)       |

## Схема таблиц

### users

| Колонка       | Тип                 | Описание                                          |
| ------------- | ------------------- | ------------------------------------------------- |
| id            | BIGSERIAL PK        | Уникальный ID                                     |
| uuid          | UUID UNIQUE         | Публичный ID                                      |
| email         | VARCHAR(255) UNIQUE | Email                                             |
| phone         | VARCHAR(20) UNIQUE  | Телефон                                           |
| password_hash | VARCHAR(255)        | Argon2id хеш                                      |
| status        | user_status_enum    | pending, active, suspended, banned, self_excluded |
| kyc_level     | SMALLINT            | 0=unverified, 1=basic, 2=enhanced, 3=full         |
| country_code  | CHAR(2)             | ISO 3166-1 alpha-2                                |
| currency_code | CHAR(3)             | ISO 4217                                          |
| created_at    | TIMESTAMPTZ         | Дата создания                                     |
| updated_at    | TIMESTAMPTZ         | Дата обновления                                   |
| last_login_at | TIMESTAMPTZ         | Последний вход                                    |
| metadata      | JSONB               | Дополнительные данные                             |

**Шардирование:** HASH(user_id), 32 шарда

### wallets

| Колонка        | Тип           | Описание                             |
| -------------- | ------------- | ------------------------------------ |
| id             | BIGSERIAL PK  | Уникальный ID                        |
| user_id        | BIGINT FK     | Ссылка на users                      |
| currency_code  | CHAR(3)       | Валюта                               |
| balance        | NUMERIC(18,8) | Доступный баланс                     |
| locked_balance | NUMERIC(18,8) | Заблокированные средства (в ставках) |
| version        | INTEGER       | Optimistic locking                   |
| created_at     | TIMESTAMPTZ   | Дата создания                        |
| updated_at     | TIMESTAMPTZ   | Дата обновления                      |

**Ограничения:**

- UNIQUE(user_id, currency_code)
- CHECK(balance >= 0)
- CHECK(locked_balance >= 0)
- CHECK(balance >= locked_balance)

**Шардирование:** HASH(user_id) — co-located с users

### wallet_transactions

| Колонка         | Тип            | Описание                                                                                   |
| --------------- | -------------- | ------------------------------------------------------------------------------------------ |
| id              | BIGSERIAL      | Уникальный ID                                                                              |
| user_id         | BIGINT         | ID пользователя                                                                            |
| wallet_id       | BIGINT         | ID кошелька                                                                                |
| type            | tx_type_enum   | deposit, withdrawal, bet_place, bet_win, bet_refund, bonus_credit, bonus_wager, adjustment |
| amount          | NUMERIC(18,8)  | Сумма                                                                                      |
| balance_before  | NUMERIC(18,8)  | Баланс до                                                                                  |
| balance_after   | NUMERIC(18,8)  | Баланс после                                                                               |
| reference_type  | VARCHAR(50)    | Тип ссылки (bet, payment, bonus)                                                           |
| reference_id    | BIGINT         | ID ссылки                                                                                  |
| idempotency_key | UUID UNIQUE    | Ключ идемпотентности                                                                       |
| status          | tx_status_enum | pending, completed, failed, reversed                                                       |
| metadata        | JSONB          | Дополнительные данные                                                                      |
| created_at      | TIMESTAMPTZ    | Дата создания                                                                              |

**Партиционирование:** RANGE(created_at) MONTHLY
**Шардирование:** HASH(user_id)

### bets

| Колонка            | Тип             | Описание                                  |
| ------------------ | --------------- | ----------------------------------------- |
| id                 | BIGSERIAL       | Уникальный ID                             |
| user_id            | BIGINT          | ID пользователя                           |
| bet_type           | bet_type_enum   | single, accumulator, system               |
| status             | bet_status_enum | pending, active, won, lost, void, cashout |
| stake              | NUMERIC(18,8)   | Сумма ставки                              |
| potential_win      | NUMERIC(18,8)   | Потенциальный выигрыш                     |
| actual_win         | NUMERIC(18,8)   | Фактический выигрыш                       |
| odds               | NUMERIC(12,6)   | Коэффициент                               |
| currency_code      | CHAR(3)         | Валюта                                    |
| sport_id           | INTEGER         | ID спорта                                 |
| event_id           | BIGINT          | ID события                                |
| market_id          | BIGINT          | ID маркета                                |
| selection_id       | BIGINT          | ID селекции                               |
| placed_at          | TIMESTAMPTZ     | Дата размещения                           |
| settled_at         | TIMESTAMPTZ     | Дата расчёта                              |
| ip_address         | INET            | IP адрес                                  |
| device_fingerprint | VARCHAR(64)     | Отпечаток устройства                      |
| metadata           | JSONB           | Дополнительные данные                     |

**Партиционирование:** RANGE(placed_at) DAILY
**Шардирование:** HASH(user_id)

### affiliate_profiles

Профили партнёров (RevShare from NGR). Деньги партнёров — отдельный леджер, не смешивается с балансом игрока.

| Колонка           | Тип              | Описание                                              |
| ----------------- | ---------------- | ----------------------------------------------------- |
| id                | UUID PK          | ID профиля                                            |
| user_id           | BIGINT UNIQUE    | ID пользователя (один игрок — один профиль)           |
| status            | affiliate_status | pending_review, active, suspended, rejected, closed   |
| affiliate_code    | VARCHAR(32) UNIQUE | Реферальный код                                     |
| commission_plan_id | UUID FK         | Тарифный план                                         |
| commission_rate   | NUMERIC(10,4)    | Ставка комиссии 0–1 (дефолт 0.20)                     |
| hold_period_days  | INT              | Hold между accrued и available (дефолт 14)            |
| min_payout_amount | NUMERIC(18,8)    | Минимальная выплата (дефолт 100)                      |
| currency          | CHAR(3)          | Валюта (дефолт USD)                                   |
| kyc_required      | BOOLEAN          | Требуется KYC для выплат                              |
| approval_mode     | affiliate_approval_mode | manual / automatic                              |
| payout_schedule   | affiliate_payout_schedule | manual / weekly / monthly                     |
| approved_by/at    | TEXT / TIMESTAMPTZ | Кто и когда одобрил                                 |

### affiliate_enrollment_requests

Заявки на партнёрство (MVP: только ручной аппрув).

| Колонка      | Тип                        | Описание                              |
| ------------ | -------------------------- | ------------------------------------- |
| id           | UUID PK                    | ID заявки                             |
| user_id      | BIGINT                     | ID пользователя                       |
| status       | affiliate_enrollment_status | pending_review, approved, rejected   |
| reason       | TEXT                       | Причина от игрока                     |
| review_notes | TEXT                       | Заметки админа                        |
| reviewed_by/at | TEXT / TIMESTAMPTZ       | Кто и когда рассмотрел                |

**Ограничение:** частичный UNIQUE(user_id) WHERE status='pending_review' — одна активная заявка.

### affiliate_commission_plans

Тарифные планы (тип всегда `revshare`, RTP не используется).

| Колонка                  | Тип       | Описание                              |
| ------------------------ | --------- | ------------------------------------- |
| id                       | UUID PK   | ID плана                              |
| name                     | VARCHAR(100) | Название (дефолт `Default RevShare 20`) |
| commission_rate          | NUMERIC(10,4) | 0–1                                |
| hold_period_days         | INT       | Hold period                           |
| min_payout_amount        | NUMERIC(18,8) | Минимальная выплата                |
| negative_carryover_enabled | BOOLEAN | MVP: всегда FALSE                     |
| is_default / is_active   | BOOLEAN   | Дефолтный (только один) / активен     |

### affiliate_links / affiliate_clicks / affiliate_attributions

Воронка `click -> registration -> FTD`.

- **affiliate_links:** id, affiliate_id FK, campaign_name, landing_page, referral_code, referral_url, utm_source/medium/campaign, is_active.
- **affiliate_clicks:** id, affiliate_id FK, link_id FK, click_id UNIQUE, ip_hash, user_agent_hash, device_fingerprint, country_code, landing_page.
- **affiliate_attributions:** id, affiliate_id FK, referred_user_id BIGINT UNIQUE (один игрок — один партнёр), click_id, attribution_model (дефолт `last_click`), attributed_at, ftd_at, is_ftd_qualified.

### affiliate_earnings

Начисления `commission = max(0, NGR × rate)`, только по финализированным данным.

| Колонка           | Тип                   | Описание                                        |
| ----------------- | --------------------- | ----------------------------------------------- |
| id                | UUID PK               | ID начисления                                   |
| affiliate_id      | UUID FK               | Партнёр                                         |
| referred_user_id | BIGINT                | Приведённый игрок                               |
| source_type/id    | VARCHAR(32/64)        | Источник (casino/sports), id раунда/ставки      |
| period_start/end  | TIMESTAMPTZ           | Период                                          |
| ggr_amount        | NUMERIC(18,8)         | GGR                                             |
| ngr_amount        | NUMERIC(18,8)         | NGR (GGR − бонусы − fees − chargebacks − налоги) |
| commission_rate   | NUMERIC(10,4)         | Снапшот ставки на момент начисления             |
| commission_amount | NUMERIC(18,8)         | Сумма комиссии                                  |
| status            | affiliate_earning_status | accrued, pending, available, paid, reversed  |
| hold_until        | TIMESTAMPTZ           | Доступно после этой даты                        |
| idempotency_key   | VARCHAR(128) UNIQUE   | Ключ идемпотентности                            |

**Индекс:** (affiliate_id, status, hold_until) — для hold-release job.

### affiliate_payout_methods / affiliate_payouts

Отдельный payout-контур (не смешивается с выводом игрока).

- **affiliate_payout_methods:** id, affiliate_id FK, method_type (`bank_transfer/crypto/ewallet`), details_encrypted, display_name, details_masked, is_default, is_verified.
- **affiliate_payouts:** id, affiliate_id FK, amount NUMERIC(18,8) CHECK>0, currency, method_id FK, idempotency_key UNIQUE, status (`requested/reviewing/approved/processing/paid/rejected/failed`), requested_at, approved_by/at, provider_reference, rejection_reason.

**Индекс:** (affiliate_id, status, requested_at DESC) — очередь выплат.

### affiliate_adjustments / affiliate_fraud_flags

- **affiliate_adjustments:** id, affiliate_id FK, type (`credit/debit`), amount CHECK>0, reason (обязателен), reference_id, created_by.
- **affiliate_fraud_flags:** id, affiliate_id FK, referred_user_id, flag_type (self_referral, kyc_match, payment_match, device_match, ...), severity (`low/medium/high/critical`), status (`open/in_review/resolved/dismissed`), details JSONB.

### affiliate_ledger_accounts / affiliate_ledger_entries / affiliate_outbox

- **affiliate_ledger_accounts:** (affiliate_id, account_type, currency) UNIQUE; типы `pending/available/processed/paid/reversed/adjusted`.
- **affiliate_ledger_entries:** double-entry проводки, UNIQUE(transaction_id, account_id, direction), idempotency_key с индексом.
- **affiliate_outbox:** BIGSERIAL id, aggregate_type/id, topic, event_key, payload/headers JSONB, published_at, retry_count. Индекс по неопубликованным.

**Миграция:** `libs/migrations/postgresql/021_affiliates.sql` (консолидация `001+002` сервисных миграций, сид дефолтного плана на фиксированном UUID).

## ENUM типы

```sql
CREATE TYPE user_status_enum AS ENUM ('pending','active','suspended','banned','self_excluded');
CREATE TYPE tx_type_enum AS ENUM ('deposit','withdrawal','bet_place','bet_win','bet_refund','bonus_credit','bonus_wager','adjustment');
CREATE TYPE tx_status_enum AS ENUM ('pending','completed','failed','reversed');
CREATE TYPE bet_type_enum AS ENUM ('single','accumulator','system');
CREATE TYPE bet_status_enum AS ENUM ('pending','active','won','lost','void','cashout');
CREATE TYPE kyc_level_enum AS ENUM ('unverified','basic','enhanced','full');
CREATE TYPE payment_method_enum AS ENUM ('card','bank_transfer','e_wallet','crypto','local');
CREATE TYPE payment_status_enum AS ENUM ('pending','processing','completed','failed','refunded');
CREATE TYPE bonus_type_enum AS ENUM ('welcome','deposit','free_bet','cashback','vip');
CREATE TYPE notification_channel_enum AS ENUM ('email','sms','push','in_app');
CREATE TYPE fraud_severity_enum AS ENUM ('low','medium','high','critical');
CREATE TYPE game_category_enum AS ENUM ('slots','table_games','live_casino','sports','virtual');
```

## RLS политики

```sql
-- users: видят только свои данные
CREATE POLICY user_isolation ON users
    USING (id = current_setting('app.current_user_id')::bigint);

-- wallets: видят только свои кошельки
CREATE POLICY wallet_isolation ON wallets
    USING (user_id = current_setting('app.current_user_id')::bigint);

-- wallet_transactions: видят только свои транзакции
CREATE POLICY tx_isolation ON wallet_transactions
    USING (user_id = current_setting('app.current_user_id')::bigint);

-- bets: видят только свои ставки
CREATE POLICY bet_isolation ON bets
    USING (user_id = current_setting('app.current_user_id')::bigint);
```

## Audit Trigger

```sql
CREATE OR REPLACE FUNCTION audit_trigger()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_log (table_name, record_id, action, old_data, new_data, user_id)
    VALUES (
        TG_TABLE_NAME,
        COALESCE(NEW.id, OLD.id),
        TG_OP,
        CASE WHEN TG_OP = 'DELETE' THEN row_to_json(OLD)::jsonb ELSE NULL END,
        CASE WHEN TG_OP IN ('INSERT', 'UPDATE') THEN row_to_json(NEW)::jsonb ELSE NULL END,
        current_setting('app.current_user_id', true)::bigint
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
```

## Transactional Outbox

```sql
CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 10,
    error_message TEXT
);
```

Функция `write_outbox()` записывает события в ту же транзакцию что и бизнес-данные.

## Performance

- **Пул соединений:** PgBouncer, transaction mode, 50 conn/pool, 10K max clients
- **Индексы:** CONCURRENTLY, BRIN для time-series, partial для активных записей
- **Autovacuum:** aggressive (5 workers, 30s naptime)
- **Мониторинг:** pg_stat_statements для анализа запросов
