# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 2: Pages, Components & Auth
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# CANONICAL ROUTES: см. docs/FRONTEND_SPEC.md

---

## 6. AUTH & ONBOARDING

AUTH DOMAIN CANONICAL: подробный registration/login/2FA/password/email/RG onboarding flow описан в `frontend-spec-part4-auth-flows.md`. Этот раздел задаёт только entry points для страниц.

### 6.0.1 Login (/login)
```
Форма: email/phone + пароль
"Запомнить меня" checkbox
Ссылка "Забыли пароль?" → /forgot-password
CTA: "Войти" (btn-primary frost)
Ошибки: inline под полем, НЕ alert
2FA: если включена → redirect на /login/2fa
Отдельная страница, НЕ модалка (SEO + deep link)
```

### 6.0.2 Register (/register)
```
Шаги:
  1. Email/phone + пароль (strength indicator)
  2. Страна + валюта (валюту НЕЛЬЗЯ менять после!)
  3. Age gate: чекбокс "Мне исполнилось 18 лет" (обязательный)
  4. Промокод (предзаполнен из ?ref= URL)
  5. Согласия: Terms, Privacy, Marketing (opt-in)

CTA: "Создать аккаунт" (btn-primary frost)
После регистрации: → главная + toast "Добро пожаловать!"

Geo-check: при загрузке страницы → если страна заблокирована → показать GeoBlockScreen
Gamstop/BetStop: проверка при submit (UK/AU)
Consent: cookie consent banner при первом визите
```

### 6.0.3 Forgot Password (/forgot-password)
```
Email → "Отправить ссылку" → проверка inbox → новый пароль
Rate limit: 3 попытки / 15 мин
Ошибка: НЕ говорить "email не найден" (утечка)
```

### 6.0.4 Session Management
```
Expired token: автоматический silent refresh
Refresh failed: SessionExpiredModal → "Войти снова"
Финансовая операция в процессе: дождаться завершения перед logout
```

---

## 7. СТРАНИЦЫ

### 6.1 Главная страница (/)

