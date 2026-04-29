# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 3: Edge Cases, Error States, Technical Requirements
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# CANONICAL ROUTES: см. docs/FRONTEND_SPEC.md

---

## 9. ERROR STATES (для КАЖДОГО компонента)

```
Каждый компонент имеет 5 состояний:
  1. Default (нормальное)
  2. Loading (skeleton, НЕ спиннер)
  3. Empty (нет данных — иконка + текст + CTA)
  4. Error (ошибка — иконка + причина + "Повторить")
  5. Success (подтверждение)

ПРИМЕРЫ:

Каталог игр — Empty:
  🔍 Ничего не найдено
  "Попробуйте изменить фильтры"
  [Сбросить фильтры] [Все игры]

Каталог игр — Error:
  ⚠️ Не удалось загрузить игры
  "Проверьте соединение"
  [Повторить]

Купон — Error (ставка отклонена):
  ❌ Ставка отклонена
  "Коэффициент изменился: 2.10 → 1.95"
  [Принять новый кэф] [Отменить]

Депозит — Error:
  ❌ Платёж не прошёл
  "• Недостаточно средств • Карта заблокирована • Превышен лимит"
  [Попробовать снова] [Другой метод] [Поддержка]

ЗАПРЕЩЕНО: "Something went wrong" без контекста.
В гемблинге на кону реальные деньги — каждая ошибка требует чёткого объяснения.
```

---

## 10. OFFLINE / DEGRADED STATE

```
Нет соединения:
  📡 Нет соединения
  "Данные могут быть неактуальны. Ставки и игры недоступны."
  [⟳ Обновить]
  "Баланс на момент последнего обновления: 12 530₽ (5 мин назад)"

Что работает offline (PWA/кэш):
  ✓ Просмотр истории ставок (кэш)
  ✓ Просмотр профиля
  ✓ Правила и Ответственная игра
  ✓ Последний известный баланс с timestamp

Что НЕ работает offline:
  ✗ Ставки, запуск игр, депозит/вывод, live-данные
```

---

## 11. WEBSOCKET RECONNECTION & DATA INTEGRITY

```
Стратегия (exponential backoff):
  Попытка 1: 1с  — silent (UI не меняется)
  Попытка 2: 2с  — overlay "Переподключение..." + все кэфы серые
  Попытка 3: 4с  — "Попытка 3..."
  Попытка 4: 8с
  Попытка 5: 15с
  Попытка 6+: 30с (каждые 30с до 5 минут, потом стоп)

Все попытки неудачны:
  ⚠️ Нет соединения
  [Обновить страницу]
  "💡 Коэффициенты могут быть неточными"

После восстановления:
  1. Все кэфы обновляются одновременно
  2. Вырос → зелёная вспышка, Упал → оранжевая, Закрыт → серый 🔒
  3. Toast "✅ Соединение восстановлено"
  4. Купон: подсветка изменённых кэфов жёлтым → подтверждение

DATA INTEGRITY:
  Каждый odds update содержит: sequence, event_id, produced_at
  Sequence gap detected → REST resync GET /api/v1/events/{id}/odds
  Out-of-order: discard если produced_at < current_version
  Duplicate messages: idempotent update (проверка sequence)
  Auth token expired during WS → refresh token + reconnect
  Backpressure: если >50 updates/sec → batch render каждые 200ms
  Stale timestamp (produced_at > 30s ago) → пометить кэфы как stale
```

---

## 12. IFRAME (Casino Games)

