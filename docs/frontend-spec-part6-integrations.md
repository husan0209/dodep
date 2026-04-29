# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 6: Integrations & Infrastructure
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# Расположение: apps/web/

---

## 39. API GATEWAY & WEBSOCKET URL STRATEGY

```
ПРОБЛЕМА: backend = микросервисы на разных портах (CONVENTIONS):
  betting-engine: 8080, wallet-core: 8081, websocket-gateway: 8082, ...
ИЗ браузера НЕ обращаемся к каждому порту отдельно (CORS hell + URL drift).
РЕШЕНИЕ: единый API Gateway через NGINX (infra/docker/nginx/) или Envoy.

URL ROUTING (NGINX):
  https://opus.casino/api/v1/auth/*          → auth:8083
  https://opus.casino/api/v1/users/*         → user:8085
  https://opus.casino/api/v1/wallet/*        → wallet-core:8081
  https://opus.casino/api/v1/bets/*          → betting-engine:8080
  https://opus.casino/api/v1/payments/*      → payment:8084
  https://opus.casino/api/v1/casino/*        → casino:8086
  https://opus.casino/api/v1/bonuses/*       → bonus:8088
  https://opus.casino/api/v1/notifications/* → notification:8087
  https://opus.casino/api/v1/kyc/*           → kyc:8089
  https://opus.casino/api/v1/affiliate/*     → affiliate:8090
  
  https://opus.casino/ws                     → websocket-gateway:8082
  https://opus.casino/sse/notifications      → notification:8087 (SSE fallback)

ENVIRONMENT VARIABLES:
  Frontend (NEXT_PUBLIC_*, expose к browser):
    NEXT_PUBLIC_API_URL          = https://opus.casino/api/v1
    NEXT_PUBLIC_WS_URL           = wss://opus.casino/ws
    NEXT_PUBLIC_SSE_URL          = https://opus.casino/sse
    NEXT_PUBLIC_RECAPTCHA_KEY    = 6LdXXX-public-key
    NEXT_PUBLIC_TURNSTILE_KEY    = 0x4AAA-public-key (Cloudflare)
    NEXT_PUBLIC_GTM_ID           = GTM-XXXXX (если используется)
    NEXT_PUBLIC_AMPLITUDE_KEY    = aXXX-public-key
    NEXT_PUBLIC_SENTRY_DSN       = https://xxx@sentry.io/yyy
    NEXT_PUBLIC_FEATURE_FLAGS_URL= https://flags.opus.casino
    NEXT_PUBLIC_CDN_URL          = https://cdn.opus.casino
    NEXT_PUBLIC_LOCALE_DEFAULT   = ru-RU
    NEXT_PUBLIC_DEFAULT_CURRENCY = RUB
    NEXT_PUBLIC_RG_HELPLINE      = +7 800 ... (зависит от региона)
  
  Server-only (Next.js API routes / SSR, НЕ префикс NEXT_PUBLIC):
    SESSION_SECRET               = (HMAC для cookie sign)
    RECAPTCHA_SECRET             = (server-side validation)
    OPENEXCHANGERATES_API_KEY    = (currency rates — SERVER ONLY!)
    SUMSUB_PRIVATE_KEY           = (KYC SDK init token)
    SENTRY_AUTH_TOKEN            = (release upload)

⚠️ ПРАВИЛО: Любой ключ который НЕ должен видеть конкурент / атакующий 
   = БЕЗ NEXT_PUBLIC_. Иначе он окажется в bundle.

CORS:
  Same-origin для production (opus.casino → /api → opus.casino).
  Dev: NGINX proxy 3000 → 8080-8090 на localhost.
  CORS preflight только для cross-origin webhooks (Stripe, etc.) — это backend.

CSP HEADERS (next.config.js):
  Content-Security-Policy:
    default-src 'self';
    script-src 'self' 'nonce-{random}' https://www.googletagmanager.com https://js.hcaptcha.com;
    style-src 'self' 'unsafe-inline';  // Tailwind requires inline (или через nonce)
    img-src 'self' data: https://cdn.opus.casino https://*.evolution.live;
    media-src 'self' https://*.evolution.live https://*.pragmaticplay.net;
    frame-src https://*.evolution.live https://*.pragmaticplay.net https://*.spribe.com;
    connect-src 'self' wss://opus.casino https://api.amplitude.com;
    font-src 'self' data:;
    object-src 'none';
    base-uri 'self';
    form-action 'self';
    frame-ancestors 'none';
  
  Per-iframe-провайдер: whitelist origins.
  ⚠️ Не используй 'unsafe-inline' для script-src в production.
```