```
СЕКЦИЯ 1 — Hero Banner:
  Mobile:  60dvh (ИСПОЛЬЗУЙ dvh, НЕ vh — iOS Safari address bar bug!)
           Под скроллом должны быть видны 1-2 карточки Live Wins (peek effect)
  Desktop: 500px (fixed)
  Fallback: min-height: 60vh; height: 60dvh; — для браузеров без dvh

  Градиентный фон + декоративное изображение (lazy, priority для LCP)
  Заголовок: "Получите 100% бонус до 50 000₽" (Montserrat 700, 30-36px)
  Подзаголовок: "5000+ игр • 50+ провайдеров • Моментальные выплаты" (14px, muted)
  CTA: "Начать играть" (btn-primary frost, крупная)
  Второстепенная CTA: "Подробнее" (btn-outline)

  Для UKGC: БОНУС БАННЕРЫ для раньше регистрации ИЛИ после логина НЕЛЬЗЯ показывать без раскрытия T&Cs
  Ссылка "Условия” под CTA, обязательно видимая

СЕКЦИЯ 2 — Live Wins Feed (бегущая строка):
  Формат: "Lucky_Wolf_3492 выиграл 125 000₽ в Gates of Olympus 🎰"
  Фильтры: множитель ≥ x20, сумма ≥ $50

  🔴 PRIVACY (GDPR Art. 6 + 152-ФЗ РФ):
    ПРИ РЕГИСТРАЦИИ ОБЯЗАТЕЛЬНЫЙ checkbox:
      “[ ] Разрешаю анонимный показ моих выигрышей в Live Feed”
    Default: false (opt-in, НЕ opt-out!)
    Backend фильтрует поток по этому флагу.
    Frontend НИКОГДА не получает выигрыши non-consenting users.

  АНОНИМИЗАЦИЯ (НЕ обрезка ника!):
    НИКОГДА: "P***r" или "Iv***ov" (deanonymizable через timing attack)
    ИСПОЛЬЗОВАТЬ: server-generated handle "Lucky_Wolf_3492", "Eagle_Player_77"
    Handle стабилен для игрока (всегда один и тот же в feed), но не раскрывает user_id

  Скорость: 1 запись / 4 секунды
  Максимум: 1 запись/мин от одного юзера
  Анимация: fade-in сверху, fade-out снизу после 4 сек
  При prefers-reduced-motion: статичный список последних 5 выигрышей (без бегущей строки)

  ПРАВИЛО: показывать ТОЛЬКО выигрыши, НИКОГДА проигрыши
  UKGC LCCP 5.1.10: выигрыши НЕ должны быть misleading — показывать фактическую выплату, не ROI

СЕКЦИЯ 3 — Jackpot Counter:
  3 уровня: MEGA (frost/cyan), MAJOR (platinum), MINI (slate)
  Шрифт: JetBrains Mono 700, 24-36px

  🔴 ИНТЕРПОЛЯЦИЯ — monotonic catch-up (UKGC LCCP 7.1.1 / MGA Player Protection):
    ПРАВИЛО: clientValue НИКОГДА не превышает последнее serverValue.
    ПРАВИЛО: НИКАКОГО randomFactor / random multipliers — это fabricated number (regulatory risk).

    Алгоритм:
      1. Сервер шлёт (timestamp, value) каждые 5с через WS
      2. Клиент вычисляет: velocity = (serverValue_new - serverValue_old) / 5s
      3. Между snapshot'ами интерполирует linear:
           displayValue = clamp(serverValue_old + velocity * elapsed, serverValue_old, serverValue_new)
      4. Если serverValue уменьшилось (джекпот выигран кем-то):
           → явный reset с toast "🎉 MEGA джекпот выигран!" + анимация сброса
           → НЕ тихое уменьшение (игрок должен увидеть почему)
      5. Если WS connection lost > 30с:
           → фриз числа + серый оверлей + tooltip "Данные обновляются..."
           → НЕ экстраполяция вперёд (можем показать больше реального джекпота!)
      6. prefers-reduced-motion: мгновенное обновление на serverValue без интерполяции

  Glow: frost/cyan свечение, пульсация

СЕКЦИЯ 4 — Топ игры казино (горизонтальный scroll):
  8 карточек, scroll-snap
  Заголовок: "🔥 Популярные игры" + ссылка "Все игры →"

СЕКЦИЯ 5 — Live спорт (3-4 события):
  Ближайшие live-матчи с коэффициентами
  Заголовок: "⚡ Live сейчас" + badge с количеством live
  Ссылка "Все ставки →"

СЕКЦИЯ 6 — Промо-баннеры (2-3 карточки):
  Gradient background, CTA button
  Адаптив: 1 колонка mobile, 3 колонки desktop

СЕКЦИЯ 7 — Провайдеры (логотипы):
  Горизонтальный scroll, grayscale → цвет на hover
```

### 6.2 Casino Lobby (/casino)

```
LAYOUT:
  Поиск (debounce 250ms, min 2 символа, для CJK — 1 символ)
  Категории: Все | Слоты | Live | Блэкджек | Рулетка | Crash | Настольные
  Провайдеры: фильтр-чипы
  Вкладки: Все игры | Избранное ★

ПЕРСОНАЛИЗАЦИЯ ("Для вас"):
  Незарегистрированный: скрыть, показать "🔥 Топ-игры недели"
  0 сессий: "🚀 Попробуй первым" (новинки за 7 дней)
  1-2 сессии: "Тренды сейчас" (глобальная популярность)
  3+ сессий, 5+ игр: "Для вас" (collaborative filtering)
  10+ сессий: "Для вас" + "Продолжить играть"

СЕТКА ИГР:
  Mobile: 2 колонки
  Tablet: 3-4 колонки
  Desktop: 5-6 колонки
  gap: 12px
  Виртуализированный список (react-window) + "Загрузить ещё" (40 игр)
  Skeleton-заглушки для незагруженных карточек

ПОВЕДЕНИЕ КАРТОЧКИ ПРИ ТАПЕ (mobile):
  Незалогиненный: bottom sheet → "Демо" + "Войти для игры на деньги"
  Залогиненный, новый: bottom sheet → "Играть" + "Демо"
  Залогиненный, опытный: настройка "Быстрый запуск" → сразу игра
  Long press: расширенная инфо (провайдер, волатильность, рейтинг)

RTP:
  Конфигурируемый флаг per jurisdiction
  UKGC: отображать обязательно
  MGA: доступен в "Информация об игре"
  Кюрасао: на усмотрение оператора
  Реализация: компонент RTP-badge, видимость из geo-config
```

