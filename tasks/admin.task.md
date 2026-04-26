# ТЗ Addendum v1.1: DOD Admin Panel
## Дополнения к основному ТЗ v1.0

**Статус:** Критические gaps закрыты, важные дополнены  
**Применяется к:** dod-admin-tz.md v1.0

---

## СТРУКТУРА ДОПОЛНЕНИЙ

```
Приоритет A — нужны ДО запуска (критические)
Приоритет B — первые итерации после запуска  
Приоритет C — nice to have, откладываем
```

---

# ПРИОРИТЕТ A: КРИТИЧЕСКИЕ ДОПОЛНЕНИЯ

---

## A1. ФАЗА 1 — Foundation: доработки

### A1.1 Session Management Policy

```sql
-- Добавить в admin_sessions:
ALTER TABLE admin_sessions 
  ADD COLUMN device_fingerprint TEXT,
  ADD COLUMN country_code        CHAR(2),
  ADD COLUMN user_agent          TEXT;

-- Policy: один активный refresh token на устройство
-- При новом логине с того же device → инвалидировать предыдущий
-- При логине с нового device → email alert + optional TOTP re-verify
-- При логине из новой страны → email alert (всегда)
```

```go
// Concurrent sessions config (в settings, без deploy)
type SessionPolicy struct {
    MaxConcurrentSessions int  // 0 = unlimited, 1 = single session
    AlertOnNewCountry     bool
    AlertOnNewDevice      bool
    RequireTOTPOnNewDevice bool
}
```

### A1.2 Rate Limiting Rules

```yaml
# DragonflyDB keys + TTL

login:
  per_ip:    10 req / 15 min  → lockout 30 min
  per_email:  5 req / 15 min  → lockout 30 min
  totp_fail:  5 attempts      → lockout 30 min + notify SUPER_ADMIN

api:
  general:          1000 req / min / admin
  bulk_export:         5 req / hour / admin  (тяжёлые запросы)
  withdrawal_approve: 100 req / hour / admin (защита от скриптов)
  balance_adjust:      20 req / hour / admin

Headers в ответе:
  X-RateLimit-Limit: N
  X-RateLimit-Remaining: M
  X-RateLimit-Reset: Unix timestamp
```

### A1.3 Multi-Currency Display

```typescript
// Глобальная настройка per-admin (хранится в admin profile)
interface CurrencyDisplaySettings {
  primary: 'USD' | 'EUR'           // всегда показывать в этой валюте
  showOriginal: boolean            // и оригинальную рядом
}

// Пример отображения в UI:
// BRL 2,450.00  (≈ $490 USD)
// TRY 15,200.00 (≈ $530 USD)

// BFF endpoint: GET /admin/settings/exchange-rates
// Источник курсов: внутренний сервис (Binance/CoinGecko feed)
// Cache: DragonflyDB 60 сек
```

```go
// BFF helper
func FormatAmount(amount decimal.Decimal, currency string, 
                  displayCurrency string, rates map[string]decimal.Decimal) AmountDisplay {
    return AmountDisplay{
        Original:         amount,
        OriginalCurrency: currency,
        Converted:        amount.Mul(rates[currency+"/"+displayCurrency]),
        DisplayCurrency:  displayCurrency,
    }
}
```

### A1.4 Maintenance Mode Middleware

```go
// Добавить в middleware stack BFF (после Auth, перед RBAC)
func MaintenanceMiddleware(cache *dragonfly.Client) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("claims").(*AdminClaims)
        if claims.Role == "SUPER_ADMIN" {
            return c.Next() // SUPER_ADMIN обходит maintenance
        }
        
        key := "admin:maintenance"
        active, _ := cache.Get(c.Context(), key).Bool()
        if !active {
            return c.Next()
        }
        
        eta, _ := cache.Get(c.Context(), key+":eta").Result()
        return c.Status(503).JSON(fiber.Map{
            "error":   "MAINTENANCE_MODE",
            "message": "System under scheduled maintenance",
            "eta":     eta,
        })
    }
}

// Settings endpoint (SUPER_ADMIN only):
// POST /admin/settings/maintenance
// body: {enabled: bool, message: string, eta: "2025-04-25T15:00:00Z"}
```

---

## A2. ФАЗА 4 — KYC: доработки

### A2.1 Document Expiry Tracking

```sql
-- Добавить в kyc_documents:
ALTER TABLE kyc_documents
  ADD COLUMN document_expires_at    DATE,
  ADD COLUMN expiry_reminder_30d_at TIMESTAMPTZ,
  ADD COLUMN expiry_reminder_7d_at  TIMESTAMPTZ;

-- Celery/Scheduler job (ежедневно в 06:00 UTC):
-- 1. Найти документы где expires_at BETWEEN now() AND now()+30d
--    AND expiry_reminder_30d_at IS NULL
--    → отправить email игроку, записать timestamp
-- 2. Expires_at < now() → статус expired, заблокировать вывод
```

```typescript
// KYC Expiry Monitor widget (в KYC раздел)
interface ExpiryMonitor {
  expiring_30d: number   // документов истекает через 30 дней
  expiring_7d:  number   // через 7 дней
  expired:      number   // уже истекли (активные игроки)
}

// GET /admin/kyc/expiry-stats
// GET /admin/kyc/expiring?days=30&page=1
```

### A2.2 PEP / Sanctions Screening

```go
// Интеграция: ComplyAdvantage (уже в стеке для AML)
// Дополнительно: Chainalysis для крипто

type ScreeningResult struct {
    PlayerID    uuid.UUID
    Status      ScreeningStatus  // clear | pep_match | sanctions_hit | review_required
    MatchedLists []string        // ["OFAC", "EU_SANCTIONS", "PEP_TIER_1"]
    MatchScore  float64          // 0-100
    ScreenedAt  time.Time
    NextScreenAt time.Time       // ежемесячный rescreening
    RawResponse jsonb
}

type ScreeningStatus string
const (
    ScreeningClear     ScreeningStatus = "clear"
    ScreeningPEP       ScreeningStatus = "pep_match"
    ScreeningSanctions ScreeningStatus = "sanctions_hit"  // → auto-block + freeze
    ScreeningReview    ScreeningStatus = "review_required"
)
```

```sql
CREATE TABLE player_screenings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL,
    status          TEXT NOT NULL,
    matched_lists   TEXT[],
    match_score     NUMERIC(5,2),
    screened_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_screen_at  TIMESTAMPTZ,
    screened_by     TEXT NOT NULL,  -- 'auto' | admin_email
    raw_response    JSONB,
    reviewed_by     UUID,           -- admin_id кто review сделал
    reviewed_at     TIMESTAMPTZ,
    review_notes    TEXT
);

CREATE INDEX player_screenings_player_idx ON player_screenings(player_id, screened_at DESC);
CREATE INDEX player_screenings_status_idx ON player_screenings(status) 
    WHERE status != 'clear';
```

```
UI в профиле игрока (PEP/Sanctions блок):
  Status badge: 🟢 Clear | 🟡 PEP | 🔴 Sanctions
  Last screened: дата | [Re-screen now]
  
  При sanctions_hit:
    Автоблокировка происходит мгновенно
    Freeze всех pending withdrawals
    Alert всем RISK_MANAGER и FINANCE_MANAGER
    Требует ручного review перед любыми действиями

Screening Queue (в KYC разделе):
  Все матчи со статусом review_required или pep_match
  RISK_MANAGER review: [False Positive] [Confirmed PEP - Enhanced DD] [Confirmed Sanctions]
```

### A2.3 KYC Team Metrics

```
KYC Dashboard (видит KYC_OFFICER и выше):

┌────────────────────────────────────────────────┐
│  KYC Operations — Today                         │
├─────────────┬─────────────┬────────────────────┤
│ Queue Depth │ Avg Review  │ SLA Breaches        │
│     67      │  4.2 min    │  3 (>24h) ⚠️       │
├─────────────┴─────────────┴────────────────────┤
│  By Officer (this week):                        │
│  Officer  │ Reviewed │ Avg Time │ Approve% │    │
│  Sarah    │ 89       │ 3.8 min  │ 82%      │    │
│  Mike     │ 74       │ 4.7 min  │ 73%      │    │
└────────────────────────────────────────────────┘

GET /admin/kyc/team-stats?period=today|week|month
```

---

## A3. ФАЗА 5 — Payments: доработки

### A3.1 Chargeback Management (полный workflow)

```sql
CREATE TABLE chargebacks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL,
    transaction_id  UUID NOT NULL,         -- исходный депозит
    amount          NUMERIC(18,2) NOT NULL,
    currency        CHAR(3) NOT NULL,
    gateway         TEXT NOT NULL,
    gateway_cb_id   TEXT,                  -- ID чарджбэка у шлюза
    reason_code     TEXT,                  -- код от банка (e.g. "4853")
    reason_text     TEXT,
    status          TEXT NOT NULL,         -- received|under_review|accepted|fighting|won|lost
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deadline_at     TIMESTAMPTZ,           -- дедлайн для оспаривания (обычно 20-45 дней)
    resolved_at     TIMESTAMPTZ,
    assigned_to     UUID,                  -- admin_id
    fight_evidence  JSONB,                 -- прикреплённые документы
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX chargebacks_player_idx  ON chargebacks(player_id);
CREATE INDEX chargebacks_status_idx  ON chargebacks(status) WHERE status NOT IN ('won','lost');
```

```go
// Webhook handler (от payment gateway)
// POST /internal/webhooks/chargeback  (не admin, внутренний endpoint)
func HandleChargebackWebhook(payload ChargebackWebhookPayload) error {
    // 1. Создать запись в chargebacks
    // 2. Автоматически:
    cb.AutoActions{
        BlockPlayer(payload.PlayerID, "chargeback_received"),
        FreezeWithdrawals(payload.PlayerID),
        AddTag(payload.PlayerID, "Chargeback"),
        AddRiskScore(payload.PlayerID, +25),
        NotifyRole("FINANCE_MANAGER", alertMsg),
    }
    // 3. Публикуем в Redpanda → admin.risk.alerts
}
```

```typescript
// UI: Chargeback Queue (FINANCE_MANAGER)

interface ChargebackQueueItem {
  id:           string
  player:       PlayerSummary
  amount:       AmountDisplay
  gateway:      string
  reason:       string
  status:       ChargebackStatus
  deadlineDays: number      // сколько дней до дедлайна
  waitHours:    number
}

// Chargeback Detail Drawer:
// Левая панель: детали CB (сумма, причина, дедлайн)
// Правая панель: досье для защиты:
//   - KYC документы игрока
//   - IP логи сессии
//   - Транзакционная история (доказательство использования)
//   - T&C acceptance log
//   - Game session при депозите

// Actions:
// [Accept CB]     → деньги списываются, игрок разблокируется (или остаётся blocked)
// [Fight CB]      → прикрепить документы, отправить в gateway
// [Assign to me]  → взять в работу

// Chargeback Stats widget (в Finance Report):
//   Total CB this month: N | Amount: $X
//   CB Rate: X% (target < 0.5%)  ← Visa/MC threshold
//   Fight Win Rate: X%
```

### A3.2 Platform Balance Sheet

```go
// GET /admin/finance/balance-sheet
type BalanceSheet struct {
    AsOf time.Time
    
    // Что ДОЛЖНЫ игрокам (liabilities)
    Liabilities struct {
        PlayerBalances    decimal.Decimal
        BonusBalances     decimal.Decimal
        FSWinningsBalance decimal.Decimal
        PendingWithdrawals decimal.Decimal
        Total             decimal.Decimal
    }
    
    // Где физически деньги (assets)
    Assets struct {
        Gateways    []GatewayBalance  // {name, balance, currency}
        CryptoHot   []WalletBalance   // {coin, amount, usd_equivalent}
        CryptoCold  []WalletBalance
        BankAccount decimal.Decimal
        Total       decimal.Decimal
    }
    
    // Ключевой индикатор
    CoverageRatio float64  // Assets.Total / Liabilities.Total
    // < 1.0 → CRITICAL ALERT
    // 1.0-1.2 → WARNING
    // > 1.2 → OK
}
```

```typescript
// Balance Sheet widget на Finance Dashboard
// Обновление: каждые 15 минут (DragonflyDB cache)
// При CoverageRatio < 1.0 → красная плашка + email SUPER_ADMIN + FINANCE_MANAGER
```

### A3.3 Crypto Wallet Management

```go
// GET /admin/finance/crypto/wallets
type CryptoWalletStatus struct {
    Coin         string
    HotBalance   decimal.Decimal
    ColdBalance  decimal.Decimal
    
    // Правило: hot wallet > 20% от среднего дневного вывода
    DailyWithdrawalAvg decimal.Decimal  // скользящее среднее 7 дней
    HotWalletThreshold decimal.Decimal  // = DailyWithdrawalAvg * 0.20
    IsLow              bool             // HotBalance < HotWalletThreshold
    
    PendingDeposits  int              // ожидают confirmations
    PendingWithdraws int
}

// При IsLow=true → алерт FINANCE_MANAGER:
// "BTC hot wallet critically low: $X available, threshold: $Y"

// Crypto compliance check (per deposit):
// Каждый крипто-депозит → проверка через Chainalysis
// Response: {risk_score, categories: ["darknet_market", "mixer"...]}
// Если high_risk → заморозка + алерт RISK_MANAGER
```

```typescript
// Crypto Wallet page (под Financial Management):
// Таблица по монетам: hot/cold balance, статус
// Timeline: pending transactions (confirmations)
// Alert если hot wallet low
// Blockchain explorer link для каждой транзакции
```

---

## A4. НОВАЯ ФАЗА: Support Ticket System

**Вставить между Фазой 5 и Фазой 6. Перенумеровать остальные.**

### Scope

Полноценная внутренняя тикет-система. Не замена live-chat (он в отдельном сервисе), а система для сложных кейсов требующих работы нескольких отделов.

### Схема

```sql
CREATE TABLE support_tickets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL,
    subject         TEXT NOT NULL,
    category        TEXT NOT NULL,   -- payment|bonus|technical|account|kyc|general
    priority        TEXT NOT NULL DEFAULT 'normal',  -- low|normal|high|urgent
    status          TEXT NOT NULL DEFAULT 'open',    -- open|pending_player|pending_internal|resolved|closed
    assigned_to     UUID,            -- admin_id
    created_via     TEXT NOT NULL,   -- chat|email|manual
    source_chat_id  TEXT,            -- ID чата в саппорт-системе
    sla_first_response_at TIMESTAMPTZ,  -- target
    first_response_at     TIMESTAMPTZ,  -- actual
    sla_resolve_at        TIMESTAMPTZ,
    resolved_at           TIMESTAMPTZ,
    closed_at             TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE ticket_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES support_tickets(id),
    author_type TEXT NOT NULL,   -- 'player' | 'admin'
    author_id   UUID NOT NULL,
    is_internal BOOLEAN DEFAULT false,  -- true = internal note, не видна игроку
    body        TEXT NOT NULL,
    attachments JSONB,           -- [{url, name, size}]
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE ticket_links (
    ticket_id      UUID NOT NULL,
    entity_type    TEXT NOT NULL,  -- 'withdrawal' | 'deposit' | 'bonus' | 'bet'
    entity_id      UUID NOT NULL,
    PRIMARY KEY (ticket_id, entity_type, entity_id)
);

-- Индексы
CREATE INDEX tickets_player_idx  ON support_tickets(player_id);
CREATE INDEX tickets_status_idx  ON support_tickets(status) WHERE status NOT IN ('resolved','closed');
CREATE INDEX tickets_assigned_idx ON support_tickets(assigned_to) WHERE status = 'open';
```

