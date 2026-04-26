# Аудит apps/admin против ТЗ (admin.task.md)

**Дата аудита:** 2026-04-26
**База для сравнения:** `tasks/admin.task.md` (v1.0 + addendum v1.1 + v1.2)

---

## Executive Summary

Админ-панель находится на стадии **раннего MVP** — реализован базовый каркас (аутентификация, layout, простые CRUD-списки), но отсутствует подавляющее большинство бизнес-критичного функционала из Приоритета A. Оценка готовности: **~12–15%** от полного ТЗ.

**Критические gaps (Приоритет A):**
- Отсутствует KYC-очередь, Support Ticket System, Chargeback workflow, Trading Terminal, Rule Builder, PEP/Sanctions screening, Document expiry tracking, Crypto wallet management, Balance sheet, Multi-currency display, Maintenance mode, Session policy
- Нет WebSocket — весь real-time функционал отсутствует
- Нет TOTP в логине
- Роли не соответствуют ТЗ
- Нет авто-logout по бездействию
- Access token хранится в localStorage (нарушение ТЗ)

---

## 1. Инвентаризация текущей кодовой базы

### 1.1 Структура файлов (apps/admin/src)

```
src/
├── App.tsx                    # Роутинг (React Router v6)
├── main.tsx                   # Entry point
├── config/routes.ts           # Конфигурация роутов + permissions
├── stores/
│   └── authStore.ts           # Zustand + persist (localStorage) ❌
├── types/
│   ├── admin.ts               # AdminRole, Permission, DashboardStats
│   ├── api.ts                 # ApiResponse, PaginatedResponse, LoginRequest/Response
│   ├── user.ts                # User, UserProfile, UserSession, UserLimits
│   ├── finance.ts             # Transaction, Deposit, Withdrawal, WalletBalance
│   ├── bet.ts                 # Bet, BetSelection, BetSearchParams
│   ├── casino.ts              # Game, GameSession, Provider
│   ├── bonus.ts               # BonusCampaign, UserBonus
│   └── risk.ts                # FraudAlert, AuditLogEntry, AlertSearchParams
├── services/
│   ├── api.ts                 # Axios client + refresh token interceptor
│   ├── auth.service.ts        # login, logout, me
│   ├── users.service.ts       # list, get, update, block, sessions, limits, activity
│   ├── finance.service.ts     # deposits, withdrawals, transactions, adjustBalance, summary
│   ├── sports.service.ts      # bets, events, suspendEvent, liability
│   ├── casino.service.ts      # games, providers, sessions, rtpReport
│   ├── bonuses.service.ts     # campaigns, grantBonus, userBonuses
│   ├── affiliates.service.ts  # affiliates, payouts, fraudFlags
│   ├── risk.service.ts        # alerts, auditLog, userRiskProfile
│   ├── system.service.ts      # dashboardStats, health, featureFlags, config
│   └── content.service.ts     # pages, promotions
├── components/
│   ├── layout/AppLayout.tsx    # Sidebar + Header + Content
│   ├── auth/ProtectedRoute.tsx # Permission-based route guard
│   └── common/
│       ├── DataTable.tsx       # Обёртка над Ant Design Table
│       ├── MoneyDisplay.tsx    # Форматирование сумм
│       ├── SearchInput.tsx     # Поисковый input
│       └── StatusTag.tsx       # Цветные теги статусов
├── pages/
│   ├── Dashboard.tsx           # 4 статистических карточки (HTTP polling 30s)
│   ├── Login.tsx               # Email + password (без TOTP)
│   ├── NotFound.tsx
│   ├── users/
│   │   ├── UserList.tsx        # Таблица игроков (фильтры: search, status, kyc_level)
│   │   └── UserDetail.tsx      # Профиль: Overview + Tabs (Transactions, Bets, Sessions) + RG Limits
│   ├── finance/
│   │   ├── Deposits.tsx        # Список депозитов
│   │   ├── Withdrawals.tsx    # Список выводов + Approve/Reject
│   │   └── Transactions.tsx    # Список транзакций
│   ├── sports/
│   │   └── Bets.tsx            # Список ставок
│   ├── casino/
│   │   ├── Games.tsx           # Список игр
│   │   └── Sessions.tsx        # Список сессий
│   ├── bonuses/
│   │   └── CampaignList.tsx    # Список бонусных кампаний
│   ├── affiliates/
│   │   ├── Affiliates.tsx      # Список партнёров
│   │   ├── AffiliateDetail.tsx # Детали партнёра
│   │   ├── AffiliatePayouts.tsx # Выплаты партнёрам
│   │   └── AffiliateFraudFlags.tsx # Фрод-флаги
│   ├── risk/
│   │   ├── FraudAlerts.tsx     # Алерты фрода (Resolve / False Positive)
│   │   └── AuditLog.tsx        # Лог аудита
│   └── system/
│       ├── Health.tsx          # Статус сервисов
│       └── Config.tsx          # Feature Flags
├── utils/
│   ├── constants.ts           # API_BASE_URL, PAGE_SIZE_OPTIONS, статус-маппинги
│   ├── format.ts              # formatDate, formatMoney
│   ├── errors.ts              # getErrorMessage
│   └── permissions.ts         # ROLE_PERMISSIONS, hasPermission
```