### 6.3 Sportsbook (/sportsbook)

```
LAYOUT (3 колонки desktop):
  Left sidebar (200px): виды спорта + счётчик событий
  Center: события
  Right sidebar (280px): купон (BetSlip)
  Mobile: sidebar скрыт, спорты = горизонтальный scroll

СОБЫТИЯ:
  Prematch: дата + время (локальное)
  Live: пульсирующий индикатор + минута/период + счёт (жёлтый)
  Odds: кнопки 1-X-2 (или 1-2 для баскетбола/тенниса)
  
ФОРМАТ КОЭФФИЦИЕНТОВ (настройка в профиле):
  Десятичный: 2.10 (default)
  Дробный: 11/10
  Американский: +110
  Переключение: event 'odds-format-changed' → все компоненты обновляются
  Анимация: fade 100ms → fade 100ms (без пустого состояния)
  Хранение: Zustand store, persist в localStorage

КЭШАУТ:
  Кнопка "Кэшаут X₽" на активных ставках
  Задержка: если сумма изменилась — показать новую, подтвердить заново
  Настройка: "Принимать при изменении до X%" (по умолчанию = ручное)
  Частичный кэшаут: слайдер, мин 10% от полного, шаг 100₽
```

### 7.4 Купон (BetSlip)

```
DESKTOP: правый sidebar, sticky
MOBILE: bottom sheet (role="dialog", drag up/down), кнопка-badge внизу

ВКЛАДКИ: Ординар | Экспресс | Система

СТАВКА:
  Ввод суммы + quick buttons: [100] [500] [1000] [5000]
  Коэффициент: JetBrains Mono, bold, #F8FAFC
  Выигрыш: JetBrains Mono, bold, #2DD4BF
  Кнопка "Сделать ставку": btn-primary frost, full-width
  Disabled: opacity 0.5, cursor not-allowed
  Double-click protection: disabled на 2с после нажатия

ARCHITECTURE (client state vs server state):
  Selections: хранятся ЛОКАЛЬНО в Zustand (client state)
  Quote: POST /api/v1/bets/quote после каждого добавления
    → возвращает: актуальные odds, max_stake, potential_payout, blocked_reason
  Place Bet: POST /api/v1/bets (server state)
    → body: bet_type, selections, stake, currency, accept_odds_changes, quote_id, idempotency_key
    → accept_odds_changes: none | higher | any (настройка пользователя)

FLOW:
  Добавление: UI обновляется мгновенно (Zustand)
    → параллельно POST /api/v1/bets/quote
    → если quote вернул blocked_reason: пометить исход красным + причина
  Подтверждение:
    → генерируется idempotency_key (UUID v4)
    → кнопка → skeleton state (НЕ spinner)
    → 200: toast "✅ Ставка принята" + receipt, купон очищается
    → 409 (кэф изменился): модалка с новым кэфом + [Принять] [Отменить]
    → 403 (лимит/RG): toast "❌ Превышен лимит" + inline error
    → timeout >10с: "⏳ Обрабатывается...", polling GET /api/v1/bets/by-idempotency/{key}
    → retry: тот же idempotency_key → сервер возвращает результат, не дублирует
    → 60с без ответа: CTA "Связаться с поддержкой"

ПЕРСИСТЕНТНОСТЬ:
  Anonymous: sessionStorage cache с TTL = 2 часа, без plain localStorage
  Authenticated: server-side BetSlip source of truth + sessionStorage cache
  Мульти-вкладки: BroadcastChannel API для синхронизации
  При загрузке: проверить актуальность каждого исхода через API
  Неактуальные: пометить красным + причина, предложить удалить
  Максимум исходов: 20 для экспресса

SYSTEM BETS (≥3 исходов):
  Показать доступные системы (2/3, 2/4, 3/4 и т.д.)
  Каждая комбинация: кэф + ставка + выплата
  Сценарии выигрыша: "Все выиграли → X₽", "3 из 4 → Y₽"
  Макс. комбинаций >120: показывать только итоги
```

