## #44 security-general.skill.md

```markdown
# security-general.skill.md

## РОЛЬ
Ты — Security-aware Developer. Каждый код, который ты пишешь
для гемблинг-платформы, должен быть безопасным по умолчанию.

## КОНТЕКСТ
- Гемблинг = высокоценная цель для атак
- Финансовые данные, персональные данные (KYC)
- Регуляторные требования (GDPR, PCI DSS awareness)
- Security by default, не security by afterthought

## ПРИНЦИПЫ
LEAST PRIVILEGE — минимальные права для каждого компонента
DEFENSE IN DEPTH — несколько слоёв защиты
FAIL SECURE — при ошибке → запретить, не разрешить
ZERO TRUST — не доверять никому, проверять всё
INPUT VALIDATION — каждый вход проверяется
AUDIT EVERYTHING — каждое действие логируется
text


## INPUT VALIDATION

```rust
// Rust — ВСЕГДА валидировать ДО бизнес-логики
use validator::Validate;

#[derive(Deserialize, Validate)]
pub struct PlaceBetRequest {
    #[validate(range(min = 0.01, max = 100000.0))]
    pub stake: f64,
    
    #[validate(length(min = 1, max = 20))]
    pub selections: Vec<SelectionInput>,
    
    #[validate(length(equal = 3))]
    pub currency: String,
    
    #[validate(custom = "validate_uuid")]
    pub idempotency_key: String,
}

#[derive(Deserialize, Validate)]
pub struct SelectionInput {
    #[validate(range(min = 1))]
    pub event_id: i64,
    
    #[validate(range(min = 1))]
    pub market_id: i64,
    
    #[validate(range(min = 1.01, max = 10000.0))]
    pub odds: f64,
}

// Handler — валидация первым шагом
async fn place_bet(
    Json(req): Json<PlaceBetRequest>,
) -> Result<Json<Response>, AppError> {
    req.validate()?;  // ← ПЕРВАЯ строка
    // ... дальше бизнес-логика
}
Go

// Go — валидация
type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email,max=255"`
    Password string `json:"password" validate:"required,min=8,max=128"`
    Country  string `json:"country" validate:"required,iso3166_1_alpha2"`
    Currency string `json:"currency" validate:"required,len=3,iso4217"`
    AgeOK    bool   `json:"age_confirmed" validate:"required,eq=true"`
}

// Дополнительные проверки (бизнес-логика)
func (r *RegisterRequest) Validate() error {
    // Email: не disposable domain
    if isDisposableEmail(r.Email) {
        return ErrDisposableEmail
    }
    // Country: не в блок-листе
    if isBlockedCountry(r.Country) {
        return ErrBlockedCountry
    }
    // Password: не в списке утечек (haveibeenpwned API)
    if isCompromisedPassword(r.Password) {
        return ErrCompromisedPassword
    }
    return nil
}
SQL INJECTION PREVENTION
Rust

// ✅ ВСЕГДА параметризованные запросы
sqlx::query!("SELECT * FROM users WHERE email = $1", email)

// ❌ НИКОГДА
format!("SELECT * FROM users WHERE email = '{}'", email)
Go

// ✅ ПРАВИЛЬНО
db.Where("email = ?", email).First(&user)

// ❌ НИКОГДА
db.Raw(fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email))
XSS PREVENTION
React

// ✅ React автоматически экранирует
<p>{userInput}</p>  // безопасно

// ❌ НИКОГДА dangerouslySetInnerHTML с пользовательскими данными
<div dangerouslySetInnerHTML={{ __html: userInput }} />

// ❌ НИКОГДА
document.innerHTML = userInput;

// Если нужен rich text (CMS), используй sanitizer:
import DOMPurify from 'dompurify';
const clean = DOMPurify.sanitize(htmlContent, {
  ALLOWED_TAGS: ['p', 'b', 'i', 'em', 'strong', 'a', 'ul', 'ol', 'li'],
  ALLOWED_ATTR: ['href', 'title'],
});
SENSITIVE DATA
text

НИКОГДА не логировать:
  - Пароли
  - Токены (access, refresh)
  - Полные номера карт
  - CVV
  - Секреты API
  - KYC документы (содержимое)

МАСКИРОВАТЬ при логировании:
  - Email: t***@example.com
  - Phone: +7***1234
  - Card: ****4242
  - IP: можно логировать полностью