---

## 40. PROTO / gRPC INTEGRATION

```
CONVENTIONS требует: единый proto namespace github.com/opus-casino/proto
Frontend использует TypeScript-сгенерированные типы из libs/proto/.

ВАРИАНТЫ ИНТЕГРАЦИИ (выбор архитектора):

ВАРИАНТ A: Connect-RPC (рекомендован для нового проекта)
  + Один контракт (proto) → backend Go/Rust + frontend TypeScript
  + Работает поверх HTTP/1.1 + HTTP/2 (без Envoy)
  + Простая поддержка streaming (Server-Sent Events / WebSocket)
  + Легко отлаживать через curl
  - Меньше adoption чем чистый gRPC
  
  Setup:
    backend: connectrpc.com/connect-go (для существующих Go services)
    frontend: @connectrpc/connect-web + @bufbuild/protobuf
    Generation: buf generate → libs/proto/gen/typescript/
  
  Использование:
    import { createPromiseClient } from "@connectrpc/connect";
    import { createConnectTransport } from "@connectrpc/connect-web";
    import { WalletService } from "@/gen/wallet/v1/wallet_connect";
    
    const transport = createConnectTransport({ baseUrl: API_URL });
    const wallet = createPromiseClient(WalletService, transport);
    const balance = await wallet.getBalance({});

ВАРИАНТ B: gRPC-Web (через Envoy)
  + "Чистый" gRPC, тот же что backend
  - Требует Envoy proxy (operational complexity)
  - Sluggish streaming через HTTP/1.1
  
  Frontend: @improbable-eng/grpc-web

ВАРИАНТ C: REST gateway (gRPC-Gateway)
  + Минимальная сложность для frontend (REST, как обычно)
  + OpenAPI swagger автоматический
  - Дополнительный layer + duplicate definitions
  - Streaming через SSE / отдельный WebSocket

РЕКОМЕНДАЦИЯ: Connect-RPC для всех новых endpoints, REST gateway для legacy.

TYPE SAFETY (КРИТИЧНО):
  ВСЕ frontend API types ДОЛЖНЫ быть из gen/typescript/.
  ❌ ЗАПРЕЩЕНО: писать types руками "// мне кажется это поле есть"
  ❌ ЗАПРЕЩЕНО: any в API response
  ❌ ЗАПРЕЩЕНО: модифицировать gen/* вручную
  ✅ ОБЯЗАТЕЛЬНО: import { XxxService } from "@/gen/...";

CI:
  Pre-commit hook: buf generate → check git status (если diff → fail).
  GitHub Action: проверка proto contracts ДО merge.
  Breaking change detection: buf breaking against main branch.

WS (Real-time):
  Subscriptions через WebSocket → channel-based:
    /ws → connect → send {type: "subscribe", channel: "odds.football"}
  Messages: protobuf binary OR JSON (легче дебажить)
    Для odds (high frequency): protobuf binary через WebSocket binary frame
    Для chat: JSON
  Backend service: websocket-gateway (port 8082, см. CONVENTIONS).
```

---

## 41. SERVICE WORKER & PWA