### 7.5 Bet Builder (/sportsbook/event/{id})

```
МАРШРУТ: /sportsbook/event/{id} (НЕ /sports/event/{id})
Точка входа: на странице матча → кнопка "🔧 Сконструировать ставку"
Только prematch (НЕ live)

Категории: Результат | Голы | Угловые | Карточки | Игроки
Каждый выбор добавляется в комбо

Итоговый кэф: пересчёт при каждом добавлении (POST /api/v1/bets/quote)
  Во время пересчёта (500-2000ms):
    → показывать skeleton quote box (НЕ spinner)
    → старый кэф остаётся видимым как stale + badge "обновляется"
    → после ответа: плавная смена числа

КОРРЕЛЯЦИЯ:
  Заблокированные: П1 + П2 (противоречие) → toast "❌ Недоступно" + disabled
  Скорректированные: П1 + Обе забьют → снижение 5-15% → toast "⚠️ Кэф скорректирован"

Макс. исходов: 10
Макс. выплата: указать лимит

Real-time: если кэф изменился пока пользователь думает
  → подсветка жёлтым + toast "Кэф изменился: 1.55 → 1.50"
```

### 7.6 Wallet (/wallet, /wallet/deposit, /wallet/withdraw, /wallet/transactions)

```
БАЛАНС:
  Реальные средства: X₽ (доступно для вывода ✓)
  Бонусные средства: Y₽ (вейджер: ████░░ 42%, нельзя вывести)
  Общий баланс: X+Y₽

ДЕПОЗИТ (/wallet/deposit):
  RG Guard: проверяется перед показом формы
  KYC Guard: jurisdiction-aware (см. §8 KYC)
  Платёжные методы: карты, крипто, e-wallets
  Ввод суммы + quick amounts
  Для мультивалютности: "Вы вносите $100 (≈ €92.00 по курсу 1.087)"
  Idempotency: каждый deposit request с idempotency_key
  3DS challenge: redirect → callback → status update
  Крипто: показать адрес + QR + сеть + предупреждение о wrong network
  Незавершённая транзакция:
    → при следующем логине проверить pending
    → успешна → зачислить + toast
    → отклонена → уведомить
    → pending >30мин → push notification
    → pending >24ч → escalation в поддержку

ВЫВОД (/wallet/withdraw):
  KYC UNVERIFIED → блокировка вывода + CTA "Начать верификацию"
  KYC VERIFIED → выбор метода, ввод суммы, idempotency_key
  Статусы: pending, processing, completed, rejected, manual_review
  AML/Source of Funds: если triggered → modal "Требуется подтверждение" + upload
  Отмена: пользователь может отменить pending withdrawal

ИСТОРИЯ ТРАНЗАКЦИЙ:
  Фильтры: тип, период, статус
  Каждая запись: дата, тип, сумма, статус (badge цветной)
```

### 7.7 Bet History (/bets, /bets/history, /bets/{id})

```
/bets — Активные ставки:
  Список open/unsettled bets
  Каждая: событие, исходы, кэф, ставка, потенциальный выигрыш
  Кнопка Cashout (если доступен)
  Кнопка Repeat Bet

/bets/history — История:
  Фильтры: период, спорт, тип (ординар/экспресс/система)
  Каждая: + Won (зелёный) / - Lost (красный) / Void / Cashout
  Ссылка на детали

/bets/{id} — Детали:
  Все исходы с результатами
  Кэшаут история (если был)
  Receipt (при размещении)
  Dispute link → /support?bet_id={id}
  Repeat Bet button

Компоненты:
  BetReceipt — показывается после успешного размещения
  ActiveBetCard — карточка активной ставки
  BetDetailView — полная информация
  CashoutPanel — UI для частичного/полного кэшаута
  RepeatBetButton — повторная ставка
```