### 1.2 Технологический стек (соответствие ТЗ)

| Требование ТЗ | Реализация | Статус |
|-----------------|------------|--------|
| React 18 + TypeScript 5 | ✅ | Соответствует |
| Ant Design 5.x | ✅ | Соответствует |
| Zustand (client state) | ✅ | Соответствует |
| TanStack Query v5 | ✅ | Соответствует |
| React Router v6 | ✅ | Соответствует |
| Vite 5 | ✅ | Соответствует |
| WebSocket клиент (к Rust WS Gateway) | ❌ | Отсутствует |
| Recharts / Ant Design Charts | ❌ | Отсутствует |
| react-i18next (EN/RU) | ❌ | Отсутствует |
| Vitest + Playwright | ❌ | Не обнаружено |

---

## 2. Маппинг по фазам ТЗ

### ФАЗА 1: Foundation & Infrastructure

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Login: email + password + TOTP | ⚠️ Частично | Есть email+password, **TOTP отсутствует** |
| JWT verify на каждом запросе | ✅ | Interceptor в `api.ts` |
| Refresh token rotation | ✅ | Автоматический refresh при 401 |
| Access token в memory (НЕ localStorage) | ❌ | Хранится в `localStorage` через `zustand/persist` |
| IP whitelist check | ❌ | Не реализовано |
| Audit log на мутирующих запросах | ⚠️ Частично | Есть endpoint `/admin/risk/audit-log`, UI листинг есть, но запись — на стороне BFF |
| WebSocket соединение | ❌ | Отсутствует |
| Sidebar по ролям | ⚠️ Частично | Есть фильтрация по permissions, но **роли не соответствуют ТЗ** |
| Auto-logout 15 мин неактивности | ❌ | Отсутствует |
| RoleGuard + redirect /403 | ✅ | `ProtectedRoute.tsx` |
| ROUTES: `/dashboard`, `/users`, `/users/:id`, `/payments`, `/bonuses`, `/risk`, `/sportsbook`, `/casino`, `/affiliates`, `/crm`, `/kyc`, `/reports`, `/cms`, `/settings`, `/audit`, `/403` | ⚠️ Частично | Многие роуты отсутствуют: `/payments` → `/finance/*`, нет `/kyc`, `/crm`, `/reports`, `/cms`, `/settings` |

**Проблемы ролей:**
- ТЗ требует: `SUPER_ADMIN`, `FINANCE_MANAGER`, `RISK_MANAGER`, `CRM_MANAGER`, `SPORTS_TRADER`, `SUPPORT_AGENT`, `KYC_OFFICER`, `AFFILIATE_MANAGER`, `CONTENT_MANAGER`, `VIEWER`, `COMPLIANCE_OFFICER`
- Реализовано: `support_l1`, `support_l2`, `risk_manager`, `finance`, `marketing`, `admin`, `super_admin`

---

