# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# =============================================
# Единая техническая спецификация для AI-агентов
# Профиль: FRONTEND_WEB_ENGINEER
# Стек: Next.js 14 (App Router) | Tailwind CSS 3 | Zustand | React Query
# Расположение: apps/web/
# =============================================

## ⛔ ПЕРЕД НАЧАЛОМ РАБОТЫ

1. Прочитай `CONVENTIONS.md` — запрещённые действия, структура проекта
2. Прочитай `AGENTS.md` — выбери профиль FRONTEND_WEB_ENGINEER
3. Загрузи skills из секции "AI АГЕНТ: ОБЯЗАТЕЛЬНЫЕ SKILLS" ниже
4. Прочитай ВСЕ 6 частей этой спецификации (таблица ниже)

---

## 🤖 AI АГЕНТ: ОБЯЗАТЕЛЬНЫЕ SKILLS

Перед написанием кода в `apps/web/` загрузи эти skills (см. AGENTS.md workflow):

```
ОБЯЗАТЕЛЬНЫЕ (всегда, 7 файлов — из _agents.md profile FRONTEND_WEB_ENGINEER):
  1. CONVENTIONS.md                                          ← закон проекта
  2. .qwen/skills/architecture/architecture-overview.skill.md  ← общая архитектура
  3. .qwen/skills/frontend/nextjs-general.skill.md             ← Next.js 14 App Router, RSC, Server Actions
  4. .qwen/skills/frontend/nextjs-components.skill.md          ← паттерны компонентов
  5. .qwen/skills/frontend/nextjs-state-management.skill.md    ← Zustand + React Query (client/server state)
  6. .qwen/skills/frontend/nextjs-api-integration.skill.md     ← fetch, mutations, WebSocket client
  7. .qwen/skills/frontend/typescript-shared.skill.md          ← TS типы, gen/typescript/
  8. .qwen/skills/security/security-general.skill.md           ← всегда при работе с данными
  9. весь FRONTEND_SPEC.md (6 частей)

ДОПОЛНИТЕЛЬНЫЕ (по задаче):
  - Sports betting UI:    .qwen/skills/domain-specific/betting-engine-logic.skill.md
  - Casino lobby / iframe: .qwen/skills/domain-specific/casino-integration.skill.md
  - Wallet UI:            .qwen/skills/domain-specific/wallet-financial-ops.skill.md
  - Bonus system UI:      .qwen/skills/domain-specific/bonus-system-logic.skill.md
  - KYC documents UI:     .qwen/skills/domain-specific/kyc-aml-compliance.skill.md
  - RG / Self-exclusion:  .qwen/skills/domain-specific/responsible-gambling.skill.md
  - Auth / 2FA flow:      .qwen/skills/security/authentication-patterns.skill.md
  - Шифрование (BetSlip): .qwen/skills/security/encryption-patterns.skill.md
  - Form validation:      .qwen/skills/security/input-validation.skill.md
  - Frontend design:      .qwen/skills/frontend/frontend-design.skill.md (Tailwind паттерны)
  - API contract:         .qwen/skills/architecture/api-design-guidelines.skill.md
```

ПРАВИЛА (из AGENTS.md):
- Максимум **5–10 skills за раз** — больше не поместится в контекст.
- Если задача пересекает 2 домена — загрузи skills из обоих.

---

## 📖 СТРУКТУРА СПЕЦИФИКАЦИИ

