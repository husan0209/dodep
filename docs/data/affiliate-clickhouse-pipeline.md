# Affiliate ClickHouse Pipeline — Postgres → Events → ClickHouse → Dashboards

**Owner:** FLUTTER_MOBILE_ENGINEER + PROTOBUF (docs), DATA_ENGINEER (migrations)
**Sources (read-only, NOT edited here):**
`services/go/affiliate/migrations/001_create_affiliate_tables.up.sql`,
`services/go/affiliate/internal/repository/{gorm_repository,commission_flow}.go`,
`libs/migrations/clickhouse/{003,004,005}_*.sql`,
`services/python/analytics/src/analytics/affiliate_reports.py`,
`infra/k8s/monitoring/grafana/affiliate-dashboard.yaml`

> Numbering note: the task brief referenced "ClickHouse 002" as the DATA
> wave2 input, but `002_casino_audit.sql` is the casino provider-callback
> audit log. The actual affiliate chain is **003 (tables) → 004 (ingestion)
> → 005 (lifecycle)**. This doc maps that real chain.

## 1. Postgres (source of truth)

All money is `NUMERIC(18,8)` (CONVENTIONS NEVER-6); all state changes write
an `affiliate_outbox` row in the same DB transaction (transactional outbox).

### `affiliate_daily_aggregates` (upserted per affiliate/day)

| PG column | Type | Fed by |
| --------- | ---- | ------ |
| `affiliate_id`, `report_date` | UUID, DATE (unique together) | accrual / release / reversal jobs |
| `clicks` | BIGINT | click tracking (`TrackClick`) |
| `registrations` | BIGINT | attribution binding (`BindReferredUser`) |
| `ftd_count` | BIGINT | FTD qualification (backend gap — see §5) |
| `active_players` | BIGINT | settled-activity aggregation |
| `ggr_amount`, `ngr_amount` | NUMERIC(18,8) | `commission_engine.ComputeNGR`: `NGR = GGR − bonuses − payment_fees − chargebacks − fraud_writeoffs − taxes − manual_adjustments` |
| `commission_accrued` | NUMERIC(18,8) | `CalculateCommission` per earning (`AccrueDaily`) |
| `commission_reversed` | NUMERIC(18,8) | `ReverseEarning` (`upsertDailyAggregateTx`, day = earning `created_at`) |

Note: the PG table has **no** `commission_released` column — releases are
visible only via the `affiliate.commission.released` event stream (§2) and
the ClickHouse `affiliate_daily_releases` aggregate (§3).

### `affiliate_earnings` (one row per commission line)

`id, affiliate_id, referred_user_id, source_type, source_id, period_start,
period_end, ggr_amount, ngr_amount, commission_rate NUMERIC(10,4),
commission_amount, status (accrued→pending→available→paid / reversed),
hold_until, idempotency_key UNIQUE, created_at`.

### `affiliate_payouts` (one row per payout request)

`id, affiliate_id, amount, currency, method_id, status
(requested→reviewing→approved→processing→paid / rejected / failed),
requested_at, approved_at, provider_reference, rejection_reason,
idempotency_key UNIQUE`.

## 2. Redpanda events (outbox → broker → ClickHouse Kafka engine)

Key = affiliate/aggregate id (ordering per affiliate preserved). Payloads
carry **no event timestamps** — ClickHouse MVs use the Kafka `_timestamp`
for all time fields. Money travels as decimal **strings**.