### ФАЗА A1 (Addendum): Foundation дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Session Management Policy (device_fingerprint, country_code, user_agent) | ❌ | Не реализовано |
| Concurrent sessions config | ❌ | Не реализовано |
| Rate Limiting Rules (DragonflyDB, X-RateLimit-* headers) | ❌ | Не реализовано в UI |
| Multi-Currency Display (CurrencyDisplaySettings) | ❌ | Не реализовано |
| Maintenance Mode Middleware | ❌ | Не реализовано |

---

### ФАЗА 2: Dashboard

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Live Overview метрики (WS каждые 5 сек) | ❌ | HTTP polling 30 сек, **WS отсутствует** |
| Online Players (Casino \| Sports \| Total) | ❌ | Не реализовано |
| GGR Today (Casino \| Sports \| Live Casino) | ❌ | Не реализовано |
| FTD Count / Amount Today | ❌ | Не реализовано |
| Pending Withdrawals (count + sum) | ⚠️ Частично | Есть count, но не в real-time |
| Pending KYC | ⚠️ Частично | Есть count в stats, но нет очереди |
| Open Support Tickets | ❌ | Не реализовано (нет тикет-системы) |
| Графики Recharts (GGR/NGR 30d, Deposits vs Withdrawals, Conversion Funnel) | ❌ | Отсутствуют |
| Top 5 Games / Events / Countries | ❌ | Не реализовано |
| Real-time alerts panel (sound + badge) | ❌ | Нет WS |
| Provider Health Widget | ❌ | Не реализовано |
| Payment Gateway Health | ❌ | Не реализовано |

---

### ФАЗА 3: Player Management

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Player List: фильтры (country, reg_date, kyc_status, tags, last_login, deposit_total, ggr, affiliate_id, player_group, balance, risk_score) | ❌ | Реализованы только: search, status, kyc_level |
| Колонки: ID, Username, Country, Reg Date, Deposits, GGR, KYC, Tags, Risk, Last Login | ⚠️ Частично | Нет: Username, Deposits, GGR, Tags, Risk |
| Виртуализация (100K+ строк) | ❌ | Обычная Ant Design Table |
| CSV/Excel export | ❌ | Не реализовано |
| Bulk actions (теги, группа, email) | ❌ | Не реализовано |
| **Player Profile — Tab 0 Overview** | ⚠️ Частично | Есть personal info, но нет: Financial summary, Gaming stats, Tags, Risk Score gauge |
| Quick Actions: [Block] [Adjust Limits] [Give Bonus] [Adjust Balance] [Add Note] [Send Message] | ❌ | Только [Block/Unblock], остальное отсутствует |
| **Tab 1: Deposits** | ⚠️ Частично | Есть "Recent Transactions" (смешаны все типы), не отдельный депозит-таб |
| **Tab 2: Withdrawals** | ⚠️ Частично | В смешанной таблице транзакций |
| **Tab 3: Bets (Casino)** | ❌ | Не реализовано |
| **Tab 4: Bets (Sports)** | ⚠️ Частично | Есть "Recent Bets" (только sports) |
| **Tab 5: Bonuses** | ❌ | Не реализовано |
| **Tab 6: KYC Documents** | ❌ | Не реализовано |
| **Tab 7: Responsible Gambling** | ⚠️ Частично | Только отображение limits, нет: RG Score trend, self-exclusion история |
| **Tab 8: Support History** | ❌ | Не реализовано |
| **Tab 9: Linked Accounts** | ❌ | Не реализовано |
| **Tab 10: Admin Notes & Audit** | ❌ | Не реализовано |
| Player Actions endpoints (block, unblock, limits, adjust-balance, tags, group, notes, give-bonus, request-kyc, send-message) | ❌ | Только block/unblock, limits (GET) |
| Search < 200ms для 1M+ игроков | ⚠️ | Frontend-фильтры, backend-оптимизация не проверена |
| Lazy loading табов | ❌ | Все запросы идут сразу при открытии профиля |
| Adjust balance audit (before/after) | ❌ | Не реализовано |

---

