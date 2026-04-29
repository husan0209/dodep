# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 5: Live Casino & Crash Games
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# Расположение: apps/web/src/app/(main)/casino/live/, apps/web/src/app/(main)/casino/crash/

---

## 33. LIVE CASINO — ARCHITECTURE

```
ПРОВАЙДЕРЫ (через iframe):
  Evolution Gaming    — №1 в индустрии, premium games
  Pragmatic Live      — большой выбор, native streams
  Ezugi               — региональные столы (turkish, hindi)
  Authentic Gaming    — land-based casino streams (Mr Vegas etc.)

INTEGRATION PATTERN:
  Аналогично slots — iframe с launch token.
  ОТЛИЧИЯ от обычных слотов:
    1. Stream latency CRITICAL (< 1s = идеал, > 2s = игроки уходят)
    2. Bet timer overlay (10-20s на ставку, синхронизировано с дилером)
    3. Side bets, insurance — отдельные UI элементы поверх стрима
    4. Chat с дилером (модерация на стороне провайдера)
    5. Tip dealer — микро-транзакции (0.50-50 единиц)
    6. Multi-table view (4 streams одновременно — premium feature)
    7. Bandwidth-adaptive quality (1080p / 720p / 480p / audio-only)

LATENCY BUDGET:
  Stream first frame:    < 2s после iframe load
  Stream → user:         < 1s (HLS latency)
  Bet placed → confirmed: < 500ms
  Total user action latency: < 1.5s
```

---

## 34. LIVE CASINO LOBBY (/casino/live)

```
LAYOUT (desktop):
  ┌─────────────────────────────────────────────────────────────┐
  │  Live Casino                                                  │
  │  ┌───────────────────────────────────────────────────────┐   │
  │  │ [Все] Roulette  Blackjack  Baccarat  Poker  Game shows│   │
  │  └───────────────────────────────────────────────────────┘   │
  │                                                               │
  │  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
  │  │ [LIVE STREAM]│[LIVE STREAM]│[LIVE STREAM]│[LIVE STREAM]│  │
  │  │ Lightning   │ Speed Black │ Crazy Time  │ Monopoly    │  │
  │  │ Roulette    │ jack        │             │ Live        │  │
  │  │ Min: 1₽     │ Min: 100₽   │ Min: 10₽    │ Min: 10₽    │  │
  │  │ Players: 487│ Players: 23 │ Players:8.2K│ Players: 3.2K│  │
  │  │ Dealer: Anna│ Dealer: Max │             │             │  │
  │  └─────────────┴─────────────┴─────────────┴─────────────┘  │
  └─────────────────────────────────────────────────────────────┘

КАРТОЧКА LIVE-СТОЛА:
  Mini-stream preview (autoplay muted, low quality 240p, ~50KB/s):
    NB: НЕ запускать на mobile если saveData=true (Network Information API)
    Pause при scroll out of viewport (IntersectionObserver)
  Текстовый overlay:
    Название игры + провайдер
    Min/Max ставка (per стол, разное)
    Кол-во игроков online
    Имя дилера + флаг страны (Russian-speaking, English, etc.)
    Badge "🔴 LIVE" + "HD" если 1080p доступен
  Hover/Tap: scale + glow + кнопка "Войти за стол"

ФИЛЬТРЫ:
  Game type: Roulette / Blackjack / Baccarat / Poker / Game Shows / Sic Bo / Dragon Tiger
  Min stake: range slider (1₽ - 100,000₽)
  Language: dealer language (RU / EN / DE / TR / HI)
  Provider: Evolution / Pragmatic / Ezugi
  Sort: Most popular / Hot / New / High limit / Low limit

TABLE STATUS:
  GREEN dot:  Open, places available
  YELLOW dot: Almost full (>80% capacity)
  RED dot:    Full (queue) — кнопка "Встать в очередь"
  GREY:       Closed (вне расписания дилера)

PERSONALIZATION (если залогинен):
  "Вы недавно играли в":  Lightning Roulette → resume
  "Похожие на ваши любимые столы"
  "VIP столы" (если bet history > $X)
```

---

## 35. LIVE CASINO IN-GAME UX

