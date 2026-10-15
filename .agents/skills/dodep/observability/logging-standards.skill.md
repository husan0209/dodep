## #48 logging-standards.skill.md

```markdown
# logging-standards.skill.md

## РОЛЬ
Ты определяешь стандарты логирования для всех сервисов
гемблинг-платформы. Логи — основа дебага и аудита.

## КОНТЕКСТ
- Формат: structured JSON
- Сбор: Vector (DaemonSet) → ClickHouse
- Корреляция: trace_id + span_id (OpenTelemetry)
- Retention: 30 дней raw, 1 год aggregated
- Sensitive data: НИКОГДА не логировать

## LOG FORMAT

```json
{
  "timestamp": "2025-01-15T10:30:45.123Z",
  "level": "info",
  "service": "betting-engine",
  "version": "v1.2.3",
  "instance": "betting-engine-7f8b9c-x2k4m",
  "trace_id": "abc123def456",
  "span_id": "span789",
  "request_id": "req_abc123",
  "user_id": 12345,
  "message": "Bet placed successfully",
  "fields": {
    "bet_id": 67890,
    "stake": 50.00,
    "odds": 2.15,
    "bet_type": "single",
    "duration_ms": 4.2
  }
}
RUST LOGGING SETUP
Rust

// main.rs — tracing setup
use tracing_subscriber::{
    fmt, layer::SubscriberExt, util::SubscriberInitExt, EnvFilter,
};

pub fn init_tracing() {
    let env_filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new("info,betting_engine=debug"));

    let fmt_layer = fmt::layer()
        .json()                          // JSON формат
        .with_target(true)               // module path
        .with_thread_ids(false)
        .with_file(false)                // не нужен в production
        .with_line_number(false)
        .with_current_span(true)
        .flatten_event(true);            // flatten вложенные поля

    // OpenTelemetry layer для distributed tracing
    let otel_layer = tracing_opentelemetry::layer()
        .with_tracer(init_otel_tracer());

    tracing_subscriber::registry()
        .with(env_filter)
        .with(fmt_layer)
        .with(otel_layer)
        .init();
}

fn init_otel_tracer() -> opentelemetry::sdk::trace::Tracer {
    opentelemetry_otlp::new_pipeline()
        .tracing()
        .with_exporter(
            opentelemetry_otlp::new_exporter()
                .tonic()
                .with_endpoint(&std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
                    .unwrap_or_else(|_| "http://otel-collector:4317".into())),
        )
        .with_trace_config(
            opentelemetry::sdk::trace::config()
                .with_resource(opentelemetry::sdk::Resource::new(vec![
                    opentelemetry::KeyValue::new("service.name", "betting-engine"),
                    opentelemetry::KeyValue::new("service.version", env!("CARGO_PKG_VERSION")),
                ]))
                .with_sampler(opentelemetry::sdk::trace::Sampler::TraceIdRatioBased(0.1)),
        )
        .install_batch(opentelemetry::runtime::Tokio)
        .expect("Failed to init tracer")
}
LOG LEVELS
text

FATAL: Сервис не может продолжать работу
  → Невозможно подключиться к БД при старте
  → Критическая ошибка конфигурации
  → Действие: PagerDuty P1 alert, немедленная реакция

ERROR: Операция не выполнена, пользователь затронут
  → Не удалось разместить ставку
  → Payment provider вернул ошибку
  → Действие: алерт, расследование в течение часа

WARN: Потенциальная проблема, система работает
  → Rate limit сработал
  → Retry запроса к внешнему сервису
  → Cache miss rate > порога
  → Действие: мониторинг, расследование если повторяется

INFO: Значимое бизнес-событие
  → Пользователь зарегистрировался
  → Ставка размещена
  → Депозит выполнен
  → Действие: нет (нормальная работа)

DEBUG: Детали для разработки
  → SQL запросы
  → Cache hit/miss
  → Входные параметры функций
  → Действие: включается только при расследовании

TRACE: Максимальная детализация
  → НЕ ВКЛЮЧАТЬ в production (огромный объём)
  → Только для локальной разработки
ПРАВИЛА ЛОГИРОВАНИЯ
Что логировать
Rust

// ✅ Бизнес-события
tracing::info!(
    user_id = user.id,
    bet_id = bet.id,
    stake = %bet.stake,
    odds = %bet.odds,
    bet_type = %bet.bet_type,
    duration_ms = elapsed.as_millis() as u64,
    "Bet placed successfully"
);

// ✅ Ошибки с контекстом
tracing::error!(
    user_id = user.id,
    error = %err,
    request_id = %req_id,
    "Failed to place bet"
);

// ✅ Performance метки
tracing::info!(
    query = "get_user_balance",
    duration_ms = elapsed.as_millis() as u64,
    cache_hit = cache_hit,
    "Database query completed"
);

// ✅ Security events
tracing::warn!(
    user_id = user.id,
    ip = %ip,
    reason = "multiple_failed_logins",
    attempts = failed_count,
    "Suspicious login activity"
);
Что НЕ логировать
Rust