### ФАЗА 4: KYC & Responsible Gambling

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| KYC Queue (HIGH/MEDIUM/LOW приоритеты) | ❌ | Не реализовано |
| KYC Document viewer (zoom, rotate, OCR comparison) | ❌ | Не реализовано |
| KYC Actions (Approve, Reject reasons, Request resubmission) | ❌ | Не реализовано |
| Sumsub integration | ❌ | Не реализовано |
| AML / Source of Funds triggers | ❌ | Не реализовано |
| RG Dashboard (RG Score trend, активные самоисключения) | ❌ | Не реализовано |
| Admin override self-exclusion (RISK_MANAGER) | ❌ | Не реализовано |

### ФАЗА A2 (Addendum): KYC дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Document Expiry Tracking | ❌ | Не реализовано |
| PEP / Sanctions Screening | ❌ | Не реализовано |
| KYC Team Metrics dashboard | ❌ | Не реализовано |

---

### ФАЗА 5: Payment Management

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Deposits: real-time via WS | ❌ | HTTP polling |
| Deposit detail popup (gateway response, timeline) | ❌ | Не реализовано |
| Manual Credit (reason + TOTP) | ❌ | Не реализовано |
| Withdrawals Queue (sort by wait time, risk score, KYC) | ⚠️ Частично | Базовый список + Approve/Reject, но нет: wait time, risk score, KYC статуса, auto-approve checklist |
| Withdrawal Review drawer (история выводов, история депозитов, checklist) | ❌ | Не реализовано |
| Auto-approve rules (risk < 20, amount < $1000, KYC verified) | ❌ | Не реализовано |
| Payment Methods Configuration (матрица Country × Method) | ❌ | Не реализовано |
| P2P Payment Management | ❌ | Не реализовано |
| Financial Reconciliation | ❌ | Не реализовано |

### ФАЗА A3 (Addendum): Payments дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Chargeback Management (full workflow) | ❌ | Не реализовано |
| Platform Balance Sheet (Assets/Liabilities/CoverageRatio) | ❌ | Не реализовано |
| Crypto Wallet Management (hot/cold balance, threshold alerts) | ❌ | Не реализовано |

---

### ФАЗА 6: Bonus Engine

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Bonus Constructor (wizard, 4 шага) | ❌ | Не реализовано |
| Bonus List & Management (Activate/Pause/Deactivate/Clone/Edit/Stats) | ❌ | Только `CampaignList.tsx` — базовый список |
| Active player bonuses (global view) | ❌ | Не реализовано |
| Wagering Monitor (real-time) | ❌ | Не реализовано |
| Bonus types: deposit_match, free_spins, cashback, freebet, express_boost, tournament | ⚠️ Частично | Типы определены в `types/bonus.ts`, но нет UI для создания |
| Game weights для вейджера | ❌ | Не реализовано |
| Preview математики (expected cost) | ❌ | Не реализовано |

---

### ФАЗА 7: Risk & Anti-Fraud

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Real-time alerts feed (WS) | ❌ | HTTP polling |
| Alert types: MULTI_ACCOUNT, MONEY_LAUNDERING, BONUS_ABUSE, CHARGEBACK, HIGH_WIN_RATE, VELOCITY, VPN_PROXY, GEO_MISMATCH | ⚠️ Частично | Реализованы: velocity, amount_anomaly, pattern, multi_account, bonus_abuse, payment_fraud, collusion. **Нет**: CHARGEBACK, HIGH_WIN_RATE, VPN_PROXY, GEO_MISMATCH |
| Actions: [Dismiss] [Block Player] [Hold Withdrawal] [Flag for Review] [View Player] | ⚠️ Частично | Только [Resolve] и [False Positive], остальное отсутствует |
| Multi-Account Detection (граф кластеров) | ❌ | Не реализовано |
| Fingerprint Management (trusted flag) | ❌ | Не реализовано |
| Risk Scoring Configuration (веса факторов, preview) | ❌ | Не реализовано |
| Blacklists & Watchlists (IP, BIN, email domains, crypto wallets) | ❌ | Не реализовано |

### ФАЗА A5 (Addendum): Risk дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Rule Builder (drag-and-drop условия + actions) | ❌ | Не реализовано |
| False Positive Management & Whitelist | ⚠️ Частично | Есть статус `false_positive` в алертах, но нет аналитики FP Rate, причин, whitelist-таблицы |

