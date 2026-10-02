# Runbook: Affiliate Service (RevShare from NGR)

Сервис: `affiliate-service` — enrollment, attribution, NGR-комиссии, payouts.
Порты: `8090` (HTTP) / `50061` (gRPC). Namespace: `platform`.
Чарт: `infra/helm/charts/affiliate`. ArgoCD: `dev-affiliate` / `production-affiliate`.
Нагрузка: `tools/testing/k6/scenarios/affiliate.js`.

Быстрые проверки:

```bash
kubectl get pods -n platform -l app.kubernetes.io/name=affiliate
kubectl logs -n platform -l app.kubernetes.io/name=affiliate --tail=100
curl -f http://affiliate.platform.svc.cluster.local:8090/health
curl -f http://affiliate.platform.svc.cluster.local:8090/ready
```

---

## AFF-RB-01: Hold-release stuck (accrued не переходят в available)

**Критичность:** P2
**Симптомы:** `affiliate_earnings` копятся в `accrued` дольше `hold_period_days + 2h`;
партнеры жалуются на нулевой `available` при растущем `pending`.

### Диагностика

```bash
# 1. Жив ли scheduler (HOLD_RELEASE_INTERVAL, default 1h)
kubectl logs -n platform -l app.kubernetes.io/name=affiliate | grep -i "hold-release"

# 2. Сколько зависло
psql "$DATABASE_URL" -c \
  "SELECT count(*) FROM affiliate_earnings WHERE status='accrued' AND hold_until < now() - interval '2 hours';"

# 3. Не встал ли outbox (релиз без публикации = тихий долг)
psql "$DATABASE_URL" -c \
  "SELECT count(*) FROM affiliate_outbox WHERE published_at IS NULL AND created_at < now() - interval '10 minutes';"
```

### Действия

1. Если в логах `Hold-release cycle failed` — смотреть ошибку БД/Redis, чинить
   зависимость, следующий тик подберет хвост сам.
2. Если scheduler молчит (нет строк за 2+ интервала) — рестарт деплоя:
   `kubectl rollout restart deployment/affiliate -n platform`.
3. Ручной прогон — только через поддерживаемую операцию (не SQL-апдейт
   статусов напрямую: сломаешь ledger-сверку `pending/available/paid`).
4. Эскалация: не resolved за 60 мин → On-call lead.

---

## AFF-RB-02: Payout queue aging (заявки висят в requested)

**Критичность:** P2
**Симптомы:** `affiliate_payouts` в `requested/reviewing` старше 48h;
партнеры эскалируют выплаты.

### Диагностика

```bash
psql "$DATABASE_URL" -c \
  "SELECT id, affiliate_id, amount, requested_at FROM affiliate_payouts \
   WHERE status IN ('requested','reviewing') AND requested_at < now() - interval '48 hours' \
   ORDER BY requested_at LIMIT 20;"

# Открытые fraud-флаги по этим партнерам?
psql "$DATABASE_URL" -c \
  "SELECT affiliate_id, count(*) FROM affiliate_fraud_flags \
   WHERE status='open' GROUP BY affiliate_id;"
```

### Действия

1. Выплаты с `open` fraud-флагами — не аппрувить, отдать risk_manager
   (`affiliate.fraud.review`).
2. Остальные — штатный admin flow approve/reject, не в обход gates
   (KYC, min payout, hold period).
3. Если очередь растет системно — проверить capacity finance/risk ролей,
   не править статусы SQL напрямую (audit trail обязателен).

---

## AFF-RB-03: NGR consumer lag (комиссии не начисляются)

**Критичность:** P1 (прямая потеря выручки партнеров)
**Симптомы:** settled bets/rounds идут, а `affiliate_earnings` не растут;
`CommissionsFail` растет в логах.

### Диагностика

```bash
# 1. Консьюмер жив и на каких топиках
kubectl logs -n platform -l app.kubernetes.io/name=affiliate | grep -i "consumer\|commission calculated" | tail -30

# 2. Лаг консьюмер-группы affiliate-service
kubectl exec -n data redpanda-0 -- \
  rpk group describe affiliate-service 2>/dev/null || \
  rpk group describe affiliate-service-production

# 3. Redpanda доступен из platform?
kubectl exec -n platform deploy/affiliate -- \
  nc -zv redpanda.data.svc.cluster.local 9092
```