| Файл | Содержимое | Разделы |
|------|-----------|---------|
| [Part 1: Design System](frontend-spec-part1-design-system.md) | Визуальный фундамент | §1-5: Цвета (color-blind safe), типографика, spacing, анимации, glassmorphism, **shadow scale**, **z-index hierarchy**, компоненты UI, layout, performance budget, accessibility (WCAG 2.2 AA) |
| [Part 2: Pages & Components](frontend-spec-part2-pages.md) | Страницы и бизнес-логика | §6-8: Главная (hero **dvh**, **jackpot monotonic**, **live wins opt-in**), Casino lobby, Sportsbook, BetSlip, Bet Builder, Wallet, **Bonuses (bonus-first list, contribution rates, forfeit, expiry)**, Profile, **KYC docs UI (liveness, blur detection, SoF)**, Responsible Gambling |
| [Part 3: Edge Cases](frontend-spec-part3-edge-cases.md) | Нестандартные сценарии | §9-26: Error states, offline, WebSocket reconnect, iframe edge cases, multi-tab + **BetSlip security**, timezone, geo-restrictions, push, logout, i18n/RTL, legal UI, deep linking, **affiliate (iOS ITP fix)**, repeat bet, multicurrency, **§25 Auth Token Strategy**, **§26 Decimal Money Handling** |
| [Part 4: Auth & Onboarding Flows](frontend-spec-part4-auth-flows.md) | **Auth domain** | §27-32: Registration funnel, Login + 2FA, Password reset, Email verification, Soc-login, KYC documents flow, GAMSTOP/BetStop/OASIS интеграция, возрастные gates |
| [Part 5: Live Casino & Crash](frontend-spec-part5-live-casino-crash.md) | **Новые домены** | §33-38: Live Casino (stream, bet timer, multi-table, dealer chat, tips, bandwidth degrade); Crash games (canvas curve, auto-cashout, provably fair, multiplayer view) |
| [Part 6: Integrations & Infra](frontend-spec-part6-integrations.md) | **Интеграции и инфра** | §39-46: API/WS URL strategy, env variables, proto/gRPC-Web, Service Worker scope, Toast system, Analytics events catalog, A/B testing, SEO (schema.org, sitemap, hreflang) |

---

## 🎯 ПРИНЦИПЫ

### Дизайн
- **Dark-only** — нет светлой темы (архитектурное решение)
- **Obsidian Frost** — графитово-чёрная база + белый/платиновый CTA + холодный cyan/mint accent
- **No Gold Theme** — золото НЕ является брендовым цветом; jackpot/VIP = frost/platinum/cyan glow, gold/yellow только как warning/внешний provider asset
- **Orange ≠ Red** — падение кэфа = оранжевый, проигрыш = красный
- **Color + Icon/Text** — НИКОГДА только цвет для передачи состояния, всегда icon/text
- **Mobile-first** — дизайн от 375px, расширяется до 1440px
- **Glassmorphism** — с fallback на opaque bg для low-end устройств

### UX
- **Skeleton > Spinner** — ВСЕГДА skeleton, НИКОГДА spinner (включая Bet Builder)
- **Optimistic Updates** — купон мгновенно, откат при ошибке
- **Single Source of Truth** — баланс = wallet service
- **Graceful Degradation** — offline кэш для истории, ставки/игры блокируются

### Технические
- **FCP < 1.5s, LCP < 2.5s, CLS < 0.1, JS < 300KB gzip**
- **WebSocket** с exponential backoff (6+ попыток), sequence numbers
- **Idempotency** — каждая финансовая операция с idempotency_key

---

## 🗺️ CANONICAL ROUTES (единственный источник истины)