---

### ФАЗА 8: Sportsbook Management

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Events & Markets (дерево Sport → League → Event → Markets → Outcomes) | ❌ | Не реализовано |
| Odds Management (manual override, diff %) | ❌ | Не реализовано |
| Margin Settings (hierarchy: Global → Sport → League → Event) | ❌ | Не реализовано |
| Limits Management (hierarchy + player group) | ❌ | Не реализовано |
| Liability Monitor (color coding, threshold alerts) | ❌ | Endpoint `/admin/sports/liability` есть, но нет UI |
| Settlement & Results (void, re-settlement) | ⚠️ Частично | Есть `voidBet` и `resettleBet` в `sports.service.ts`, но нет UI |
| Cashout Configuration | ❌ | Не реализовано |

### ФАЗА A6 (Addendum): Sportsbook дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Trading Terminal View (отдельный layout для SPORTS_TRADER) | ❌ | Не реализовано |
| Live Score Integration (Sportradar) | ❌ | Не реализовано |
| Keyboard shortcuts (S/R/A/±/Enter/Esc) | ❌ | Не реализовано |

---

### ФАЗА 9: Casino Games Management

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Game Catalog (5000+ игр, виртуализация, фильтры) | ❌ | Базовый `Games.tsx` — простая таблица |
| Provider Management (wizard, health metrics: latency, error rate, uptime) | ❌ | Базовый список провайдеров |
| RTP Configuration (матрица Provider × Player Group, TOTP + 2-й админ) | ❌ | Не реализовано |
| Game Display Settings (display name, category, sort weight, thumbnail, badge) | ❌ | Не реализовано |
| Jackpot Management (pools, Daily Drops) | ❌ | Не реализовано |

---

### ФАЗА 10: Affiliate Management

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Partner List & Stats (NGR, Owed, Deal Type) | ⚠️ Частично | Базовый список, детали партнёра |
| Deal Configuration (Revenue Share / CPA / Hybrid) | ❌ | Не реализовано |
| Fraud Detection (автоматические флаги) | ⚠️ Частично | Есть `AffiliateFraudFlags.tsx`, но базовый |
| Payouts (Calculate Period, NGR расчёт, carryover) | ❌ | Базовый список выплат |

### ФАЗА B4 (Addendum): Affiliates дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Postback Configuration (JSONB в affiliate) | ❌ | Не реализовано |
| Postback Logs (retry, replay) | ❌ | Не реализовано |

---

### ФАЗА 11: CRM & Retention

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Player Segments constructor (AND/OR, ClickHouse) | ❌ | Не реализовано |
| Automated Triggers (event-based, A/B Test) | ❌ | Не реализовано |
| Manual Campaigns (target segment, schedule, preview) | ❌ | Не реализовано |
| Communication Templates (WYSIWYG, variables, multilingual) | ❌ | Не реализовано |

### ФАЗА A7 (Addendum): CRM дополнения

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Suppression Lists (unsubscribed, hard_bounce, spam_complaint, self_excluded, gdpr_erasure) | ❌ | Не реализовано |
| RG + Marketing Suppression link | ❌ | Не реализовано |
| Communication Frequency Caps (DragonflyDB counters) | ❌ | Не реализовано |

---

### ФАЗА 12: Analytics & System Settings

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Financial Reports (GGR/NGR, Deposit/Withdrawal, P&L, Hold Report) | ❌ | Не реализовано |
| Player Analytics (Cohort, Conversion Funnel, Unit Economics) | ❌ | Не реализовано |
| Game Analytics (Actual RTP vs Theoretical) | ❌ | Не реализовано |
| Admin Users Management (только SUPER_ADMIN) | ❌ | Не реализовано |
| IP Whitelist Management | ❌ | Не реализовано |
| Audit Log Viewer (фильтры, export, нельзя удалить) | ⚠️ Частично | Базовый `AuditLog.tsx`, нет фильтров по admin_id/action/entity_type/date, нет export |
| Feature Flags (toggle без deploy) | ✅ | `Config.tsx` — переключение флагов |
| Maintenance mode settings | ❌ | Не реализовано |

---