### Действия

1. Лаг из-за упавшего брокера → чинить Redpanda (см. RB-007/RB-009),
   consumer догонит сам (idempotent по `idempotency_key`).
2. `invalid affiliate_id` в логах — битая атрибуция, не ретраить вручную;
   проверить `affiliate_attributions` на дубли (один referred user =
   один affiliate).
3. `affiliate not found` — партнер suspended/closed; начисления пропускаются
   штатно, проверить карточку партнера в админке.

---

## AFF-RB-04: Click tracking down (`/r/*` отдает 5xx)

**Критичность:** P1 (теряется партнерский трафик здесь и сейчас)
**Симптомы:** 5xx на `/r/{code}`; падение `affiliate_click_tracked`.

### Диагностика

```bash
curl -o /dev/null -s -w "%{http_code}\n" https://api.opus.casino/r/HEALTHCHECK
kubectl get virtualservice affiliate-api-routing -n platform -o yaml
kubectl get authorizationpolicy affiliate-service-policy -n platform -o yaml
```

### Действия

1. 5xx только на `/r/*`, API цело → смотреть `affiliate-api-routing`
   VirtualService и `affiliate-service-policy` AuthorizationPolicy.
2. Все лежит → RB-006/RB-007 по деплою `affiliate`.
3. Откат чарта: `helm rollback affiliate -n platform` или revert через
   ArgoCD (`production-affiliate`, manual sync).

---

## AFF-RB-05: Scheduled reconciliation failing

**Критичность:** P1
**Симптомы:** CronJob `affiliate-ledger-reconciliation` в `data` падает
(`diverging_affiliates > 0`), либо в логах `FAIL: affiliate ledger divergence`.

Это единственная автоматическая проверка баланса affiliate-ledger. Go-эквивалент
(`ReconcileLedger`) существует только как admin-эндпоинт и запускается только
если его вручную дёрнуть.

### Диагностика

```bash
# 1. Недавние прогоны
kubectl get jobs -n data -l app.kubernetes.io/component=affiliate-reconciliation
kubectl logs -n data job/affiliate-ledger-reconciliation-<ts> | tail -50

# 2. Конкретные расхождения (постранично)
kubectl logs -n data job/affiliate-ledger-reconciliation-<ts> \
  | sed -n '/divergence detail/,/FAIL/p'
```

### Действия

1. **Не «чинить» баланс руками.** Расхождение materialized vs derived означает
   либо потерянную проводку, либо лишнюю. Сначала найти источник:
   - нет строки в `affiliate_ledger_entries` → пропала проводка (`postLedgerTx`)
   - строка есть, а баланс не изменился → не сработал `version`/optimistic lock
   - расходится только `paid` → payout прошёл у PSP, а проводка упала (см. AFF-RB-02)
2. Сверять с `GET /admin/affiliates/{id}/ledger/reconciliation` — тот же расчёт
   по конкретному партнёру.
3. `backoffLimit: 0` намеренно: повторные прогоны замаскировали бы реальное
   расхождение за ретраями. Не поднимать без разбора.
4. Эскалация: не resolved за 30 мин → On-call lead + Finance.

### Почему нельзя «откатить» расхождение

Ledger — источник правды для affiliate-денег, но он производный от
earnings/payouts/adjustments. Откат баланса в `affiliate_ledger_accounts`
разорвёт соответствие и замаскирует баг. Правильный порядок: починить
проводку, затем пересчитать.

---

## Откат релиза

```bash
helm history affiliate -n platform
helm rollback affiliate <REVISION> -n platform
# либо ArgoCD: production-affiliate -> History and Rollback (manual sync)
kubectl rollout status deployment/affiliate -n platform --timeout=300s
```

Проверка после отката: `/health`, `/ready`, k6-сценарий
`tools/testing/k6/scenarios/affiliate.js` с `BASE_URL` на прод-сервис.