```
/                                ← Главная (hero + промо)
/casino                          ← Лобби казино
/casino/live                     ← Live casino lobby
/casino/live/multi               ← Multi-table live casino
/casino/crash                    ← Crash games lobby
/casino/aviator                  ← Crash/Aviator game page
/casino/favorites                ← Избранное
/casino/game/{slug}              ← Страница игры (iframe)
/casino/category/{slug}          ← Категория казино
/casino/provider/{slug}          ← Страница провайдера
/sportsbook                      ← Ставки prematch
/sportsbook/live                 ← Live события
/sportsbook/event/{id}           ← Событие + Bet Builder
/sportsbook/{sport}              ← Страница вида спорта
/sportsbook/{sport}/{league}     ← Страница лиги
/bets                            ← Активные ставки
/bets/history                    ← История ставок
/bets/{id}                       ← Детали ставки
/wallet                          ← Обзор кошелька
/wallet/deposit                  ← Депозит
/wallet/withdraw                 ← Вывод
/wallet/transactions             ← История транзакций
/bonuses                         ← Бонусы и промо
/promotions/{slug}               ← Страница промо-акции
/affiliate                       ← Реферальная программа
/profile                         ← Профиль
/profile/settings                ← Настройки
/profile/security/2fa            ← Настройка 2FA
/profile/notifications           ← Уведомления и marketing consent
/profile/privacy                 ← Privacy, cookies, GDPR export/erasure
/profile/kyc                     ← Верификация
/profile/responsible             ← Ответственная игра
/profile/responsible/self-exclude ← Самоисключение
/support                         ← Live-чат + FAQ
/notifications                   ← Центр уведомлений
/about                           ← О нас
/rules                           ← Правила
/responsible-gambling             ← Ответственная игра (публичная)
/privacy                         ← Конфиденциальность
/terms                           ← Условия
/login                           ← Вход
/login/2fa                       ← Подтверждение 2FA
/register                        ← Регистрация
/forgot-password                 ← Восстановление пароля
/reset-password                  ← Новый пароль по token
/auth/verify-email               ← Подтверждение email
/onboarding/limits               ← RG pre-commitment onboarding
/provably-fair-explained         ← Объяснение provably fair
/blog/{slug}                     ← SEO/blog article

ЗАПРЕЩЁННЫЕ АЛЬТЕРНАТИВЫ:
  /sports/event/{id}   → ИСПОЛЬЗОВАТЬ /sportsbook/event/{id}
  /casino/{slug}       → ИСПОЛЬЗОВАТЬ /casino/game/{slug}
  /casino?tab=live     → ИСПОЛЬЗОВАТЬ /casino/live
  /casino?tab=favorites → ИСПОЛЬЗОВАТЬ /casino/favorites
  /profile/deposit     → ИСПОЛЬЗОВАТЬ /wallet/deposit
  /promo               → ИСПОЛЬЗОВАТЬ /bonuses или /promotions/{slug}
  /event/{id}          → ИСПОЛЬЗОВАТЬ /sportsbook/event/{id}
```

---

## 📡 CANONICAL API (frontend → backend)