### UI

```
Support Module Layout:
├── Ticket List (фильтры: status, category, priority, agent, date)
├── Ticket Detail (открывается справа / в drawer)
│   ├── Header: player info, status, priority, SLA timer
│   ├── Message thread (публичные + internal notes)
│   ├── Sidebar: player summary (баланс, GGR, теги, риск)
│   ├── Linked entities (транзакции, бонусы привязанные к тикету)
│   └── Actions: [Reply] [Internal Note] [Assign] [Change Priority] [Resolve] [Link Transaction]
└── Team Dashboard (Team Lead view)
    ├── Tickets by status (bar chart)
    ├── SLA breach list
    ├── Agent workload (tickets per agent)
    └── Avg resolution time (today | week | month)

SLA Configuration (в Settings):
  Category  │ First Response │ Resolution
  ──────────┼────────────────┼───────────
  payment   │ 30 min         │ 4 hours
  bonus     │ 2 hours        │ 24 hours
  kyc       │ 1 hour         │ 24 hours
  general   │ 4 hours        │ 48 hours
  
  VIP priority → все тикеты автоматически "high"
  
SLA breach → алерт Team Lead + FINANCE_MANAGER (для payment tickets)
```

### Auto-priority Rules

```go
// При создании тикета автоматически:
func DeterminePriority(ticket Ticket, player Player) Priority {
    if player.HasTag("VIP") || player.HasTag("Whale") {
        return PriorityHigh
    }
    if ticket.Category == "payment" && player.HasPendingWithdrawal() {
        return PriorityHigh
    }
    if ticket.Category == "payment" {
        return PriorityNormal
    }
    return PriorityLow
}
```

### API

```
GET  /admin/tickets?status=open&category=payment&page=1
GET  /admin/tickets/:id
POST /admin/tickets              # создать вручную
POST /admin/tickets/:id/messages # ответить / internal note
PUT  /admin/tickets/:id/status
PUT  /admin/tickets/:id/assign
POST /admin/tickets/:id/links    # привязать транзакцию
GET  /admin/tickets/stats        # team dashboard данные
```

---

## A5. ФАЗА 7 — Risk: доработки

### A5.1 Rule Builder (настраиваемые правила без deploy)

```go
// Структура правила (хранится в PostgreSQL как JSONB)
type RiskRule struct {
    ID          uuid.UUID
    Name        string
    Description string
    IsActive    bool
    Priority    int      // порядок выполнения при конфликте
    
    Conditions  []RuleCondition  // AND логика между ними
    Actions     []RuleAction
    
    // Статистика для false positive анализа
    FiredCount      int
    DismissedCount  int
    FPRate          float64  // вычисляется: DismissedCount/FiredCount
}

type RuleCondition struct {
    Field    string   // "deposit_amount" | "wagering_ratio" | "session_count"...
    Operator string   // ">" | "<" | "=" | "between" | "in"
    Value    any
}

type RuleAction struct {
    Type    string   // "trigger_alert" | "hold_withdrawal" | "block_player" | "add_tag" | "notify"
    Params  map[string]any
}
```

```typescript
// Rule Builder UI (drag-and-drop условия):
// 
// IF  [deposit_amount]    [>]        [$1000]
// AND [wagering_ratio]   [<]        [20%]
// AND [session_count]    [between]  [0] [3]
//
// THEN
//   ✅ trigger_alert(severity: HIGH, message: "Low wagering ratio")
//   ✅ hold_withdrawal()
//
// [Test against last 1000 events] → показать сколько бы сработало
// [Save] [Activate]

// Rule Testing:
// POST /admin/risk/rules/test
// body: {rule: RuleDefinition, test_period_days: 30}
// response: {fired_count: N, sample_players: [...]}
```

### A5.2 False Positive Management & Whitelist

```sql
-- При dismiss алерта добавить поле reason_category:
-- genuine_player | vpn_for_privacy | professional_bettor | rule_error | other

ALTER TABLE risk_alerts ADD COLUMN dismiss_reason_category TEXT;

-- Whitelist правил для конкретных игроков
CREATE TABLE risk_rule_whitelist (
    player_id   UUID NOT NULL,
    rule_id     UUID NOT NULL,           -- или alert_type если нет rule_id
    alert_type  TEXT,
    reason      TEXT NOT NULL,
    added_by    UUID NOT NULL,           -- admin_id
    added_at    TIMESTAMPTZ DEFAULT now(),
    expires_at  TIMESTAMPTZ,             -- NULL = бессрочно
    PRIMARY KEY (player_id, rule_id)
);
```

```
False Positive Analytics (в Risk Settings):

Rule Performance Table:
  Rule Name         │ Fired (30d) │ Dismissed │ FP Rate │ Action
  ──────────────────┼─────────────┼───────────┼─────────┼────────
  VPN Detection     │ 234         │ 189       │ 81% ⚠️  │ [Review Rule]
  Multi-Account     │  45         │   3       │  7% ✅  │ [Details]
  Low Wagering      │  89         │  34       │ 38% 🟡  │ [Tune]

При FP Rate > 70% → автоматическое предложение: 
"Rule 'X' имеет высокий false positive rate. Рассмотрите ужесточение условий."
```

---

## A6. ФАЗА 8 — Sportsbook: доработки

### A6.1 Trading Terminal View

```typescript
// Отдельный layout для SPORTS_TRADER (не sidebar-based)
// URL: /admin/sportsbook/terminal

interface TradingTerminalLayout {
  leftPanel:   'EventsTree'           // Sport → League → Event иерархия
  centerPanel: 'EventMarketsTable'    // все маркеты + коэффициенты + ставки
  rightPanel:  'LiabilityMonitor'     // liability для выбранного события
  topBar:      'LiveScoreFeed'        // счёт + статистика матча
}

// Keyboard shortcuts (настраиваемые):
// S → Suspend выбранный маркет
// R → Resume выбранный маркет
// A → Suspend всё событие
// +/- → коэффициент ±0.05 для выбранного исхода
// Enter → применить изменение
// Esc → отменить

// Все shortcuts логируются в audit (reason = "keyboard_shortcut")
```

### A6.2 Live Score Integration

```go
// Sportradar уже в стеке — он даёт и scores, и коэффициенты

type LiveEventData struct {
    EventID     string
    SportID     string
    HomeTeam    string
    AwayTeam    string
    Score       Score
    Minute      int
    Period      string    // "1H" | "2H" | "HT" | "FT"
    Statistics  EventStats
    LastUpdated time.Time
}

type EventStats struct {
    HomeShots        int
    AwayShots        int
    HomeCorners      int
    AwayCorners      int
    HomeYellowCards  int
    AwayYellowCards  int
    HomeRedCards     int
    AwayRedCards     int
    HomePossession   float64
}
```

```typescript
// Score widget в Trading Terminal
// ⚽ Real Madrid 2 - 1 Barcelona  [65'] 2H
// Corners: 6-4 | Shots: 8-5 | Poss: 58%-42%

// WebSocket push из Rust Gateway (topic: sportsbook.live.scores)
```

---

## A7. ФАЗА 11 — CRM: доработки

### A7.1 Suppression Lists

```sql
CREATE TABLE communication_suppressions (
    player_id   UUID NOT NULL,
    reason      TEXT NOT NULL,  -- unsubscribed|hard_bounce|spam_complaint|self_excluded|gdpr_erasure
    channel     TEXT,           -- NULL = все каналы, иначе 'email'|'sms'|'push'
    added_at    TIMESTAMPTZ DEFAULT now(),
    added_by    TEXT NOT NULL,  -- 'player' | 'auto' | admin_email
    expires_at  TIMESTAMPTZ,    -- NULL = бессрочно
    PRIMARY KEY (player_id, reason, COALESCE(channel, 'all'))
);
```

```go
// BFF middleware для отправки — ВСЕГДА проверять перед отправкой
func IsSuppressed(playerID uuid.UUID, channel string, db *pgxpool.Pool) (bool, string) {
    var reason string
    err := db.QueryRow(ctx, `
        SELECT reason FROM communication_suppressions
        WHERE player_id = $1
          AND (channel IS NULL OR channel = $2)
          AND (expires_at IS NULL OR expires_at > now())
        LIMIT 1
    `, playerID, channel).Scan(&reason)
    
    if err == pgx.ErrNoRows {
        return false, ""
    }
    return true, reason
}

// При срабатывании → логировать как "skipped: suppression_reason"
// Это важно для compliance аудита
```

### A7.2 RG + Marketing Suppression связка

```go
// CRM Campaign/Trigger перед отправкой ОБЯЗАТЕЛЬНО:
func FilterRGSuppressed(playerIDs []uuid.UUID) []uuid.UUID {
    // Исключить:
    // 1. Self-excluded
    // 2. Cool-off активен
    // 3. RG Score > настраиваемого порога (default: 70)
    // 4. Самоисключённые (communication_suppressions.reason = 'self_excluded')
    
    // Логировать количество исключённых + причину
    // "Campaign 'Weekend Reload': 234 players excluded due to RG restrictions"
}
```

```typescript
// При создании кампании — preview с breakdown:
// Target segment: 5,420 players
// ─────────────────────────────────
// ✅ Will receive:      4,891
// ❌ RG excluded:         234  (self-excluded: 45, cool-off: 67, high RG score: 122)
// ❌ Unsubscribed:        185
// ❌ Frequency cap:       110
// ─────────────────────────────────
// Final reach:          4,891
```

### A7.3 Communication Frequency Caps

```go
// Настройки (в CRM Settings, без deploy):
type FrequencyCaps struct {
    EmailPerDay   int  // default: 1
    EmailPerWeek  int  // default: 3
    SMSPerDay     int  // default: 0
    SMSPerWeek    int  // default: 2
    PushPerDay    int  // default: 3
    PushPerHour   int  // default: 1
}

// DragonflyDB counter per player per channel per period:
// key: "comm:cap:{player_id}:{channel}:{YYYY-MM-DD}"
// value: count | TTL: 24h (для daily), 7d (для weekly)

// При отправке: проверить cap → если превышен → skip + log
```

---

# ПРИОРИТЕТ B: ВАЖНЫЕ ДОПОЛНЕНИЯ (после запуска)

---

## B1. ФАЗА 2 — Dashboard: доработки

### B1.1 Shift Report

```typescript
// Кнопка "End Shift" в header (видна всем операторам)
// При нажатии → генерируется PDF отчёт

interface ShiftReport {
  operator:      AdminUser
  shiftStart:    Date
  shiftEnd:      Date
  
  financial: {
    depositsCount:    number
    depositsAmount:   AmountDisplay
    withdrawalsApproved: { count: number, amount: AmountDisplay }
    withdrawalsDeclined: { count: number, amount: AmountDisplay }
    p2pProcessed:     { count: number, amount: AmountDisplay }
  }
  
  operations: {
    kycVerified:    number
    kycRejected:    number
    playersBlocked: number
    riskAlerts:     { total: number, handled: number, dismissed: number }
    ticketsResolved: number
  }
  
  notes: string  // поле для передачи информации следующей смене
}

// POST /admin/reports/shift
// Автоматически отправляется в Telegram группу операций
```

### B1.2 GEO Map Widget

```typescript
// WorldMap component (D3 или react-simple-maps)
// Источник: ClickHouse, active sessions per country (15 мин cache)

// GET /admin/dashboard/geo-distribution
// Response: [{country_code: "DE", active_players: 234, ggr_today: 12400}]

// Клик по стране → открывает Players List с фильтром country=DE + status=online
// Tooltip: флаг, название, онлайн кол-во, GGR за сегодня
```

---

## B2. ФАЗА 3 — Players: доработки

### B2.1 Player Merge

```go
// Критическая операция: SUPER_ADMIN + TOTP + второе подтверждение

// POST /admin/players/merge
type MergeRequest struct {
    PrimaryID   uuid.UUID  // этот аккаунт остаётся активным
    SecondaryID uuid.UUID  // этот блокируется после merge
    Reason      string
    TOTPCode    string
    ConfirmedBy uuid.UUID  // второй SUPER_ADMIN
}

// Merge операция выполняется как транзакция:
// 1. Перевести баланс secondary → primary (если > 0)
// 2. Обновить player_id в: bets, transactions, bonuses, kyc_documents
// 3. Скопировать теги (union)
// 4. Заблокировать secondary аккаунт (reason: merged_into:primary_id)
// 5. Создать admin_note на primary: "Merged from account #{secondary_id}"
// 6. Написать полный audit trail

// Preview перед merge:
// GET /admin/players/merge/preview?primary={id}&secondary={id}
// Response: конфликты данных, итоговое состояние
```

### B2.2 Unified Communication Timeline

```typescript
// Добавить Tab в Player Profile: "Communications"
// Объединённая лента всех коммуникаций:

interface CommunicationEntry {
  type:      'email' | 'sms' | 'push' | 'chat' | 'call'
  date:      Date
  subject:   string
  preview:   string
  channel:   string
  status:    'sent' | 'delivered' | 'opened' | 'clicked' | 'failed' | 'suppressed'
  campaign?: string  // если из CRM кампании
}

// GET /admin/players/:id/communications?page=1
```

---

## B3. ФАЗА 9 — Casino: доработки

### B3.1 Provider Revenue Settlement

```sql
CREATE TABLE provider_settlements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     UUID NOT NULL,
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    ggr             NUMERIC(18,2) NOT NULL,
    revenue_share_pct NUMERIC(5,2) NOT NULL,
    amount_owed     NUMERIC(18,2) NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending|approved|paid
    paid_at         TIMESTAMPTZ,
    paid_by         UUID,
    invoice_url     TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

```typescript
// Provider Settlements page (FINANCE_MANAGER)
// Таблица по провайдерам за период:
// Provider | GGR | Revenue Share % | Owed | Status | Actions
//
// [Calculate Period] → создать записи за прошлый месяц
// [Export Invoice Summary] → CSV/Excel для бухгалтерии
// [Mark as Paid] → после фактической оплаты
```

---

## B4. ФАЗА 10 — Affiliates: доработки

### B4.1 Postback Configuration

```sql
-- Добавить к affiliate таблице:
ALTER TABLE affiliates ADD COLUMN postback_configs JSONB DEFAULT '[]';

-- Структура postback_configs:
-- [{
--   "event": "registration" | "ftd" | "deposit" | "redeposit",
--   "url": "https://partner.com/postback?...",
--   "method": "GET" | "POST",
--   "variables": ["{click_id}", "{player_id}", "{amount}"],
--   "retry_count": 3,
--   "retry_backoff": "exponential"
-- }]
```

```go
// Postback Log
type PostbackLog struct {
    ID          uuid.UUID
    AffiliateID uuid.UUID
    Event       string
    PlayerID    uuid.UUID
    URL         string          // с подставленными переменными
    HTTPStatus  int
    Response    string
    AttemptNo   int
    SentAt      time.Time
    Success     bool
}