---

### 6.7 Bonuses (/bonuses)

```
ТИПЫ:
  Welcome Bonus: +100% до X₽, вейджер x35, срок 30 дней
  Free Spins: N спинов в конкретной игре, ставка X₽, вейджер x30
  Cashback: % от проигрыша за неделю, вейджер x3
  Reload: % бонус на повторный депозит (второй, третий deposit)
  No-deposit: фиксированный бонус без депозита (маленький, x50+ wagering)

ОТОБРАЖЕНИЕ КАРТОЧКИ:
  Header: название + badge (Active / Available / Expired)
  Body: сумма бонуса + вейджер multiplier + срок
  Progress: progress bar + "Отыграно: 1,250₽ из 5,000₽ (25%)"
  Countdown: обратный отсчёт (color-coded: green / yellow / red)
  CTA: [Активировать] или [Отказаться] или [Подробнее]
  Expandable: Правила бонуса + Contribution table

  ⚠️ Баннер сверху: "Вывод невозможен до выполнения условий"
  ⚠️ Ссылка "T&C" ОБЯЗАТЕЛЬНО видима (UKGC LCCP 5.1.6)

ПОРЯДОК СПИСАНИЯ С БАЛАНСОВ (индустриальный стандарт):
  При АКТИВНОМ бонусе (wagering не выполнен):
    → сначала БОНУСНЫЕ средства (иначе бонус не отыграется — dark pattern по UKGC)
    → потом real когда bonus = 0
  При ОТСУТСТВИИ бонуса или выполненном wagering:
    → real (он единственный)

  CONTRIBUTION RATES к wagering (отображаем в бонусе):
    Слоты:                 100%
    Рулетка (American/Eu):  10-25% (per правилам бонуса)
    Блэкджек:            5-10%
    Live игры:            обычно 0% (исключены)
    Crash games:           50-100%
    Sportsbook:            отдельный бонус (обычно не использует casino bonus)
  Таблица ОБЯЗАТЕЛЬНО видима в /bonuses (раскрываемая секция "Как отыграть")

  FORFEIT (отказ от бонуса):
    Кнопка "Отказаться от бонуса" → modal:
      "Бонус X₽ будет аннулирован. Реальные средства станут доступны для вывода."
      [Подтвердить] [Отменить]
    ПОСЛЕ forfeit: бонус = 0, real освобождён для withdrawal

  EXPIRY WARNINGS:
    48ч до expiry: баннер в /bonuses "Бонус истекает через 48ч"
    24ч: жёлтый toast на любой странице
    1ч: красный toast с обратным отсчётом
    Сжигается в 00:00 expiry-day: badge "Истёк" + перенос в archive

  STICKY vs NON-STICKY бонусы:
    Sticky ("привязанный"): вывод только выигрыша, бонус сжигается при withdrawal
      → UI badge "Sticky" + tooltip
    Non-sticky: всё выводится после wagering
      → без баджа
```

### 7.9 Profile (/profile, /profile/settings, /profile/kyc, /profile/responsible)

```
Обзор: баланс, уровень, XP
Личные данные
Смена пароля + 2FA
Формат коэффициентов (настройка)
Язык / валюта (валюту нельзя менять после регистрации!)
Уведомления (категории push)
Верификация KYC
Ответственная игра (лимиты, тайм-аут, самоисключение)
Реферальная программа
```

---

## 8. KYC FLOW (Jurisdiction-Aware)