```
SCOPE:
  Service Worker регистрируется на root (/), управляет всем доменом.
  Файл: apps/web/public/sw.js (генерируется next-pwa или Workbox).

LIFECYCLE:
  1. Регистрация: navigator.serviceWorker.register('/sw.js')
     При первом visit (не сразу — после load + idle)
  2. Install: cache static assets (HTML shell, CSS, fonts, base JS)
  3. Activate: cleanup старых кэшей
  4. Fetch: intercept network requests
  5. Update: при deploy → новый SW → "Update available" toast → user click → skipWaiting

CACHE STRATEGIES (per route pattern):
  
  Static assets (/_next/static/*, /fonts/*):
    Strategy: CacheFirst (immutable hashed filenames)
    TTL: 1 год
  
  Images (/cdn/games/*):
    Strategy: StaleWhileRevalidate
    TTL: 7 дней, max 200 entries (LRU)
  
  HTML shell (/, /casino, /sportsbook):
    Strategy: NetworkFirst, fallback Cache (offline)
    Timeout: 3 секунды → fall back to cache
  
  API odds (/api/v1/odds/*):
    Strategy: NetworkOnly (никогда не кэшировать! Stale odds = financial risk)
    НЕ интерцептить, прозрачно проксировать
  
  API balance (/api/v1/wallet/balance):
    Strategy: NetworkOnly (всегда свежий)
  
  API history (/api/v1/bets/history):
    Strategy: NetworkFirst, fallback Cache
    TTL: 1 час
    Show "Updated 5 min ago" если из кэша

OFFLINE PAGE:
  /offline.html — pre-cached
  Содержание:
    📡 Нет соединения
    "Данные могут быть неактуальны.
     Ставки и игры недоступны без интернета."
    [Обновить] [История ставок (offline)] [Профиль]

PUSH NOTIFICATIONS:
  Web Push API + Notifications API.
  Workflow:
    1. User opt-in (см. Part 3 §16) → user clicks "Allow"
    2. PushManager.subscribe(applicationServerKey)
    3. Send subscription к backend → store
    4. Backend sends push → SW receives 'push' event → show notification
  
  iOS Safari (≥ 16.4):
    ТОЛЬКО в PWA mode (display: standalone после "Add to Home Screen").
    Не работает в обычном браузере.
    Соответственно: prompt to "Add to Home Screen" перед push opt-in для iOS.

BACKGROUND SYNC:
  Use case: пользователь делает bet, теряет соединение → bet ставится в queue.
  При восстановлении сети → SW отправляет накопленный queue.
  
  ⚠️ ОПАСНО для ставок: cohort может проигнорировать → дублёж.
  РЕКОМЕНДАЦИЯ: Background Sync ТОЛЬКО для:
    - Submitting feedback / chat messages
    - Sending analytics events
    - Не для bet placement (всегда synchronous request)

PWA MANIFEST:
  apps/web/public/manifest.webmanifest:
    name: "OPUS Casino"
    short_name: "OPUS"
    display: "standalone"
    theme_color: "#0A0E1A"
    background_color: "#0A0E1A"
    icons: 192x192, 512x512, maskable
    start_url: /
    scope: /
    orientation: any (don't lock)
    categories: ["games", "entertainment"]

INSTALL PROMPT (deferred):
  ❌ НЕ показывать сразу при первом visit
  ✅ Показать после 2-3 сессий или после первого депозита
  ✅ Custom UI (banner) перед browser prompt:
    "📱 Установите OPUS на главный экран для быстрого доступа"
    [Установить] [Позже]
```

---

## 42. TOAST / NOTIFICATION SYSTEM

```
STACK BEHAVIOR:
  Max 3 одновременно visible (выше — queue).
  Position desktop:  top-right (с offset 16px от края + от header)
  Position mobile:   bottom-center (выше mobile-nav на 16px)
  
  Stack order: новые сверху (push), старые fade out снизу.
  Animation: slide-in 250ms cubic-bezier(0.4,0,0.2,1)

SEVERITY LEVELS:
  info     — синий left border, иконка ℹ️
  success  — зелёный left border, иконка ✓
  warning  — жёлтый left border, иконка ⚠️
  error    — красный left border, иконка ✕
  critical — красный fill background, modal-like (НЕ toast, отдельный компонент)

AUTO-DISMISS TIMERS:
  info     — 4s
  success  — 3s
  warning  — 6s
  error    — manual close (user должен явно закрыть)
  critical — never auto-dismiss

ACTIONS в toast:
  Опционально 1-2 кнопки:
    error toast "Ставка отклонена" → [Изменить кэф] [Отменить]
    success toast "Депозит зачислен" → [Открыть кошелёк]

DISMISS:
  Click ✕ кнопка
  Click outside (только для info, не для error)
  Esc на focused toast (если accessible)
  Auto-timer (per severity)

ACCESSIBILITY:
  role="status" для info/success
  role="alert" для warning/error/critical
  aria-live="polite" / "assertive"
  Focus management: error toast → optional auto-focus (для CTA)

DUPLICATE PREVENTION:
  Если показывается toast с тем же {message + severity} → ignore (или update timestamp).
  Иначе при многих API errors: 5x "Network error" одновременно.

QUEUE / PRIORITY:
  Если queue > 5 и приходит warning/error → выкинуть самый старый info.
  Critical приоритет: показывается мгновенно, может вытеснить info.

PERSISTED TOASTS (cross-tab):
  BroadcastChannel "toasts" → новый toast на одной вкладке = appears на всех.
  Use case: depositSuccess в одной вкладке → user видит во всех.

FINANCIAL TOASTS:
  ВСЕГДА продублировать в inline UI (компонент):
    Bet success → toast + "Bet receipt" в купоне
    Deposit success → toast + entry в Wallet history
    Withdrawal request → toast + entry в transactions list
  Toast без inline = риск что user не увидел (свайпнул).

КОМПОНЕНТ:
  Использовать sonner / react-hot-toast / radix-ui-toast — НЕ писать с нуля.
  Wrapper: <Toaster> в root layout.
  Usage: import { toast } from "@/lib/toast"; toast.error("Bet rejected", { ... });
```

