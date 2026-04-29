# OPUS CASINO — FRONTEND UI/UX SPECIFICATION
# Part 1: Design System & Layout
# Роль: FRONTEND_WEB_ENGINEER | Стек: Next.js 14, Tailwind CSS 3, Zustand, React Query
# Расположение: apps/web/

---

## 1. DESIGN TOKENS

### 1.1 Цветовая палитра

```
BACKGROUNDS:
  --bg-primary:    #0A0E1A     // Основной фон
  --bg-secondary:  #111827     // Карточки, панели
  --bg-tertiary:   #1A2035     // Elevated surfaces
  --bg-surface:    #1E2642     // Интерактивные поверхности
  --bg-hover:      #252D45     // Hover state

BRAND:
  --color-frost:      #F8FAFC  // Primary CTA, активные элементы
  --color-platinum:   #CBD5E1  // Вторичный премиум-акцент
  --color-cyan:       #67E8F9  // Focus, интерактив, real-time акцент
  --color-mint:       #2DD4BF  // Выигрыш, финансовый плюс

SEMANTIC (ЕДИНАЯ ТАБЛИЦА — без конфликтов):
  FINANCIAL_POSITIVE:  #2DD4BF  // Выигрыш, прибыль, cashout, депозит OK
  FINANCIAL_NEGATIVE:  #FF3B3B  // Проигрыш, убыток, ошибка, лимит
  COEFFICIENT_UP:      #22C55E  // Кэф вырос (зелёный + ↑ иконка, color-blind safe)
  COEFFICIENT_DOWN:    #FF6E40  // Кэф упал (ОРАНЖЕВЫЙ + ↓ иконка, НЕ красный!)
  LIVE:                #FF1744  // Live-индикаторы, пульсация
  PRIMARY_ACTION:      #F8FAFC  // CTA кнопки, белый/платиновый action
  INFO:                #67E8F9  // Информация, бейджи, подсказки
  WARNING:             #FFB300  // Предупреждения, лимиты
  DISABLED:            #4A4A5A  // Заблокированные элементы

ПРАВИЛО 1: Coefficient DOWN = оранжевый. Красный ТОЛЬКО для проигрыша/ошибки.
ПРАВИЛО (WCAG 1.4.1): Цвет НИКОГДА не единственный сигнал. Up/Down всегда + arrow icon.
Для дейтеранопии (~6% мужчин) зелёный/оранжевый практически неразличимы.
ПРАВИЛО 2: НИКОГДА только цвет. Каждый status ОБЯЗАН иметь icon или text:
  Odds up:       ↑ 2.10  (зелёный + стрелка)
  Odds down:     ↓ 1.95  (оранжевый + стрелка)
  Market closed: 🔒 —    (серый + замок)
  Win:           +1,230₽ Won  (зелёный + текст)
  Loss:          -500₽ Lost   (красный + текст)
  Live:          ● LIVE       (красный + пульсирующий круг + текст)

TEXT:
  --text-primary:   #F8FAFC   // Основной текст
  --text-secondary: #CBD5E1   // Второстепенный
  --text-muted:     #94A3B8   // Приглушённый
  --text-disabled:  #64748B   // Неактивный

BORDERS:
  --border:       #1E293B     // Основная граница
  --border-light: #334155     // Hover/focus граница
```

### 1.2 Типографика

```
ШРИФТЫ:
  Body:     Inter (latin, cyrillic) — основной текст
  Display:  Montserrat (latin, cyrillic) — заголовки, CTA
  Mono:     JetBrains Mono — баланс, кэфы, суммы (ТОЛЬКО цифры)

РАЗМЕРЫ:
  --text-xs:   12px / 16px    // Метки, провайдер, badges (МИНИМУМ для UI)
  --text-sm:   14px / 20px    // Основной интерактивный текст (МИНИМУМ)
  --text-base: 16px / 24px    // Крупный текст
  --text-lg:   18px / 28px    // Подзаголовки
  --text-xl:   20px / 28px    // Заголовки секций
  --text-2xl:  24px / 32px    // Заголовки страниц
  --text-3xl:  30px / 36px    // Hero-текст
  --text-4xl:  36px / 40px    // Джекпот-счётчик

МИНИМАЛЬНЫЕ РАЗМЕРЫ (ОБЯЗАТЕЛЬНО):
  Интерактивный текст (кнопки, ввод, ставки): ≥14px
  Odds value: ≥14px (JetBrains Mono bold)
  Odds label (1, X, 2): ≥12px
  Badges: ≥11px (допустимо, т.к. не единственный носитель инфо)
  Provider name: ≥12px
  Game name: ≥13px
  ЗАПРЕЩЕНО: 9px, 10px для ЛЮБОГО текста

ЗАГРУЗКА ШРИФТОВ:
  1. JetBrains Mono — preload, subset (U+0030-0039, $, ., ,, +, -) = ~5KB
  2. font-display: swap для всех шрифтов
  3. Fallback metric matching: ascent-override 80%, descent-override 20%
  4. Skeleton для баланса пока шрифт грузится
```