### ФАЗА A4 (Addendum): Support Ticket System — НОВАЯ ФАЗА

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Support Ticket System (полный модуль) | ❌ | **Полностью отсутствует** |
| Schema: support_tickets, ticket_messages, ticket_links | ❌ | Не реализовано |
| UI: Ticket List, Ticket Detail (drawer), Message thread, Linked entities, Team Dashboard | ❌ | Не реализовано |
| SLA Configuration (first response, resolution) | ❌ | Не реализовано |
| Auto-priority Rules (VIP/Whale, pending withdrawal) | ❌ | Не реализовано |

---

### ФАЗА 14: Regulatory Reporting (Addendum v1.2)

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| COMPLIANCE_OFFICER role | ❌ | Не реализовано |
| Regulatory Reports (UKGC, MGA, General) | ❌ | Не реализовано |
| SAR Management (confidential, tipping-off lock) | ❌ | Не реализовано |
| Complaints Log (ADR escalation) | ❌ | Не реализовано |
| Tax Configuration (jurisdiction → rate) | ❌ | Не реализовано |
| Player Funds Reconciliation (segregation ratio) | ❌ | Не реализовано |

---

### ФАЗА B1–B3 (Post-launch)

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Shift Report (PDF, Telegram) | ❌ | Не реализовано |
| GEO Map Widget (WorldMap, D3/react-simple-maps) | ❌ | Не реализовано |
| Player Merge (SUPER_ADMIN + TOTP + 2-й админ) | ❌ | Не реализовано |
| Unified Communication Timeline | ❌ | Не реализовано |
| Provider Revenue Settlement | ❌ | Не реализовано |

---

### Приоритет C (Nice to have)

| Требование | Статус | Комментарий |
|------------|--------|-------------|
| Custom Report Builder | ❌ | Не реализовано |
| Custom Markets (Sportsbook) | ❌ | Не реализовано |
| Affiliate Media Library | ❌ | Не реализовано |
| Game Demo/Testing Mode | ❌ | Не реализовано |
| In-app Changelog | ❌ | Не реализовано |
| Alert Sound Settings | ❌ | Не реализовано |

---

## 3. Критические проблемы безопасности и архитектуры

### 🔴 HIGH

1. **Access Token в localStorage** (`authStore.ts:27` использует `zustand/persist`)
   - ТЗ требует: "access_token хранится в memory (НЕ localStorage)"
   - Риск: XSS-атака может украсть токен
   - **Рекомендация:** Убрать persist для access_token, хранить только в Zustand state (memory). Refresh token можно оставить в cookie (httpOnly) на стороне BFF.

2. **Отсутствует TOTP в логине**
   - ТЗ требует: email → password → если TOTP enabled → запрос TOTP кода
   - Текущий `Login.tsx` отправляет только email + password
   - **Риск:** 2FA не работает, критично для финансовых операций

3. **Роли не соответствуют ТЗ**
   - Отсутствуют критичные роли: `FINANCE_MANAGER`, `RISK_MANAGER` (есть `risk_manager` в snake_case, но нет `FINANCE_MANAGER`), `KYC_OFFICER`, `SPORTS_TRADER`, `CONTENT_MANAGER`, `COMPLIANCE_OFFICER`
   - Права `withdrawal.approve_large`, `transaction.adjust`, `content.manage` не используются в UI роутах
   - **Риск:** RBAC не обеспечивает разделение обязанностей по ТЗ

4. **Нет auto-logout по бездействию**
   - ТЗ: "15 минут auto-logout при неактивности"
   - Отсутствует таймер неактивности

### 🟡 MEDIUM

5. **Нет WebSocket — весь real-time функционал не работает**
   - Dashboard не обновляется live
   - Новые withdrawals не появляются мгновенно
   - Risk alerts не приходят в real-time
   - Provider health не мониторится live

6. **Нет i18n (react-i18next)**
   - ТЗ требует EN/RU минимум
   - Весь UI на английском, без механизма перевода

7. **User Detail загружает все данные сразу**
   - ТЗ требует lazy loading табов
   - Сейчас 5+ параллельных запросов при открытии профиля

