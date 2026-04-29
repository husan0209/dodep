# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 4: Auth & Onboarding Flows
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# Расположение: apps/web/src/app/(auth)/

---

## 27. REGISTRATION FUNNEL (/register)

```
ЦЕЛЬ: max conversion + UKGC/MGA compliance + GAMSTOP проверка + age verification.

ПРИНЦИП: progressive disclosure — НЕ показывать все поля сразу. 
Один шаг → одна цель. Конверсия растёт на 15-30% vs single-page form.

ШАГ 1 — EMAIL + PASSWORD (минимум для создания записи):
  ┌─────────────────────────────────────────┐
  │  Создайте аккаунт                       │
  │                                          │
  │  Email *                                 │
  │  [____________________________]          │
  │  ✓ Будет использован для верификации    │
  │                                          │
  │  Пароль *                                │
  │  [____________________________] [👁]    │
  │  Сила: ████░░░░░░  Средний              │
  │  • 8+ символов  • Цифра  • Буква         │
  │                                          │
  │  [Продолжить →]  (frost CTA)             │
  │                                          │
  │  Уже есть аккаунт? Войти                │
  └─────────────────────────────────────────┘

  EMAIL валидация (real-time):
    Format: HTML5 type=email + RegExp
    Disposable email block: 10minutemail, mailinator (через server check)
    Уникальность: debounced 500ms POST /api/v1/auth/check-email → { available: bool }
    Error inline: "Email уже зарегистрирован → [Войти]"

  PASSWORD требования (CONVENTIONS: Argon2id на backend):
    Минимум: 8 символов
    Рекомендуется: 12+ символов, верх/низ регистр, цифра, спецсимвол
    Strength meter: zxcvbn library (показывает crack time)
    HIBP check: server-side проверка через Pwned Passwords API (k-anonymity)
      → если в утечках: warn "Этот пароль скомпрометирован, выберите другой"
    Мониторинг: НЕ блокировать слабый, но show красный strength bar + warning

ШАГ 2 — ЛИЧНЫЕ ДАННЫЕ (для KYC):
  ┌─────────────────────────────────────────┐
  │  Расскажите о себе                       │
  │                                          │
  │  Имя *           Фамилия *               │
  │  [_________]    [_________]              │
  │                                          │
  │  Дата рождения *  Страна *               │
  │  [DD.MM.YYYY]    [▼ Россия]              │
  │                                          │
  │  Телефон * (с кодом страны)              │
  │  [+7 ___ ___ __ __]                      │
  │                                          │
  │  Валюта счёта * (НЕЛЬЗЯ менять!)         │
  │  [▼ RUB - Российский рубль]              │
  │  ⚠️ После регистрации валюта фиксируется. │
  │                                          │
  │  [← Назад]    [Продолжить →]             │
  └─────────────────────────────────────────┘

  AGE GATE (КРИТИЧНО):
    DOB → calculate age client-side
    Per jurisdiction:
      UK / Германия / Испания: ≥18
      США (NJ, PA, MI): ≥21
      Япония: ≥20
    Если age < limit → BLOCK:
      ❌ "К сожалению, регистрация недоступна. Возрастной лимит: 18+/21+"
      Кнопка [На главную] (НЕ кнопка retry — anti-bypass)
    UKGC LCCP 3.2.1: возраст должен быть проверен ДО любого депозита.
    
  COUNTRY DETECTION:
    1. IP geolocation (через CDN edge headers)
    2. Browser locale (fallback)
    3. User selection (override)
    Validation: IP country MUST match selected country (anti-VPN)
    Geo-restricted countries: list загружается с /api/v1/geo/restrictions
      → если selected country в blocklist → block с "Регистрация недоступна"

  CURRENCY LOCK warning:
    Modal перед confirm:
      "Валюту нельзя изменить после регистрации.
       Для смены валюты потребуется обращение в поддержку.
       Продолжить с RUB?"
      [Подтвердить] [Изменить]

  PHONE verification:
    Format: libphonenumber-js (parse + format per country)
    SMS OTP: 6-digit code, TTL 10 минут
    Rate limit: 1 SMS / 60s, max 5/hour per number
    Whatsapp fallback: если SMS не доходит (через 90с)

ШАГ 3 — ПРОВЕРКА GAMSTOP / SELF-EXCLUSION (UK):
  ⚠️ КРИТИЧНО для UK / Австралии / Германии.
  
  AUTOMATIC CHECK (без UI визита, fail silent):
    Backend POST /api/v1/compliance/gamstop-check
      Body: { firstName, lastName, dob, postcode }
      Response: { excluded: bool, until: timestamp | null }
    
    Если excluded=true:
      Frontend BLOCK:
      ┌─────────────────────────────────────────┐
      │  🛑 Регистрация недоступна              │
      │                                          │
      │  Согласно проверке GAMSTOP, вы           │
      │  находитесь в реестре самоисключения.   │
      │                                          │
      │  До: 15.04.2027                          │
      │                                          │
      │  Помощь и информация:                    │
      │  → BeGambleAware.org                     │
      │  → GamCare 0808 8020 133                 │
      │                                          │
      │  [На главную]                            │
      └─────────────────────────────────────────┘
      → НЕ создавать account на backend. НЕ показывать [Retry].

  Аналогично:
    Австралия: POST /api/v1/compliance/betstop-check
    Германия: POST /api/v1/compliance/oasis-check
    Финляндия: POST /api/v1/compliance/peluuri-check (если backend включил региональный endpoint)

ШАГ 4 — TERMS / RESPONSIBLE GAMBLING / MARKETING:
  ┌─────────────────────────────────────────┐
  │  Последний шаг                           │
  │                                          │
  │  ☐ Я подтверждаю, что мне ≥ 18 лет *    │
  │  ☐ Я согласен с Правилами и Политикой * │
  │     конфиденциальности (→ /terms)        │
  │  ☐ Я ознакомился с правилами             │
  │     ответственной игры * (→ /rg)         │
  │                                          │
  │  ─────────────────────────────────       │
  │  Опционально:                            │
  │  ☐ Получать промо-уведомления (email)   │
  │  ☐ Получать Live Wins показы             │
  │     (мои выигрыши анонимно в feed)       │
  │  ☐ Установить лимиты сейчас (UKGC pre-   │
  │     commitment) → /onboarding/limits     │
  │                                          │
  │  [Зарегистрироваться] (frost CTA)        │
  └─────────────────────────────────────────┘

  * — обязательные, default unchecked, кнопка disabled пока не отмечены.

  GDPR / 152-ФЗ:
    Marketing checkbox = explicit opt-in (default unchecked).
    Granular consents: email/SMS/push отдельно (можно расширить).
    Withdraw consent: всегда доступно в /profile/notifications.

  ОТВЕТСТВЕННАЯ ИГРА (PRE-COMMITMENT, UKGC LCCP 2.1.1):
    Чекбокс "Установить лимиты сейчас" → onboarding в /onboarding/limits.
    Если не отмечено: всё равно доступно в профиле, но non-mandatory.
    UK: эта опция МУСТ HAVE на регистрации.

ШАГ 5 — EMAIL VERIFICATION:
  После submit → email с magic link.
  ┌─────────────────────────────────────────┐
  │  📧 Подтвердите email                   │
  │                                          │
  │  Мы отправили письмо на:                 │
  │  user@example.com                        │
  │                                          │
  │  Перейдите по ссылке в письме чтобы      │
  │  активировать аккаунт.                   │
  │                                          │
  │  Не пришло за 5 минут?                   │
  │  • Проверьте спам                        │
  │  • [Отправить повторно] (cooldown 60s)   │
  │  • [Изменить email]                      │
  │                                          │
  │  Tip: можете депозитить и играть         │
  │  пока email не подтверждён, но вывод     │
  │  заблокирован.                           │
  └─────────────────────────────────────────┘

  Magic link TTL: 15 минут.
  Click → GET /api/v1/auth/verify-email?token=... → success → auto-login → /
  Resend cooldown: 60 секунд (rate limit).

CAPTCHA:
  hCaptcha (privacy-friendly) или Cloudflare Turnstile (бесплатно, без UX-puzzle)
  Включается на ШАГЕ 1 после 2 failed attempts с одного IP.
  invisible by default, challenge только при подозрении на bot.

ANALYTICS (для CRO):
  registration.step1.viewed
  registration.step1.completed (email_valid: bool)
  registration.step2.viewed / .completed
  registration.gamstop.checked (passed: bool)
  registration.completed (total_time_seconds, currency, country)
  registration.abandoned (last_step, reason)
```