| Topic (live ✅) | Emitted when | Payload fields |
| --------------- | ------------ | -------------- |
| `affiliate.click.tracked` ✅ | `TrackClick` | `click_id, affiliate_id, country_code, landing_page` |
| `affiliate.attribution.created` ✅ | `BindReferredUser` | `attribution_id, affiliate_id, referred_user_id, click_id` |
| `affiliate.commission.accrued` ✅ | `CalculateCommission` | `CommissionPayload`: `earning_id, affiliate_id, referred_user_id, commission_amount, commission_rate, ngr_amount, status, hold_until, idempotency_key` |
| `affiliate.commission.released` ✅ | `ReleaseEligibleEarnings` (hold passed) | `earning_id, affiliate_id, referred_user_id, commission_amount, previous_status, status=available, hold_until, released_at` |
| `affiliate.commission.reversed` ✅ | `ReverseEarning` | `earning_id, affiliate_id, referred_user_id, commission_amount, previous_status, status=reversed, reason, idempotency_key` |
| `affiliate.payout.requested` ✅ | `RequestPayout` | `PayoutPayload`: `payout_id, affiliate_id, amount, currency, method_id, status, idempotency_key` |
| `affiliate.payout.updated` ✅ | non-terminal transition (approved/reviewing/failed) | `payout_id, affiliate_id, status, approved_by, provider_reference, rejection_reason` |
| `affiliate.payout.approved` ✅ | `ApproveAffiliatePayout` | `PayoutPayload` + `approved_by, provider_reference` |
| `affiliate.payout.paid` ✅ | `MarkPayoutPaid` (new gRPC `MarkAffiliatePayoutPaid` maps here) | `PayoutPayload` + `provider_reference` |
| `affiliate.payout.rejected` ✅ | `RejectAffiliatePayout` | `PayoutPayload` + rejection reason |
| `affiliate.fraud.flagged` ✅ | `FlagAffiliateFraud` | `fraud_flag_id, affiliate_id, referred_user_id, flag_type, severity, status` |
| `affiliate.adjustment.created` ✅ | `CreateAdjustment` | `adjustment_id, affiliate_id, adjustment_type, amount, reason, created_by` |
| `affiliate.player.ftd` ❌ | — | **NOT emitted (known gap)** — `ftd_at` / `is_ftd_qualified` never set |

## 3. ClickHouse (analytics, append-only — no UPDATE/DELETE)

### 003 — base tables (`MergeTree` / `SummingMergeTree`)

| CH table | Grain | Key columns |
| -------- | ----- | ----------- |
| `affiliate_clicks_raw` | 1 row / click | `click_id, affiliate_id, country_code, landing_page, campaign, created_at` (TTL 90d) |
| `affiliate_attributions` | 1 row / referred user | `attribution_id, affiliate_id, referred_user_id, click_id, ftd_at NULL, is_ftd_qualified=0` |
| `affiliate_daily_aggregates` | 1 row / affiliate / day (`SummingMergeTree`) | `clicks, registrations, ftd_count, active_players, ggr_amount, ngr_amount, commission_accrued, commission_released, commission_reversed, currency, updated_at` |
| `affiliate_earnings` | 1 row / commission event | `earning_id, affiliate_id, referred_user_id, source_type, source_id, period_start/end, ggr_amount, ngr_amount, commission_rate Decimal(10,4), commission_amount, status, hold_until, created_at` |
| `affiliate_payouts` | **1 row / lifecycle event** (not 1 row / payout) | `payout_id, affiliate_id, amount, currency, status, requested_at, approved_at NULL, provider_reference NULL, created_at`. Paid/rejected rows carry `amount=0`/empty currency — totals must join back to `requested` rows |
| `referred_player_activity` | 1 row / player / day (`SummingMergeTree`) | `bets_count/amount, wins_amount, ggr, deposits_*, withdrawals_*, bonuses_received, currency` |
| `affiliate_fraud_signals` | 1 row / flag (TTL 365d) | `signal_id, affiliate_id, referred_user_id, signal_type, severity, details JSON, detected_at` |
| `affiliate_funnel_hourly` | 1 row / affiliate / hour (`SummingMergeTree`, TTL 30d) | `clicks, registrations, ftds, active_players, conversion_*` |

### 004 — ingestion (Kafka engine → MV → 003)

Five queue tables (`affiliate_{clicks,attributions,commissions,payouts,fraud}_queue`,
consumer groups `clickhouse_affiliate_*`, `JSONEachRow`) with MVs
(`affiliate_{clicks,attributions,commissions,payouts,fraud}_mv`) that
normalize partial payloads (`toDecimal64OrZero`, `ifNull/nullIf` on empty
strings, `_timestamp` for time). The payouts MV subscribes to
`requested,paid,rejected`; commission MV covers `accrued` (zeros for
missing GGR/NGR/source/period — enriching the publisher payload needs no
MV change).