8. **Отсутствует Idempotency-Key header**
   - ТЗ требует для всех мутирующих запросов

9. **Нет Recharts / графиков**
   - ТЗ требует для Dashboard, RG Score trend, Cohort Analysis и др.

---

## 4. Сводная таблица готовности по фазам

| Фаза | Модуль | Готовность | Критичность |
|------|--------|------------|-------------|
| 1 | Foundation & Infrastructure | ~40% | 🔴 Критично |
| A1 | Foundation дополнения | ~5% | 🔴 Критично |
| 2 | Dashboard | ~15% | 🔴 Критично |
| 3 | Player Management | ~25% | 🔴 Критично |
| 4 | KYC & RG | ~5% | 🔴 Критично |
| A2 | KYC дополнения | 0% | 🔴 Критично |
| 5 | Payment Management | ~20% | 🔴 Критично |
| A3 | Payments дополнения | 0% | 🔴 Критично |
| A4 | Support Ticket System | 0% | 🔴 Критично (новая фаза) |
| 6 | Bonus Engine | ~15% | 🟡 Важно |
| 7 | Risk & Anti-Fraud | ~20% | 🟡 Важно |
| A5 | Risk дополнения | ~5% | 🟡 Важно |
| 8 | Sportsbook Management | ~10% | 🟡 Важно |
| A6 | Sportsbook дополнения | 0% | 🟡 Важно |
| 9 | Casino Games Management | ~15% | 🟡 Важно |
| 10 | Affiliate Management | ~20% | 🟡 Важно |
| A7 | CRM дополнения | 0% | 🟡 Важно |
| 11 | CRM & Retention | 0% | 🟢 После запуска |
| 12 | Analytics & Settings | ~10% | 🟢 После запуска |
| 14 | Regulatory Reporting | 0% | 🔴 Критично (лицензия) |
| B1–B4 | Post-launch дополнения | 0% | 🟢 После запуска |

---

## 5. Рекомендации по порядку реализации

### Этап 1: Foundation Fix (1–2 недели)
1. Исправить хранение access_token (убрать из localStorage)
2. Добавить TOTP flow в Login
3. Пересмотреть роли и permissions под ТЗ
4. Добавить auto-logout по бездействию (15 мин)
5. Добавить WebSocket клиент (Rust Gateway)
6. Добавить i18n scaffolding (react-i18next)

### Этап 2: Критичные бизнес-модули (3–4 недели)
7. KYC Queue (документы, approve/reject, Sumsub)
8. Support Ticket System (новая фаза A4)
9. Withdrawals Queue с checklist + auto-approve
10. Chargeback workflow
11. Document Expiry Tracking

### Этап 3: Dashboard & Real-time (2–3 недели)
12. WebSocket topics: admin.metrics.live, admin.risk.alerts, admin.withdrawals.new
13. Recharts: GGR/NGR, Conversion Funnel, Deposits vs Withdrawals
14. Provider Health Widget
15. GEO Map Widget (B1.2)

### Этап 4: Player Management deep dive (2–3 недели)
16. Player Detail tabs (KYC, Bonuses, Casino Bets, Linked Accounts, Admin Notes)
17. Quick Actions (Adjust Balance, Give Bonus, Add Note, Send Message)
18. Player List: расширенные фильтры, CSV export, bulk actions
19. Player Merge (B2.1)

### Этап 5: Risk & Compliance (2–3 недели)
20. Rule Builder (A5.1)
21. PEP/Sanctions Screening (A2.2)
22. Multi-Account Detection граф
23. Regulatory Reporting (Фаза 14)

### Этап 6: Sportsbook & Casino (2–3 недели)
24. Trading Terminal (A6.1)
25. Live Score Integration (A6.2)
26. RTP Configuration (Фаза 9.3)
27. Jackpot Management (Фаза 9.5)

### Этап 7: CRM & Analytics (2–3 недели)
28. Player Segments + Triggers
29. Communication Templates
30. Financial Reports (ClickHouse)
31. Shift Reports (B1.1)

---

*Аудит выполнен на основе файлов:*
- `tasks/admin.task.md` (3688 строк)
- `apps/admin/src/*` (весь React-код админ-панели)