```
AUTH:
  POST /api/v1/auth/register
  POST /api/v1/auth/check-email
  POST /api/v1/auth/login
  POST /api/v1/auth/refresh
  POST /api/v1/auth/logout
  POST /api/v1/auth/forgot-password
  POST /api/v1/auth/password-reset-request
  POST /api/v1/auth/password-reset
  GET  /api/v1/auth/verify-email
  POST /api/v1/auth/2fa/enroll
  POST /api/v1/auth/2fa/verify
  POST /api/v1/auth/2fa/disable
  POST /api/v1/auth/verify-2fa

BETS:
  POST /api/v1/bets/quote              ← Валидация + актуальные odds
  POST /api/v1/bets                    ← Размещение (idempotency_key обязателен)
  GET  /api/v1/bets/{id}               ← Детали ставки
  GET  /api/v1/bets/by-idempotency/{key} ← Проверка результата после timeout
  GET  /api/v1/bets?status=active      ← Активные ставки
  GET  /api/v1/bets?status=settled     ← История
  POST /api/v1/bets/{id}/cashout       ← Кэшаут (idempotency_key)
  GET  /api/v1/sports/events/{id}/odds ← Resync live/prematch odds

CASINO:
  POST /api/v1/casino/games/{slug}/launch  ← Получить one-time launch token
  POST /api/v1/casino/games/{slug}/end     ← Завершить сессию
  POST /api/v1/casino/live/tables/{id}/launch ← Live table launch token
  POST /api/v1/casino/live/tables/{id}/end    ← Завершить live table session
  GET  /api/v1/casino/games                ← Каталог (paginated)

CRASH:
  GET  /api/v1/crash/games
  POST /api/v1/crash/{game}/bet
  POST /api/v1/crash/{game}/cashout
  GET  /api/v1/crash/{game}/fairness/{round_id}

PAYMENTS:
  POST /api/v1/payments/deposit        ← Создать депозит (idempotency_key)
  POST /api/v1/payments/withdraw       ← Создать вывод (idempotency_key)
  GET  /api/v1/payments/transactions   ← История

WALLET:
  GET  /api/v1/wallet/balance          ← Текущий баланс (real + bonus)

KYC:
  POST /api/v1/kyc/session
  POST /api/v1/kyc/documents
  GET  /api/v1/kyc/status

RESPONSIBLE GAMBLING:
  GET  /api/v1/rg/status
  POST /api/v1/rg/limits
  POST /api/v1/rg/cooling-off
  POST /api/v1/rg/self-exclude

COMPLIANCE / GEO:
  GET  /api/v1/geo/restrictions
  GET  /api/v1/geo/legal-config
  POST /api/v1/compliance/gamstop-check
  POST /api/v1/compliance/betstop-check
  POST /api/v1/compliance/oasis-check

NOTIFICATIONS:
  GET  /api/v1/notifications
  POST /api/v1/notifications/push/subscribe

USER / PRIVACY:
  POST /api/v1/user/data-export
  POST /api/v1/user/erasure-request
```

---

## 🔐 RESPONSIBLE GAMBLING GLOBAL GUARD

```
RG Guard проверяется ПЕРЕД:
  - запуском casino игры
  - каждой ставкой (place bet)
  - депозитом
  - возобновлением casino session

СОСТОЯНИЯ:
  OK              → всё разрешено
  WARNING_50      → toast
  WARNING_75      → toast + banner
  MODAL_90        → модальное окно
  BLOCKED_100     → ставки/игры/депозиты disabled, вывод доступен
  COOLING_OFF     → всё disabled кроме вывода, CTA поддержка
  SELF_EXCLUDED   → всё disabled, CTA support, no marketing

PANIC BUTTON (MGA требование):
  Расположение: header (desktop) + mobile nav (долгий нажим на профиль)
  Действие: мгновенный cooling_off на 24ч
  Подтверждение: одно нажатие (без "вы уверены?")

COOLING-OFF ПРИ ПОВЫШЕНИИ ЛИМИТА:
  Повышение deposit/loss/session лимита → задержка 24ч
  UI: "⏳ Новый лимит вступит в силу через 24ч"
  Понижение → мгновенно
```

---

## 🔄 STATE DIAGRAMS

### Auth/Session
```
ANONYMOUS → LOGIN → AUTHENTICATED → TOKEN_EXPIRED → REFRESH → AUTHENTICATED
                                   → LOGOUT → ANONYMOUS
                                   → PASSWORD_CHANGED → ALL_SESSIONS_INVALIDATED → ANONYMOUS
```

### KYC
```
UNVERIFIED → DOCUMENTS_UPLOADED → PENDING_REVIEW → VERIFIED
                                                  → REJECTED → REUPLOAD → PENDING_REVIEW
                                                  → EXPIRED → REUPLOAD
VERIFIED → SOURCE_OF_FUNDS_REQUIRED → SOF_PENDING → VERIFIED / SOF_REJECTED
```

### BetSlip
```
EMPTY → HAS_SELECTIONS → QUOTED (POST /bets/quote) → PLACING (POST /bets)
  → ACCEPTED (200) → EMPTY + receipt
  → REJECTED (4xx) → HAS_SELECTIONS + error
  → TIMEOUT (>10s) → PENDING_CHECK (polling by idempotency_key)
    → RESOLVED → ACCEPTED / REJECTED
```