```
GAME LAUNCH TOKEN (SECURITY-CRITICAL):
  Token одноразовый, TTL 30-60 секунд
  Генерируется ТОЛЬКО после preflight checks:
    auth, geo, KYC, RG guard, game enabled, provider available
  Endpoint: POST /api/v1/casino/games/{slug}/launch
  НИКОГДА не хранить в localStorage/sessionStorage
  Если token истёк до загрузки iframe:
    → показать "Сессия запуска истекла" + [Запустить заново]
  Если preflight failed:
    → actionable error ("KYC не пройден", "Игра недоступна в вашем регионе", etc.)

BALANCE RACE CONDITION:
  Источник истины: wallet service (хедер платформы)
  Поток: пользователь спинит → провайдер → wallet → WS → хедер
  Задержка 100-500ms: countup анимация скрывает разницу
  Если расхождение >5 сек → принудительный REST запрос баланса

MOBILE ORIENTATION:
  НЕ проверять: window.orientation (устарело)
  Проверять: viewportWidth / viewportHeight > 1.0
  Portrait → overlay "🔄 Поверните телефон" + [Играть в портрет]
  Landscape / iPad → iframe на весь экран

BACK BUTTON:
  При открытии игры: pushState({game: "slug"})
  Hardware back / popstate → закрыть iframe, вернуться в лобби
  Кнопка "← Лобби" поверх iframe

FROZEN/CRASHED IFRAME:
  Если iframe не отвечает >10 сек → overlay поверх iframe (role="dialog", focus trap)
  [Перезагрузить игру] [Вернуться в лобби]

ПОТЕРЯ СОЕДИНЕНИЯ ВО ВРЕМЯ ИГРЫ:
  Обычный спин: провайдер восстанавливает сессию, баланс обновить
  Бонусный раунд: toast "Сессия восстановлена. Бонусный раунд продолжается"
  Провайдер упал: "Игра временно недоступна. Баланс не затронут" + предложить другую

ДОПОЛНИТЕЛЬНЫЕ EDGE CASES:
  Provider postMessage не получен: timeout 10с → retry → overlay
  Iframe blocked by browser/CSP: показать ошибку + "Отключите блокировщик"
  Game disabled after deep link: "Игра временно недоступна" + альтернативы
  Game country-blocked after catalog loaded: "Недоступна в вашем регионе"
  Provider maintenance: banner на карточке игры + disabled
  Round settlement delayed: "⏳ Результат обрабатывается"
  Bonus round recovery after device switch: провайдер восстанавливает
  Full-screen exit on iOS Safari: кнопка "↩ Вернуться" в safe area
  Audio autoplay: permission prompt, muted by default
```

---

## 13. MULTI-SESSION / MULTI-TAB

```
Мульти-вкладки:
  Купон: BroadcastChannel API (fallback: StorageEvent)
  Баланс: WebSocket пушит во все вкладки

BETSLIP PERSISTENCE (безопасность):
  НЕ хранить BetSlip в plain localStorage — XSS из iframe-провайдера прочитает.
  Варианты (в порядке предпочтения):
    1. SERVER-SIDE BetSlip (source of truth) через GET /betslip/current — самый надёжный
       → sessionStorage только как UI cache (без confidential data)
    2. sessionStorage (исчезает при закрытии tab) — ок для UX
    3. localStorage С ШИФРОВАНИЕМ (WebCrypto AES-GCM):
       Инициализация: key в IndexedDB (CryptoKey, non-extractable)
       Быстро: serialize betslip → encrypt → base64 → localStorage
       NB: защищает от "другой юзер на общем ПК", но НЕ от XSS в той же origin
  РЕКОМЕНДОВАНО: server-side как source of truth, sessionStorage как UX cache.
  BroadcastChannel unavailable: fallback на StorageEvent

Мульти-устройство:
  Баланс: WS синхронизация
  Одновременные депозиты: lock на уровне wallet service
  Одновременно 2 слота: провайдер разрешает только 1 сессию → первый завершается
  Logout на одном: сессии независимы. Смена пароля → инвалидация ВСЕХ сессий

PRIVACY / SHARED DEVICE:
  User A logs out, User B logs in same browser:
    → очистить localStorage, sessionStorage, IndexedDB
    → очистить BetSlip, favorites, preferences
  Offline cache содержит profile/balance:
    → НЕ кэшировать PII (имя, email, документы)
    → Кэш баланса: только с timestamp, без истории
  Incognito/private browsing:
    → no persistent storage, корректная работа без localStorage
```