// Retry worker: при !Success → retry с exponential backoff (1min, 5min, 30min)
// После 3 неудач → alert AFFILIATE_MANAGER + логировать как failed
```

```typescript
// Postback Logs view в профиле партнёра (Tab):
// Event | Player | Sent At | Status | Attempt | [Replay]
// [Replay] → ручная повторная отправка
```

---

# ПРИОРИТЕТ C: NICE TO HAVE (откладываем)

```
C1. Player Merge                    → B2.1 (перенесён в Приоритет B)
C2. Custom Report Builder           → Фаза 12, итерация 2
C3. Custom Markets (Sportsbook)     → Фаза 8, итерация 2
C4. Affiliate Media Library         → Фаза 10, итерация 2
C5. Game Demo/Testing Mode          → Фаза 9, итерация 2

C6. In-app Changelog для операторов:
    При деплое → запись в changelog таблицу
    При входе → popup "What's new" если есть непрочитанные
    Simple: {version, date, items: [{type: 'fix'|'new'|'change', text}]}
    
C7. Alert Sound Settings:
    Per-admin настройка (localStorage)
    Do Not Disturb часы
    Разные звуки по severity
    Реализация: простой React hook + Web Audio API
```

---

# ОБНОВЛЁННЫЙ ПЛАН ФАЗ

```
Фаза  │ Модуль                              │ Добавлено
──────┼─────────────────────────────────────┼─────────────────────────────
1     │ Foundation & Infrastructure         │ Session policy, rate limits,
      │                                     │ multi-currency, maintenance
──────┼─────────────────────────────────────┼─────────────────────────────
2     │ Dashboard                           │ + GEO Map (B)
──────┼─────────────────────────────────────┼─────────────────────────────
3     │ Player Management                   │ + Unified comm timeline (B),
      │                                     │ + Player Merge (B)
──────┼─────────────────────────────────────┼─────────────────────────────
4     │ KYC & Responsible Gambling          │ + Doc expiry tracking (A),
      │                                     │ + PEP/Sanctions screening (A),
      │                                     │ + KYC team metrics (A)
──────┼─────────────────────────────────────┼─────────────────────────────
5     │ Payment Management                  │ + Chargeback workflow (A),
      │                                     │ + Balance Sheet (A),
      │                                     │ + Crypto wallet mgmt (A)
──────┼─────────────────────────────────────┼─────────────────────────────
6     │ Support Ticket System               │ НОВАЯ ФАЗА (A)
──────┼─────────────────────────────────────┼─────────────────────────────
7     │ Bonus Engine                        │ (без изменений)
──────┼─────────────────────────────────────┼─────────────────────────────
8     │ Risk & Anti-Fraud                   │ + Rule Builder (A),
      │                                     │ + False Positive mgmt (A)
──────┼─────────────────────────────────────┼─────────────────────────────
9     │ Sportsbook Management               │ + Trading Terminal (B),
      │                                     │ + Live Score feed (B)
──────┼─────────────────────────────────────┼─────────────────────────────
10    │ Casino Games Management             │ + Provider settlements (B)
──────┼─────────────────────────────────────┼─────────────────────────────
11    │ Affiliate Management                │ + Postback config detail (B)
──────┼─────────────────────────────────────┼─────────────────────────────
12    │ CRM & Retention                     │ + Suppression lists (A),
      │                                     │ + RG+marketing link (A),
      │                                     │ + Frequency caps (A)
──────┼─────────────────────────────────────┼─────────────────────────────
13    │ Analytics & System Settings         │ + Shift Reports (B)
──────┴─────────────────────────────────────┴─────────────────────────────

Итого: 13 фаз (добавилась Фаза 6 — Support Ticket System)
       Это обоснованно: Support — отдельный domain со своей схемой, SLA, UI
```

---

## ИТОГОВОЕ ПОКРЫТИЕ

```
После применения addendum v1.1:

✅ Foundation + Auth + Session + Rate Limiting
✅ Dashboard + GEO Map + Provider Health
✅ Player Management + Merge + Communication Timeline
✅ KYC + Doc Expiry + PEP/Sanctions + Team Metrics
✅ Payments + Chargebacks + Balance Sheet + Crypto
✅ Support Ticket System (новая фаза)
✅ Bonus Engine + Wagering Validation
✅ Risk + Rule Builder + False Positive Management
✅ Sportsbook + Trading Terminal + Live Scores
✅ Casino + Provider Settlements
✅ Affiliates + Postback Configuration
✅ CRM + Suppression + RG Link + Frequency Caps
✅ Analytics + Shift Reports + Scheduled Reports

Покрытие: ~97% функциональности production-уровня admin панели
```

---

*Addendum v1.1 — применять вместе с dod-admin-tz.md v1.0*
# ТЗ: DOD Admin Panel
## Техническое задание — полная версия

**Версия:** 1.0  
**Рынок:** Mixed (серый + легальный)  
**Разработка:** Solo, фундаментальный подход  
**Горизонт:** 12 фаз, без deadline pressure

---

## 0. КОНТЕКСТ И ЦЕЛИ

### Что строим

Полнофункциональная внутренняя операционная платформа для управления гемблинг-бизнесом (казино + букмекер). Не просто CRUD-интерфейс — это ERP-уровня инструмент с real-time данными, audit trail, гранулярными ролями и интеграцией во все микросервисы DOD-платформы.

### Принципы проектирования

```
Density over verbosity     — каждый экран максимально информативен
Action where data is       — операции прямо в контексте (не переходить куда-то)
Audit everything           — каждое действие логируется, ничто не удаляется
Fail loudly                — ошибки явные, не скрытые
Real-time by default       — данные живые, не требуют refresh
```

### Scope ограничения (что НЕ входит в admin)

- Сам игровой фронтенд (Next.js / Flutter — отдельные проекты)
- ML-модели (Python сервисы — отдельно, admin только потребляет их output)
- Платёжные процессинговые ядра (Go-сервис Payment — admin только UI над ним)

---

## 1. ТЕХНИЧЕСКИЙ СТЕК ADMIN PANEL

```yaml
frontend:
  framework:    React 18 + TypeScript 5
  ui_library:   Ant Design 5.x
  state:        Zustand (client) + TanStack Query v5 (server)
  real_time:    WebSocket клиент (к Rust WS Gateway)
  charts:       Recharts + Ant Design Charts
  tables:       Ant Design Table (виртуализация для 100K+ строк)
  routing:      React Router v6
  i18n:         react-i18next (EN/RU минимум)
  bundler:      Vite 5
  testing:      Vitest + Playwright

api_layer:
  transport:    REST/JSON (внешний) + gRPC-web (внутренние сервисы)
  auth:         JWT (Ed25519) + refresh token rotation
  real_time:    WebSocket → Rust Gateway → Redpanda topics

backend_for_frontend:
  # Admin НЕ общается напрямую с Rust/Go сервисами
  # Все запросы идут через Admin BFF (Go/Fiber)
  bff_language: Go (Fiber)
  bff_role:     Auth check, роль-фильтрация, агрегация данных из N сервисов
  bff_db:       Читает из PostgreSQL (OLTP) + ClickHouse (аналитика)

infrastructure:
  deploy:       Отдельный K8s deployment (namespace: admin)
  access:       IP whitelist + VPN only (не публичный интернет)
  cdn:          Нет (внутренний инструмент)
  ssl:          cert-manager (внутренний CA)
```

### Архитектура Admin BFF

```
Browser (React)
    ↓ HTTPS + JWT
Admin BFF (Go/Fiber) ← единственная точка входа
    ├── Auth check (Vault + JWT verify)
    ├── RBAC filter (проверка роли + разрешений)
    ├── Audit log writer (все мутирующие запросы)
    └── Fan-out к сервисам:
        ├── User Service (Go) — gRPC
        ├── Payment Service (Go) — gRPC
        ├── Betting Engine (Rust) — gRPC
        ├── Risk Engine (Rust) — gRPC
        ├── Bonus Service (Go) — gRPC
        ├── Analytics (ClickHouse) — прямой SQL через BFF
        └── WebSocket Gateway (Rust) — для real-time push
```

---

## 2. RBAC — СИСТЕМА РОЛЕЙ

```
Роли (от полного доступа к минимальному):

SUPER_ADMIN       — полный доступ, только 1-2 человека
FINANCE_MANAGER   — payments, reports, финансовые настройки
RISK_MANAGER      — антифрод, блокировки, KYC решения
CRM_MANAGER       — бонусы, сегменты, рассылки
SPORTS_TRADER     — коэффициенты, лимиты, сусид маркеты
SUPPORT_AGENT     — читать профили, базовые чат-операции
KYC_OFFICER       — только KYC очередь и документы
AFFILIATE_MANAGER — только партнёрский раздел
CONTENT_MANAGER   — только CMS
VIEWER            — read-only dashboard + отчёты
```

### Матрица разрешений (ключевые операции)

| Операция | SUPER | FIN | RISK | CRM | TRADER | SUPPORT |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Approve withdrawal | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Block player | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| Adjust balance | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Create bonus | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Edit odds | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| View player profile | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| View financials | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Change RTP config | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Export reports | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Manage admin roles | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

---

## 3. AUDIT LOG — ТРЕБОВАНИЯ

Каждый HTTP запрос в BFF, изменяющий состояние (POST/PUT/PATCH/DELETE), обязан:

```go
// Структура записи аудита
type AuditEntry struct {
    ID          uuid.UUID   // immutable
    Timestamp   time.Time
    AdminID     uuid.UUID
    AdminEmail  string
    AdminRole   string
    IPAddress   string
    UserAgent   string
    Action      string      // "player.block", "withdrawal.approve" etc.
    EntityType  string      // "player", "withdrawal", "bonus"...
    EntityID    string
    Before      jsonb       // состояние до
    After       jsonb       // состояние после
    Reason      string      // обязателен для критических операций
    TraceID     string      // OpenTelemetry trace
}
```

**Хранение:** PostgreSQL (append-only таблица, без DELETE/UPDATE прав у app-user) + архивация в S3 через 90 дней.

**Critical actions** (требуют 2FA подтверждения при выполнении):
- Approve withdrawal > $10,000
- Block VIP / Whale игрока
- Изменение RTP конфигурации
- Изменение лимитов платёжных методов
- Adjust balance (любой)

---

## 4. REAL-TIME АРХИТЕКТУРА

```
Rust WebSocket Gateway → Redpanda Topics → Admin BFF → Browser WebSocket

Topics для admin:
  admin.metrics.live      — GGR/NGR/deposits каждые 5 сек
  admin.risk.alerts       — новые риск-алерты мгновенно
  admin.withdrawals.new   — новая заявка на вывод
  admin.kyc.submitted     — новый KYC документ
  admin.provider.health   — статус провайдеров
  admin.bets.large        — ставки > threshold (real-time)
```

Browser держит один постоянный WebSocket. BFF подписывается на Redpanda topics и мультиплексирует в подключённые admin-сессии по ролям (RISK_MANAGER видит risk.alerts, FINANCE видит withdrawals.new и т.д.).

---

# ФАЗЫ РАЗРАБОТКИ

---

## ФАЗА 1: Foundation & Infrastructure
**Цель:** Рабочий скелет, на который нанизываются все остальные фазы.

### Deliverables

**1.1 Admin BFF (Go/Fiber)**
```
endpoints:
  POST /admin/auth/login          # email + password + TOTP
  POST /admin/auth/refresh        # refresh token
  POST /admin/auth/logout
  GET  /admin/auth/me             # текущий admin профиль
  WS   /admin/ws                  # WebSocket соединение

middleware stack:
  1. Rate limiting (DragonflyDB)
  2. JWT verify + role extraction
  3. IP whitelist check
  4. Request ID inject
  5. Audit log (для мутирующих запросов)
  6. OpenTelemetry tracing
```

**1.2 RBAC модуль**
```go
// Permissions как битовые маски или enum, НЕ строки в коде
type Permission uint64
const (
    PermWithdrawalView    Permission = 1 << 0
    PermWithdrawalApprove Permission = 1 << 1
    PermPlayerBlock       Permission = 1 << 2
    PermBalanceAdjust     Permission = 1 << 3
    // ...
)

// Role = набор Permission
type Role struct {
    Name        string
    Permissions Permission // bitmask
}
```

**1.3 Схема БД (admin-специфичная)**
```sql
-- Admin пользователи (отдельная таблица, НЕ players)
CREATE TABLE admin_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,      -- Argon2id
    totp_secret TEXT,                 -- зашифрован Vault
    role        TEXT NOT NULL,
    permissions BIGINT NOT NULL,      -- bitmask
    is_active   BOOLEAN DEFAULT true,
    ip_whitelist TEXT[],              -- разрешённые IP
    created_at  TIMESTAMPTZ DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    last_login_ip TEXT
);

-- Audit log (append-only)
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ts          TIMESTAMPTZ NOT NULL DEFAULT now(),
    admin_id    UUID NOT NULL,
    admin_email TEXT NOT NULL,
    admin_role  TEXT NOT NULL,
    ip_address  TEXT NOT NULL,
    action      TEXT NOT NULL,
    entity_type TEXT,
    entity_id   TEXT,
    before_state JSONB,
    after_state  JSONB,
    reason      TEXT,
    trace_id    TEXT
);
CREATE INDEX audit_log_admin_id_idx ON audit_log(admin_id);
CREATE INDEX audit_log_entity_idx   ON audit_log(entity_type, entity_id);
CREATE INDEX audit_log_ts_idx       ON audit_log(ts DESC);
-- Запретить DELETE на уровне PostgreSQL row security

-- Admin сессии / refresh tokens
CREATE TABLE admin_sessions (
    id              UUID PRIMARY KEY,
    admin_id        UUID NOT NULL REFERENCES admin_users(id),
    refresh_token   TEXT UNIQUE NOT NULL,  -- bcrypt hash
    expires_at      TIMESTAMPTZ NOT NULL,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    revoked_at      TIMESTAMPTZ            -- null = активна
);
```

**1.4 React приложение — базовый layout**
```
Layout:
  ├── Sidebar (навигация по модулям, свёртываемый)
  ├── Header (текущий admin, роль, logout, 2FA статус, clock UTC)
  ├── Content area
  └── Notification center (real-time алерты)

Роутинг (React Router):
  /                   → redirect → /dashboard
  /dashboard          → Dashboard
  /players            → Players List
  /players/:id        → Player Profile
  /payments           → Payments
  /bonuses            → Bonus Management
  /risk               → Risk & Fraud
  /sportsbook         → Sportsbook
  /casino             → Casino Games
  /affiliates         → Affiliates
  /crm                → CRM & Retention
  /kyc                → KYC Queue
  /reports            → Reports
  /cms                → Content
  /settings           → Settings
  /audit              → Audit Log
  /403                → Access Denied (если роль не имеет доступа)

Guards:
  Каждый роут оборачивается в <RoleGuard required={Permission.xxx}/>
  При отсутствии права → redirect /403
```

**1.5 Auth Flow**
```
Login:
  1. POST /admin/auth/login {email, password}
  2. If TOTP enabled → 401 с флагом totp_required
  3. POST /admin/auth/login {email, password, totp_code}
  4. Ответ: {access_token (15min), refresh_token (7d, httpOnly cookie)}
  5. access_token хранится в memory (НЕ localStorage)

Refresh:
  Каждые 14 минут автоматически (TanStack Query refetch)
  При 401 → interceptor вызывает refresh → retry