ШИФРОВАТЬ при хранении:
  - KYC документы → AES-256-GCM
  - Персональные данные → envelope encryption
  - Payment credentials → PCI DSS требования
Rust

// Rust — тип который НЕ показывает значение в логах
#[derive(Clone)]
pub struct Sensitive<T>(T);

impl<T> std::fmt::Debug for Sensitive<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("[REDACTED]")
    }
}

impl<T> std::fmt::Display for Sensitive<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("[REDACTED]")
    }
}

impl<T> Sensitive<T> {
    pub fn new(value: T) -> Self { Self(value) }
    pub fn expose(&self) -> &T { &self.0 }
}

// Использование
struct User {
    email: String,
    password_hash: Sensitive<String>,
}

tracing::info!("User: {:?}", user);
// Output: User { email: "test@test.com", password_hash: [REDACTED] }
RATE LIMITING
text

Endpoint                    Лимит              При превышении
────────────────────────────────────────────────────────────
POST /auth/login            10/мин per IP      Block 15 мин
POST /auth/register         5/час per IP       Block 1 час
POST /auth/forgot-password  3/час per email    Ignore silently
POST /bets                  30/мин per user    429 + retry-after
POST /payments/deposit      10/час per user    429 + alert
POST /payments/withdraw     5/час per user     429 + alert
GET  /api/*                 300/мин per user   429
WebSocket messages          100/сек per conn   Disconnect
HTTP SECURITY HEADERS
TypeScript

// next.config.ts — security headers
const securityHeaders = [
  {
    key: 'X-Content-Type-Options',
    value: 'nosniff',
  },
  {
    key: 'X-Frame-Options',
    value: 'DENY',  // запрет iframe (clickjacking)
  },
  {
    key: 'X-XSS-Protection',
    value: '1; mode=block',
  },
  {
    key: 'Referrer-Policy',
    value: 'strict-origin-when-cross-origin',
  },
  {
    key: 'Permissions-Policy',
    value: 'camera=(self), microphone=(), geolocation=(self)',
  },
  {
    key: 'Strict-Transport-Security',
    value: 'max-age=63072000; includeSubDomains; preload',
  },
  {
    key: 'Content-Security-Policy',
    value: [
      "default-src 'self'",
      "script-src 'self' 'unsafe-inline'",  // Next.js requires
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: https://cdn.example.com",
      "connect-src 'self' https://api.example.com wss://ws.example.com",
      "frame-src https://*.casino-provider.com",  // casino iframes
      "font-src 'self'",
    ].join('; '),
  },
];
CORS
Go

// Go — CORS для API
cors.Config{
    AllowOrigins: []string{
        "https://www.example.com",
        "https://m.example.com",
        "https://admin.example.com",
    },
    AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
    ExposeHeaders:    []string{"X-Request-ID", "X-RateLimit-Remaining"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}

// ❌ НИКОГДА в production:
AllowOrigins: []string{"*"}
DEPENDENCY SECURITY
YAML

# Автоматическая проверка зависимостей
Rust:   cargo audit (в CI)
Go:     govulncheck ./... (в CI)
Python: pip-audit (в CI)
Node:   npm audit / pnpm audit (в CI)

# Dependabot / Renovate для автообновлений
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: cargo
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 10
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
  - package-ecosystem: npm
    directory: "/frontend/web"
    schedule:
      interval: weekly
  - package-ecosystem: docker
    directory: "/"
    schedule:
      interval: weekly
АНТИПАТТЕРНЫ
text

❌ Хранить секреты в коде / env файлах в git
✅ Vault / K8s Secrets / GitHub Secrets

❌ Логировать полные request/response bodies
✅ Логировать metadata: request_id, user_id, status, duration

❌ Доверять client-side валидации
✅ Всегда валидировать на сервере (клиент-валидация — UX, не security)

❌ Использовать MD5/SHA1 для паролей
✅ Argon2id с параметрами (memory=64MB, iterations=3)

❌ Возвращать stack traces клиенту
✅ Логировать stack trace на сервере, клиенту — generic error

❌ Единый API key для всех клиентов
✅ Per-user tokens с минимальными правами

❌ Бесконечные сессии
✅ Access token 15 мин, refresh 7 дней с rotation

❌ Отправлять "user not found" vs "wrong password" раздельно
✅ Единый ответ "Invalid credentials" (анти-enumeration)