---

## 28. LOGIN (/login)

```
LAYOUT (Desktop):
  Centered card, max-width 400px
  Logo + "Вход в OPUS"
  Поле Email/телефон
  Поле Пароль (с toggle visibility 👁)
  ☐ Запомнить меня (30 дней refresh)
  [Войти] (frost CTA, full-width)
  ─── или ───
  [G] Войти через Google     (если включено)
  [🍎] Войти через Apple      (обязательно для iOS app store)
  ─────────────────
  Забыли пароль?    Регистрация

ВАЛИДАЦИЯ:
  Email: HTML5 + format check
  Phone: libphonenumber, auto-format
  Password: min 8 (НЕ показываем требования при логине!)

POST /api/v1/auth/login → 4 возможных response:
  
  1. 200 + { access_token, requires_2fa: false }
     → store token in-memory → redirect / (или intended URL)
  
  2. 200 + { requires_2fa: true, methods: ["totp", "sms"], session_id }
     → /login/2fa с {session_id}
  
  3. 401 { reason: "invalid_credentials" }
     → toast "❌ Неверный email или пароль"
     → НЕ показывать "user not found" vs "wrong password" (security)
     → После 5 fails: rate limit 15 минут + captcha
  
  4. 403 { reason: "account_locked" | "self_excluded" | "kyc_rejected" }
     → modal с пояснением + контакт поддержки

REMEMBER ME:
  Если checked: refresh cookie TTL 30 дней
  Если unchecked: refresh cookie TTL 1 день (только сессия)
  ВАЖНО: REMEMBER ME does NOT extend access_token (всё ещё 15 мин).

SOCIAL LOGIN:
  Google OAuth 2.0 (PKCE flow):
    Redirect → Google consent → callback /auth/google/callback
    Backend линкует Google email → существующий аккаунт
    Если новый email: shorten registration (skip email/password шаг)
  
  Apple Sign In (ОБЯЗАТЕЛЬНО для iOS app store):
    "Hide my email" feature: backend получает relay email
    Apple revocation: при revoke → user logs out + email notification
  
  Conflict: если email уже существует с password-аккаунтом:
    "Этот email уже зарегистрирован. Войдите паролем или свяжите Google в профиле."

ACCOUNT LOCKOUT (CONVENTIONS NEVER-5: пароли через Argon2id):
  5 wrong password attempts within 15 min → lock 15 min
  10 attempts within 1 hour → lock 1 hour + email warning
  20 attempts within 24h → lock 24h + admin alert + потенциальная попытка fraud

NEW DEVICE LOGIN:
  Detect: User-Agent + IP geo не совпадают с предыдущей сессией
  → Email уведомление "Новый вход с устройства X из города Y"
  → 2FA challenge даже если 2FA не включён (одноразовая email верификация)
```