Logout:
  Revoke refresh token в БД
  Clear memory
```

### Acceptance Criteria Фазы 1
- [ ] Login с email+password+TOTP работает
- [ ] JWT верифицируется на каждом запросе BFF
- [ ] IP не из whitelist → 403
- [ ] Каждый мутирующий запрос пишет в audit_log
- [ ] WebSocket соединение держится, переподключается при обрыве
- [ ] Sidebar рендерит только разделы, доступные текущей роли
- [ ] Автоmatический logout через 15 минут неактивности

---

## ФАЗА 2: Dashboard
**Цель:** Пульт управления бизнесом в реальном времени.

### Экраны

**2.1 Live Overview (главный)**
```
Метрики верхней полосы (обновление каждые 5 сек через WS):
  Online Players (Casino | Sports | Total)
  GGR Today (Casino | Sports | Live Casino)
  Deposits Today / Withdrawals Today
  FTD Count Today / FTD Amount Today
  Pending Withdrawals (count + sum)
  Pending KYC (count)
  Open Support Tickets

Графики (Recharts):
  GGR/NGR — последние 30 дней (линейный, с toggle сравнения с прошлым периодом)
  Deposits vs Withdrawals — bar chart по часам сегодня
  Conversion Funnel — Visits → Reg → FTD → 2nd Deposit

Таблицы:
  Top 5 Games by GGR (сегодня)
  Top 5 Sports Events by Stakes (сегодня)
  Top 5 Countries by Active Players

Алерты (real-time, из WS topic admin.risk.alerts):
  Отдельная панель, новые алерты — sound + visual badge
```

**2.2 Provider Health Widget**
```
Статус каждого game provider:
  ✅ Online | ⚠️ Degraded | ❌ Down
  Latency (p99 последние 5 минут)
  Error rate (%)
  GGR за сегодня

Статус payment gateways:
  Success rate (последние 15 минут)
  Если success_rate < 80% → жёлтый алерт
  Если success_rate < 60% → красный + push уведомление
```

### API Endpoints (BFF)

```
GET /admin/dashboard/metrics/live
  Response: {
    online: {casino: int, sports: int},
    ggr_today: {casino: decimal, sports: decimal, live: decimal},
    deposits_today: decimal,
    withdrawals_today: decimal,
    ftd_today: {count: int, amount: decimal},
    pending_withdrawals: {count: int, amount: decimal},
    pending_kyc: int
  }
  Cache: DragonflyDB 5 сек

GET /admin/dashboard/charts/ggr?period=30d&compare=prev
GET /admin/dashboard/top-games?limit=5&period=today
GET /admin/dashboard/top-events?limit=5&period=today
GET /admin/dashboard/provider-health

WS: topic admin.metrics.live → push каждые 5 сек
```

### Data Sources
- Метрики: ClickHouse (агрегаты за период) + DragonflyDB (live counters)
- Provider health: VictoriaMetrics (scrape провайдеров) → BFF запрашивает VM API

### Acceptance Criteria Фазы 2
- [ ] Live метрики обновляются без refresh страницы
- [ ] При падении провайдера — алерт на дашборде в течение 10 секунд
- [ ] График GGR корректно сравнивает два периода
- [ ] Алерты из risk engine появляются в реальном времени

---

## ФАЗА 3: Player Management
**Цель:** Полное управление базой игроков.

### 3.1 Player List

```
Фильтры (все комбинируемые):
  country, registration_date (range), kyc_status,
  tags (multi-select), last_login (range),
  deposit_total (range), ggr (range),
  affiliate_id, player_group, balance (range),
  risk_score (range), search (id / email / username / phone)

Колонки таблицы:
  ID | Username | Country | Reg Date | Deposits | GGR | KYC | Tags | Risk | Last Login

Функции:
  Сортировка по любой колонке
  Виртуализация (Ant Design Table + virtual scroll для 100K+ строк)
  CSV/Excel экспорт (с учётом текущих фильтров)
  Bulk actions: добавить тег / изменить группу / отправить email (для CRM_MANAGER)
```

### 3.2 Player Profile (табы)

```
Tab 0: Overview
  Personal: имя, email, phone, DOB, country, city
  Financial summary: deposits total, withdrawals total, net deposits, balance, bonus balance
  Gaming stats: casino GGR, sports GGR, total bets, favorite game, avg bet
  Tags (редактируемые)
  Risk Score (gauge 0-100 + факторы)
  Quick Actions (доступны по роли):
    [Block] [Adjust Limits] [Give Bonus] [Adjust Balance] [Add Note] [Send Message]

Tab 1: Deposits
  Пагинированная таблица: date, amount, method, status, tx_id, gateway
  Фильтр по методу, статусу, дате

Tab 2: Withdrawals
  Аналогично + approved_by

Tab 3: Bets (Casino)
  Дата | Game | Provider | Bet | Win | Balance After | Session ID
  Агрегация по сессиям (collapse/expand)

Tab 4: Bets (Sports)
  Дата | Event | Type (single/combo) | Odds | Stake | Result | Payout

Tab 5: Bonuses
  Дата | Bonus | Amount | Wagering Req | Progress | Status | Expires

Tab 6: KYC Documents
  Тип | Статус | Uploaded At | Reviewed By | Decision | Notes

Tab 7: Responsible Gambling
  Лимиты, установленные игроком
  RG Score trend (график)
  Self-exclusion история

Tab 8: Support History
  Все чаты с агентами (read-only для большинства ролей)

Tab 9: Linked Accounts
  Граф связей (device fingerprint / IP / payment method совпадения)
  Список аккаунтов с указанием типа связи

Tab 10: Admin Notes & Audit
  Заметки от операторов (с автором, датой)
  История admin-действий с этим игроком (из audit_log)
```

### 3.3 Player Actions

```go
// Каждое действие требует reason (строка) для audit log
// Некоторые требуют TOTP подтверждения

POST /admin/players/:id/block
  body: {type: "full"|"casino"|"sports"|"temporary", duration_hours?: int, reason: string}

POST /admin/players/:id/unblock
  body: {reason: string}

POST /admin/players/:id/limits
  body: {
    max_deposit_daily?: decimal,
    max_deposit_weekly?: decimal,
    max_withdrawal_daily?: decimal,
    max_bet?: decimal,
    max_loss_daily?: decimal,
    reason: string
  }

POST /admin/players/:id/adjust-balance   # требует TOTP
  body: {amount: decimal, currency: string, type: "credit"|"debit", reason: string}

POST /admin/players/:id/tags
  body: {add?: string[], remove?: string[], reason: string}

POST /admin/players/:id/group
  body: {group: "standard"|"vip"|"vvip"|"whale", reason: string}

POST /admin/players/:id/notes
  body: {text: string}

POST /admin/players/:id/give-bonus
  body: {bonus_id: uuid, reason: string}

POST /admin/players/:id/request-kyc
  body: {type: "identity"|"address"|"source_of_funds", message?: string}

POST /admin/players/:id/send-message
  body: {channel: "email"|"sms"|"push", subject?: string, body: string}
```

### Схема (расширение player profile в BFF)

```
BFF агрегирует из нескольких Go-сервисов:
  User Service    → personal data, status, tags, group
  Payment Service → deposit/withdrawal history
  Betting Service → bet history, stats
  Bonus Service   → bonus history
  Risk Service    → risk score, alerts, linked accounts
  KYC Service     → documents, status
  Support Service → chat history

Всё это BFF собирает в один response для /admin/players/:id/overview
Параллельные gRPC calls с timeout 2 сек каждый (graceful degradation)
```

### Acceptance Criteria Фазы 3
- [ ] Поиск по 1M+ игроков возвращает результат < 200ms
- [ ] Все 10 табов загружаются лениво (только при открытии)
- [ ] Adjust balance пишет в audit с before/after
- [ ] Linked accounts показывает граф связей
- [ ] Bulk export CSV работает для 50K игроков (async job + download link)

---

## ФАЗА 4: KYC & Responsible Gambling
**Цель:** Управление верификацией игроков и защитой от лудомании.

### 4.1 KYC Queue

```
Очередь с приоритетами:
  HIGH    — есть pending вывод (нельзя вывести без KYC)
  MEDIUM  — превышен deposit threshold
  LOW     — плановая проверка

Колонки: Player | Type | Submitted | Wait Time | Priority | Assigned To | Actions

При открытии заявки:
  Левая панель: изображение документа (zoom, rotate)
  Правая панель: данные из профиля игрока (имя, DOB, страна)
  OCR данные (автоматически распознанные Sumsub)
  Сравнение: OCR ↔ Profile (highlight несовпадений)
  
  Кнопки решения:
  [✅ Approve]
  [❌ Reject: Poor quality]
  [❌ Reject: Data mismatch]
  [❌ Reject: Expired document]
  [❌ Reject: Suspected fake]
  [🔄 Request resubmission]
  
  Поле Notes (обязательно при Reject)
```

### 4.2 Sumsub Integration

```
Sumsub webhook → BFF → KYC Service (Go) → PostgreSQL

Auto-approve: если Sumsub вернул HIGH confidence → автоматически
Manual review: если MEDIUM или LOW → попадает в очередь

BFF endpoint для получения Sumsub SDK URL:
  GET /admin/kyc/sessions/:player_id/sumsub-link
  → генерирует одноразовый токен для просмотра Sumsub dashboard по игроку
```

### 4.3 AML / Source of Funds

```
Настройка триггеров AML:
  Cumulative deposit threshold: $X за N дней → запросить SOF
  Single transaction threshold: $Y → автоматический SAR
  
Управление SOF запросами:
  Список открытых SOF запросов
  Срок предоставления (deadline)
  Если не предоставили → автоблокировка вывода
  
  Принятие документов SOF:
  bank_statement | salary_slip | inheritance | property_sale | crypto_portfolio
```

### 4.4 Responsible Gambling Module

```
RG Dashboard:
  Игроки с высоким RG Score (таблица, убывающий порядок)
  Trend: RG Score за последние 30 дней (кто ухудшился)
  Активные самоисключения
  Превышения лимитов за сегодня

Player RG Settings (из профиля):
  Лимиты (установленные игроком):
    Daily/Weekly/Monthly deposit limit
    Daily/Weekly/Monthly loss limit
    Session time limit
    Reality check frequency
  
  Охлаждение / Self-Exclusion:
    Cool-off: 24h, 7d, 30d
    Self-exclusion: 6 months, 1 year, permanent
  
  Admin override:
    Принудительное самоисключение (RISK_MANAGER только)
    Нельзя снять раньше срока даже SUPER_ADMIN — это compliance требование

Автоматические RG алерты (из Risk Engine):
  - Chasing losses (проиграл X, сразу депозит)
  - Late night sessions (22:00-06:00 > 3 часов)
  - Rapid deposit increase (3x за 7 дней)
  - Session duration > 4 часов
```

### Acceptance Criteria Фазы 4
- [ ] KYC очередь сортируется по приоритету
- [ ] Изображение документа загружается за < 1 сек
- [ ] Решение KYC мгновенно меняет статус вывода игрока
- [ ] RG самоисключение нельзя снять раньше срока (проверка в UI + API)
- [ ] AML триггеры настраиваемы без deploy

---

## ФАЗА 5: Payment Management
**Цель:** Полный контроль над депозитами, выводами и платёжной маршрутизацией.

### 5.1 Deposits

```
Таблица всех депозитов (real-time через WS):
  ID | Player | Amount | Currency | Method | Gateway | Status | Time | Actions

Фильтры: status, method, gateway, country, amount range, date range, player

Детали депозита (popup):
  Все поля транзакции
  Gateway response (raw)
  Timeline: created → sent to gateway → callback received → credited

Manual Credit (редкий случай — если gateway зачислил, но callback не пришёл):
  POST /admin/payments/deposits/:id/manual-credit
  Требует: reason + TOTP
```

### 5.2 Withdrawals Queue — критический модуль

```
Pending Withdrawals (real-time обновление):
  ID | Player | Amount | Method | Wallet/Account | Wait Time | Risk Score | KYC | Actions

Сортировка по умолчанию: по времени ожидания (самые долгие — сверху)

При нажатии [Review]:
  Открывается drawer с:
  
  Левая колонка: детали запроса
    Сумма, метод, реквизиты
    История предыдущих выводов на этот же реквизит
    История депозитов (откуда пришли деньги)
    
  Правая колонка: автоматический чеклист
    ✅/❌ KYC verified
    ✅/❌ Bonus fully wagered
    ✅/❌ Withdrawal method matches deposit method
    ✅/❌ No active chargeback
    ✅/❌ Risk score < 40
    ✅/❌ Amount within limits
    ✅/❌ No open AML flags
    
  Действия:
    [✅ Approve] → немедленно отправляет в payment gateway
    [❌ Decline] → выбрать причину (KYC required / Bonus not wagered / AML hold / Other)
    [⏸ Hold for review] → добавить в отдельную очередь с заметкой
    [🔍 View full player] → открыть профиль в новой вкладке

Авто-approve (настраивается):
  Если risk_score < 20 AND amount < $1,000 AND KYC=verified AND все чеклисты ✅
  → автоматически approve без участия человека
```

### 5.3 Payment Methods Configuration

```
Настройка по странам (матрица):
  Country → Method → {enabled_deposit, enabled_withdrawal, min, max, fee_percent, fee_fixed}

Операции:
  Включить/выключить метод по стране
  Изменить лимиты (min/max)
  Изменить fee
  Установить временное ограничение (например, карты выключены на 2 часа)

Payment Gateway Configuration:
  Priority (order) для cascading
  Timeout settings
  Retry policy
  Health monitoring (success_rate threshold для auto-disable)
```

### 5.4 P2P Payment Management

```
Для серых рынков (Турция и т.д.):

Входящие P2P депозиты:
  Список ожидающих подтверждения
  Игрок | Сумма | Метод (Papara/Bank) | Время создания | Чек (фото/скриншот)
  
  Финансист проверяет поступление вручную → нажимает [Confirm] или [Reject]
  
  KPI финансиста: среднее время обработки P2P

Исходящие P2P выводы:
  Список одобренных выводов, ожидающих ручного перевода
  Реквизиты получателя
  После перевода: [Mark as Sent] + загрузить подтверждение

P2P статистика:
  Количество P2P транзакций в день
  Среднее время обработки
  Суммарный объём
```

### 5.5 Financial Reconciliation

```
Ежедневная сверка:
  Expected balance on gateway vs Actual balance on gateway
  Расхождения — выделяются красным
  
  Причины расхождений:
    Pending transactions
    Failed callbacks
    Chargebacks
  
  Экспорт для бухгалтерии (CSV)
```

### Acceptance Criteria Фазы 5
- [ ] Новый вывод появляется в очереди через WS без refresh
- [ ] Чеклист заполняется автоматически (BFF дёргает нужные сервисы)
- [ ] Auto-approve работает по настроенным правилам
- [ ] P2P очередь обрабатывается корректно
- [ ] Изменение лимитов платёжных методов вступает в силу сразу

---

## ФАЗА 6: Bonus Engine
**Цель:** Создание, управление и мониторинг бонусной системы.

### 6.1 Bonus Constructor

```
Мастер создания бонуса (wizard, 4 шага):