```
ОТКРЫТИЕ СТОЛА:
  1. Click карточки → POST /api/v1/casino/live/tables/{table_id}/launch
  2. Response: { iframe_url, token, stream_url, audio_url }
  3. Loading skeleton (3-5 секунд):
     ┌─────────────────────────────────────────┐
     │  [LOGO LOADING ANIMATION]                │
     │  Подключение к столу Lightning Roulette  │
     │  Дилер: Anna                             │
     │  ████████░░░  Загрузка стрима...         │
     │  [Отменить]                              │
     └─────────────────────────────────────────┘
  4. Iframe готов → fade in stream

CONTROLS OVERLAY (на iframe, позиционируется поверх):
  TOP-LEFT:    [← Лобби]   ← возврат в /casino/live
  TOP-RIGHT:   Balance + Currency  /  Settings ⚙️  /  Fullscreen ⛶  /  Close ✕
  BOTTOM:     Bet timer (если ставки открыты) + Quick Stake buttons

  ⚠️ ВАЖНО: provider iframe управляет основным UI ставок.
  Наш overlay — только wrapper-functions (баланс, навигация, settings).
  Не overlay'ить provider's chip placement или bet table!

BET TIMER (provider контролирует, но мы можем дублировать):
  Шкала "10... 5... 4... 3... 2... 1... NO MORE BETS"
  Цвет:  green → yellow (≤5s) → red (≤2s)
  Звук: tick-tick-tick (опционально, default off)
  Vibration на mobile (если supported и user enabled)

SIDE BETS (visible в provider UI, но мы можем делать quick links):
  Lightning Roulette: Lightning numbers 50x-500x
  Blackjack: Insurance, Perfect Pairs, 21+3
  Baccarat: Tie, Pair, Big/Small

CHAT С ДИЛЕРОМ (provider feature, но влияем на UX):
  Кнопка [💬 Чат] открывает provider's chat panel.
  Modaration: provider responsibility, мы только показываем UI.
  Tip-dealer: button "💰 Чаевые" → quick amounts $1 / $5 / $10 / Custom.

MULTI-TABLE VIEW (premium feature):
  Доступно VIP / Tier 2+ игрокам
  /casino/live/multi → grid 2x2 streams (max 4)
  Каждый стол: уменьшенный, синхронизированные ставки
  Audio: mute all, unmute one (выбираешь активный)
  Performance: 4 HLS streams = ~3-5 Mbps + 4 iframe = тяжёлый профиль
    → требует bandwidth detection (Network Information API)
    → автодаунгрейд до 480p при < 5 Mbps

BANDWIDTH-ADAPTIVE QUALITY:
  Auto detect (Network Information API):
    effectiveType "4g" + downlink > 3 Mbps  → 1080p
    effectiveType "4g" + downlink 1-3 Mbps  → 720p
    effectiveType "3g" or saveData=true     → 480p
    Если provider не поддерживает adaptive → audio-only fallback
  Manual override: settings → Quality dropdown.

NETWORK DROP DURING ACTIVE BET:
  Connection lost mid-spin (между bet placed и result):
    → overlay "📡 Соединение прервано. Bet остаётся активным."
    → backend transparently продолжает (provider session)
    → reconnect: показать result с анимацией catch-up
    → balance update тоже catch up

TIMEOUT (idle player):
  20 минут без bet activity → modal "Вы всё ещё здесь?"
  → [Продолжить] / [Выйти]
  → 5 минут не отвечает → auto-leave (UKGC anti-fatigue)
  → Server-side: bet inactive, table seat freed

LEAVING TABLE:
  Click [← Лобби] или ✕:
    1. POST /api/v1/casino/live/tables/{table_id}/end
    2. Provider session ends (player leaves seat)
    3. Recent table показывается в "Continue playing" в /casino/live
    4. Если активная ставка → block выхода + warning

KEYBOARD SHORTCUTS (если provider не override):
  Esc:        выход в лобби
  F:          fullscreen toggle
  M:          mute audio
  Q:          settings panel
```

---

## 36. CRASH GAMES (/casino/crash или /casino/aviator)

```
ИГРЫ В КАТЕГОРИИ:
  Aviator (Spribe) — №1
  JetX (Smartsoft)
  Spaceman (Pragmatic)
  Mines (Spribe)
  Penalty Shoot Out (Evoplay)
  Plinko (Spribe / BGaming)

УНИКАЛЬНОСТЬ vs другие casino:
  ✓ Multiplayer (видишь ставки других игроков)
  ✓ Provably fair (commit-reveal cryptography)
  ✓ Round-based (новый раунд каждые 5-10 секунд)
  ✓ Auto-cashout (set цель × до начала раунда)
  ✓ 2 одновременные ставки (параллельные)
  ✓ Социальный элемент (chat, leaderboard)
```