---

## 29. TWO-FACTOR AUTHENTICATION

```
ENROLLMENT (/profile/security/2fa):

  Шаг 1: Method selection
    [TOTP authenticator]  (Google Authenticator, Authy, 1Password)
    [SMS]                 (резервный, не основной для regulator's required)
    [Email]               (только как secondary fallback)
    [Hardware key (FIDO2/WebAuthn)]  ← премиум, для VIP

  Шаг 2 (TOTP path):
    QR-код + manual key (для копирования)
    Поле "Введите 6-значный код для подтверждения"
    Backup codes: показать 10 одноразовых кодов → "Сохранили? [Подтвердить]"
    
    UI:
    ┌─────────────────────────────────────────┐
    │  Двухфакторная аутентификация            │
    │                                          │
    │  1. Установите приложение Authenticator: │
    │     • Google Authenticator              │
    │     • Authy                             │
    │     • 1Password                         │
    │                                          │
    │  2. Отсканируйте QR-код:                │
    │  ┌─────────────────┐                    │
    │  │ [QR CODE IMAGE] │                    │
    │  └─────────────────┘                    │
    │  Или введите ключ вручную:              │
    │  JBSWY3DPEHPK3PXP                       │
    │                                          │
    │  3. Введите 6-значный код:               │
    │  [_ _ _ _ _ _]  (auto-advance per digit) │
    │                                          │
    │  [Подтвердить]                           │
    └─────────────────────────────────────────┘

LOGIN WITH 2FA:

  POST /api/v1/auth/login → requires_2fa=true → /login/2fa
  
  ┌─────────────────────────────────────────┐
  │  Подтверждение входа                     │
  │                                          │
  │  Откройте приложение и введите код:      │
  │  [_ _ _ _ _ _]                           │
  │                                          │
  │  Не работает приложение?                 │
  │  → Использовать backup-код               │
  │  → Запросить SMS                         │
  │                                          │
  │  [Подтвердить]                           │
  └─────────────────────────────────────────┘

  Auto-submit: при вводе 6-й цифры → автоматический POST.
  Wrong code: shake animation + error inline.
  3 wrong codes: lock 15 min + email alert.

DISABLE 2FA:
  Требует: текущий пароль + 2FA код + email confirmation.
  НЕ должно быть "one-click disable" — это security regression.

BACKUP CODES:
  10 codes, одноразовые, alphanumeric 8 символов.
  Showing once, after enrollment. Если потерял — re-generate (invalidates old).
  PDF export option.

REMEMBER THIS DEVICE (опционально, UKGC внимательно):
  Checkbox "Не запрашивать 2FA на этом устройстве 30 дней".
  Если включено: device fingerprint stored → skip 2FA для known device.
  ОТКЛЮЧИТЬ для UK по умолчанию (UKGC может расценить как weakening MFA).
```