Шаг 1: Основные параметры
  Name, Description, Status (draft/active/paused)
  Bonus Type: deposit_match | free_spins | cashback | freebet | express_boost | tournament
  Valid from / Valid to
  Max uses (глобально / на игрока)

Шаг 2: Условия и суммы
  Deposit Match:
    Match percentage, Max bonus amount, Min deposit
    
  Free Spins:
    Count, Game (выбор из списка), Spin value
    Выдача: immediately | daily (N per day for M days)
    
  Cashback:
    Percentage, Calculation: net_loss | ggr
    Period: daily | weekly | monthly
    Min loss for activation
    
  Freebet:
    Amount, Min odds, Allowed: single | combo | both
    Return stake on win: yes | no

Шаг 3: Вейджер и ограничения
  Wagering multiplier (на что: bonus | deposit+bonus)
  Wagering timeframe (дней)
  Max bet while bonus active
  Max win from bonus
  
  Game weights для вейджера:
    Slots: 100%
    Live Casino: input %
    Table Games: input %
    Sports: input %
    
  Excluded games (multi-select из каталога)
  Sticky vs Non-sticky

Шаг 4: Таргетинг
  Eligible countries (whitelist / blacklist)
  Excluded tags (Bonus Hunter, High Risk, etc.)
  Player groups (standard | vip | all)
  Promo code (optional)
  Auto-assign trigger: on_ftd | on_redeposit | manual | scheduled
  Can be combined with other bonuses: yes | no
  
  Preview математики:
    При вейджере Xx и RTP 96% ожидаемый остаток: $Y
    Ожидаемый cost на 1000 активаций: $Z
```

### 6.2 Bonus List & Management

```
Список всех бонусов:
  Name | Type | Status | Active Count | Total Issued | Total Cost | Conversion Rate

Actions:
  Activate / Pause / Deactivate
  Clone (быстрое создание похожего)
  Edit (только draft или paused)
  View stats (сколько игроков, сколько отыграли, сколько истекло)

Активные бонусы игроков (global view):
  Игрок | Бонус | Issued | Wagering Progress | Expires | Status
  Фильтр по бонусу, статусу, прогрессу отыгрыша
  
  Action: Manual void (если абуз) + reason
```

### 6.3 Wagering Monitor

```
Real-time view активных бонусов в отыгрыше:
  Игроки которые близко к завершению (>80% wagered) — потенциальный вывод скоро
  Игроки с аномально быстрым вейджером (возможный абуз max-bet)
  Истекающие через 24 часа
```

### Acceptance Criteria Фазы 6
- [ ] Bonus constructor валидирует математику (wagering × RTP = expected cost)
- [ ] Auto-assign срабатывает при FTD (интеграция с Payment Service)
- [ ] Max bet violation → автоматический void бонуса
- [ ] Экспорт статистики по бонусу

---

## ФАЗА 7: Risk & Anti-Fraud
**Цель:** Центр управления безопасностью платформы.

### 7.1 Risk Alerts Dashboard

```
Real-time feed алертов (WS topic admin.risk.alerts):
  Severity | Player | Alert Type | Details | Time | Actions

Alert Types:
  MULTI_ACCOUNT    — совпадение fingerprint/IP/payment
  MONEY_LAUNDERING — deposit → min bets → withdrawal
  BONUS_ABUSE      — паттерн bonus hunting
  CHARGEBACK       — получен чарджбэк
  HIGH_WIN_RATE    — спортивный арбитражник
  VELOCITY         — слишком много транзакций
  VPN_PROXY        — использование VPN
  GEO_MISMATCH     — логин из другой страны

Actions на алерт:
  [Dismiss] — снять без действия + reason
  [Block Player] — немедленная блокировка
  [Hold Withdrawal] — удержать конкретный вывод
  [Flag for Review] — поставить в очередь на ручную проверку
  [View Player] — профиль в новой вкладке
```

### 7.2 Multi-Account Detection

```
Граф кластеров (визуализация):
  Узлы = аккаунты
  Рёбра = тип связи (device / ip / payment)
  Цвет узла = статус (active / blocked / suspicious)
  
  При клике на кластер:
    Список аккаунтов с деталями совпадений
    Суммарный депозит / вывод по кластеру
    Bulk action: заблокировать весь кластер

Fingerprint Management:
  Просмотр всех fingerprint по игроку
  Ручная пометка fingerprint как "trusted" (для VPN-пользователей VIP)
```

### 7.3 Risk Scoring Configuration

```
Настройка весов risk score (без deploy):
  Factor                    Current Weight
  VPN/Proxy detected        +10
  Device match other acct   +15
  Payment match other acct  +20
  Chargeback received       +25
  Bonus hunting pattern     +15
  Win rate > 60% sports     +20
  Deposit without play      +10
  ...

Пороги действий:
  score >= 40 → hold withdrawals
  score >= 60 → manual review required
  score >= 80 → auto-block

Preview: изменение весов → пересчёт для 100 последних алертов
```

### 7.4 Blacklists & Watchlists

```
Управление списками:
  Заблокированные IP / IP-подсети
  Заблокированные BIN (первые 6 цифр карты) 
  Заблокированные email домены (временные почты)
  Заблокированные крипто-кошельки (OFAC sanctions)
  Known fraud fingerprints

Watchlist (мониторинг без блокировки):
  Добавить игрока → все его действия автоматически попадают в отдельный feed
```

### Acceptance Criteria Фазы 7
- [ ] Алерты появляются в реальном времени (< 5 сек от события)
- [ ] Граф связей рендерится для кластеров до 50 узлов
- [ ] Изменение весов risk score активно без перезапуска сервисов
- [ ] Bulk block кластера блокирует всех игроков атомарно

---

## ФАЗА 8: Sportsbook Management
**Цель:** Управление букмекерской линией, маржой, лимитами и ответственностью.

### 8.1 Events & Markets

```
Дерево событий:
  Sport → League → Event → Markets → Outcomes

Для каждого события:
  Status: prematch | live | suspended | closed | settled
  Quick actions: [Suspend all markets] [Resume] [Set custom margin]
  
Для каждого market:
  Текущие коэффициенты (с источником: provider | manual override)
  Суммарные ставки по каждому исходу
  Liability (потенциальный убыток если исход выиграет)
  [Suspend market] [Edit odds] [Set market limits]

Live events отдельный tab:
  Только текущие live события
  Обновление коэффициентов в реальном времени (WS)
  Быстрая кнопка suspend для экстренных ситуаций
```

### 8.2 Odds Management

```
Manual override коэффициентов:
  Выбрать event → market → outcome
  Текущий провайдерный коэффициент
  Поле для override (с отображением разницы в %)
  [Apply] [Reset to provider]
  
  Предупреждение если override > 20% от провайдерного (ошибка ли?)

Bulk adjustment:
  Применить маржу X% ко всем событиям выбранного спорта/лиги
```

### 8.3 Margin Settings

```
Иерархия маржи:
  Global default: X%
  ↓ Sport override
  ↓ League override
  ↓ Event override (наивысший приоритет)

Для каждого уровня:
  Prematch margin | Live margin (обычно выше)

Интерфейс:
  Таблица спортов с текущими маржами
  Inline редактирование
  История изменений (кто, когда, что изменил)
```

### 8.4 Limits Management

```
Иерархия лимитов:
  Global → Sport → League → Event → Market → Player Group

  Max stake (макс ставка)
  Max win (макс выигрыш с одной ставки)
  Max liability (макс суммарный риск по исходу)

Player-level limits:
  При пометке игрока как [Arbitrageur] или [Winner]:
  Автоматически устанавливаются индивидуальные лимиты
  Конфигурируемые значения: "Winner max bet = X% от стандартного"
```

### 8.5 Liability Monitor

```
Топ событий по liability прямо сейчас:
  Event | Market | Outcome | Stakes | Potential Liability | Alert Level

Color coding:
  Зелёный: liability < 50% от threshold
  Жёлтый: 50-80%
  Красный: > 80% → нужно действие трейдера

Детальный view события:
  Breakdown по исходам
  Breakdown по группам игроков (обычные vs VIP vs отмеченные)
  График ставок по времени (нет ли подозрительного всплеска)
```

### 8.6 Settlement & Results

```
Расчёт ставок:
  События ожидающие результата → вводится вручную (если провайдер не дал)
  Автоматический расчёт от провайдера (Sportradar)
  
Void & Re-settlement:
  List событий с возможностью void
  Причина (palpable error / event cancelled / weather)
  Re-settlement: если результат изменён (VAR и т.д.)
  
Cashout Configuration:
  Включить/выключить глобально или по спорту
  Cashout margin (%)
  Частичный cashout: разрешить да/нет
```

### Acceptance Criteria Фазы 8
- [ ] Live коэффициенты обновляются в UI в реальном времени
- [ ] Liability monitor корректно считает потенциальный убыток
- [ ] Manual odds override логируется с before/after
- [ ] Re-settlement пересчитывает ставки корректно

---

## ФАЗА 9: Casino Games Management
**Цель:** Управление игровым каталогом, провайдерами и конфигурацией.

### 9.1 Game Catalog

```
Список всех игр (может быть 5000+):
  Виртуализированная таблица
  Фильтры: provider, category, status, country_restriction, rtp_config
  
  Для каждой игры:
  Name | Provider | Category | RTP Config | Status (по странам) | GGR (7d)

  Inline actions:
  [Enable/Disable globally]
  [Configure by country]
  [Set RTP config]
  [Edit display settings]
  [View stats]
```

### 9.2 Provider Management

```
Список провайдеров:
  Provider | Integration Type | Games Count | Status | GGR Today | Revenue Share %

Добавление провайдера (wizard):
  Шаг 1: Тип интеграции (direct seamless API / через агрегатор)
  Шаг 2: API credentials (зашифровано через Vault)
  Шаг 3: Revenue share %, settlement currency
  Шаг 4: Test game (sandbox)
  Шаг 5: Activate

Per-provider health:
  Latency p50/p95/p99 (последние 24h)
  Error rate
  Uptime %
  Successful rounds vs failed rounds
```

### 9.3 RTP Configuration

```
CRITICAL: Это финансово чувствительная настройка
Требует SUPER_ADMIN роль + TOTP подтверждение

Матрица RTP:
  Provider × Player Group → RTP Config ID

  Пример:
                 Standard  VIP    Whale  Bonus Hunter  Dormant
  Pragmatic Play   94.50%  96.50% 96.50%  87.00%       96.50%
  BGaming          94.00%  96.00% 96.00%  87.00%       96.00%
  
  + override по стране (лицензированные рынки имеют фиксированный RTP по закону)

При изменении:
  Предупреждение с расчётом impact (ожидаемое изменение GGR на X%)
  Требует второго подтверждения от другого SUPER_ADMIN (4-eyes principle)
  Полный audit trail с before/after
```

### 9.4 Game Display Settings

```
Для каждой игры:
  Display name (кастомный)
  Category (New, Popular, Megaways, Live, Crash...)
  Sort weight (для ручного упорядочивания)
  Thumbnail (upload или URL)
  Badge (NEW, HOT, EXCLUSIVE...)
  Countries где отображается / скрыта
  
Bulk operations:
  Выбрать N игр → установить категорию / badge / статус
```

### 9.5 Jackpot Management

```
Jackpot Pools:
  Pool name | Current value | Seed value | Contribution % | Last won

Создание нового пула:
  Eligible games (multi-select)
  Contribution rate per bet (%)
  Seed value (минимальное значение после win)
  
Daily Drops конфигурация:
  Daily pool amount
  Number of drops per day
  Min / Max drop amount
  Eligible games
```

### Acceptance Criteria Фазы 9
- [ ] Каталог из 5000 игр рендерится без лагов (виртуализация)
- [ ] Изменение RTP требует 2 SUPER_ADMIN подтверждений
- [ ] Provider health показывает метрики из VictoriaMetrics
- [ ] Изменение статуса игры по стране немедленно влияет на фронтенд

---

## ФАЗА 10: Affiliate Management
**Цель:** Управление партнёрской программой.

### 10.1 Partner List & Stats

```
Список партнёров:
  ID | Partner Name | Deal Type | Players | NGR | Owed | Status

Детальный профиль партнёра:
  Tab 1: Monthly Stats
    FTDs | Deposits | GGR | Bonus Cost | NGR | Payout | Balance Owed
    График по месяцам
    
  Tab 2: Players
    Все игроки, пришедшие от этого партнёра (с их stats)
    
  Tab 3: Tracking
    Реферальные ссылки и промокоды партнёра
    Статистика кликов / регистраций / FTDs по каждой ссылке
    
  Tab 4: Payments History
    История выплат партнёру
```

### 10.2 Deal Configuration

```
При создании / редактировании партнёра:

Deal Type: Revenue Share | CPA | Hybrid

Revenue Share:
  Percentage: X%
  Negative carryover: yes | no
  Reset period: monthly | quarterly | never
  
CPA:
  Amount per FTD: $X
  Min deposit for qualification: $Y
  Min wagering for qualification: $Z
  Hold period (дней до выплаты): N
  
Hybrid:
  CPA amount + RevShare %

Sub-affiliate:
  Разрешить: yes | no
  Sub-affiliate commission: X% от их дохода
```

### 10.3 Fraud Detection (Affiliate)

```
Флаги подозрительного трафика:
  FTD amount ниже минимального (motivated traffic)
  Нет повторных депозитов (одноразовые игроки)
  Все регистрации с одного IP/подсети
  Аномально высокий bonus abuse rate
  
Quarantine: удержать выплату до расследования
Вычет фрод-трафика из расчёта с партнёром
```

### 10.4 Payouts

```
Расчёт выплат (кнопка "Calculate Period"):
  Выбрать период (обычно прошлый месяц)
  Система рассчитывает NGR по каждому партнёру
  С учётом carryover, вычетов фрод-трафика
  Предпросмотр суммы к выплате
  
  [Approve Payouts] → создаёт payment request в Payment Service
  → вывод на реквизиты партнёра
  
История выплат: дата | партнёр | сумма | метод | статус
```

### Acceptance Criteria Фазы 10
- [ ] NGR расчёт с carryover математически корректен
- [ ] Postback URL партнёра вызывается при FTD (интеграция с Payment Service)
- [ ] Фрод-флаги автоматически выставляются при аномалиях

---

## ФАЗА 11: CRM & Retention
**Цель:** Автоматизация коммуникаций и удержания игроков.

### 11.1 Player Segments

```
Конструктор сегментов:
  Условия (AND/OR логика):
    last_login: between X and Y days ago
    total_deposits: > X
    ftd_completed: true | false
    redeposit_count: = 0 | >= 1 | >= N
    ggr: > X | < X
    favorite_game_category: slots | sports | live
    country: in [list]
    tags: has [tag]
    balance: > X
    churn_probability: > X% (из ML модели)
    ...
  
  Preview: сколько игроков попадает в сегмент (live count)
  Save segment → отображается в списке с количеством
  
Стандартные сегменты (преднастроенные):
  New (0-7d), FTD No Redeposit, Dormant 7d, Dormant 30d,
  VIP Active, High Rollers, Losing Streak, Big Winners и т.д.