---

## 37. CRASH GAME UX (/casino/aviator)

```
LAYOUT (desktop):
  ┌─────────────────────────────────────────────────────────────┐
  │  Aviator                                                      │
  │                                                               │
  │  ┌─────────────────────────────────┬─────────────────────┐ │
  │  │                                  │  All Bets   My Bets │ │
  │  │                                  │  Top                │ │
  │  │      ✈️                          │ ─────────────────── │ │
  │  │           ╱                      │  P1***  100₽ ×2.45  │ │
  │  │         ╱                        │  P2***  500₽ x1.20  │ │
  │  │       ╱                          │  ...                │ │
  │  │     ╱                            │                     │ │
  │  │  ╱                               │ History (last 10):  │ │
  │  │ ─────────────────                │ x1.45 x2.10 x1.02   │ │
  │  │ Current: x2.45                   │ x5.20 x1.00 x3.45   │ │
  │  │                                  │                     │ │
  │  ├──────────────────┬───────────────┴─────────────────────┤ │
  │  │ BET 1            │ BET 2                                │ │
  │  │ Stake: [100₽ ▼]  │ Stake: [50₽ ▼]                      │ │
  │  │ Auto: ☐ x2.00    │ Auto: ☑ x1.50                       │ │
  │  │ [PLACE BET]      │ [CASH OUT x1.43] (active!)           │ │
  │  └──────────────────┴───────────────────────────────────── │ │
  └─────────────────────────────────────────────────────────────┘

CANVAS / WEBGL CURVE:
  Используем Canvas 2D (легче) или WebGL (плавнее)
  Animation: requestAnimationFrame loop
  Curve: parametric (часто exponential или x^1.5)
  Multiplier display: top-left of canvas, JetBrains Mono Bold 48px
  Color gradient: green (low x) → yellow (high) → red (very high)
  Plane icon: летит вдоль кривой, при crash → explosion + fall
  
  Performance:
    60 FPS target on desktop, 30 FPS minimum on mobile
    Reduce animations for prefers-reduced-motion (статичная шкала)
    Pause render when tab inactive (visibilitychange)

ROUND STATES:
  WAITING:        "Round starts in 5... 4... 3..."
                  Bet inputs ENABLED
                  Stake quick-buttons активны
  
  IN_PROGRESS:    Plane flies, multiplier rises
                  Bet inputs DISABLED (можно только cash out)
                  Cashout button shows current multiplier
  
  CRASHED:        "FLEW AWAY at x2.45"
                  3-second pause for results
                  Inactive bets lose, active losses shown
                  History updates с новым результатом

BET INPUT (2 параллельные ставки):
  Stake: number input + quick buttons [50] [100] [500] [1000]
  Auto-cashout (опциональный):
    Toggle ☐ + multiplier input (например x2.00)
    Если в раунде multiplier ≥ x2.00 → автоматический cashout
  Auto-bet (опциональный, premium):
    Toggle ☐ + кол-во раундов
    Auto-stop conditions: "если выиграю N подряд" / "если проиграю N подряд"
  Place Bet button:
    DURING bet phase: "PLACE BET" → disable until next round
    DURING round (if your bet is active): "CASH OUT x1.43" → cash out now

PROVABLY FAIR (visible UI):
  Каждый раунд имеет:
    - Server seed hash (commit, опубликован ДО раунда)
    - Client seed (player can rotate)
    - Nonce (round number)
  
  После crash: server seed публикуется → можно verify:
    HMAC-SHA256(server_seed, client_seed + nonce) → result
  
  UI:
    Кнопка "Provably Fair" в меню → opens panel:
    ┌─────────────────────────────────────────┐
    │  Provably Fair                           │
    │                                          │
    │  Server seed (revealed):  abc123...      │
    │  Server seed hash:        def456...      │
    │  Client seed (yours):     [____] [Edit]  │
    │  Nonce:                   12453          │
    │                                          │
    │  Result: x2.45                           │
    │  Verify: [Открыть в внешнем калькуляторе]│
    │                                          │
    │  Что это? → /provably-fair-explained     │
    └─────────────────────────────────────────┘

CHAT (опционально, провайдер-зависимо):
  Right sidebar или bottom panel
  Message format: handle + message
  Anti-spam: 1 message / 5s, max 30/hour
  Profanity filter: server-side
  Mute user (per individual): локально
  Tip user via @handle (если поддерживается)

LEADERBOARD (per round / daily / weekly):
  Top 10 winners by multiplier or by amount
  Updates real-time (websocket)

NOTIFICATIONS:
  "Вы не закешаут до x10!" (если auto-cashout не сработал)
  "Поздравляем! +5,000₽" (после cashout)
  "Раунд начинается через 5 секунд" (если active player)

SOUND:
  Tick-tick-tick (rising multiplier) — default OFF, settings ON
  Crash sound (explosion) — default OFF
  Win sound (cashout) — default ON, volume 50%
  Mute toggle всегда видим

RESPONSIBLE GAMBLING (специфично для crash):
  Crash games — высокий risk of addiction (быстрые раунды).
  Auto-stop after N losses (player setting):
    "Стоп после 5 проигрышей подряд" → auto-disable bets
  Session limit: после 30 минут → Reality Check
  Loss limit: специфический для crash (более жёсткий)

ANTI-FRAUD UI:
  Если AI detects "автоматизированное поведение" (slip-stream attacks):
    → ban с показом конкретной причины + appeal flow
  CAPTCHA challenge перед каждой 100-й ставкой (если подозрительно)
```