---

## 43. ANALYTICS EVENTS CATALOG

```
СТЕК (выбор архитектора):
  Amplitude (cohort analysis, funnel) — рекомендуется для casino
  PostHog (self-hosted, GDPR-friendly) — альтернатива
  GA4 (если требуется Google ads attribution)
  Mixpanel — премиум альтернатива

NAMING: domain.action_object format (lowercase, snake_case after dot)

EVENTS CATALOG (минимум v1):

  AUTH:
    auth.signup_started
    auth.signup_step_completed { step: 1|2|3|4 }
    auth.signup_completed { time_seconds, currency, country }
    auth.signup_abandoned { last_step }
    auth.login_attempted { method: "password"|"google"|"apple" }
    auth.login_succeeded
    auth.login_failed { reason }
    auth.2fa_enrolled { method }
    auth.password_reset_requested
  
  WALLET:
    wallet.deposit_initiated { method, amount_bucket, currency }
    wallet.deposit_completed { method, amount_bucket, currency, time_to_complete }
    wallet.deposit_failed { method, reason }
    wallet.withdrawal_requested { method, amount_bucket, currency }
    wallet.withdrawal_approved
    wallet.withdrawal_rejected { reason }
  
  KYC:
    kyc.started { tier: 1|2|3 }
    kyc.documents_uploaded { types: ["passport", "selfie"] }
    kyc.completed { tier, time_seconds }
    kyc.rejected { reason, tier }
  
  CASINO:
    casino.lobby_viewed { category, filters }
    casino.game_card_clicked { game_id, position }
    casino.game_launched { game_id, mode: "real"|"demo" }
    casino.game_ended { game_id, session_duration, total_bet_bucket, net_pl_bucket }
  
  SPORTSBOOK:
    sportsbook.event_viewed { sport, event_id }
    sportsbook.odds_clicked { event_id, market, outcome }
    sportsbook.betslip_added { event_id, odds, stake_bucket }
    sportsbook.bet_placed { type: "single"|"express"|"system", stake_bucket, total_odds, currency }
    sportsbook.cashout_requested { bet_id, current_value_bucket }
  
  CRASH:
    crash.round_joined { game, bet_amount_bucket, auto_cashout_multiplier }
    crash.cashout { game, multiplier, payout_bucket }
    crash.crashed { game, lost_amount_bucket }
  
  LIVE_CASINO:
    live_casino.table_entered { table_id, provider }
    live_casino.bet_placed { table_id, amount_bucket, side_bets }
    live_casino.left_table { session_duration, total_bets_bucket, net_pl_bucket }
  
  BONUS:
    bonus.activated { bonus_id, type }
    bonus.completed { bonus_id, time_to_complete_days }
    bonus.expired { bonus_id }
    bonus.forfeited { bonus_id, remaining_amount_bucket }
  
  RG (Responsible Gambling):
    rg.limit_set { type, amount_bucket, period }
    rg.limit_increased_pending { type }
    rg.session_timeout_triggered { duration_minutes }
    rg.reality_check_shown
    rg.self_excluded { duration }
    rg.panic_button_clicked
  
  ERROR:
    error.api_failed { endpoint, status_code, retry_count }
    error.websocket_disconnect { reason, attempt_count }
    error.iframe_failed { provider, game_id }

USER PROPERTIES:
  user_id, country, currency, vip_tier, kyc_status, created_at,
  total_deposits, total_withdrawals, lifetime_bets, marketing_consent,
  language, theme (dark — у нас всегда), device_type

PRIVACY:
  GDPR: запрещено tracking без consent → cookie banner (см. §46)
  Anonymous tracking до consent → assignable user_id после login
  Right to erasure: POST /api/v1/user/erasure-request → wipe/anonymize в Amplitude после AML retention checks
  Money buckets: 0-10, 10-50, 50-100, 100-500, 500-1000, 1000+
  
  ⚠️ НИКОГДА не tracking:
    - PII (email, phone, address) — только user_id
    - Exact money amounts (особенно > $1000 и VIP); только amount_bucket
    - Specific game outcomes
    - 2FA codes / passwords / tokens
```