---

## 30. PASSWORD RESET (/forgot-password)

```
ШАГ 1: Email request
  Поле "Email от вашего аккаунта"
  [Отправить ссылку]
  POST /api/v1/auth/password-reset-request
    Response: ALWAYS 200 (не раскрываем существование email)
    "Если аккаунт существует, мы отправили ссылку на email"
  Rate limit: 1 / 60s, max 5 / day per email.

ШАГ 2: Click email link
  /reset-password?token=xxx (TTL 30 минут)
  Verify token → если expired/invalid → "Ссылка истекла, [Запросить новую]"

ШАГ 3: New password form
  Новый пароль (с strength meter)
  Подтверждение пароля
  [Сменить пароль]
  POST /api/v1/auth/password-reset { token, new_password }
    → 200: invalidate ALL sessions across devices (CONVENTIONS security)
    → email notification "Пароль изменён, IP: X.X.X.X"
    → auto-login + redirect /

CRITICAL:
  Password reset = invalidate ВСЕ refresh tokens пользователя.
  WS connections force-close.
  Active iframe casino sessions ended.
  Это защита от компрометации.
```

---

## 31. EMAIL VERIFICATION

```
СОСТОЯНИЯ:
  PENDING (после регистрации):
    Banner в header: "📧 Подтвердите email для разблокировки вывода"
    [Отправить повторно] (cooldown 60s, max 5/day)
  
  VERIFIED:
    Banner исчезает.
    Profile → "✓ Email подтверждён"

ВЕРИФИКАЦИЯ:
  Magic link → /auth/verify-email?token=xxx → GET /api/v1/auth/verify-email?token=xxx
  TTL 15 минут.
  Single-use (после клика → invalidated).
  GET (не POST) — чтобы работало из email-клиентов и мобильных.
  Server-side: token verify → user.email_verified = true → auto-login → redirect /

CHANGE EMAIL:
  Profile → "Изменить email"
  Шаг 1: Подтверждение паролем.
  Шаг 2: Новый email + send verification.
  Шаг 3: Кликнуть link в новом email.
  Шаг 4: Старый email теряет access (notify обоих).

UNVERIFIED LIMITS (UKGC + MGA):
  ✓ Депозит: разрешён (но per jurisdiction может быть запрет)
  ✓ Игра в casino: разрешена
  ✓ Sports bets: разрешены
  ✗ Withdrawal: ЗАБЛОКИРОВАН до verify
  ✗ Bonus claim: некоторые типы заблокированы
```

---

## 32. RESPONSIBLE GAMBLING ONBOARDING (/onboarding/limits)