### Casino Session
```
PREFLIGHT (auth+geo+KYC+RG+provider) → TOKEN_ISSUED → IFRAME_LOADING
  → ACTIVE → INTERRUPTED (connection loss) → RESUMED / ENDED
  → ENDED (user exit / back button)
PREFLIGHT FAILED → ERROR (actionable message)
```

### Payment
```
INITIATED → PSP_REDIRECT/3DS_CHALLENGE → PROCESSING → COMPLETED / FAILED / MANUAL_REVIEW
PENDING >30min → push notification
PENDING >24h → support escalation
```

---

## ✅ CHECKLIST ДЛЯ AI-АГЕНТА

```
□ Маршруты: только из CANONICAL ROUTES (выше)
□ API: только из CANONICAL API (выше)
□ Шрифты: основной текст ≥14px (НЕ 9-11px)
□ Цвета: Obsidian Frost — white/platinum CTA + cyan/mint accent, НЕ gold theme и НЕ generic blue
□ Кэф упал = ОРАНЖЕВЫЙ (#FF6E40) + ↓, НЕ красный
□ Кэф вырос = ЗЕЛЁНЫЙ (#00E676) + ↑ (color-blind safe)
□ Touch targets: ≥44×44px на mobile (WCAG 2.5.8)
□ Focus ring: cyan/frost (#67E8F9 или #F8FAFC), НЕ зелёный (зелёный = FINANCIAL_POSITIVE)
□ Hero: 60dvh mobile (НЕ 100vh, iOS Safari bug)
□ Skeleton loading для КАЖДОГО async-компонента
□ Error state для КАЖДОГО компонента (не "Something went wrong")
□ prefers-reduced-motion учтён (jackpot, ticker, animations)
□ Z-index из иерархии §1.7, НЕ произвольные числа

A11Y:
□ aria-label для иконок без текста
□ aria-live="polite" для баланса, "assertive" для ошибок
□ Color-blind safe: arrows ↑/↓ рядом с цветом
□ Focus trap в модалах и bottom sheets

MONEY:
□ decimal.js везде (НЕ Number / parseFloat)
□ API возвращает string для баланса
□ Balance через JetBrains Mono + Intl.NumberFormat из ПРОФИЛЯ юзера
□ Перед отправкой: full precision, БЕЗ округления

SECURITY (CONVENTIONS NEVER):
□ user_id ИСКЛЮЧИТЕЛЬНО из JWT (НЕ из body или query)
□ Access token in-memory (Zustand), НЕ localStorage
□ Refresh token в httpOnly+Secure+SameSite=Strict cookie
□ CSRF token в X-CSRF-Token header для mutating endpoints
□ Idempotency-Key для всех bet/deposit/withdrawal
□ NEXT_PUBLIC_* только для ПУБЛИЧНЫХ значений
□ dangerouslySetInnerHTML НЕ используется (или DOMPurify)
□ iframe sandbox: allow-scripts; allow-same-origin только per-provider exception после security review
□ postMessage origin checks (whitelist)

INTEGRATION:
□ API типы из gen/typescript/ (НЕ вручную!)
□ WebSocket re-auth flow (§25 в part3)
□ Single-flight refresh при 401 (§25)
□ SSE fallback при блокировке WS

QA:
□ Responsive: проверен на 375px, 768px, 1280px
□ TypeScript: npm run type-check проходит
□ ESLint: npm run lint проходит
□ Протестирован на iOS Safari (dvh, ITP, audio context)
□ Color-blind тест в Chrome DevTools (deuteranopia, protanopia)
□ prefers-reduced-motion: все анимации отключаются
```

---

## ⛔ EXPLICIT OUT OF SCOPE (v1)

```
- Poker/PvP lobby и matchmaking
- Trade bots / skin betting / external inventory
- Fantasy sports
- P2P betting exchange
- Multi-player tournament tables (live dealer tournaments через провайдера — IN scope)
```