---

## 14. TIMEZONE HANDLING

```
Хранение: UTC (ISO 8601) в базе и API
Отображение: клиент конвертирует в локальное время

Определение:
  1. Intl.DateTimeFormat().resolvedOptions()
  2. Fallback: UTC
  3. Override: настройки профиля (ручной выбор)

Форматы:
  Предстоящий матч: "Сегодня в 21:00" или "Завтра в 03:15 (GMT+5)"
  Live: "67'" (игровое время)
  История: "14 янв 2025, 21:45 (GMT+3)"

DST: браузер учитывает автоматически
Миссии reset: 00:00 UTC → показывать таймер в локальном времени
```

---

## 15. GEO-RESTRICTIONS

```
Пользователь в заблокированной юрисдикции:
  🌍 Сервис недоступен в вашем текущем регионе
  [Связаться с поддержкой]
  ⚠️ Использование VPN запрещено и ведёт к блокировке

Проверка IP: при каждом запуске игры (не только логин)
Mid-session VPN: предупреждение, не мгновенный выход
Уровни: полная блокировка / только спорт / только определённые провайдеры
```

---

## 16. PUSH-УВЕДОМЛЕНИЯ

```
КОГДА ЗАПРАШИВАТЬ:
  ❌ Сразу при первом визите (90% отклонят)
  ✅ После первого депозита или 3-й сессии
  ✅ При подписке на событие

КАК: сначала свой UI, потом браузерный Notification.requestPermission()

ТИПЫ:
  Результат ставки: "✅ Выиграли +5 230₽" → deep link /bets
  Начало матча: "⚽ Arsenal–Chelsea через 15м" → /sportsbook/event/{id}
  Бонус: "🎁 20 фриспинов" → /bonuses
  Промо: "🔥 Турнир 1M₽" → /promotions/{slug}
  Транзакция: "💰 Вывод одобрен" → /wallet

ЛИМИТЫ: ≤3 push/день, промо ≤1/день, транзакционные — без лимита
Настройки категорий: в профиле
```

---

## 17. LOGOUT BEHAVIOR

```
Casino iframe открыт:
  1. iframe закрывается
  2. POST /api/v1/casino/games/{slug}/end или POST /api/v1/casino/live/tables/{id}/end
  3. При следующем входе (<24ч): toast "Вы играли в X" + [Продолжить]

Купон:
  1. Anonymous: sessionStorage cache TTL=2ч, без plain localStorage
  2. Authenticated: server-side BetSlip source of truth + sessionStorage cache
  3. При логине: merge только после проверки актуальности исходов через quote API
  4. Другой пользователь → очистка sessionStorage + server-side reload

Незавершённый депозит:
  → транзакция завершается на стороне PSP
  → при следующем входе: toast "Депозит $500 обработан" или "не удался"
  → pending >30мин → статус PENDING + push

SOFT LOGOUT (закрыл таб): timeout 30мин, при возврате → авто-вход если жива
HARD LOGOUT (кнопка "Выйти"): очистка, invalidation на сервере
```

---

## 18. ЛОКАЛИЗАЦИЯ

```
RTL (арабский, иврит):
  Направление текста, порядок колонок, позиции иконок, свайпы
  Числа: остаются LTR даже в RTL!
  Конфиг: западные (1,2,3) vs восточно-арабские (١,٢,٣)

ДЛИНА ТЕКСТА:
  "Deposit" (7) vs "Tallettaminen" (13)
  Решение: min-width по длинному языку, или text-overflow + tooltip, или clamp()
  Тестирование UI на всех языках — обязательно

КУЛЬТУРНЫЕ РАЗЛИЧИЯ:
  Европа: футбол, зелёный=удача, DD.MM.YYYY, 1.000 или 1 000
  Азия: крикет/e-sports, красный/золотой=удача, YYYY/MM/DD, 1,000
  Латинская Америка: футбол/бейсбол, DD/MM/YYYY, 1.000
```

