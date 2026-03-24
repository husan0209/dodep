#50 tracing-opentelemetry.skill.md
Markdown

# tracing-opentelemetry.skill.md

## РОЛЬ
Ты инструментируешь distributed tracing через OpenTelemetry
для всех сервисов гемблинг-платформы.

## КОНТЕКСТ
- OpenTelemetry SDK для Rust, Go, Python, JavaScript
- Collector: OpenTelemetry Collector (DaemonSet + Gateway)
- Backend: Jaeger (с ClickHouse storage)
- Sampling: 100% errors, 10% normal traffic
- Propagation: W3C Trace Context (traceparent header)

## TRACE ARCHITECTURE
┌──────────────────────────────────────────────────────────┐
│ TRACE FLOW │
│ │
│ Client → CloudFlare → API Gateway → Service A → Service B│
│ │ │ │ │ │ │
│ │ traceparent traceparent traceparent traceparent│
│ │ │ │ │ │ │
│ └──────────┴────────────┴────────────┴───────────┘ │
│ │ │
│ OpenTelemetry Collector │
│ │ │
│ Jaeger │
│ (ClickHouse) │
└──────────────────────────────────────────────────────────┘

Один trace = одна пользовательская операция (e.g. place bet)
Содержит spans от каждого сервиса, через который прошёл запрос

text


## RUST TRACING SETUP

