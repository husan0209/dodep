# Runbook: RG Service (Responsible Gambling)

Сервис: `rg` — enforcement gate (limits, self-exclusion, time-outs).
Порты: `8091` (HTTP) / `50062` (gRPC). Namespace: `platform`.
Чарт: `infra/helm/charts/rg`. ArgoCD: `dev-rg` / `staging-rg` / `production-rg`.
API: `docs/api/rg.md`. Нагрузка: `tools/testing/k6/scenarios/rg-service.js`.

RG сидит на критическом пути каждой ставки, открытия игры и депозита.
Отказ RG = невозможность играть. Отказ «на разрешить» = регуляторный инцидент.
Сервис **fail-closed**: при недоступности gate игнорировать нельзя, но и
«молча разрешать» нельзя — решение принимает вызывающий сервис по своей
политике деградации.

Быстрые проверки:

```bash
kubectl get pods -n platform -l app.kubernetes.io/name=rg
kubectl logs -n platform -l app.kubernetes.io/name=rg --tail=100
curl -f http://rg.platform.svc.cluster.local:8091/health
curl -f http://rg.platform.svc.cluster.local:8091/ready
```

Таблицы: `rg_service_limits`, `rg_service_pending_changes`,
`rg_service_exclusions`, `rg_service_timeouts`, `rg_service_spend_daily`,
`rg_service_outbox`.

---

## RG-RB-01: Массовые 403 на bet/game_launch (игроки не могут играть)

**Критичность:** P1 — выручка и игровой трафик останавливаются.
**Симптомы:** `RG_SELF_EXCLUDED` / `RG_COOL_OFF` / `RG_*_LIMIT` в логах
betting-engine и casino; рост `403` на `/api/v1/rg/check`.

### Диагностика

```bash
# 1. Жив ли сервис и не деградировал ли он по БД
kubectl get endpoints svc/rg -n platform
kubectl logs -n platform -l app.kubernetes.io/name=rg | grep -iE "error|timeout" | tail -50

# 2. Распределение reason-кодов (быстрый срез по логам).
#    Полный список: RG_SELF_EXCLUDED, RG_COOL_OFF, RG_DEPOSIT_LIMIT,
#    RG_LOSS_LIMIT, RG_WAGER_LIMIT, RG_SESSION_LIMIT.
kubectl logs -n platform -l app.kubernetes.io/name=rg --tail=5000 \
  | grep -oE "RG_(SELF_EXCLUDED|COOL_OFF|DEPOSIT_LIMIT|LOSS_LIMIT|WAGER_LIMIT|SESSION_LIMIT)" \
  | sort | uniq -c | sort -rn

# 3. Разыгрываются ли лимиты (частая причина — баг применения повышений)
psql "$DATABASE_URL" -c \
  "SELECT limit_type, count(*) FROM rg_service_pending_changes \
   WHERE status='pending' AND effective_at <= now() GROUP BY limit_type;"
```

### Действия

1. `RG_SELF_EXCLUDED` массово → проверить, не включился ли плановый
   `apply-due`/массовая операция: `SELECT count(*) FROM rg_service_exclusions
   WHERE status='active' AND created_at > now() - interval '15 minutes';`
2. `RG_COOL_OFF` массово → найти свежие time-outs:
   `SELECT user_id, until FROM rg_service_timeouts WHERE until > now()
   ORDER BY created_at DESC LIMIT 50;`
3. Лимиты «залипли» в pending → вручную прогнать поддерживаемую операцию
   `POST /admin/rg/apply-due` (не SQL-апдейтом: сломается
   `rg_service_outbox` и аудит).
4. Если сервис не готов (БД/Redis) → `kubectl rollout restart deployment/rg
   -n platform` после устранения причины. Не отключать gate: это либо
   остановка игры, либо снятие защиты.
5. Эскалация: не resolved за 15 мин → On-call lead; за 30 мин →
   продукт-команда (потенциальная регуляторная выборка по отказам).

---

## RG-RB-02: Повышение лимита не применилось (pending не уходит в applied)

**Критичность:** P3 (клиентский эффект, не выручка).
**Симптомы:** игрок видит «повышение вступит в силу позже», а лимит не
меняется дольше cooling-периода (default 24h).

### Диагностика

```bash
psql "$DATABASE_URL" -c \
  "SELECT limit_type, status, effective_at, created_at FROM rg_service_pending_changes \
   WHERE status='pending' ORDER BY effective_at LIMIT 50;"

# Жива ли задача применения (в проде её дёргает POST /admin/rg/apply-due)
kubectl logs -n platform -l app.kubernetes.io/name=rg | grep -i "apply" | tail -30
```

### Действия

1. `effective_at` в будущем → это ожидаемое поведение, cooling не истёк.
2. `effective_at` в прошлом → прогнать `POST /admin/rg/apply-due` вручную
   (admin JWT), проверить ответ `{"applied": n}` и статус строк.
3. Повышенный лимит не должен применяться, пока игрок не запросил
   подтверждение: смотреть `rg.limit.increase_cancelled` в логах.