---

## 44. A/B TESTING / FEATURE FLAGS

```
СТЕК (выбор):
  LaunchDarkly (premium, fastest)
  Unleash (open-source, self-hosted) — рекомендуется
  ConfigCat (cheap)
  Self-hosted JSON через CDN (минимально)

FLAG TYPES:
  Boolean: feature_X_enabled
  Variant: hero_variant ("A" | "B" | "C")
  Number: bonus_match_percent (50, 100, 200)
  JSON: payment_methods_order (массив)

USAGE:

  // 1. Server-side (RSC, Server Actions):
  import { getFlag } from "@/lib/flags/server";
  const heroVariant = await getFlag("hero_variant", { userId, country });
  
  // 2. Client-side:
  import { useFlag } from "@/lib/flags/client";
  const showCrash = useFlag("crash_games_enabled");

EVALUATION CONTEXT:
  Per user: { userId, vipTier, country, registrationAge, isFirstDeposit }
  Per session: { utmSource, deviceType, referrer }

A/B TEST SETUP:
  1. Define flag в admin panel
  2. Set variants (A=control, B=treatment)
  3. Set targeting (e.g. only RU users, only new signups)
  4. Set traffic allocation (50/50, или 90/10 ramp-up)
  5. Set success metric (e.g. signup_completed_within_24h)

EVENT TRACKING:
  При показе вариант: experiment.variant_shown { experiment_id, variant }
  При success metric: связано с user_id → flag service вычисляет конверсию.

CACHING:
  Flag evaluation на client: cache в memory (LRU), TTL 5 минут.
  Server: cache в Redis (TTL 5 минут, invalidation через webhook).
  При offline: использовать last cached value, не блокировать UI.

GUARDRAILS:
  Никогда НЕ A/B тестировать:
    - Responsible Gambling UI (legally fixed)
    - KYC steps (regulator-defined)
    - Currency formatting (privacy/clarity)
    - Auth security (2FA, password strength) — может weaken security

ROLLBACK:
  Один клик в admin → flag = false → instant rollback (без deploy).

KILL SWITCH:
  emergency_kill_switch flag → всему product показывает maintenance page.
  Latency overhead: < 50ms per page load.
```

---

## 45. SEO