```
СОСТОЯНИЯ (state machine):
  UNVERIFIED → jurisdiction-aware: UKGC депозит запрещён до KYC; MGA/Curaçao депозит возможен с лимитами. Вывод всегда запрещён.
  IN_PROGRESS → игрок в процессе загрузки (sumsub session active)
  PENDING → документы загружены, ждём проверки. Таймер "до 24 часов"
  ADDITIONAL_INFO → нужны доп. документы (верификатор запросил)
  VERIFIED → полный доступ
  REJECTED → причина + "Загрузить повторно" (или final reject → блокировка)
  EXPIRED → push "Паспорт истекает через N дн", экран обновления
  BANNED → KYC провален по fraud/AML → account permanently disabled

ДОКУМЕНТЫ (минимум для Tier 1):
  1. ID document: passport / national ID / driver license
     → Front side + Back side (если ID card)
     → Client-side validation: blur detection (Laplacian variance > 100)
     → Format check: JPEG/PNG/PDF, max 10MB
     → EXIF strip перед upload (приватность)
  2. Selfie + Liveness:
     → Browser camera API (getUserMedia)
     → Face detection (face-api.js или native sumsub SDK)
     → Liveness: 3 random gestures (turn left, blink, smile)
     → Fallback: video upload если camera permission denied
  3. Proof of address (для Tier 2 / withdrawal > $5K):
     → Utility bill / bank statement (< 3 months old)
     → OCR проверка адреса в фоне (matches profile)
  4. PEP/Sanctions declaration:
     → Checkbox "Я НЕ являюсь PEP" + "Не в sanctions list"
     → Если PEP=true → enhanced due diligence flow
  5. Source of Funds (Tier 3 / VIP):
     → Bank statement / employment letter / tax return
     → Trigger: depo > $10K/30д OR bets > $50K/30д

УПЛОАД UI:
  Drag & drop zone или [Выбрать файл]
  Preview thumbnail перед submit
  Прогресс-бар при upload (resumable upload для файлов > 5MB)
  Client-side checks ДО отправки:
    - Blur (Laplacian variance < 100 → "Фото размыто, переснимите")
    - Brightness (mean < 50 → "Тёмно, улучшите освещение")
    - Resolution (< 1000px → "Слишком маленькое разрешение")
    - Face presence в селфи (lib face-api.js)
  ОШИБКИ: inline error под zone, не трогая уже загруженные файлы
    Кредитные карты: ЗАПРЕЩЕНЫ (UKGC)

  Если jurisdiction меняется во время сессии (VPN, переезд):
    Активная deposit form инвалидируется → modal с причиной

SOURCE OF FUNDS (VIP):
  Trigger: депозит >$10,000/30дн или ставки >$50,000/30дн
  Запрос: выписка из банка / справка о доходах
  State: VERIFIED → SOURCE_OF_FUNDS_REQUIRED → SOF_PENDING → VERIFIED / SOF_REJECTED

EDGE CASES:
  Загрузка документа прервалась → resume upload
  Camera permission denied → manual upload fallback
  Документ просрочен во время review → EXPIRED + re-upload
  Underage → мгновенный бан + возврат средств
  Sanctions/PEP → compliance block + support
  KYC provider unavailable → retry + manual review path
  Blurry image rejection → "Загрузите чёткое фото" (НЕ fraud rejection)
```

---

## 8. RESPONSIBLE GAMBLING

```
NET DEPOSIT TRACKER (виджет в профиле):
  Депозиты: $5,000 ↑
  Выводы: $2,300 ↓
  NET: $2,700 (цвет: зелёный если <0, жёлтый >$1K, красный >$5K)
  График за 6 месяцев

SESSION TIMER:
  Расположение: профиль + footer (маленький)
  >2 часов: жёлтый + кнопка "Пауза"
  >4 часов: красный + модальное Reality Check

LOSS LIMIT WARNINGS:
  50%: toast
  75%: toast жёлтый + баннер
  90%: модальное окно с опциями
  100%: блокировка ставок до reset

ВНЕШНИЕ ИНТЕГРАЦИИ:
  UK: Gamstop API при регистрации
  Австралия: BetStop
  Германия: OASIS
  Если в базе самоисключения → блокировка регистрации
```