### 1.3 Spacing & Layout

```
SPACING SCALE: 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80, 96px
BORDER RADIUS: sm=6px, md=10px, lg=16px, xl=20px, full=9999px
CONTAINER: max-width 1440px, padding 16px (mobile) / 24px (desktop)

BREAKPOINTS (Tailwind default):
  sm: 640px   md: 768px   lg: 1024px   xl: 1280px   2xl: 1536px

TOUCH TARGETS: минимум 44×44px (Apple HIG)
```

### 1.4 Анимации

```
MOTION TOKENS:
  --duration-instant: 100ms   // Micro-interactions (hover, active)
  --duration-fast:    150ms   // Tooltips, fade
  --duration-normal:  250ms   // Модалки, slide
  --duration-slow:    400ms   // Page transitions
  --easing-default:   cubic-bezier(0.4, 0, 0.2, 1)
  --easing-bounce:    cubic-bezier(0.34, 1.56, 0.64, 1)

АНИМАЦИИ:
  glow-gold:     0 0 20px rgba(255,215,0,0.3)  // Золотое свечение CTA
  glow-green:    0 0 15px rgba(0,255,159,0.2)   // Свечение выигрыша
  pulse-live:    пульсация 1.5s infinite          // Live-индикатор
  shimmer:       градиент 1.5s infinite           // Skeleton loading
  slide-up:      translateY(10px)→0, opacity 0→1  // Появление снизу
  scale-in:      scale(0.95)→1, opacity 0→1       // Появление с масштабом
  countup:       числовая анимация для баланса     // 300-500ms

REDUCED MOTION (@media (prefers-reduced-motion: reduce)):
  ticker (live wins):   статичный список последних выигрышей
  jackpot interpolation: мгновенное обновление числа
  odds flash:           замена на border/icon change (↑↓)
  pulse-live:           статичный бейдж LIVE (без пульсации)
  shimmer:              статичный skeleton (bg цвет)
  countup:              мгновенное обновление числа
  glow:                 отключить box-shadow animation
  scale/slide:          отключить transform, показывать мгновенно
```

### 1.5 Glassmorphism

```css
.glass {
  background: rgba(17, 24, 39, 0.7);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

/* LOW-END FALLBACK: disable blur on weak devices */
@media (prefers-reduced-transparency: reduce) {
  .glass {
    background: rgba(17, 24, 39, 0.95);
    backdrop-filter: none;
  }
}

/* JS fallback: detect low-end via navigator.deviceMemory < 4 or hardwareConcurrency < 4 */
/* → add class .low-end-device → same opaque fallback */
```

### 1.6 Elevation / Shadows (на dark UI)

```
--shadow-xs:    0 1px 2px rgba(0,0,0,0.3)               // Кнопки в спокойном состоянии
--shadow-sm:    0 2px 4px rgba(0,0,0,0.4)               // Cards default
--shadow-md:    0 4px 12px rgba(0,0,0,0.5)              // Hover на карточках
--shadow-lg:    0 8px 24px rgba(0,0,0,0.6)              // Bottom sheets, dropdowns
--shadow-xl:    0 16px 48px rgba(0,0,0,0.7)             // Modals
--shadow-glow-gold:   0 0 20px rgba(255,215,0,0.3)      // CTA
--shadow-glow-green:  0 0 15px rgba(0,255,159,0.2)      // Win celebration
--shadow-glow-live:   0 0 12px rgba(255,23,68,0.4)      // Live pulse
--shadow-inset-top:   inset 0 1px 0 rgba(255,255,255,0.08) // Highlight на gold кнопках
```

### 1.7 Z-Index Hierarchy (единая шкала, без коллизий)

```
0      base content
10     sticky header
20     dropdowns / popovers / tooltips
30     mobile bottom nav (fixed)
40     drawers / overlays / scrim
50     bottom sheets / modals
60     toast notifications
70     critical alerts (panic button, RG limits, GAMSTOP block)
80     onboarding spotlight (если есть)
100    DEV/DEBUG overlay (только в development)

ПРАВИЛО: НЕ используй произвольные значения (z-index: 99999).
Всегда из этой шкалы. Если коллизия — пересмотри иерархию, не повышай число.
```

---

## 2. КОМПОНЕНТЫ UI

### 2.1 Кнопки