```

### 11.2 Automated Triggers

```
Конструктор триггеров:
  
  Trigger Condition (event-based):
    ftd_completed | no_redeposit_after_Xdays | login_after_dormant
    session_loss > $X | no_login_Xdays | birthday | deposit_count = N
    big_win > $X | churn_score > X% | major_event_starting
    
  Action (одно или несколько):
    send_email {template_id, subject}
    send_sms {template_id}
    send_push {title, body}
    give_bonus {bonus_id}
    assign_to_manager
    add_tag {tag}
    
  Delay: immediate | after X hours | at specific time
  
  A/B Test (optional):
    50% получают action A, 50% — action B
    Метрика для определения победителя: click_rate | redeposit_rate | ngr

Список триггеров:
  Name | Condition | Action | Status | Fired (30d) | Conversion Rate
```

### 11.3 Manual Campaigns

```
Одноразовая рассылка:
  Name (internal)
  Target: Segment (выбрать) | Manual list (upload CSV)
  Channel: Email | SMS | Push | All
  Template: выбрать или создать
  Schedule: immediately | at datetime
  
  Preview: количество получателей
  Test send: отправить себе тест
  [Launch Campaign]
  
Результаты кампании (после отправки):
  Sent | Delivered | Opened | Clicked | Converted (redeposit) | Revenue
```

### 11.4 Communication Templates

```
Редактор шаблонов:
  WYSIWYG для email (HTML)
  Переменные: {{player_name}}, {{balance}}, {{bonus_amount}}, {{promo_code}}...
  Preview с реальными данными игрока
  
  SMS: plain text + counter символов (160)
  Push: title (50ch) + body (100ch) + deep link
  
Мультиязычность:
  Шаблон → N языковых версий
  Автоматический выбор по country/language игрока
```

### Acceptance Criteria Фазы 11
- [ ] Сегмент строится за < 3 сек (ClickHouse query)
- [ ] Триггер FTD срабатывает в течение 60 сек после события
- [ ] A/B тест корректно разделяет аудиторию 50/50
- [ ] Email шаблон рендерит переменные без ошибок

---

## ФАЗА 12: Analytics, Reports & System Settings
**Цель:** Business Intelligence + системное управление.

### 12.1 Financial Reports

```
Стандартные отчёты (ClickHouse backend):

  GGR/NGR Report
    Период | Breakdown: daily/weekly/monthly
    Split: casino | sports | live casino
    Фильтры: country, provider, game, affiliate
    
  Deposit/Withdrawal Report
    По методам, статусам, суммам
    Success rate по gateway
    
  P&L Statement
    GGR - Bonuses - Provider Fees - Payment Fees - Affiliate Payouts = NGR
    
  Hold Report (Sportsbook)
    Held% = GGR / Total Stakes
    По виду спорта, лиге, типу ставки
    
  Все отчёты:
    Экспорт CSV / Excel
    Scheduled email delivery (настраивается)
    Сравнение с предыдущим периодом
```

### 12.2 Player Analytics

```
  Cohort Analysis
    Когорты по месяцу FTD
    Retention: Day 1, 7, 30, 60, 90
    LTV динамика
    
  Conversion Funnel
    Visits → Registrations → FTDs → 2nd Deposit → Active
    По источнику трафика (affiliate / direct / organic)
    
  Unit Economics
    CAC, LTV, LTV/CAC ratio
    ARPDAU (Average Revenue Per Daily Active User)
    Churn Rate (monthly)
```

### 12.3 Game Analytics

```
  Top Games by GGR (за любой период)
  Actual RTP vs Theoretical RTP (важно для compliance)
  Unique players per game
  Rounds / Spins count
  
  Provider Performance:
    GGR, uptime, error rate, revenue share → net revenue
```

### 12.4 Admin System Settings

```
Admin Users Management (только SUPER_ADMIN):
  Список всех admin пользователей
  Создать / Edit / Deactivate
  Сброс TOTP
  Просмотр последней активности
  
IP Whitelist Management:
  Глобальный список разрешённых IP
  Per-admin список

Feature Flags:
  Список всех feature flags (из DragonflyDB)
  Toggle without deploy
  
Global Platform Settings:
  Default currency
  Maintenance mode (с сообщением для игроков)
  KYC threshold settings
  AML threshold settings
  
Audit Log Viewer:
  Полный лог всех admin действий
  Фильтры: admin, action, entity, date range
  Export
  Нельзя удалить / изменить записи (UI не предоставляет такой возможности)
```

### Acceptance Criteria Фазы 12
- [ ] Cohort report строится по ClickHouse за < 10 сек (даже за 12 месяцев)
- [ ] Actual RTP vs Theoretical RTP показывает реальное расхождение
- [ ] Audit log нельзя очистить из UI (кнопки нет)
- [ ] Feature flag toggle применяется во всей системе за < 1 сек

---

## 5. НЕФУНКЦИОНАЛЬНЫЕ ТРЕБОВАНИЯ

### Производительность

```
Admin BFF response time (p99):
  Простые запросы (player profile): < 200ms
  Сложные агрегаты (report за месяц): < 3 сек
  Real-time метрики: WebSocket push < 5 сек

UI rendering:
  Таблица с 10,000 строк: без лагов (virtual scroll)
  Initial page load: < 2 сек (Vite build + CDN)
  
ClickHouse queries:
  Dashboard агрегаты: < 1 сек (с материализованными view)
  Cohort analysis: < 10 сек
```

### Безопасность (summary)

```
Network:
  Admin panel недоступен из публичного интернета
  Только через VPN или IP whitelist
  CloudFlare WAF не применяется (внутренний)
  mTLS между BFF и Go/Rust сервисами (Istio)

Auth:
  JWT (Ed25519) + TOTP обязателен для всех
  Refresh token rotation
  15 минут auto-logout при неактивности
  Failed TOTP: 5 попыток → 30 минут lockout

Data:
  Sensitive поля (card numbers, crypto wallets) — только masked в UI
  KYC документы — presigned S3 URL (истекают через 15 минут)
  Нет загрузки данных игроков на клиент без need-to-know
  
Critical operations:
  Двойное подтверждение для: adjust balance, change RTP, bulk block
  Все мутации → audit log
```

### Доступность

```
Admin panel не требует 99.99% uptime (внутренний инструмент)
Target: 99.5% (~ 3.5 часа downtime в месяц допустимо)
Planned maintenance: уведомление за 24 часа в Slack/Telegram

При недоступности сервисов:
  Graceful degradation: показывать что доступно
  Явные сообщения об ошибках (не пустые экраны)
```

---

## 6. ПЛАН ПО ФАЗАМ (timeline для solo)

```
Фаза  | Модуль                        | Ориентировочно
──────┼───────────────────────────────┼───────────────
1     | Foundation & Infrastructure   | 3-4 недели
2     | Dashboard                     | 2-3 недели
3     | Player Management             | 3-4 недели
4     | KYC & Responsible Gambling    | 2-3 недели
5     | Payment Management            | 3-4 недели
6     | Bonus Engine                  | 2-3 недели
7     | Risk & Anti-Fraud             | 2-3 недели
8     | Sportsbook Management         | 3-4 недели
9     | Casino Games Management       | 2-3 недели
10    | Affiliate Management          | 2 недели
11    | CRM & Retention               | 3-4 недели
12    | Analytics & Settings          | 2-3 недели
──────┼───────────────────────────────┼───────────────
ИТОГО |                               | ~9-12 месяцев

Замечание: фазы 1-5 — критический путь (без них бизнес не работает).
Фазы 6-12 можно итерировать после запуска.
```

---

## 7. API CONVENTIONS (BFF)

```
Base URL: /admin/v1/

Auth: Authorization: Bearer {access_token}

Pagination:
  GET /admin/v1/players?page=1&page_size=50&sort=created_at&order=desc
  Response: {data: [...], meta: {total: N, page: 1, page_size: 50}}

Errors:
  {
    "error": "PERMISSION_DENIED",
    "message": "You don't have permission to approve withdrawals",
    "trace_id": "abc123"
  }

HTTP status codes:
  200 OK
  201 Created
  400 Bad Request (validation error)
  401 Unauthorized (bad token)
  403 Forbidden (insufficient role)
  404 Not Found
  409 Conflict (e.g., already approved)
  422 Unprocessable Entity (business logic error)
  500 Internal Server Error

Idempotency:
  Все мутирующие запросы принимают Idempotency-Key header
  BFF хранит ключи в DragonflyDB 24 часа
  Повторный запрос с тем же ключом → вернуть тот же результат
```

---

## 8. DIRECTORY STRUCTURE

```
dod-admin/
├── apps/
│   ├── web/                    # React приложение
│   │   ├── src/
│   │   │   ├── app/            # Layout, routing, providers
│   │   │   ├── features/       # Один folder per phase/module
│   │   │   │   ├── dashboard/
│   │   │   │   ├── players/
│   │   │   │   ├── payments/
│   │   │   │   ├── bonuses/
│   │   │   │   ├── risk/
│   │   │   │   ├── sportsbook/
│   │   │   │   ├── casino/
│   │   │   │   ├── affiliates/
│   │   │   │   ├── crm/
│   │   │   │   ├── kyc/
│   │   │   │   ├── reports/
│   │   │   │   └── settings/
│   │   │   ├── shared/         # Общие компоненты, hooks, utils
│   │   │   │   ├── components/
│   │   │   │   ├── hooks/
│   │   │   │   ├── api/        # TanStack Query hooks
│   │   │   │   └── ws/         # WebSocket client
│   │   │   └── types/          # TypeScript типы
│   │   ├── vite.config.ts
│   │   └── package.json
│   │
│   └── bff/                    # Go Admin BFF
│       ├── cmd/
│       │   └── server/
│       ├── internal/
│       │   ├── auth/
│       │   ├── handlers/       # HTTP handlers per module
│       │   ├── middleware/
│       │   ├── rbac/
│       │   ├── audit/
│       │   ├── ws/
│       │   └── clients/        # gRPC clients к сервисам DOD
│       ├── pkg/
│       └── go.mod
│
├── k8s/                        # Kubernetes manifests
│   ├── namespace.yaml
│   ├── deployment-web.yaml
│   ├── deployment-bff.yaml
│   ├── service.yaml
│   ├── ingress.yaml            # internal ingress
│   └── network-policy.yaml    # IP whitelist enforcement
│
└── docker/
    ├── Dockerfile.web
    └── Dockerfile.bff
```

---

*ТЗ v1.0 — живой документ. Обновляется по ходу разработки.*
# ТЗ Addendum v1.2: DOD Admin Panel
## Дополнения: Regulatory, Cashout, Mirror/Domain, Rakeback

**Применяется поверх:** v1.0 + Addendum v1.1  
**Новых фаз:** +2 (Фаза 14: Regulatory Reporting, Фаза 15: Mirror/Domain Management)  
**Расширяет:** Фаза 8 (Cashout), Фаза 7 (Bonus Engine → Rakeback)

---

# ФАЗА 14: REGULATORY REPORTING

**Приоритет:** Обязателен до получения лицензии  
**Доступ:** SUPER_ADMIN, FINANCE_MANAGER (view only), Compliance Officer (новая роль)

---

## 14.1 Новая роль: COMPLIANCE_OFFICER

```go
// Добавить в RBAC
const (
    PermRegulatoryView     Permission = 1 << 20
    PermRegulatoryExport   Permission = 1 << 21
    PermSARCreate          Permission = 1 << 22
    PermSARView            Permission = 1 << 23
    PermPlayerFundsView    Permission = 1 << 24
    PermComplaintsManage   Permission = 1 << 25
)

// COMPLIANCE_OFFICER видит:
// - Regulatory Reports (все)
// - SAR queue
// - Player Complaints
// - KYC/AML данные (read-only)
// НЕ видит: финансовые операции, настройки системы
```

---

## 14.2 Схема данных

```sql
-- Регуляторные отчёты (хранить всё, отчёты нельзя удалять)
CREATE TABLE regulatory_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction    TEXT NOT NULL,      -- 'MGA' | 'UKGC' | 'CURACAO' | 'GENERAL'
    report_type     TEXT NOT NULL,      -- см. ниже
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft',  -- draft|generated|submitted|accepted|rejected
    generated_at    TIMESTAMPTZ,
    submitted_at    TIMESTAMPTZ,
    submitted_by    UUID,               -- admin_id
    regulator_ref   TEXT,              -- номер присвоенный регулятором
    file_url        TEXT,              -- S3 URL PDF/XML
    data_snapshot   JSONB,             -- raw data на момент генерации
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- SAR (Suspicious Activity Reports)
CREATE TABLE sar_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction    TEXT NOT NULL,
    player_id       UUID NOT NULL,
    trigger_type    TEXT NOT NULL,      -- 'manual' | 'auto_aml' | 'auto_threshold'
    trigger_alert_id UUID,              -- если из риск-алерта
    status          TEXT NOT NULL DEFAULT 'draft',  -- draft|internal_review|submitted|acknowledged
    amount_involved NUMERIC(18,2),
    currency        CHAR(3),
    description     TEXT NOT NULL,
    supporting_data JSONB,             -- транзакции, логи, скриншоты
    assigned_to     UUID,              -- COMPLIANCE_OFFICER
    internal_notes  TEXT,
    submitted_at    TIMESTAMPTZ,
    submitted_by    UUID,
    regulator_ref   TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    -- ВАЖНО: SAR конфиденциален, игрок не должен знать
    -- Флаг запрещает любой контакт с игроком пока SAR активен
    tipping_off_lock BOOLEAN DEFAULT true
);

-- Жалобы игроков (complaints log — требование UKGC/MGA)
CREATE TABLE player_complaints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id       UUID NOT NULL,
    ticket_id       UUID,              -- ссылка на support ticket если есть
    category        TEXT NOT NULL,     -- 'payment' | 'bonus' | 'technical' | 'fairness' | 'rg'
    description     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'open',  -- open|investigating|resolved|escalated_to_adr
    adr_ref         TEXT,              -- Alternative Dispute Resolution ref
    resolution      TEXT,
    resolved_at     TIMESTAMPTZ,
    assigned_to     UUID,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Налоговые данные по юрисдикциям
CREATE TABLE jurisdiction_ggr (
    period          DATE NOT NULL,     -- первый день месяца
    jurisdiction    TEXT NOT NULL,     -- страна/регион
    currency        CHAR(3) NOT NULL,
    casino_ggr      NUMERIC(18,2) DEFAULT 0,
    sports_ggr      NUMERIC(18,2) DEFAULT 0,
    live_ggr        NUMERIC(18,2) DEFAULT 0,
    tax_rate        NUMERIC(5,4),      -- из настроек
    tax_amount      NUMERIC(18,2),
    PRIMARY KEY (period, jurisdiction, currency)
);
-- Заполняется ежемесячным scheduler job из ClickHouse
```

---

## 14.3 Report Types по юрисдикциям

### UK (UKGC)

```go
type UKGCReportType string
const (
    // Quarterly Regulatory Return
    // Подаётся каждый квартал через UKGC portal
    UKGCQuarterlyReturn UKGCReportType = "quarterly_return"
    
    // SAR — Suspicious Activity Report
    // Подаётся в NCA (National Crime Agency) через SARS portal
    // Обязателен при подозрении на money laundering
    // Tipping off (сообщить игроку о SAR) — уголовное преступление
    UKGCSAR UKGCReportType = "sar"
    
    // Social Responsibility Data
    // Данные о RG мерах: самоисключения, RG инструменты
    UKGCSocialResponsibility UKGCReportType = "social_responsibility"
    
    // Complaints Data
    // Все жалобы + исходы за период
    UKGCComplaints UKGCReportType = "complaints_log"
)

