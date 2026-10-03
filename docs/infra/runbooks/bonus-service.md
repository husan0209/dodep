# Runbook: Bonus Service (welcome bonuses, wagering, conversion credit)

Сервис: `bonus` — приветственные бонусы, вейджинг, кредит конвертации.
Порты: `8088` (HTTP) / `50056` (gRPC). Namespace: `platform`.
Чарт: `infra/helm/charts/bonus`. API: `docs/api/bonus.md`.
Метрики: `bonus_*` на `/metrics` (ServiceMonitor включён).
Алерты: `infra/k8s/monitoring/alerting/bonus-alerting-rules.yaml`.
Нагрузка: `tools/testing/k6/scenarios/bonus-service.js`.

Денежный контур: **award → wagering → conversion credit**. Именно последний
шаг переводит бонус в реальные деньги игрока, и именно он исторически
ломался тихо: бонус помечался `completed`, а кредит в `wallet-core` не
уходил. Сейчас credit выполняется **до** записи в БД, но алерты на
`bonus_conversion_credits_total{result="failed"}` всё равно нужны — отказ
`wallet-core` в рантайме точно так же оставляет игрока без денег.

Быстрые проверки:

```bash
kubectl get pods -n platform -l app.kubernetes.io/name=bonus
kubectl logs -n platform -l app.kubernetes.io/name=bonus --tail=100
curl -f http://bonus.platform.svc.cluster.local:8088/health
curl -f http://bonus.platform.svc.cluster.local:8088/ready
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep '^bonus_'
```

Таблицы: `bonuses` (см. `internal/repository`), плюс состояние выдачи в
`wallet-core` приходит по gRPC.

Алерт, который всегда первым к делу: **`bonus-completed-without-credit`**.

---

## BON-RB-01: Bonus Service Not Ready

**Критичность:** P1 — приветственные бонусы не начисляются, вейджинг не пишется.
**Симптомы:** `bonus-no-ready-pods` / `bonus-crash-looping`; партнёрский кабинет
(`/api/v1/bonuses/*`) отдаёт 503 или таймаут.

### Диагностика

```bash
# 1. Живы ли поды и почему не ready
kubectl get pods -n platform -l app.kubernetes.io/name=bonus
kubectl describe pod -n platform -l app.kubernetes.io/name=bonus | tail -40

# 2. Что именно не проходит: liveness (/health) или readiness (/ready)
kubectl logs -n platform -l app.kubernetes.io/name=bonus --tail=200 | grep -iE "error|fatal|panic"

# 3. Жива ли БД (сервис fail-fast на подключении)
kubectl exec -n platform deploy/bonus -- sh -c 'nc -z -w3 postgres.platform.svc.cluster.local 5432 && echo pg-ok'

# 4. Не блокирует ли NetworkPolicy метрики (scraping -> вечные no-op алерты)
kubectl get networkpolicy -n platform -l app.kubernetes.io/name=bonus -o yaml | grep -A3 monitoring
```

### Лечение

```bash
# 1. Однократный рестарт, если CrashLoop из-за застрявшего коннекта
kubectl rollout restart deployment/bonus -n platform

# 2. Если БД мигрировали несовместимо — откатить миграцию (libs/migrations),
#    затем рестарт. Не применять миграции вручную на проде.

# 3. Проверить, что после рестарта поток возобновился
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_payment_events_total
```

**Важно:** рестарт НЕ приводит к двойному начислению — consumer идемпотентен
по `event_id`, а кредит конвертации имеет ключ `bonus-wagering-<bonus_id>`.

---

## BON-RB-02: Conversion credit failed (игрок остался без денег)

**Критичность:** P1 — деньги, которые игрок заработал бонусом, не начислены.
**Симптомы:** `bonus-conversion-credit-failed`, `bonus-completed-without-credit`
или `bonus-grpc-errors`. В логах `bonus.*` — ошибки gRPC к `wallet-core`.

### Диагностика

```bash
# 1. Кому именно не начислили и с каким результатом
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_conversion_credits_total

# 2. Живой ли wallet-core и на том ли адресе
kubectl get endpoints svc/wallet-core -n platform
kubectl exec -n platform deploy/bonus -- sh -c 'nc -z -w3 wallet-core.platform.svc.cluster.local 50053 && echo wallet-ok'

# 3. Что отвечает wallet-core в логах
kubectl logs -n platform -l app.kubernetes.io/name=wallet-core --tail=200 | grep -iE "error|credit|refus"

# 4. Есть ли бонусы, застрявшие в completed без кредита (точечная сверка)
```

```sql
-- Бонусы, завершённые, но с нулевым кредитом: их надо доначислить вручную
-- через wallet-core с ключом bonus-wagering-<id>, идемпотентно.
SELECT id, user_id, currency, amount, type, completed_at
FROM bonuses
WHERE status = 'completed'
  AND completed_at > now() - interval '1 hour'
ORDER BY completed_at DESC;
```

### Лечение