```
URL STRUCTURE (SEO-friendly):
  /                                ← Home
  /casino                          ← Casino lobby
  /casino/game/{slug}              ← Game page (e.g. /casino/game/sweet-bonanza)
  /casino/category/{slug}          ← Category (e.g. /casino/category/slots)
  /casino/provider/{slug}          ← Provider (e.g. /casino/provider/pragmatic)
  /sportsbook                      ← Sports lobby
  /sportsbook/{sport}              ← Sport (e.g. /sportsbook/football)
  /sportsbook/{sport}/{league}     ← League
  /sportsbook/event/{id}           ← Event (slug можно хранить в metadata, но URL canonical без второго варианта)
  /promotions/{slug}               ← Promo page
  /blog/{slug}                     ← Articles (если есть blog)

METADATA (Next.js generateMetadata):
  Per-page:
    title: "Sweet Bonanza слот — играть онлайн | OPUS Casino"
    description: "RTP 96.51%, max win x21,100. Играйте Sweet Bonanza от Pragmatic Play."
    canonical: https://opus.casino/casino/game/sweet-bonanza
    og:title, og:description, og:image (game thumbnail 1200x630)
    twitter:card = "summary_large_image"

SCHEMA.ORG (JSON-LD):
  Для game page:
  {
    "@context": "https://schema.org",
    "@type": "VideoGame",
    "name": "Sweet Bonanza",
    "publisher": "Pragmatic Play",
    "genre": "Slot",
    "aggregateRating": {
      "@type": "AggregateRating",
      "ratingValue": "4.5",
      "reviewCount": "1234"
    }
  }
  
  Для event page (sportsbook):
  {
    "@context": "https://schema.org",
    "@type": "SportsEvent",
    "name": "Arsenal vs Chelsea",
    "startDate": "2025-04-27T20:00:00Z",
    "location": { ... }
  }

SITEMAP.XML:
  Generation: app/sitemap.ts (Next.js 14 native)
  Включить:
    - Все casino games (5000+) с lastmod
    - Все promo pages
    - Все blog articles
    - Категории и провайдеры
  
  ИСКЛЮЧИТЬ:
    - /profile, /wallet, /admin (private)
    - /api/* (internal)
    - /login, /register (low SEO value)

ROBOTS.TXT:
  User-agent: *
  Allow: /
  Disallow: /api/
  Disallow: /admin/
  Disallow: /profile/
  Disallow: /wallet/
  Disallow: /betslip/
  
  Sitemap: https://opus.casino/sitemap.xml

HREFLANG (multi-language):
  <link rel="alternate" hreflang="ru" href="https://opus.casino/" />
  <link rel="alternate" hreflang="en" href="https://en.opus.casino/" />
  <link rel="alternate" hreflang="x-default" href="https://opus.casino/" />

GAMBLING-SPECIFIC SEO RESTRICTIONS:
  UK: "free play", "no deposit" — только если есть real disclaimer + 18+ icon
  Реклама в SERP с misleading заголовками = ASA штрафы
  Не ranking по queries про детей/уязвимые группы
  Robots для крупных indexes:
    - DE / NL: дополнительные restrictions per region
    - Russia: ranking ОК, но нужен российский license

PERFORMANCE FOR SEO (Core Web Vitals):
  LCP < 2.5s (Google ranking factor)
  INP < 200ms
  CLS < 0.1
  Сайтовая скорость напрямую влияет на ranking в гемблинге (high competition).

IMAGE SEO:
  alt-text ОБЯЗАТЕЛЬНО для всех game thumbnails
    "Sweet Bonanza slot game by Pragmatic Play"
  Lazy load с loading="lazy"
  Next.js Image component для auto-sizing + WebP
```

---

## 46. COMPLIANCE UI (GDPR, ePrivacy, Cookie Banner)

```
COOKIE BANNER:
  Показать ВСЕМ новым посетителям (без cookie 'consent_v1').
  Position: bottom (full width на mobile, fixed bottom-left card desktop).
  Z-index: 70 (см. иерархию part1 §1.7).
  
  UI:
  ┌─────────────────────────────────────────────────────────┐
  │  🍪 Использование cookies                                │
  │                                                          │
  │  Мы используем необходимые cookies для работы сайта     │
  │  и опциональные для аналитики и персонализации.          │
  │                                                          │
  │  [Принять все]  [Только необходимые]  [Настроить]       │
  │                                                          │
  │  → Подробнее в Политике конфиденциальности              │
  └─────────────────────────────────────────────────────────┘

  При "Настроить":
    ┌────────────────────────────────────────┐
    │  Настройки cookies                      │
    │                                          │
    │  ☑ Необходимые (обязательно)             │
    │     Auth, корзина, базовый функционал   │
    │  ☐ Аналитика                             │
    │     Amplitude, GA4 — улучшение UX        │
    │  ☐ Маркетинг                             │
    │     Реклама, ремаркетинг                 │
    │  ☐ Персонализация                        │
    │     Рекомендации игр на основе истории   │
    │                                          │
    │  [Сохранить] [Отмена]                    │
    └────────────────────────────────────────┘

CONSENT STORAGE:
  Cookie 'consent_v1' (1 год TTL):
    JSON: { necessary: true, analytics: bool, marketing: bool, personalization: bool, ts: timestamp }
  Server-side: also сохранить в users.consents (audit trail).

CONDITIONAL TRACKING:
  Aналитика загружается ТОЛЬКО если consent.analytics = true.
  Маркетинг pixels (Facebook, Google Ads) — ТОЛЬКО если marketing = true.
  Personalized recommendations — ТОЛЬКО если personalization = true.

WITHDRAW CONSENT:
  Footer: "Cookie настройки" → re-open banner (без необходимости).
  Profile → Privacy → "Cookie & tracking" — те же controls.

GDPR DATA EXPORT:
  Profile → Privacy → "Скачать мои данные" → request POST /api/v1/user/data-export
  Backend: async generation, email с download link через 24-48 часов.
  Format: JSON + CSV для transactions, bets, sessions.

GDPR RIGHT TO ERASURE:
  Profile → Privacy → "Удалить аккаунт" →
    Step 1: warning ("Эти данные будут удалены: ...")
    Step 2: typed confirmation "УДАЛИТЬ"
    Step 3: 30-day grace period (можно отменить)
    Step 4: после 30 дней — anonymization (НЕ полное удаление по AML 5-10 лет retention)

LEGAL UI ELEMENTS (footer обязательны):
  ✓ Лицензия (текст + номер + ссылка на reg)
  ✓ Логотип регулятора (UKGC / MGA / Curaçao)
  ✓ "18+" badge
  ✓ "Ответственная игра" link
  ✓ "BeGambleAware.org" / "GamCare" / "Гамблинг Анонимус" — per region
  ✓ "Cookie настройки" link
  ✓ Privacy Policy link
  ✓ Terms link
  ✓ Контакты support

PER-JURISDICTION CONFIG:
  /api/v1/geo/legal-config?country=DE
  Response:
    {
      "license_logo": "...",
      "license_text": "...",
      "helpline_phone": "+49 800 ...",
      "helpline_org": "Spielen mit Verantwortung",
      "min_age": 18,
      "responsible_gambling_required": true,
      "panic_button_required": true,
      "reality_check_minutes": 60,
      "credit_cards_allowed": false,  // UK
      "self_exclusion_database": "GAMSTOP"  // UK
    }
  Frontend применяет эти правила conditionally.
```