// Quarterly Return структура данных:
type UKGCQuarterlyData struct {
    Period              string  // "Q1 2025"
    
    // Financial
    GrossGamingRevenue  decimal.Decimal
    CustomerFunds       decimal.Decimal  // средства игроков на балансе
    
    // Players
    ActivePlayers       int
    NewRegistrations    int
    SelfExclusions      int
    
    // RG
    SpentOnRGTools      decimal.Decimal  // расходы на RG
    InteractionsCount   int              // RG взаимодействия с игроками
    
    // Complaints
    ComplaintsReceived  int
    ComplaintsResolved  int
    ComplaintsADR       int              // escalated to ADR
}
```

### Malta (MGA)

```go
type MGAReportType string
const (
    // Monthly Financial Return
    // Подаётся до 15-го числа следующего месяца через MGA licensee portal
    MGAMonthlyFinancial MGAReportType = "monthly_financial"
    
    // Player Funds Report
    // Подтверждение segregation of player funds
    MGAPlayerFunds MGAReportType = "player_funds"
    
    // Key Persons Changes
    // При смене директоров, ключевых сотрудников
    MGAKeyPersons MGAReportType = "key_persons"
)

type MGAMonthlyData struct {
    Period string  // "2025-03"
    
    GGR          decimal.Decimal
    NGR          decimal.Decimal
    
    // Player funds segregation
    PlayerFundsTotal    decimal.Decimal  // суммарный баланс игроков
    FundsInSegregated   decimal.Decimal  // сколько в segregated account
    SegregationRatio    float64          // должен быть >= 1.0
    
    // Player activity
    RegisteredPlayers   int
    ActivePlayers       int
    FTDPlayers          int
    
    // GGR Tax (MGA: 5% of GGR for B2C)
    GGRTaxRate          float64          // из настроек
    GGRTaxAmount        decimal.Decimal
}
```

### General (все юрисдикции)

```go
// GGR Tax Report — налог с азартных игр
// Ставки отличаются по странам:
// UK: 21% POC (Point of Consumption) tax
// Malta: 5% GGR tax
// Germany: 5.3% turnover tax (не GGR, а оборот!)
// Curacao: нет налога (только лицензионный сбор)

type GGRTaxReport struct {
    Period         string
    Jurisdiction   string
    GGR            decimal.Decimal
    TaxBase        string           // "ggr" | "turnover" — зависит от юрисдикции
    TaxableAmount  decimal.Decimal
    TaxRate        decimal.Decimal
    TaxAmount      decimal.Decimal
    Currency       string
}

// Provider Settlement Reports
// Уже есть в Addendum v1.1 (B3.1), просто добавляем экспорт
// в формате нужном провайдеру (обычно PDF + CSV)

// Affiliate Commission Reports  
// Уже есть в Фазе 10, добавляем:
// - Экспорт в формате для налоговых органов
// - Если партнёр получил > X в год → форма 1099 (US) / аналог
```

---

## 14.4 UI Regulatory Module

```
Regulatory Reports (/admin/regulatory)
├── Overview Dashboard
│   ├── Upcoming deadlines (ближайшие 30 дней)
│   │   🔴 UKGC Quarterly Return — due in 5 days
│   │   🟡 MGA Monthly Financial — due in 12 days
│   │   🟢 MGA Player Funds — due in 25 days
│   ├── Submitted reports (последние 6 месяцев)
│   └── SAR stats (confidential count only)
│
├── Report Generator
│   ├── Выбрать юрисдикцию + тип + период
│   ├── [Generate Preview] → показать данные
│   ├── [Generate PDF/XML] → сформировать файл
│   ├── [Mark as Submitted] → установить статус + регуляторный ref
│   └── История: все отчёты за всё время (нельзя удалить)
│
├── SAR Management
│   ├── Queue: draft | under_review | submitted
│   ├── Create SAR (из риск-алерта или вручную)
│   ├── Tipping-off lock: предупреждение при любом контакте с игроком
│   └── History (только COMPLIANCE_OFFICER + SUPER_ADMIN)
│
├── Complaints Log
│   ├── Список всех жалоб с исходами
│   ├── Привязка к support ticket
│   ├── Эскалация в ADR (Alternative Dispute Resolution)
│   └── Export для регулятора (период)
│
├── Tax Configuration
│   ├── Jurisdiction → Tax Type → Rate → Currency
│   ├── Germany: turnover tax (5.3% от stakes, не GGR!)
│   ├── UK: POC tax (21% от GGR)
│   └── Расчёт monthly tax liability
│
└── Player Funds Reconciliation
    ├── Total player balances (PostgreSQL)
    ├── Funds in segregated accounts (manual entry / API)
    ├── Segregation ratio
    └── Alert если ratio < 1.0
```

### Tipping-Off Protection

```go
// КРИТИЧНО: если на игрока подан SAR,
// операторы НЕ ДОЛЖНЫ сообщать ему об этом (уголовная ответственность в UK)

// В профиле игрока с активным SAR:
// 1. Предупреждение для оператора: "⚠️ SAR ACTIVE — Do not discuss with player"
// 2. Запрет отправки сообщений игроку через CRM (suppression auto-added)
// 3. Support агенты видят флаг но НЕ видят детали SAR
// 4. Вывод: на hold (нельзя одобрить пока SAR не закрыт)

// Middleware в Support Chat:
func CheckTippingOff(playerID uuid.UUID) error {
    hasActiveSAR, _ := sarRepo.HasActiveSAR(playerID)
    if hasActiveSAR {
        // Показать предупреждение агенту, но не блокировать чат
        // Агент должен сам избегать упоминания расследования
        return ErrSARActive
    }
    return nil
}
```

---

# РАСШИРЕНИЕ ФАЗЫ 8: ДЕТАЛЬНЫЙ CASHOUT MANAGEMENT

---

## 8-ext.1 Полная модель Cashout

```go
// Настройки cashout (хранятся в DragonflyDB, применяются без deploy)

type CashoutConfig struct {
    // Глобальный переключатель
    GlobalEnabled bool
    
    // По типу ставки
    PreMatchEnabled bool
    LiveEnabled     bool
    
    // Margin (сколько казино "срезает" с "честной" суммы)
    // Разные для разных контекстов
    MarginPercent struct {
        PreMatch  float64   // обычно 5-8%
        Live      float64   // обычно 8-12%
        ByOddsRange []OddsMarginRule  // маленькие коэффициенты → меньше маржа
    }
    
    // По спорту (override глобального margin)
    SportOverrides map[string]SportCashoutConfig
    
    // Временная задержка перед подтверждением
    // Защита от арбитражников которые кэшаутятся за миллисекунды
    DelaySeconds struct {
        PreMatch int  // обычно 3-5 сек
        Live     int  // обычно 5-10 сек
    }
    
    // Если за время delay коэффициент изменился
    OddsChangePolicy string  // "accept_new" | "cancel" | "offer_new"
    
    // Частичный cashout
    PartialEnabled     bool
    PartialMinPercent  float64  // минимум 10% от суммы
    
    // Auto cashout (игрок устанавливает порог)
    AutoCashoutEnabled bool
    
    // Лимиты
    MinCashoutAmount decimal.Decimal
    MaxCashoutAmount decimal.Decimal
}

type SportCashoutConfig struct {
    Enabled       bool
    MarginPreMatch float64
    MarginLive     float64
    DelaySeconds   int
    // Например: Table Tennis — suspend cashout на последние 2 минуты матча
    SuspendBeforeEndMinutes int
}

// Event-level suspend (трейдер может вручную отключить для конкретного события)
// POST /admin/sportsbook/events/:id/cashout/suspend
// body: {reason: string}
```

## 8-ext.2 Cashout Calculation Engine

```go
// Формула расчёта cashout суммы:

// 1. Получить текущие odds для ставки (от провайдера, live)
// 2. Рассчитать "честную" стоимость:
//    fair_value = stake * (original_odds / current_odds)
//    Если выигрываем: fair_value приближается к potential_win
//    Если проигрываем: fair_value < stake
// 3. Применить маржу:
//    cashout_amount = fair_value * (1 - margin_percent)

// Пример:
// Ставка: $100 на Real Madrid @ 2.50 (потенциал $250)
// Счёт 1-0 в пользу Real, текущий коэффициент: 1.40
// Fair value = $100 * (2.50 / 1.40) = $178.57
// Маржа 8%: cashout = $178.57 * 0.92 = $164.29

type CashoutOffer struct {
    BetID         uuid.UUID
    OriginalStake decimal.Decimal
    PotentialWin  decimal.Decimal
    CurrentOffer  decimal.Decimal   // что предлагаем
    FairValue     decimal.Decimal   // до маржи (не показываем игроку)
    MarginApplied float64
    ValidForMs    int               // через сколько мс предложение устаревает
    IsPartial     bool
    PartialOptions []decimal.Decimal  // предлагаемые частичные суммы (25%, 50%, 75%)
}
```

## 8-ext.3 Auto Cashout Engine

```go
// Игрок устанавливает порог через фронтенд:
// "Автоматически кэшаутить если предложение >= $X"

// BFF хранит условие в DragonflyDB:
// key: "auto_cashout:{bet_id}"
// value: {threshold: 150.00, player_id: uuid}

// Rust Betting Engine при каждом обновлении odds:
// 1. Рассчитать cashout offer для всех активных ставок
// 2. Проверить auto_cashout условия в DragonflyDB
// 3. Если offer >= threshold → выполнить cashout автоматически
// 4. Notify игрока через WebSocket: "Auto cashout executed: $150.23"

// Лог auto cashout событий (для аналитики):
CREATE TABLE auto_cashout_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bet_id      UUID NOT NULL,
    player_id   UUID NOT NULL,
    threshold   NUMERIC(18,2) NOT NULL,
    offer_at_execution NUMERIC(18,2) NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 8-ext.4 Cashout P&L Analytics

```typescript
// GET /admin/sportsbook/cashout/analytics?period=month

interface CashoutAnalytics {
    period: string
    
    // Объём
    totalCashouts:    number
    totalCashedOut:   AmountDisplay  // сколько выплатили игрокам
    totalFairValue:   AmountDisplay  // сколько было бы без маржи
    marginEarned:     AmountDisplay  // = FairValue - CashedOut
    
    // Сравнение: если бы игроки не кэшаутились
    // (иногда казино выгоднее когда игрок кэшаутит проигрышную ставку)
    settledResult:    AmountDisplay  // как ставки завершились бы
    cashoutsVsSettled: {
        favoredCasino:  number  // кол-во кэшаутов выгодных казино
        favoredPlayer:  number  // кол-во кэшаутов выгодных игроку
        difference:     AmountDisplay  // net benefit/loss от кэшаутов
    }
    
    // Breakdown по спорту, типу (pre/live), partial vs full
    bySport:     Record<string, CashoutSportStats>
    byType:      { preMatch: CashoutTypeStats, live: CashoutTypeStats }
    
    // Auto cashout отдельно
    autoCashouts: number
    autoCashoutsAmount: AmountDisplay
}
```

## 8-ext.5 Trading Terminal интеграция

```typescript
// В Trading Terminal (из Addendum v1.1):
// При клике на событие — добавить панель Cashout Status:

interface EventCashoutStatus {
    enabled:     boolean
    suspended:   boolean     // вручную трейдером
    margin:      number
    delaySeconds: number
    
    // Live stats для этого события:
    cashoutsToday: number
    amountCashedOut: AmountDisplay
    marginEarned:    AmountDisplay
}

// Hotkeys для трейдера (добавить в Trading Terminal):
// C → suspend cashout для выбранного события
// Shift+C → resume cashout
```

---

# ФАЗА 15: MIRROR / DOMAIN MANAGEMENT

**Приоритет:** Критично для серого рынка  
**Доступ:** SUPER_ADMIN, специальная роль DOMAIN_MANAGER

---

## 15.1 Новая роль: DOMAIN_MANAGER

```go
const (
    PermDomainView    Permission = 1 << 26
    PermDomainManage  Permission = 1 << 27
    PermDomainCreate  Permission = 1 << 28
    PermTelegramBot   Permission = 1 << 29
)
// Обычно это DevOps или Tech Lead, не бизнес-роль
```

---

## 15.2 Схема данных

```sql
CREATE TABLE mirror_domains (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain          TEXT UNIQUE NOT NULL,      -- "casino-mirror3.com"
    status          TEXT NOT NULL DEFAULT 'active',  -- active|blocked|ssl_error|degraded|retired
    is_primary      BOOLEAN DEFAULT false,      -- основной домен
    priority        INT DEFAULT 0,             -- порядок переключения
    
    -- Health monitoring
    last_check_at   TIMESTAMPTZ,
    last_ok_at      TIMESTAMPTZ,
    response_time_ms INT,                      -- последний ping
    ssl_expires_at  DATE,
    ssl_issuer      TEXT,
    
    -- Traffic
    requests_24h    BIGINT DEFAULT 0,          -- обновляется из CloudFlare API
    active_sessions INT DEFAULT 0,
    
    -- Metadata
    registrar       TEXT,
    registered_at   DATE,
    expires_at      DATE,
    auto_renew      BOOLEAN DEFAULT true,
    nameservers     TEXT[],
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    created_by      UUID
);

CREATE TABLE domain_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id   UUID NOT NULL REFERENCES mirror_domains(id),
    event_type  TEXT NOT NULL,  -- 'status_change'|'ssl_renewed'|'traffic_spike'|'blocked_detected'
    old_value   TEXT,
    new_value   TEXT,
    details     JSONB,
    detected_at TIMESTAMPTZ DEFAULT now(),
    notified_at TIMESTAMPTZ
);

-- Telegram bot конфигурация
CREATE TABLE telegram_bot_config (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_token   TEXT NOT NULL,  -- зашифрован через Vault
    channels    JSONB NOT NULL, -- [{chat_id: "-100xxx", name: "mirrors_ru", lang: "ru"}]
    is_active   BOOLEAN DEFAULT true,
    updated_at  TIMESTAMPTZ DEFAULT now(),
    updated_by  UUID
);

-- История публикаций зеркал в Telegram
CREATE TABLE telegram_publications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id   UUID NOT NULL,
    channel_id  TEXT NOT NULL,
    message_id  BIGINT,         -- Telegram message ID
    message     TEXT,
    published_at TIMESTAMPTZ DEFAULT now(),
    published_by TEXT NOT NULL  -- 'auto' | admin_email
);
```

---

## 15.3 Health Monitoring