---

## 19. ЮРИДИЧЕСКИЕ UI-ТРЕБОВАНИЯ

```
UKGC (Великобритания):
  ✓ Верификация до первого депозита
  ✓ RTP доступен
  ✓ "Ответственная игра" на каждой странице
  ✓ Запрет auto-play (или жёсткие ограничения)
  ✓ Net Deposit Tracker обязателен
  ✓ Запрет кредитных карт
  ✓ Time-out и self-exclusion легкодоступны

MGA (Мальта):
  ✓ Panic button (мгновенный тайм-аут)
  ✓ Reality Check каждые 60 мин (настраивается)
  ✓ Баланс всегда видим
  ✓ Разделение реальных/бонусных средств

Кюрасао:
  ✓ Лого лицензиата в footer
  ✓ Ссылка на правила

РЕАЛИЗАЦИЯ: feature flags per jurisdiction + geo-config
```

---

## 20. SITEMAP

```
См. CANONICAL ROUTES в docs/FRONTEND_SPEC.md — единственный источник истины.
ВСЕ маршруты определяются ТОЛЬКО в FRONTEND_SPEC.md.
Этот файл НЕ дублирует их.
```

---

## 21. REPEAT BET (Повторная ставка)

```
Все исходы доступны:
  → добавить в купон, подсветить изменённые кэфы, сумму НЕ копировать

Часть недоступна:
  → модалка: ✅ доступные + ❌ недоступные
  → [Добавить доступные] [Найти похожие] [Отменить]

Все недоступны:
  → toast + "Похожие события сегодня:" + 3 карточки

Кэф изменился >10%:
  → отдельное предупреждение: "2.10 → 1.80" [Принять] [Отказаться]
```

---

## 22. МУЛЬТИВАЛЮТНОСТЬ

```
Base currency: EUR (внутренний расчёт)
Display currency: то что видит пользователь (выбирается при регистрации)
Нельзя менять после регистрации (обращение в поддержку)

Курсы фиат: каждые 15 мин (Open Exchange Rates / Fixer.io)
Курсы крипто: каждые 10-15 сек через WS
Фиксация: в момент создания транзакции

В казино: курс НЕ показывается (путаница)
При депозите/выводе: "Вы вносите $100 (≈ €92.00 по курсу 1.087)"
Крипта: 8 знаков после запятой (0.00012345 BTC)
```

---

## 23. AFFILIATE TRACKING

```
URL: https://platform.com/?ref=PARTNER123&utm_source=telegram

ХРАНЕНИЕ (иерархия, в порядке надёжности):
  1. Server-side first-party cookie (TTL 30/90 дней) — ЧЕРЕЗ EDGE/CDN (Set-Cookie response header)
     ОБЯЗАТЕЛЬНО для iOS Safari — ITP режет client-side cookies до 7 дней
     Set-Cookie: aff_ref=PARTNER123; Domain=.platform.com; HttpOnly; Secure; SameSite=Lax; Max-Age=2592000
  2. Backup: client-side cookie (для браузеров без ITP)
  3. Backup: localStorage (для WebView и PWA, обнуляется iOS через 7 дней без interaction)

⚠️ iOS Safari ITP REALITY:
  Реальный retention без server-side cookie: ~7 дней (не 30!)
  Решение: edge function (Vercel/Cloudflare Worker) ставит first-party cookie при первом визите.
  Apple ATT (на iOS приложении через WebView): нужен permission prompt.

При регистрации: привязка ref_code к аккаунту
Приоритет: first-touch или last-touch (настраиваемо в admin)
Self-referral detection + fraud (много кликов с 1 IP, подозрительные user-agents)
UX: промокод предзаполнен из URL, процесс регистрации идентичен
ПОСЛЕ регистрации: очистить aff_ref cookie (уже привязан к аккаунту)
```