4. Эскалация: повторно не применилось после ручного прогона → владелец
   RG-сервиса.

---

## RG-RB-03: Клиент жалуется на «не могу играть», а в логах всё зелёное

**Критичность:** P2 (support-эскалация).
**Симптомы:** конкретный `user_id` получает отказ, глобальной деградации нет.

### Диагностика

```sql
-- Активное исключение?
SELECT id, type, status, until, permanent, created_at
FROM rg_service_exclusions
WHERE user_id = <ID> AND status = 'active' ORDER BY created_at DESC;

-- Активный time-out?
SELECT id, until FROM rg_service_timeouts WHERE user_id = <ID> AND until > now();

-- Текущие лимиты и висящие повышения
SELECT * FROM rg_service_limits WHERE user_id = <ID>;
SELECT limit_type, old_value, new_value, effective_at, status
FROM rg_service_pending_changes WHERE user_id = <ID> AND status = 'pending';
```

### Действия

1. Исключение `active` → **снимать нельзя**, если срок не истёк. Для
   `permanent` отмена через API невозможна by design (UKGC/GAMSTOP).
2. Временное истёкшее исключение снимается только через
   `POST /api/v1/rg/self-exclusion/revoke` с `{"confirm": true}` —
   и только после cooling-периода.
3. `RG_*_LIMIT` → показать игроку остаток (`remaining_amount` в ответе gate)
   и объяснить, что понижение лимита действует сразу.
4. Вывод средств при исключении **не блокируется** — если клиент жалуется,
   что не может вывести, это баг RG (P1), см. RG-RB-04.

---

## RG-RB-04: Вывод средств заблокирован (нарушение правил)

**Критичность:** P1 + регуляторный инцидент.
**Симптомы:** `withdrawal` получает `403` от gate. По контракту
вывод не должен блокироваться никогда (`CHANNEL_WITHDRAWAL` — audit only).

### Диагностика

```bash
kubectl logs -n platform -l app.kubernetes.io/name=rg \
  | grep -i "withdrawal" | tail -50
```

### Действия

1. Воспроизвести: `POST /api/v1/rg/check {"channel":"withdrawal"}` —
   обязан вернуть `allowed: true`.
2. Если возвращает `403` — немедленный rollback последнего деплоя RG
   (`argocd app rollback production-rg` или `helm rollback`), затем
   фикс и ре-деплой.
3. Зафиксировать инцидент: количество и список затронутых `user_id`,
   время окна — материал для регуляторного отчёта.
4. Эскалация сразу: On-call lead + Compliance.

---

## RG-RB-05: Outbox не разбирается (события RG не уходят в Redpanda)

**Критичность:** P2.
**Симптомы:** состояние в БД изменилось, downstream (notification, CRM,
analytics) не видит `rg.exclusion.started` / `rg.limit.set`.

### Диагностика

```sql
SELECT topic, count(*), min(created_at), max(created_at)
FROM rg_service_outbox
WHERE published_at IS NULL
GROUP BY topic ORDER BY 2 DESC LIMIT 20;
```

### Действия

1. Отсутствие публикатора в dev/staging ожидаемо (`LogPublisher`) — там
   события только логируются.
2. В production: проверить доступность Redpanda, затем дождаться ретрая
   outbox-воркера; ручная пере публикация — только через сервис, не SQL.
3. Эскалация: очередь растёт дольше 10 минут → владелец RG-сервиса.

---

## Точки входа (для on-call)

| Метод | Путь | Что делает |
| ------ | ---- | ---------- |
| POST | `/api/v1/rg/check` | Enforcement gate (`channel`: `bet` \| `game_launch` \| `deposit` \| `withdrawal`) |
| GET | `/api/v1/rg/limits` | Текущие лимиты + висящие повышения |
| PUT | `/api/v1/rg/limits` | Изменить лимиты (понижение — сразу, повышение — через cooling) |
| POST | `/api/v1/rg/timeout` | Cooling-off time-out (`24h`/`48h`/`7d`/`30d`) |
| POST | `/api/v1/rg/self-exclusion` | Самоисключение |
| POST | `/api/v1/rg/self-exclusion/revoke` | Снять **истёкшее временное** исключение (`{"confirm": true}`) |
| POST | `/admin/rg/apply-due` | Применить наступившие повышения лимитов (scheduler) |
| POST | `/admin/rg/exclusions` | Исключение от оператора/регулятора |

## Деградация и откат

- **Откат сервиса:** `argocd app rollback production-rg`, либо
  `helm rollback rg -n platform <revision>`. Canary-конфигурация
  применяется на проде через Argo Rollouts (`cd-production.yml`).
- **Откат схемы:** миграции RG backward-compatible (только ADD). Откат
  приложения не требует отката схемы.
- **Чего нельзя делать:** править `rg_service_exclusions` /
  `rg_service_limits` напрямую SQL-апдейтами — это ломает outbox, аудит и
  согласуется с регуляторными требованиями к записям.