```go
// Background job (каждые 60 секунд):
// Запускается в BFF или отдельном Go worker

type DomainHealthChecker struct {
    domains    []MirrorDomain
    httpClient *http.Client  // timeout: 10 сек
}

func (c *DomainHealthChecker) Check(domain MirrorDomain) DomainHealthResult {
    start := time.Now()
    
    // 1. HTTP GET https://domain/health или /
    resp, err := c.httpClient.Get("https://" + domain.Domain + "/api/health")
    latency := time.Since(start).Milliseconds()
    
    // 2. Проверить SSL сертификат
    sslExpiry := getSSLExpiry(domain.Domain)
    
    // 3. Определить статус
    status := "active"
    if err != nil {
        status = "blocked"       // не отвечает → заблокирован провайдером?
    } else if resp.StatusCode >= 500 {
        status = "degraded"
    } else if sslExpiry.Before(time.Now().Add(14 * 24 * time.Hour)) {
        status = "ssl_warning"   // SSL истекает через 2 недели
    }
    
    return DomainHealthResult{
        DomainID:      domain.ID,
        Status:        status,
        ResponseTimeMs: int(latency),
        SSLExpiresAt:  sslExpiry,
        CheckedAt:     time.Now(),
    }
}

// При изменении статуса → запись в domain_events + уведомление
func (c *DomainHealthChecker) HandleStatusChange(old, new string, domain MirrorDomain) {
    if old == "active" && new == "blocked" {
        // КРИТИЧНО: зеркало заблокировано
        notifyRole("DOMAIN_MANAGER", domain.Domain + " is blocked!")
        notifyTelegram(domain)    // автоматически публикует следующее зеркало
        autoSwitchTraffic(domain) // если настроен CloudFlare failover
    }
    if new == "ssl_warning" {
        // SSL истекает, нужно продлить
        notifyRole("DOMAIN_MANAGER", "SSL expiring: " + domain.Domain)
    }
}
```

## 15.4 Auto Traffic Switching

```go
// CloudFlare API интеграция
// При блокировке основного домена → автоматически:
// 1. Понизить priority заблокированного домена
// 2. Повысить priority следующего активного зеркала
// 3. Обновить DNS routing через CF API

type CloudFlareRouter struct {
    apiToken string  // из Vault
    zoneIDs  map[string]string  // domain → zone ID
}

func (r *CloudFlareRouter) Failover(blocked MirrorDomain) error {
    // Найти следующий активный домен с наивысшим приоритетом
    next := findNextActive(blocked)
    if next == nil {
        return ErrNoMirrorsAvailable  // критический алерт
    }
    
    // CloudFlare: включить load balancer rule для next домена
    // Или: обновить DNS record для основного имени → новый IP
    return r.updateRouting(next)
}

// Настраивается в Mirror Settings:
// Auto-failover: ✅ Enabled
// Failover trigger: response_time > 5000ms OR status = blocked
// CloudFlare API: [configured] / [not configured]
```

## 15.5 Telegram Bot Integration

```go
// При обнаружении блокировки → автоматически:
// Публикует в сконфигурированные каналы сообщение с новым зеркалом

type TelegramPublisher struct {
    bot      *tgbotapi.BotAPI  // зашифрованный token из Vault
    channels []TelegramChannel
}

func (p *TelegramPublisher) PublishMirror(newDomain string, reason string) error {
    for _, ch := range p.channels {
        msg := p.buildMessage(newDomain, ch.Language)
        // msg пример (RU): "🔗 Актуальная ссылка: https://casino-mirror5.com"
        
        _, err := p.bot.Send(tgbotapi.NewMessage(ch.ChatID, msg))
        // Логировать в telegram_publications
    }
    return nil
}

// Шаблоны сообщений (настраиваемые в UI):
// RU: "🔗 Актуальная ссылка: {domain}\n✅ Все данные сохранены"
// EN: "🔗 New mirror: {domain}\n✅ All accounts are safe"
// TR: "🔗 Yeni ayna: {domain}\n✅ Tüm hesaplar güvende"

// Также: manual publish (кнопка в UI)
// POST /admin/domains/:id/publish-telegram
```

## 15.6 UI Domain Management

```
Mirror/Domain Management (/admin/domains)
│
├── Domain List
│   ┌──────────────────┬────────┬────────┬──────────┬──────────────┬──────────┐
│   │ Domain           │ Status │ Prio   │ Response │ SSL Expires  │ Traffic  │
│   ├──────────────────┼────────┼────────┼──────────┼──────────────┼──────────┤
│   │ casino.com       │ ✅ OK  │  1     │  145ms   │ 2026-01-15   │ 12,400/h │
│   │ casino-m1.com    │ ✅ OK  │  2     │  203ms   │ 2025-08-20   │    230/h │
│   │ casino-m2.com    │ 🔴 Blocked│ 3   │ timeout  │ 2025-12-01   │      0/h │
│   │ casino-m3.com    │ ⚠️ SSL │  4     │  189ms   │ 2025-05-02 ⚠│    120/h │
│   └──────────────────┴────────┴────────┴──────────┴──────────────┴──────────┘
│
│   Actions: [Add Domain] [Check All Now] [Reorder Priorities]
│
├── Domain Detail (при клике)
│   ├── Health history (график response time 24h)
│   ├── Status events log
│   ├── SSL certificate details
│   ├── Telegram publications history
│   └── Actions: [Suspend] [Set as Primary] [Publish to Telegram] [Retire]
│
├── Telegram Bot Settings
│   ├── Bot token (masked, редактируется)
│   ├── Channels list (добавить/удалить)
│   ├── Message templates (по языкам)
│   ├── Auto-publish on block: ✅/❌
│   └── Test send: [Send test message]
│
├── SSL Certificate Monitor
│   ├── Все домены с датами истечения SSL
│   ├── Алерт за 14 дней до истечения
│   └── Quick link к cert-manager / registrar
│
├── Traffic Distribution
│   ├── Pie chart: трафик по доменам
│   ├── Timeline: переключения трафика
│   └── CloudFlare integration status
│
└── Auto-Failover Settings
    ├── Enabled: ✅/❌
    ├── Trigger conditions (response time / status)
    ├── CloudFlare API token (masked)
    └── Failover log (когда, с какого на какой)
```

---

# РАСШИРЕНИЕ ФАЗЫ 7: RAKEBACK ENGINE

**Добавить как подраздел к Фазе 7 (Bonus Engine) или как отдельный модуль**

---

## Rak-1. Концепция Rakeback

```
Rakeback отличается от cashback:

Cashback: % от net_loss за период (ты проиграл $1000, получи $100 обратно)
Rakeback: % от house_edge на каждую ставку (постоянно, независимо от исхода)

House edge (rake) = ставка × (1 - theoretical_RTP) для казино
                 = ставка × margin для спорта

Пример (казино, слот с RTP 96%):
  Ставка: $10
  House edge: $10 × 4% = $0.40
  Rakeback rate: 20%
  Rakeback earned: $0.40 × 20% = $0.08 за этот спин

За день игры на $1000:
  House edge: $40
  Rakeback: $8 (credited в rakeback balance)

Игрок может claim ракебэк в любой момент нажав кнопку.
```

## Rak-2. Схема данных

```sql
-- Конфигурация ракебэка по уровням
CREATE TABLE rakeback_config (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_group    TEXT NOT NULL,     -- 'bronze'|'silver'|'gold'|'platinum'|'diamond'
    game_type       TEXT NOT NULL,     -- 'slots'|'live'|'table'|'sports'|'crash'
    rate_percent    NUMERIC(5,2) NOT NULL,  -- % от house edge
    is_active       BOOLEAN DEFAULT true,
    updated_at      TIMESTAMPTZ DEFAULT now(),
    updated_by      UUID,
    UNIQUE(player_group, game_type)
);

-- Накопленный ракебэк (обновляется при каждой ставке)
CREATE TABLE player_rakeback_balance (
    player_id           UUID PRIMARY KEY,
    pending_balance     NUMERIC(18,2) DEFAULT 0,  -- накоплено, но не claimed
    claimed_total       NUMERIC(18,2) DEFAULT 0,  -- всего claimed за всё время
    last_claim_at       TIMESTAMPTZ,
    last_updated_at     TIMESTAMPTZ DEFAULT now()
);

-- История начислений (для аналитики, агрегируется почасово)
CREATE TABLE rakeback_accruals (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id   UUID NOT NULL,
    bet_id      UUID NOT NULL,
    game_type   TEXT NOT NULL,
    stake       NUMERIC(18,2) NOT NULL,
    house_edge  NUMERIC(18,2) NOT NULL,   -- рассчитанный house edge для этой ставки
    rate        NUMERIC(5,2) NOT NULL,    -- применённый %
    amount      NUMERIC(18,2) NOT NULL,   -- начислено ракебэка
    accrued_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)
PARTITION BY RANGE (accrued_at);  -- партиционирование по месяцам

-- Claims
CREATE TABLE rakeback_claims (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id   UUID NOT NULL,
    amount      NUMERIC(18,2) NOT NULL,
    claimed_at  TIMESTAMPTZ DEFAULT now(),
    credited_to TEXT NOT NULL DEFAULT 'main_balance'  -- 'main_balance' | 'bonus_balance'
);
```

## Rak-3. Расчёт house edge per bet

```go
// Вызывается из Betting Engine (Rust) через gRPC при каждой ставке
// Rust → Go Bonus Service: AccrueRakeback(bet)

type RakebackAccrueRequest struct {
    PlayerID   uuid.UUID
    BetID      uuid.UUID
    GameType   string          // "slots" | "live" | "sports" | "crash"
    Stake      decimal.Decimal
    
    // Для казино:
    TheoreticalRTP decimal.Decimal  // из game config (96.50% = 0.9650)
    
    // Для спорта:
    Margin decimal.Decimal          // маржа матча (0.07 = 7%)
}

func CalculateHouseEdge(req RakebackAccrueRequest) decimal.Decimal {
    switch req.GameType {
    case "slots", "live", "table", "crash":
        // House edge = ставка × (1 - RTP)
        return req.Stake.Mul(decimal.NewFromFloat(1).Sub(req.TheoreticalRTP))
    case "sports":
        // House edge = ставка × margin
        return req.Stake.Mul(req.Margin)
    }
}

func AccrueRakeback(req RakebackAccrueRequest, config RakebackConfig) decimal.Decimal {
    houseEdge := CalculateHouseEdge(req)
    rakebackAmount := houseEdge.Mul(config.RatePercent.Div(decimal.NewFromInt(100)))
    
    // UPDATE player_rakeback_balance SET pending_balance += rakebackAmount
    // INSERT INTO rakeback_accruals ...
    
    return rakebackAmount
}
```

## Rak-4. Claim механика

```go
// Игрок нажимает "Claim Rakeback" на фронтенде

// POST /api/players/rakeback/claim (игровой API, не admin)
func ClaimRakeback(playerID uuid.UUID) (decimal.Decimal, error) {
    // 1. Получить pending_balance
    // 2. Если == 0 → return error "nothing to claim"
    // 3. Атомарная транзакция:
    //    - SET pending_balance = 0
    //    - ADD to player main_balance
    //    - INSERT rakeback_claims
    //    - INSERT wallet_transaction (type: "rakeback_claim")
    // 4. Return claimed amount
    
    // Ограничения:
    // - Min claim amount: $0.01 (настраивается)
    // - Cooldown: 0 (instant, нет вейджера)
    // - Нет вейджера на ракебэк — это ключевое преимущество модели
}
```

## Rak-5. Admin UI

```
Rakeback Management (/admin/bonuses/rakeback)
│
├── Configuration Matrix
│   
│   Rate по уровням и типам игр (%):
│   ┌──────────────┬────────┬────────┬────────┬──────────┬───────┐
│   │ Player Level │ Slots  │  Live  │ Table  │  Sports  │ Crash │
│   ├──────────────┼────────┼────────┼────────┼──────────┼───────┤
│   │ Bronze       │  5%    │  3%    │  3%    │   2%     │  5%   │
│   │ Silver       │  8%    │  5%    │  5%    │   3%     │  8%   │
│   │ Gold         │  12%   │  8%    │  8%    │   5%     │  12%  │
│   │ Platinum     │  18%   │  12%   │  12%   │   8%     │  18%  │
│   │ Diamond      │  25%   │  18%   │  15%   │  12%     │  25%  │
│   └──────────────┴────────┴────────┴────────┴──────────┴───────┘
│   
│   Inline редактирование всех ячеек
│   [Save Changes] → применяется для новых ставок (не retroactive)
│
├── Analytics
│   
│   Период: [Week ▼]
│   
│   Total Accrued:  $45,234   ← ракебэк начислен игрокам
│   Total Claimed:  $32,100   ← игроки забрали
│   Unclaimed:      $13,134   ← висит на балансах
│   
│   Cost as % of GGR: 3.2%   ← ключевой KPI
│   
│   По уровням (bar chart):
│     Diamond:  $18,900 accrued, $14,200 claimed
│     Platinum: $12,400 accrued, $9,100 claimed
│     ...
│   
│   По типу игры (pie chart):
│     Slots: 68% | Crash: 18% | Live: 9% | Sports: 5%
│
└── Player Rakeback Balances
    
    Топ 50 игроков по unclaimed балансу:
    Player | Level | Pending Balance | Last Claim | Total Earned
    
    Возможность: форсировать claim (admin initiated)
    POST /admin/players/:id/rakeback/force-claim
    Кейс: если игрок давно не заходил и мы хотим показать баланс при reactivation
```

## Rak-6. Интеграция с Loyalty Levels (из Addendum v1.1)

```go
// При повышении уровня игрока → ставка ракебэка обновляется
// Не retroactively — только для ставок после повышения

// Event: player.level_upgraded
// Handler: обновить конфиг в rakeback расчёте

// В Loyalty Program (Фаза 7, Bonus Engine):
// Показать ожидаемый ракебэк per level:
// "Gold: вы будете получать ~12% rakeback на слоты"
// "Diamond: ~25% rakeback — максимальный уровень"
```

---

# ОБНОВЛЁННЫЙ ИТОГОВЫЙ ПЛАН ФАЗ

```
Фаза  │ Модуль                              │ Статус
──────┼─────────────────────────────────────┼─────────────
1     │ Foundation & Infrastructure         │ v1.0 + v1.1
2     │ Dashboard                           │ v1.0 + v1.1
3     │ Player Management                   │ v1.0 + v1.1
4     │ KYC & Responsible Gambling          │ v1.0 + v1.1
5     │ Payment Management                  │ v1.0 + v1.1
6     │ Support Ticket System               │ v1.1
7     │ Bonus Engine + Rakeback             │ v1.0 + v1.2
8     │ Risk & Anti-Fraud + Rule Builder    │ v1.0 + v1.1
9     │ Sportsbook + Cashout (детальный)    │ v1.0 + v1.2
10    │ Casino Games Management             │ v1.0 + v1.1
11    │ Affiliate Management                │ v1.0 + v1.1
12    │ CRM & Retention                     │ v1.0 + v1.1
13    │ Analytics & System Settings         │ v1.0 + v1.1
14    │ Regulatory Reporting                │ v1.2  ← НОВАЯ
15    │ Mirror / Domain Management          │ v1.2  ← НОВАЯ
──────┴─────────────────────────────────────┴─────────────

Итого: 15 фаз
Покрытие: ~99% production-уровня гемблинг-администрирования
```

---

## Критический порядок для серого рынка

```
Если запуск в серую юрисдикцию (Curacao / без лицензии):

Обязательно до запуска:    Фазы 1-6, 15 (Mirror — критично!)
Первый месяц после:        Фазы 7, 8, 9
Первый квартал:            Фазы 10-13
Параллельно при росте:     Фаза 14 (Regulatory — при движении к лицензии)
```

---

*Addendum v1.2 — применяется поверх v1.0 + v1.1*  
*Общий объём ТЗ: v1.0 (1644 строк) + v1.1 (1048 строк) + v1.2 (≈900 строк)*