---

## 24. DEEP LINKING

```
Schema: только CANONICAL ROUTES из `docs/FRONTEND_SPEC.md`.

Push → Deep Link:
  "⚽ ГОЛ! Arsenal 2:1" → /sportsbook/event/67890
  Service Worker → clients.openWindow(url)

Promo → Deep Link:
  "50 фриспинов на Sweet Bonanza!" → /casino/game/sweet-bonanza?bonus=freespins50
  Залогинен → игра + бонус
  Не залогинен → регистрация → redirect на игру

Affiliate → Deep Link:
  /?ref=PARTNER123&landing=welcome → кастомная главная
```

---

## 25. AUTH TOKEN STRATEGY (CONVENTIONS: JWT Ed25519 TTL 15 мин)

```
ЦЕЛЬ: совместить JWT TTL=15min (CONVENTIONS) с casino-сессией 30+ минут без разрывов.

XSS риск в казино высок (iframe провайдеры → если CSP пробит → кража баланса).
ПОЭТОМУ храним токены так:

ХРАНЕНИЕ ТОКЕНОВ:
  Access token (15 мин, JWT Ed25519):
    → IN-MEMORY (Zustand store, БЕЗ persist)
    → НИКОГДА localStorage / sessionStorage / cookie
    → Инжектится в Authorization header каждого API request
  Refresh token (долгий TTL, обычно 7-30 дней):
    → httpOnly + Secure + SameSite=Strict cookie
    → НЕДОСТУПЕН из JS (даже при XSS)
    → path=/auth/refresh (только этот endpoint видит cookie)
  CSRF token (для mutating endpoints):
    → <meta name="csrf-token"> при SSR → axios вычитывает
    → Шлётся в X-CSRF-Token header
    → Backend сверяет с cookie session

SILENT REFRESH FLOW:
  1. При login: получаем access_token + refresh в cookie
  2. Таймер в ApiClient: за 60-120с до expiry → POST /auth/refresh
  3. Новый access_token заменяется в store без прерывания UX
  4. Рефреш не удался (refresh token истёк) → logout флоу

401 RESPONSE HANDLING (race condition with refresh):
  Сценарий: 5 API requests в parallel → все получают 401
  Решение: SINGLE-FLIGHT refresh:
    — Первый 401 запускает refreshPromise
    — Остальные ждут тот же promise (НЕ дублируют)
    — После success — retry все 5 реквестов с новым токеном
    — После fail — logout один раз

IDEMPOTENCY (критично для финансовых):
  ВСЕ mutating requests (bet, deposit, withdrawal) ОБЯЗАНЫ иметь:
    Idempotency-Key: <uuid v4 генерируется клиентом>
  При retry после 401: TOT ЖЕ Idempotency-Key (иначе дубль ставки!)
  Backend возвращает тот же результат (idempotent semantics).
  TTL idempotency cache: 24ч на backend.

WEBSOCKET RE-AUTH:
  Сценарий: WS живёт 1ч, JWT истёк через 15 мин.
  Опция A: backend шлёт {type: "auth_required"} когда token expiry approaching
    → frontend: берёт свежий access_token → шлёт {type: "auth", token: "..."}
    → backend: re-validate, продолжает push'ить поток
  Опция B (проще): backend рвёт WS по expiry → client reconnect с новым JWT

SSE FALLBACK:
  10-15% корпоративных firewalls блокируют WebSocket.
  Detect: WS connect failed 3 раза за 30с → fallback на SSE (Server-Sent Events) через EventSource API.
  SSE проходит через HTTP/HTTPS, не блокируется.

LOGOUT:
  Hard logout (user click):
    1. POST /auth/logout (инвалидация refresh на backend)
    2. Очистка access token в store
    3. Очистка refresh cookie (Set-Cookie expired)
    4. Закрыть все вкладки (BroadcastChannel "logout")
    5. Redirect на /
  Soft logout (вкладка закрыта):
    1. Refresh token жив, access истёк в памяти — это ок
    2. При возврате: silent refresh → явный login если fail

ПРАВИЛО: НИКОГДА не хранить access_token в localStorage. Это не обсуждаемо.
ПРАВИЛО: НИКОГДА не логгировать токены в console / Sentry / Datadog.
```

