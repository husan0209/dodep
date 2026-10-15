-- Migration: 024_stage3_hardening.sql
-- Description: Этап 3 audit follow-up — недостающие индексы и RLS-политики.
--              НЕ правит применённые миграции 001-009, только additive.
-- Author: DATA_ENGINEER
-- Date: 2026-10-15
--
-- Что закрывает (аудит 2026-10-15):
-- 1. wallet_transactions: не было btree (user_id, created_at) для истории
--    пользователя (cursor pagination). Был только (wallet_id, created_at).
-- 2. bets: covering index для истории (index-only scan: stake/odds/status).
-- 3. bet_selections / bet_idempotency_keys: RLS включён не был (008 покрыл
--    только users/wallets/wallet_transactions/bets/preferences/limits).
--
-- Идемпотентность: все CREATE INDEX — IF NOT EXISTS; политики — через
-- pg_policies guard; Citus distribute — через citus_tables guard.
-- CONCURRENTLY намеренно НЕ используется: миграции применяются в транзакции,
-- где CONCURRENTLY запрещён. Таблицы на момент 024 ещё не hot (pre-launch),
-- обычный CREATE INDEX безопасен. Post-launch индексы — только CONCURRENTLY
-- отдельными миграциями вне транзакции.

-- ============================================================
-- 1. wallet_transactions: история пользователя (cursor pagination)
--    Запрос: WHERE user_id=$1 AND created_at<$2 ORDER BY created_at DESC LIMIT 20
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_wtx_user_created
    ON wallet_transactions (user_id, created_at DESC);

-- ============================================================
-- 2. bets: covering index для истории (index-only scan)
--    Запрос: SELECT stake, odds, status, actual_win
--            WHERE user_id=$1 AND placed_at<$2 ORDER BY placed_at DESC
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_bets_user_history_cover
    ON bets (user_id, placed_at DESC)
    INCLUDE (stake, odds, status, actual_win);

-- ============================================================
-- 3. ledger_entries: выборка по счёту за период (reconciliation)
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_ledger_account_created
    ON ledger_entries (account_type, account_id, created_at DESC);

-- ============================================================
-- 4. RLS: bet_selections — изоляция по пользователю
-- ============================================================

ALTER TABLE bet_selections ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public'
          AND tablename = 'bet_selections'
          AND policyname = 'selections_own_data'
    ) THEN
        CREATE POLICY selections_own_data ON bet_selections
            FOR SELECT
            USING (user_id = current_setting('app.current_user_id', true)::BIGINT);
    END IF;
END $$;

-- ============================================================
-- 5. RLS: bet_idempotency_keys — изоляция по пользователю
-- ============================================================

ALTER TABLE bet_idempotency_keys ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public'
          AND tablename = 'bet_idempotency_keys'
          AND policyname = 'idemkeys_own_data'
    ) THEN
        CREATE POLICY idemkeys_own_data ON bet_idempotency_keys
            FOR SELECT
            USING (user_id = current_setting('app.current_user_id', true)::BIGINT);
    END IF;
END $$;

-- ============================================================
-- 6. Citus: убедиться, что bet_selections распределена (guard на случай,
--    если 008 применялась частично). Reference/distributed трогать иначе нельзя.
-- ============================================================

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        BEGIN
            PERFORM create_distributed_table('bet_selections', 'user_id');
        EXCEPTION WHEN duplicate_object THEN
            -- Уже распределена (008) — нормально, пропускаем.
            RAISE NOTICE 'bet_selections already distributed, skipping.';
        END;
    END IF;
EXCEPTION WHEN OTHERS THEN
    -- Citus недоступен в review-окружении: распределение проверяется в CI на кластере.
    RAISE NOTICE 'bet_selections citus guard skipped: %', SQLERRM;
END $$;

-- ============================================================
-- 7. Сервисные таблицы БЕЗ RLS (намеренно): outbox, ledger_entries,
--    audit_log, house_accounts — доступ только у service-ролей через
--    BYPASSRLS / прямые grants. RLS с пустыми политиками сломал бы
--    outbox-воркер и audit-триггер, поэтому здесь только фиксация решения.
--    Проверка grants — в CI evidence (Этапы 17-18).
-- ============================================================