---

## 38. CROSS-CUTTING (Live + Crash общее)

```
PROVIDER OUTAGE:
  Stream/Game не доступен > 30s:
    → toast "Игра временно недоступна"
    → "Попробуйте позже" + "Другие игры"
    → Refund активных bets автоматически
  
PROVIDER MAINTENANCE WINDOW:
  Pre-scheduled maintenance: показать banner за 24ч + email
  "Maintenance в 03:00-05:00 UTC. Не размещайте долгие auto-bets."

AUDIO POLICY (iOS Safari requirement):
  Любой sound должен быть triggered user gesture (click).
  Auto-play silent OK, но unmute = требует click.
  UI: при первом entrance → sound icon с "Tap to enable audio"

NETWORK DETECTION:
  Если Network Information API доступен:
    saveData=true → reduce stream quality, отключить animations
    effectiveType "slow-2g" → "Соединение слишком медленное" + retry
  
PWA STANDALONE MODE:
  Game в iframe внутри PWA (display: standalone):
    HARDWARE BACK button → handle через History API (НЕ закрывать app)
    Status bar color: dark (matches casino UI)
    Splash screen: brand logo + "Loading..."

ANALYTICS:
  live_casino.table_entered (table_id, provider, latency_ms)
  live_casino.bet_placed (table_id, amount, side_bets)
  live_casino.cashout (table_id, amount, multiplier)
  live_casino.left_table (session_duration, total_bets, net_pl)
  
  crash.round_joined (game, bet_amount, auto_cashout_multiplier)
  crash.cashout (game, multiplier, payout)
  crash.crashed (game, lost_amount)
```

---

## ✅ CHECKLIST для Live Casino & Crash

```
LIVE CASINO:
□ Iframe sandbox: allow-scripts, allow-forms; allow-same-origin только per-provider exception после security review
□ Stream latency < 1.5s end-to-end
□ Bet timer overlay синхронизирован с дилером
□ Tip dealer: quick amounts $1/$5/$10
□ Multi-table: только VIP, max 4, audio управляемый
□ Bandwidth-adaptive: auto degrade при slow connection
□ Mini-preview pause при scroll out (IntersectionObserver)
□ Network Information API: saveData → auto reduce quality
□ Audio: enable только после user gesture (iOS)
□ Idle timeout: 20 минут warning, 25 минут force leave

CRASH GAMES:
□ Canvas 60 FPS target, 30 FPS mobile
□ prefers-reduced-motion: статичная шкала
□ Provably fair UI: server seed hash visible ДО раунда
□ Auto-cashout: configurable per ставка
□ 2 параллельные ставки в одной игре
□ Round states: WAITING / IN_PROGRESS / CRASHED
□ Live "All bets" feed (multiplayer)
□ History: последние 10-20 раундов visible
□ Auto-stop after N losses (player setting)
□ Sound default OFF, settings to enable
□ Reality Check каждые 30 мин (более частый чем casino)

ОБЩЕЕ:
□ Provider outage → refund + notify
□ Maintenance window: banner за 24ч
□ Audio policy: user gesture for unmute
□ PWA standalone: hardware back через History API
□ Все analytics events ИМЯ.ДЕЙСТВИЕ format (см. Part 6 §44)
```