```
btn-primary (Frost CTA):
  bg: linear-gradient(135deg, #FFFFFF, #CBD5E1)
  text: #020617 (тёмный)
  hover: brightness(1.04) + box-shadow glow-frost
  active: scale(0.98)
  font: Montserrat 600, 14px
  padding: 10px 20px
  border-radius: 10px

btn-secondary:
  bg: --bg-surface
  text: --text-primary
  border: 1px solid --border
  hover: border --border-light, bg --bg-hover

btn-outline:
  bg: transparent
  border: 1px solid --border
  hover: border --border-light, bg rgba(255,255,255,0.05)

btn-destructive:
  bg: #FF3B3B/10
  text: #FF3B3B
  hover: bg #FF3B3B/20
```

### 2.2 Odds Button (кнопка коэффициента)

```
Default:
  bg: --bg-primary
  border: 1px solid --border
  padding: 8px 12px
  min-width: 64px
  layout: label(top, 12px, muted) + value(bottom, 14px, bold, JetBrains Mono)
  aria-label: "{outcomeName}, коэффициент {value}"

Selected:
  border-color: #FFD700
  bg: rgba(255,215,0, 0.08)
  value color: #FFD700
  aria-pressed: true

Кэф вырос:    flash зелёным (#00E676) 600ms + ↑ иконка
Кэф упал:     flash оранжевым (#FF6E40) 600ms + ↓ иконка
Заблокирован: opacity 0.4, cursor not-allowed, 🔒 текст, aria-disabled

Keyboard: Enter/Space для выбора, focus ring 2px #FFD700
Screen reader: aria-live throttled (max 1 update/5s per button)
```

### 2.3 Badges (ПРИОРИТЕТ — показывать ТОЛЬКО 1 на карточке)

```
Приоритет (от высшего к низшему):
  1. JACKPOT:    bg золотой + пульсация 3с + 💎
  2. EXCLUSIVE:  bg пурпурный градиент + 🔒
  3. HOT:        bg красный + 🔥
  4. BONUS BUY:  bg #00B4D8 + ⚡
  5. NEW:        bg #00FF9F + текст NEW

Размер: padding 4px 8px, font 11px 600, border-radius 6px
Провайдер: watermark (не бейдж!) в левом нижнем углу, opacity 0.7
```

### 2.4 Cards

```
Game Card:
  bg: --bg-secondary
  border: 1px solid --border
  border-radius: 12px
  overflow: hidden
  hover: scale(1.03), border rgba(255,215,0,0.3), shadow glow-gold
  transition: 200ms ease-out
  
  Image: aspect-ratio 3/4, object-fit cover, lazy loading, fixed dimensions
  Overlay on hover: bg black/60, кнопки "Играть"(gold) + "Демо"(outline)
  Info: padding 10px, name 13px 500, provider 12px muted
  RTP badge: отдельная зона (bottom-right), НЕ конкурирует с promotional badge
  Keyboard: focus → show overlay, Enter → играть
  Long press alternative: explicit ℹ️ button for keyboard/mouse users

Sports Event Card:
  bg: --bg-secondary
  border: 1px solid --border
  Live: left border 3px solid #FF1744 + ● LIVE текст
  hover: border --border-light
```

---

## 3. LAYOUT

### 3.1 Header (sticky, 56px)

```
Структура:
┌─────────────────────────────────────────────────────────────┐
│ OPUS(logo)  │ Казино  Спорт  Live  Промо │  💰12,530₽  [Депозит]  👤 │
└─────────────────────────────────────────────────────────────┘

Детали:
  height: 56px
  bg: --bg-primary + backdrop-blur(12px) при скролле
  border-bottom: 1px solid --border
  z-index: 50

Logo: "OPUS" — Montserrat 700, 20px, с frost/cyan акцентом на "O"

Навигация (desktop):
  items: Казино, Спорт, Live Casino, Промо
  active: text-white + underline 2px cyan/frost
  hover: text-white

Баланс (залогиненный):
  font: JetBrains Mono 600, 14px
  color: #00FF9F
  анимация countup при изменении
  skeleton пока грузится

Кнопка "Депозит": btn-primary (frost), subtle cyan glow на hover

Незалогиненный:
  "Войти" (text link) + "Регистрация" (btn-primary frost)

Mobile (< 1024px):
  Logo + Баланс + Burger menu
```

### 3.2 Mobile Bottom Nav (fixed, 60px)