---

## ✅ CHECKLIST для Integrations & Infra

```
URL / ENV:
□ NEXT_PUBLIC_API_URL, _WS_URL — определены и используются ВСЕ места
□ Server-only env (BEZ NEXT_PUBLIC_) — НЕ leakage в bundle
□ NGINX routing для всех /api/v1/* → правильные сервисы (CONVENTIONS ports)
□ wss:// для production (НЕ ws://)
□ CSP headers (next.config.js) включают все iframe-провайдеры

PROTO:
□ Все API types из gen/typescript/ (НЕ вручную)
□ Connect-RPC client настроен с base URL
□ Pre-commit hook: buf generate
□ CI: buf breaking detection

SERVICE WORKER / PWA:
□ SW регистрируется после load (НЕ во время первого render)
□ Cache strategy per route pattern (NetworkOnly для odds!)
□ Push notifications: opt-in после 2-3 сессий
□ iOS: prompt "Add to Home Screen" перед Push (требование iOS 16.4+)
□ Background Sync ТОЛЬКО для feedback / analytics (НЕ для bets)
□ PWA manifest: dark theme color, no orientation lock

TOAST:
□ Max 3 одновременно, queue для остальных
□ Severity (info/success/warning/error/critical) с разными timers
□ Финансовые ошибки ТАКЖЕ показываются inline
□ aria-live="assertive" для error
□ BroadcastChannel для cross-tab toast sync

ANALYTICS:
□ События по naming convention domain.action_object
□ User properties без PII (только user_id)
□ НИКОГДА exact money amounts; использовать amount_bucket, особенно для VIP > $1K
□ НИКОГДА tokens / passwords / 2FA codes
□ Conditional loading через cookie consent

A/B / FEATURE FLAGS:
□ Flags evaluation context: user, session
□ Никогда A/B на RG / KYC / security
□ Kill switch flag для emergency
□ Experiment.variant_shown event tracking

SEO:
□ Per-page metadata (title, description, canonical, og)
□ Schema.org JSON-LD для games / events
□ sitemap.xml с lastmod
□ robots.txt с Disallow для private routes
□ hreflang для multi-language
□ Image alt-text для всех thumbnails

GDPR / Compliance:
□ Cookie banner до tracking
□ Consent granular (analytics / marketing / personalization)
□ Conditional pixel loading
□ Withdraw consent в footer + profile
□ Data export endpoint
□ Right to erasure с 30-day grace
□ Per-jurisdiction legal config
```