```bash
# 1. Адрес кошелька задаётся переменной, проверяем фактическое значение
kubectl get deploy bonus -n platform -o jsonpath='{.spec.template.spec.containers[0].env}' | tr ',' '\n' | grep WALLET

# 2. Если адрес указывал на другой namespace — исправить values.yaml
#    (WALLET_GRPC_ADDR), НЕ хардкодить в манифесте

# 3. Перезапустить, чтобы consumer доиграл очередь
kubectl rollout restart deployment/bonus -n platform

# 4. Убедиться, что failed перестал расти
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep 'conversion_credits.*failed'
```

**Не делать:** не начислять вручную, пока не подтверждено, что кредит
не прошёл — повторное начисление вернёт игроку деньги дважды. Сначала
сверить по `bonus-wagering-<id>` в wallet-core.

---

## BON-RB-03: Wager credit failed

**Критичность:** P2 — игроки играют против бонуса, который не оплачен.
Тот же разбор покрывает высокую долю 5xx и превышение бюджета p95.
**Симптомы:** `bonus-wager-credit-failed`, `bonus-http-5xx-rate`,
`bonus-http-p95-latency` (бюджет p95 < 500 мс).

### Диагностика

```bash
# 1. Какие операции деградировали (route-лейбл, не сырой путь)
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_http_requests_total

# 2. Распределение латентности по хендлерам
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_http_request_duration_seconds_bucket | head

# 3. Логи хендлеров и внешних вызовов
kubectl logs -n platform -l app.kubernetes.io/name=bonus --tail=300 | grep -iE "wallet|timeout|context deadline"
```

### Лечение

```bash
# 1. Проверить лимиты на пользователя (при 429 latency выглядит как ошибка)
curl -si http://bonus.platform.svc.cluster.local:8088/api/v1/bonuses/active -H "Authorization: Bearer $TOKEN" | grep -i ratelimit

# 2. Проверить, что RateLimit-заголовки вообще есть: их отсутствие означает,
#    что запрос шёл мимо нашего middleware

# 3. Проверить лимиты БД (медленные запросы видны по p95)
kubectl exec -n platform deploy/bonus -- sh -c 'psql "$DATABASE_URL" -c "select 1"'
```

**Важно:** 401/404/409/429 в k6-сценарии — ожидаемые ответы, а не сбои.
Смотреть надо на долю 5xx, а не на общий `http_req_failed`.

---

## BON-RB-04: Metrics endpoint missing

**Критичность:** P2 — сам сервис может быть жив, но все `bonus_*` алерты
слепые: они возвращают пустые серии и не срабатывают.
**Симптомы:** `bonus-metrics-absent` (этот алерт и есть индикатор проблемы).

### Диагностика

```bash
# 1. Экспозиция отвечает?
kubectl exec -n platform deploy/bonus -- sh -c 'wget -qO- localhost:8088/metrics | grep -c "^bonus_"'

# 2. ServiceMonitor вообще выбран сервисом?
kubectl get servicemonitor -n monitoring -o yaml | grep -B5 -A5 bonus

# 3. Именованный порт metrics есть ли в Service (без него target не находится)
kubectl get svc bonus -n platform -o jsonpath='{.spec.ports}' | tr ',' '\n' | grep -i metrics
```

### Лечение

```bash
# Порт metrics объявлен через service.extraPorts в values.yaml чарта.
# Убедиться, что он указывает на тот же containerPort, что и HTTP (8088):
# экспозиция /metrics живёт на основном HTTP-порту.
helm get values bonus -n platform | grep -A6 extraPorts

# После правки values — helm upgrade, затем проверить targets в Grafana
```

**Почему это важно:** молчащая потеря метрик хуже отсутствия алерта —
алерты формально есть, но не могут сработать. Правило `bonus-metrics-absent`
существует именно для этого класса отказов.

---

## BON-RB-05: Welcome bonus not awarded

**Критичность:** P3 — маркетинговые потери, деньги игроков не затронуты.
**Симптомы:** `bonus-award-invalid-amount`,
`bonus-payment-events-failing`.

### Диагностика

```bash
# 1. Почему отказы: invalid_amount (конфиг) или уже_awarded (норма)?
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_bonuses_award_skipped_total

# 2. Читаются ли переменные бонуса сервисом
kubectl get deploy bonus -n platform -o jsonpath='{.spec.template.spec.containers[0].env}' \
  | tr ',' '\n' | grep WELCOME_BONUS

# 3. Доходят ли события депозита
curl -fs http://bonus.platform.svc.cluster.local:8088/metrics | grep bonus_payment_events_total
```

### Лечение

```bash
# invalid_amount почти всегда означает, что значение не парсится как
# положительный decimal (NEVER-6: деньги только decimal, никакого float).
# Проверить формат значения в values.yaml, поправить и НЕ менять на float.

# skipped_not_first — это норма для повторных депозитов: бонус выдаётся
# только на первом. Рост этого лейбла при отсутствии роста awarded
# означает, что фильтр «первый депозит» перестал работать.
```

**Не делать:** не «компенсировать» выдачу вручную, пока не выяснено,
почему award отклоняется — иначе часть игроков получит бонус дважды
(`already_awarded` существует именно как защита).