// ❌ НИКОГДА: пароли
tracing::info!(password = req.password, "Login attempt");

// ❌ НИКОГДА: токены
tracing::info!(token = access_token, "Token generated");

// ❌ НИКОГДА: полные номера карт
tracing::info!(card = "4242424242424242", "Payment");

// ❌ НИКОГДА: KYC документы
tracing::info!(document = base64_doc, "KYC uploaded");

// ❌ НИКОГДА: response body целиком
tracing::debug!(body = %serde_json::to_string(&response)?, "Response");

// ✅ Маскированная версия если нужно
tracing::info!(
    email = mask_email(&user.email),  // t***@example.com
    card_last4 = &card[card.len()-4..],  // 4242
    "Payment processed"
);
GO LOGGING
Go

// Go — structured logging с slog
import "log/slog"

func InitLogger(service, version string) *slog.Logger {
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })

    logger := slog.New(handler).With(
        slog.String("service", service),
        slog.String("version", version),
    )

    slog.SetDefault(logger)
    return logger
}

// Использование
func (s *BetService) PlaceBet(ctx context.Context, req *PlaceBetRequest) (*Bet, error) {
    logger := slog.With(
        slog.Int64("user_id", req.UserID),
        slog.String("request_id", middleware.GetRequestID(ctx)),
        slog.String("trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID().String()),
    )

    start := time.Now()

    bet, err := s.repo.CreateBet(ctx, req)
    if err != nil {
        logger.Error("Failed to place bet",
            slog.String("error", err.Error()),
            slog.Float64("stake", req.Stake),
        )
        return nil, err
    }

    logger.Info("Bet placed successfully",
        slog.Int64("bet_id", bet.ID),
        slog.Float64("stake", req.Stake),
        slog.Float64("odds", bet.Odds),
        slog.Duration("duration", time.Since(start)),
    )

    return bet, nil
}
REQUEST LOGGING MIDDLEWARE
Rust

// Rust — Axum request logging middleware
pub async fn request_logging(
    request: Request,
    next: Next,
) -> Response {
    let method = request.method().clone();
    let uri = request.uri().clone();
    let request_id = request.headers()
        .get("x-request-id")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("unknown")
        .to_string();

    let user_id = request.extensions()
        .get::<AuthUser>()
        .map(|u| u.id);

    let start = Instant::now();
    let response = next.run(request).await;
    let duration = start.elapsed();
    let status = response.status().as_u16();

    if status >= 500 {
        tracing::error!(
            method = %method,
            uri = %uri,
            status = status,
            duration_ms = duration.as_millis() as u64,
            request_id = %request_id,
            user_id = user_id,
            "Request failed"
        );
    } else if status >= 400 {
        tracing::warn!(
            method = %method,
            uri = %uri,
            status = status,
            duration_ms = duration.as_millis() as u64,
            request_id = %request_id,
            user_id = user_id,
            "Client error"
        );
    } else {
        tracing::info!(
            method = %method,
            uri = %uri,
            status = status,
            duration_ms = duration.as_millis() as u64,
            request_id = %request_id,
            user_id = user_id,
            "Request completed"
        );
    }

    response
}
MASKING HELPERS
Rust

pub fn mask_email(email: &str) -> String {
    match email.split_once('@') {
        Some((local, domain)) => {
            let masked = if local.len() <= 2 {
                "***".to_string()
            } else {
                format!("{}***", &local[..1])
            };
            format!("{}@{}", masked, domain)
        }
        None => "***".to_string(),
    }
}

pub fn mask_phone(phone: &str) -> String {
    if phone.len() <= 4 {
        return "***".to_string();
    }
    format!("{}***{}", &phone[..3], &phone[phone.len()-4..])
}

pub fn mask_card(card: &str) -> String {
    if card.len() < 4 {
        return "****".to_string();
    }
    format!("****{}", &card[card.len()-4..])
}
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: println! вместо tracing
println!("User logged in: {}", user_id);

// ✅ ПРАВИЛЬНО:
tracing::info!(user_id = user_id, "User logged in");

// ❌ ПЛОХО: неструктурированные логи
tracing::info!("User {} placed bet {} for ${} at odds {}", 
    user_id, bet_id, stake, odds);

// ✅ ПРАВИЛЬНО: структурированные
tracing::info!(
    user_id = user_id,
    bet_id = bet_id,
    stake = %stake,
    odds = %odds,
    "Bet placed"
);

// ❌ ПЛОХО: логировать в цикле
for bet in &bets {
    tracing::info!(bet_id = bet.id, "Processing bet");
}

// ✅ ПРАВИЛЬНО: одна запись для batch
tracing::info!(
    count = bets.len(),
    bet_ids = ?bets.iter().map(|b| b.id).collect::<Vec<_>>(),
    "Processing bet batch"
);

// ❌ ПЛОХО: debug логи в production по умолчанию
// Огромный объём, забивает ClickHouse

// ✅ ПРАВИЛЬНО: info по умолчанию, debug через env
// RUST_LOG=info,betting_engine=debug