---

## 26. DECIMAL MONEY HANDLING (CONVENTIONS: NUMERIC(18,8))

```
ЦЕЛЬ: 0 потерь precision от backend (NUMERIC(18,8)) до UI.
ПРОБЛЕМА: JavaScript Number = float64 → 0.1 + 0.2 → 0.30000000000000004.
РЕШЕНИЕ: decimal.js везде где касаемся денег.

LIBRARY: decimal.js (~5KB gzipped, без зависимостей)
  Alternatives: dinero.js (хорош для currency formatting), big.js (легче но меньше фич)
  НЕ использовать: parseFloat(), Number(), операторы +-*/ для денег.

API FORMAT (backend возвращает):
  {
    "balance": "12530.50",          ← STRING, НЕ number
    "currency": "RUB",
    "precision": 2                  ← currency-specific decimals
  }
  Или (расширенно):
  {
    "amount": "0.00012345",         ← BTC: 8 decimals
    "currency": "BTC",
    "precision": 8
  }

DISPLAY PRECISION (per currency):
  RUB / JPY / KRW:    0 decimals    "12 530"
  USD / EUR / GBP:    2 decimals    "125.50"
  BTC:                8 decimals    "0.00012345"
  ETH:                6 decimals    "0.123456" (не 18, иначе нечитаемо на UI)
  USDT:               2 decimals (показываем как USD)

ROUNDING:
  Display (только UI): half-up (обычное роундинг)
  Арифметика (potential payout calc): banker's rounding (RoundHalfEven) — избегаем bias
  НИКОГДА НЕ округляем ПРИ ОТПРАВКЕ на backend — шлём full precision string.

FORMATTING (Intl.NumberFormat вызываем ПОСЛЕ decimal.js):
  // РАСЧГТЫ: decimal.js
  const balance = new Decimal('12530.50');
  const stake = new Decimal('100');
  const odds = new Decimal('2.10');
  const payout = stake.times(odds);  // Decimal('210')

  // ДИСПЛЕЙ: Intl.NumberFormat с user.locale ИЗ ПРОФИЛЯ
  const formatter = new Intl.NumberFormat(user.locale, {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });
  formatter.format(balance.toNumber()); // "12 530 ₽" (ru-RU) / "₽12,530" (en-US)

LOCALE RULES (ИЗ ПРОФИЛЯ юзера, НЕ navigator.language!):
  ru-RU: "12 530,50 ₽"     (space — thousand sep, comma — decimal)
  en-US: "$12,530.50"        (comma — thousand, dot — decimal)
  de-DE: "12.530,50 €"      (dot — thousand, comma — decimal)
  fr-CH: "12'530.50 CHF"     (apostrophe — thousand)
  ja-JP: "¥12,530"           (no decimals)

  ПОЧЕМУ ИЗ ПРОФИЛЯ:
    VPN-юзер с ru-RU профилем но с navigator.language=en-US увидит "$" — confusion + chargeback.

JETBRAINS MONO (CONVENTIONS из part1):
  Цифры balance / odds / stake рендерятся в JetBrains Mono.
  Символ currency (₽, $, €) — в Inter (иначе выглядит странно).
  Реализация: <CurrencyAmount value={balance} currency="RUB" /> — один компонент везде.

ПРАВИЛО (CONVENTIONS NEVER-6): НИКОГДА float/double для денег.
ПРАВИЛО: Все финансовые операции (stake × odds, balance + win) через decimal.js.
ПРАВИЛО: API всегда возвращает string — НИКОГДА number.
```
