# PostgreSQL migration order (canonical)

Runner: `tools/migrate.sh` (see also `Makefile: prod-migrate`).
How it applies files (`pg_files` in `migrate.sh`):

1. All `libs/migrations/postgresql/*.sql` **sorted by filename** (byte order).
2. `*.down.sql` files are **excluded** (never applied by the runner).
3. Each file runs once with `psql -v ON_ERROR_STOP=1 -f`; on success its
   filename is recorded in `schema_migrations(filename)`.
4. ClickHouse files in `libs/migrations/clickhouse/` are re-applied
   idempotently on every run (no ledger).

Do NOT renumber or edit applied files (001-024). New migrations append
with the next free number.

## Canonical order

| # | File | Content |
|---|------|---------|
| 1 | `001_extensions_and_enums.sql` | extensions, ENUM types, `update_updated_at()` |
| 2 | `002_users.sql` | users, user_preferences, user_limits |
| 3 | `003_wallets.sql` | wallets, house_accounts |
| 4 | `004_transactions.sql` | wallet_transactions + ledger_entries (monthly partitions) |
| 5 | `005_bets.sql` | bets, bet_selections, bet_idempotency_keys (daily partitions) |
| 6 | `006_reference_data.sql` | currencies, countries, sports, game_configs (Citus reference) |
| 7 | `007_audit.sql` | audit_log (append-only, partitions) |
| 8 | `008_rls_and_triggers.sql` | RLS policies, audit triggers, Citus distribution |
| 9 | `009_outbox.sql` | transactional outbox |
| 10a | `010_kyc_rg.sql` | kyc_documents, kyc_reviews, screenings, RG limits/alerts |
| 10b | `010_seed_reference.sql` | seed: currencies, countries, kyc_limits, game_providers, platform_config |
| 11a | `011_casino_tables.sql` | game_providers, games, sessions/rounds, kyc_limits, platform_config, bonuses |
| 11b | `011_payments.sql` | chargebacks, snapshots, crypto_wallets, p2p, reconciliation |
| 12 | `012_support.sql` | support tickets, messages, SLA config |
| 13 | `013_admin_bff.sql` | admin BFF centralised tables |
| 14 | `014_payments_core.sql` | payments, withdrawals (+ `014_payments_core.down.sql` rollback only) |
| 15 | `015_wallet_core.sql` | wallet-core ledger (fund_locks, transactions) |
| 16 | `016_casino_admin.sql` | casino admin read-only monitoring tables |
| 17 | `020_affiliate_admin_bff_split.sql` | resolves affiliate collisions between 013 and 021 |
| 18 | `021_affiliates.sql` | affiliate service tables (service 001+002 consolidation) |
| 19 | `022_payment_review.sql` | withdrawal manual-review queue, payment_methods |
| 20 | `023_wallet_lock_settlement.sql` | fund-lock settlement (consume) |
| 21 | `024_stage3_hardening.sql` | additive audit follow-up: indexes, RLS, Citus guards |
| 22 | `025_notifications.sql` | notifications + notification_settings (+ `.down.sql`) |
| 23 | `026_affiliate_currency.sql` | affiliate_adjustments.currency backfill (+ `.down.sql`) |

Gaps `017-019` are intentionally unused (reserved, no files). `queries/`
holds runnable checks (reconciliation), not migrations.

## Duplicate numbers (010 / 011) — do not touch

Two files share one number because DDL and seed were added in parallel
during centralisation. Filename sort makes the effective order
deterministic:

- `010_kyc_rg.sql` < `010_seed_reference.sql` (`k` < `s`)
- `011_casino_tables.sql` < `011_payments.sql` (`c` < `p`)

Rule for future work: never reuse a number. If a fix depends on a file
that sorts later, put the fix in a NEW numbered migration, not in a
duplicate.

## Known hazard (fresh-DB apply): seed runs before casino DDL

`010_seed_reference.sql` INSERTs into `game_providers`, `kyc_limits` and
`platform_config`, but those tables are created by `011_casino_tables.sql`,
which sorts AFTER the seed. On a database where 011 was never applied, the
runner aborts at the seed (`relation "game_providers" does not exist`,
`ON_ERROR_STOP=1`, nothing recorded). Existing databases that already
applied both files are unaffected (all statements are idempotent).

Fixing this requires either a rename or a rescue migration — both out of
scope for this wave (001-021 are frozen). Next step for the owning wave:
add a forward-only migration that is a no-op when tables exist, or split
the seed per owning domain.

## Verification (against the ledger)

The ledger has no integer `version` column — tracking key is the filename,
so `SELECT max(version)` does not apply. Equivalent checks:

```sql
-- 1. How many migrations applied (expect = number of *.sql minus *.down.sql).
SELECT count(*) FROM schema_migrations;

-- 2. Applied files in canonical (filename) order.
SELECT filename, applied_at FROM schema_migrations ORDER BY filename;

-- 3. Fresh-DB completeness: every non-down file must be present.
--    Compare this output against: ls libs/migrations/postgresql/*.sql
--    (excluding *.down.sql). Any filename missing here was never applied.
SELECT filename FROM schema_migrations ORDER BY filename;

-- 4. Most recently applied (operational "head pointer").
SELECT filename, applied_at FROM schema_migrations
ORDER BY applied_at DESC LIMIT 5;
```

A run is green when check 3 returns exactly the directory listing
(minus `*.down.sql`) and no file failed with `ON_ERROR_STOP`.