```rust
use opentelemetry::global;
use opentelemetry::sdk::trace::{self, Sampler, TracerProvider};
use opentelemetry::sdk::Resource;
use opentelemetry::KeyValue;
use opentelemetry_otlp::WithExportConfig;
use tracing_opentelemetry::OpenTelemetryLayer;
use tracing_subscriber::{layer::SubscriberExt, Registry};

pub fn init_tracing(service_name: &str, version: &str) {
    let exporter = opentelemetry_otlp::new_exporter()
        .tonic()
        .with_endpoint(
            std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
                .unwrap_or_else(|_| "http://otel-collector:4317".into()),
        );

    let tracer = opentelemetry_otlp::new_pipeline()
        .tracing()
        .with_exporter(exporter)
        .with_trace_config(
            trace::config()
                .with_resource(Resource::new(vec![
                    KeyValue::new("service.name", service_name.to_string()),
                    KeyValue::new("service.version", version.to_string()),
                    KeyValue::new("deployment.environment",
                        std::env::var("ENVIRONMENT").unwrap_or_else(|_| "dev".into())),
                ]))
                .with_sampler(Sampler::ParentBased(Box::new(
                    Sampler::TraceIdRatioBased(0.1), // 10% sampling
                ))),
        )
        .install_batch(opentelemetry::runtime::Tokio)
        .expect("Failed to init tracer");

    let otel_layer = OpenTelemetryLayer::new(tracer);

    let subscriber = Registry::default()
        .with(tracing_subscriber::fmt::layer().json())
        .with(tracing_subscriber::EnvFilter::from_default_env())
        .with(otel_layer);

    tracing::subscriber::set_global_default(subscriber)
        .expect("Failed to set subscriber");
}

pub fn shutdown_tracing() {
    global::shutdown_tracer_provider();
}
SPAN CREATION
Rust

// Rust — создание spans
use tracing::{instrument, Span};

// Автоматический span через #[instrument]
#[instrument(
    name = "place_bet",
    skip(self, pool),
    fields(
        user_id = %req.user_id,
        bet_type = %req.bet_type,
        stake = %req.stake,
        otel.kind = "server",
    )
)]
pub async fn place_bet(
    &self,
    pool: &PgPool,
    req: PlaceBetRequest,
) -> Result<Bet> {
    // Дочерний span для проверки odds
    let odds = self.check_odds(&req.selections).await?;

    // Дочерний span для risk check
    let risk_ok = self.risk_check(&req, &odds).await?;

    // Дочерний span для wallet debit
    let tx = self.wallet_debit(req.user_id, req.stake).await?;

    // Записать результат в текущий span
    Span::current().record("bet_id", bet.id);
    Span::current().record("total_odds", odds.total);

    Ok(bet)
}

// Ручной span для детализации
async fn check_odds(&self, selections: &[Selection]) -> Result<OddsResult> {
    let span = tracing::info_span!(
        "check_odds",
        selections_count = selections.len(),
        cache_hit = tracing::field::Empty,
    );
    let _guard = span.enter();

    let cached = self.cache.get_odds(selections).await?;
    Span::current().record("cache_hit", cached.is_some());

    match cached {
        Some(odds) => Ok(odds),
        None => {
            let odds = self.odds_service.get_current(selections).await?;
            self.cache.set_odds(selections, &odds).await?;
            Ok(odds)
        }
    }
}

// Database span
#[instrument(
    name = "db.query",
    skip(pool),
    fields(
        db.system = "postgresql",
        db.statement = query,
        db.operation = "SELECT",
    )
)]
pub async fn get_user(pool: &PgPool, user_id: i64) -> Result<User> {
    sqlx::query_as!(User, "SELECT * FROM users WHERE id = $1", user_id)
        .fetch_one(pool)
        .await
        .map_err(Into::into)
}
GO TRACING SETUP
Go

package tracing

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
    "go.opentelemetry.io/otel/trace"
)

func InitTracer(ctx context.Context, serviceName, version string) (func(), error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    res, _ := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(version),
        ),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.ParentBased(
            sdktrace.TraceIDRatioBased(0.1),
        )),
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})

    return func() { tp.Shutdown(ctx) }, nil
}

// Использование в сервисе
var tracer = otel.Tracer("auth-service")

func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
    ctx, span := tracer.Start(ctx, "auth.login",
        trace.WithAttributes(
            attribute.String("auth.method", "password"),
            attribute.String("user.email_hash", hashEmail(req.Email)),
        ),
    )
    defer span.End()

    // Дочерний span для проверки пароля
    ctx, passSpan := tracer.Start(ctx, "auth.verify_password")
    valid, err := s.verifyPassword(ctx, req.Email, req.Password)
    passSpan.End()

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    // Дочерний span для генерации токена
    ctx, tokenSpan := tracer.Start(ctx, "auth.generate_tokens")
    tokens, err := s.generateTokens(ctx, user)
    tokenSpan.End()

    span.SetAttributes(
        attribute.Bool("auth.success", true),
        attribute.Bool("auth.2fa_required", user.TwoFAEnabled),
    )

    return tokens, nil
}
CONTEXT PROPAGATION
Rust

// Rust — propagation через HTTP headers
// Axum middleware автоматически extract/inject traceparent

// gRPC — tonic interceptor
pub fn tracing_interceptor(mut req: tonic::Request<()>) -> Result<tonic::Request<()>, Status> {
    let cx = Span::current().context();
    global::get_text_map_propagator(|propagator| {
        propagator.inject_context(&cx, &mut MetadataMap(req.metadata_mut()));
    });
    Ok(req)
}

// Redpanda — inject trace context в headers
pub fn inject_trace_to_headers(headers: &mut OwnedHeaders) {
    let cx = Span::current().context();
    global::get_text_map_propagator(|propagator| {
        let mut carrier = HashMap::new();
        propagator.inject_context(&cx, &mut carrier);
        for (key, value) in carrier {
            headers = headers.insert(Header {
                key: &key,
                value: Some(&value),
            });
        }
    });
}
Go

// Go — HTTP propagation middleware
func TracingMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Extract trace context from incoming headers
        ctx := otel.GetTextMapPropagator().Extract(
            c.Context(),
            propagation.HeaderCarrier(c.GetReqHeaders()),
        )

        ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", c.Method(), c.Route().Path),
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()

        c.SetUserContext(ctx)

        err := c.Next()

        span.SetAttributes(
            attribute.Int("http.status_code", c.Response().StatusCode()),
        )

        if err != nil || c.Response().StatusCode() >= 500 {
            span.RecordError(err)
            span.SetStatus(codes.Error, "request failed")
        }

        return err
    }
}
OTEL COLLECTOR CONFIG
YAML

# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1000
    send_batch_max_size: 2000

  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128

  # Tail sampling: 100% errors, 10% normal
  tail_sampling:
    decision_wait: 10s
    num_traces: 50000
    policies:
      - name: errors
        type: status_code
        status_code:
          status_codes: [ERROR]
      - name: slow-requests
        type: latency
        latency:
          threshold_ms: 1000
      - name: probabilistic
        type: probabilistic
        probabilistic:
          sampling_percentage: 10

  # Добавить атрибуты
  attributes:
    actions:
      - key: environment
        value: production
        action: upsert
      - key: cluster
        value: eu-west-1
        action: upsert

exporters:
  otlp/jaeger:
    endpoint: jaeger-collector:4317
    tls:
      insecure: true

  # Span metrics → VictoriaMetrics
  prometheus:
    endpoint: 0.0.0.0:8889
    metric_expiration: 5m

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, tail_sampling, attributes, batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
SPAN NAMING CONVENTIONS
text

HTTP spans:
  "HTTP {METHOD} {route}"
  "HTTP GET /api/v1/bets/:id"
  "HTTP POST /api/v1/bets"

gRPC spans:
  "{package}.{Service}/{Method}"
  "wallet.v1.WalletService/Debit"

Database spans:
  "db.{operation}"
  "db.query SELECT users"
  "db.query INSERT bets"

Cache spans:
  "cache.{operation}"
  "cache.get odds:{event_id}"
  "cache.set session:{user_id}"

Event spans:
  "event.{action}"
  "event.publish bets.bet.placed"
  "event.consume bets.bet.settled"

External service spans:
  "external.{service}.{operation}"
  "external.sportradar.get_odds"
  "external.stripe.create_payment"
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: слишком много spans (span per loop iteration)
for bet in &bets {
    let span = tracing::info_span!("process_bet", bet_id = bet.id);
    let _guard = span.enter();
    process(bet);
}
// 10000 ставок → 10000 spans → overhead

// ✅ ПРАВИЛЬНО: один span для batch
let span = tracing::info_span!("process_bets_batch", count = bets.len());
let _guard = span.enter();
for bet in &bets {
    process(bet);
}

// ❌ ПЛОХО: sensitive data в span attributes
span.set_attribute(KeyValue::new("user.email", email));
span.set_attribute(KeyValue::new("user.password", password));

// ✅ ПРАВИЛЬНО: только IDs и non-sensitive data
span.set_attribute(KeyValue::new("user.id", user_id));

// ❌ ПЛОХО: 100% sampling в production
Sampler::AlwaysOn  // огромный объём данных

// ✅ ПРАВИЛЬНО: 10% normal + 100% errors
Sampler::ParentBased(Sampler::TraceIdRatioBased(0.1))

// ❌ ПЛОХО: не propagate context между сервисами
// Каждый сервис создаёт отдельный trace

// ✅ ПРАВИЛЬНО: всегда propagate через headers
// HTTP: traceparent header
// gRPC: metadata
// Redpanda: record headers