```
┌──────────────────────────────────────────────┐
│  🏠 Главная  │  ⚽ Спорт  │  🎰 Казино  │  📋 Купон(3)  │  👤 Профиль  │
└──────────────────────────────────────────────┘

5 вкладок, grid-cols-5
height: 60px + safe-area-inset-bottom
bg: glass (backdrop-blur)
active: цветная иконка + glow-эффект снизу
inactive: text-gray-500
badge на "Купон": количество ставок, bg #F8FAFC, text #020617, font 11px

Показывать ТОЛЬКО на mobile (< 1024px)
```

### 3.3 Footer

```
4 колонки: О платформе | Игры | Поддержка | Информация
Логотипы платёжных систем (Visa, MC, BTC, ETH, USDT)
Бейдж 18+ (обязательно)
Лицензия (текст + номер)
Ссылка "Ответственная игра" (требование UKGC/MGA)
© 2025 OPUS Casino

bg: --bg-secondary, border-top, padding 48px 0
text: 12-13px, --text-muted
```

---

## 4. PERFORMANCE BUDGET

```
FCP:  < 1.5s     (критично > 3s)
LCP:  < 2.5s     (критично > 4s)
FID:  < 100ms    (критично > 300ms)
CLS:  < 0.1      (критично > 0.25)
TTI:  < 3s       (критично > 5s)
JS bundle (gzip): < 300KB initial
Images first screen: < 500KB
WebSocket latency:   < 200ms
API p95:             < 300ms

Изображения: WebP + JPEG fallback, lazy load, responsive (150/300/600px), blur placeholder
JS: code split по маршрутам (/casino, /sports), dynamic import для модалок
Шрифты: preload, WOFF2, subset
```

---

## 5. ACCESSIBILITY

```
Контраст: 4.5:1 для текста, 3:1 для крупного (18px+) — WCAG 2.2 AA
Touch targets: 44×44px минимум (WCAG 2.5.8)
Focus ring: 2px solid #67E8F9 + offset 2px  ← cyan/frost, НЕ зелёный (зелёный = FINANCIAL_POSITIVE)
Focus appearance: видимый на ВСЕХ фонах (WCAG 2.4.11)
Focus trap: в модальных окнах
Keyboard: Tab, Enter/Space, Escape, Arrow Keys

ARIA:
  aria-label для иконок без текста
  aria-live="polite" для обновления баланса
  aria-live="assertive" для ошибок
  role="alert" для критических уведомлений
  Кэфы: aria-label="Победа Арсенала, коэффициент 2.10"
  Live event: aria-label="Live, 67 минута, счёт 2:1, Арсенал в атаке"
  Odds change: aria-live="polite" объявляет "Кэф изменился: 2.10 → 1.95"

COLOR BLINDNESS:
  Кэф up/down: цвет + arrow icon (↑↓) — НЕ только цвет
  Win/Loss в истории: + иконка (✓ / ✗) — НЕ только цвет
  Live indicator: цвет + pulsing dot + текст "LIVE"
  Тестировать: Chrome DevTools Rendering → Emulate vision deficiencies

HIGH CONTRAST MODE (Windows / iOS):
  Использовать system colors через CSS Forced Colors API
  Кнопки: button { forced-color-adjust: none; } для критических CTA
  Тестирование: macOS "Increase Contrast" + Windows High Contrast

СКРИНРИДЕРЫ:
  Тестирование: VoiceOver (macOS), NVDA (Windows), TalkBack (Android)
  Casino iframe: НЕ accessible (provider-controlled) — предупредить в alt-text

КОМПОНЕНТНЫЕ МОДЕЛИ:

  Bottom Sheet (BetSlip mobile):
    role="dialog", aria-modal="true"
    focus trap: Tab циклит внутри
    Escape: закрыть
    focus return: на элемент, открывший sheet
    drag handle: aria-label="Перетащите для изменения размера"

  Cashout Slider:
    role="slider", aria-valuemin, aria-valuemax, aria-valuenow
    aria-label="Сумма кэшаута"
    Keyboard: ←→ step 100₽, Home/End min/max

  Live Wins Ticker:
    role="log", aria-live="off" (по умолчанию)
    Кнопка "Пауза": aria-label="Приостановить ленту выигрышей"
    При паузе: aria-live="polite", статичный список

  Casino Iframe Overlay:
    role="dialog" при появлении ошибки/reconnect
    focus trap внутри overlay
    Escape: вернуться в лобби
    focus return: на карточку игры

  Long Press (game info):
    Альтернатива: visible ℹ️ кнопка для keyboard/mouse
    Touch: 500ms long press → info panel
    Keyboard: Shift+Enter на карточке → info panel

  Toast Notifications:
    role="status" для информационных
    role="alert" для ошибок
    Финансовые ошибки: ТАКЖЕ отображаются inline в компоненте
    Auto-dismiss: 5s info, 8s warning, manual close для errors
```