```
Триггер: при чекбоксе на регистрации (UKGC pre-commitment).
Также доступно в /profile/responsible.

UI:
┌─────────────────────────────────────────┐
│  Установите ваши лимиты                  │
│  Это поможет играть осознанно            │
│                                          │
│  ───── Лимиты депозитов ─────            │
│  В сутки:    [____] ₽                    │
│  В неделю:   [____] ₽                    │
│  В месяц:    [____] ₽                    │
│                                          │
│  ───── Лимиты ставок ─────              │
│  Макс. ставка:     [____] ₽              │
│                                          │
│  ───── Лимиты убытков ─────             │
│  В сутки:    [____] ₽                    │
│  В неделю:   [____] ₽                    │
│  В месяц:    [____] ₽                    │
│                                          │
│  ───── Лимит времени ─────              │
│  Макс. сессия:  [60] минут               │
│  Reality Check каждые: [60] минут        │
│                                          │
│  [Сохранить] [Пропустить]                │
└─────────────────────────────────────────┘

ПРАВИЛА (UKGC LCCP 2.1.x):
  Уменьшение лимита: МГНОВЕННО (мгновенный эффект)
  Увеличение лимита: с задержкой (cooling-off):
    UK: 24 часа
    Германия: 7 дней
    После задержки: email confirmation → активация
  Снятие лимитов полностью: запрос в support + 7-дневный cooling-off

САМОИСКЛЮЧЕНИЕ (Self-Exclusion):
  /profile/responsible/self-exclude
  Опции: 6 месяцев / 1 год / 5 лет / Permanent

  UI с typed confirmation:
  ┌─────────────────────────────────────────┐
  │  ⚠️ САМОИСКЛЮЧЕНИЕ                       │
  │                                          │
  │  Вы выбрали: 1 год                       │
  │  Период: до 27.04.2027                   │
  │                                          │
  │  ВАЖНО:                                  │
  │  • Аккаунт будет полностью заблокирован │
  │  • Депозиты и ставки невозможны          │
  │  • Бонусы аннулируются                   │
  │  • Вывод средств доступен                │
  │  • Действие НЕОБРАТИМО                   │
  │                                          │
  │  Введите "ИСКЛЮЧИТЬ" для подтверждения:  │
  │  [____________________]                  │
  │                                          │
  │  [Подтвердить] [Отменить]                │
  └─────────────────────────────────────────┘

  После confirm:
    1. POST /api/v1/rg/self-exclude (irreversible)
    2. Email confirmation
    3. SMS confirmation
    4. Logout всех сессий
    5. Block re-registration (по DOB + email + phone)
    6. GAMSTOP/BetStop/OASIS API call (если в этой юрисдикции)

PANIC BUTTON (MGA требование):
  Floating button "Стоп" в правом нижнем углу (mobile + desktop).
  Click → 24h time-out (мгновенно):
    Logout + блок ре-логина 24ч + email confirmation.
  z-index: 70 (см. иерархию part1 §1.7).
  Доступен ТОЛЬКО залогиненным.

REALITY CHECK (MGA):
  Каждые N минут (default 60, настраивается 15-180):
    Full-screen modal:
    ┌─────────────────────────────────────────┐
    │  Перерыв                                 │
    │                                          │
    │  Вы играете уже 1 час                    │
    │                                          │
    │  Сегодня:                                │
    │  Депозиты:  +5,000 ₽                     │
    │  Ставки:    -3,500 ₽                     │
    │  Выигрыши:  +2,800 ₽                     │
    │  Net P&L:   -700 ₽                       │
    │                                          │
    │  [Продолжить через 5с...]  ← disabled    │
    │  [Сделать перерыв 30 мин]                │
    │  [Самоисключение]                        │
    └─────────────────────────────────────────┘
    "Continue" disabled первые 5 секунд (forced reflection).
    Закрыть Esc нельзя.
    Aria: role="alertdialog", focus trap.
```

---

## ✅ CHECKLIST для Auth-flows

```
□ Registration: 4-5 шагов, не single-page
□ Email verification: magic link, 15 min TTL
□ Password: zxcvbn meter, HIBP check, Argon2id (backend)
□ Age gate: client-side от DOB → block если < limit
□ GAMSTOP/BetStop/OASIS: автоматическая проверка ДО создания аккаунта (UK/AU/DE)
□ Currency lock: warning modal перед confirm
□ Country mismatch (IP vs selection): block с warning
□ Marketing: explicit opt-in (default unchecked)
□ Live Wins: explicit opt-in для показа выигрышей (default unchecked)
□ Pre-commitment limits: UKGC обязательно при регистрации
□ Login: 2FA enrollment optional but encouraged
□ TOTP: Google Authenticator / Authy / 1Password support
□ Backup codes: 10 кодов, показ один раз
□ Login throttling: 5 fails → 15 min lock + captcha
□ New device: email notification + 2FA challenge
□ Password reset: invalidate ALL sessions
□ Email change: confirm by old email + verify new
□ Self-exclusion: typed confirmation "ИСКЛЮЧИТЬ"
□ Panic button: floating, z-index 70, 24h instant time-out
□ Reality Check: 5s forced delay before continue
□ Все error messages: НЕ "user not found" vs "wrong password" (security)
□ Cooldown timers: 60s resend SMS/email, max 5/day
```