### 005 — lifecycle (closes the 004 gaps)

| CH table | Fills | Source topic |
| -------- | ----- | ------------ |
| `affiliate_commission_releases` | hold releases (`earning_id, affiliate_id, referred_user_id, commission_amount, released_at`, TTL 3y) | `affiliate.commission.released` |
| `affiliate_adjustments_audit` | manual money edits (`adjustment_id, type credit/debit, amount, reason, created_by`, TTL 5y) | `affiliate.adjustment.created` |
| `affiliate_payout_transitions` | non-terminal states (`status, approved_by, provider_reference, rejection_reason, transitioned_at`, TTL 3y) | `affiliate.payout.updated` |
| `affiliate_daily_releases` (`SummingMergeTree`) | per affiliate/day `released_amount, released_earnings` | MV over releases |
| `affiliate_daily_adjustments` (`SummingMergeTree`) | per affiliate/day `credited_amount, debited_amount, adjustment_count` | MV over adjustments audit |

## 4. Dashboards & reports

### Grafana `grafana-dashboard-affiliate` (ClickHouse datasource)

- **Funnel** — panel 5 "Conversion Funnel" + panel 6 "Daily Funnel"
  (`affiliate_daily_aggregates` sums + `affiliate_funnel_hourly` timeseries;
  `click_to_reg / reg_to_ftd / ftd_to_active` rates; `greatest(den,1)` guards).
- **NGR trend** — panel 7 "NGR vs Commission Accrued (daily)" on
  `affiliate_daily_aggregates`; panel 13 "Accrued vs Released" joins
  `affiliate_daily_releases` (runbook **AFF-RB-01**: released flat while
  accrued rises = hold-release stalled); panel 16 "Hold-Release Stale > 48h".
- **Payout aging** — panel 9 "Payouts Awaiting Review > 48h" on
  `affiliate_payouts WHERE status IN ('requested','reviewing')`;
  panel 15 "Payout Transitions" on `affiliate_payout_transitions`
  (runbook **AFF-RB-02**). Amount totals join `requested` rows (lifecycle
  rows carry `amount=0`).
- Also: panel 3 paid totals (`status='paid'`), panel 10 fraud heatmap,
  panel 14 adjustments-by-admin (fraud/audit of the new
  `ReverseCommission` / adjustment flows), panels 1–2–4–8 NGR/commission/FTD stats.

### Python analytics (`affiliate_reports.py`, read-only queries)

`build_funnel_query` (funnel), `build_ngr_trend_query` +
`build_daily_summary_query` (NGR trend), `build_payout_aging_query`
(`status NOT IN ('paid','rejected','failed)` ordered by `requested_at`),
plus `build_top_affiliates_query`, `build_ltv_query`, `build_earnings_report_query`
(status allowlist includes `reversed`), `summarize_earnings_by_status`.
Money formatted via `decimal_str` (8dp strings); all values parameterized
(`%(name)s`, no interpolation).

## 5. Gaps & remainders (not fixed here)

1. **No `affiliate.player.ftd` event** — `ftd_count` in PG aggregates and
   `ftd_at`/`is_ftd_qualified` in CH attributions stay unset; funnel panels
   undercount FTD until the backend ships FTD emission (+6th queue/MV in 004).
2. **No `affiliate.enrollment.rejected` emission** — only `requested`/`approved`
   are written to the outbox; rejected enrollments are invisible downstream.
3. **PG `affiliate_daily_aggregates` has no `commission_released` column**
   (only CH does) — PG↔CH reconciliation for released must use
   `affiliate_earnings(status=available)` or the event stream, not the PG aggregate.
4. **Mobile AuthInterceptor still reads the Hive box** (`secure_storage`)
   directly — after the token migration in
   `auth_local_datasource_impl.dart` it must be repointed at
   `AuthLocalDataSource`/secure storage (file outside this task's scope).
5. **Push subscribe route** `POST /api/v1/notifications/push/subscribe` must
   exist on the API gateway (notification-service is gRPC-only); FCM server
   key stays in env (`PUSH_FIREBASE_KEY`), production sender is still the
   `LogPushSender` stub.
