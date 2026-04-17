## #49 metrics-instrumentation.skill.md

```markdown
# metrics-instrumentation.skill.md

## РОЛЬ
Ты инструментируешь микросервисы метриками для мониторинга
гемблинг-платформы. VictoriaMetrics + Grafana.

## КОНТЕКСТ
- Метрики: Prometheus-совместимый формат
- Сбор: VictoriaMetrics (pull model, /metrics endpoint)
- Дашборды: Grafana
- Алерты: Grafana Alerting → PagerDuty
- Naming: prometheus naming conventions

## METRIC TYPES
Counter: только растёт (запросы, ошибки, ставки)
Gauge: может расти/падать (active connections, queue size)
Histogram: распределение значений (latency, bet size)
Summary: как histogram, но с percentiles на клиенте
(используй histogram, не summary)

text


## NAMING CONVENTIONS
{namespace}{subsystem}{name}_{unit}

namespace = service name (betting, wallet, auth)
subsystem = component (http, grpc, db, cache)
name = что измеряем
unit = _total, _seconds, _bytes, _ratio

Примеры:
betting_http_requests_total
betting_http_request_duration_seconds
wallet_grpc_requests_total
wallet_db_query_duration_seconds
auth_login_attempts_total
auth_active_sessions_gauge

text


## RUST METRICS

```rust
// Rust — prometheus metrics с metrics crate
use metrics::{counter, gauge, histogram, describe_counter, 
              describe_gauge, describe_histogram};
use metrics_exporter_prometheus::PrometheusBuilder;

pub fn init_metrics() -> PrometheusHandle {
    let builder = PrometheusBuilder::new();
    let handle = builder
        .install_recorder()
        .expect("Failed to install metrics recorder");

    // Описания метрик
    describe_counter!(
        "betting_bets_placed_total",
        "Total number of bets placed"
    );
    describe_counter!(
        "betting_bets_settled_total",
        "Total number of bets settled"
    );
    describe_histogram!(
        "betting_bet_placement_duration_seconds",
        "Time to place a bet"
    );
    describe_histogram!(
        "betting_bet_stake_amount",
        "Distribution of bet stake amounts"
    );
    describe_gauge!(
        "betting_active_bets_count",
        "Number of currently active (unsettled) bets"
    );
    describe_counter!(
        "betting_http_requests_total",
        "Total HTTP requests"
    );
    describe_histogram!(
        "betting_http_request_duration_seconds",
        "HTTP request duration"
    );

    handle
}

// Metrics endpoint для VictoriaMetrics scraping
async fn metrics_handler(
    State(handle): State<PrometheusHandle>,
) -> String {
    handle.render()
}

// Использование в бизнес-логике
pub async fn place_bet(&self, req: PlaceBetRequest) -> Result<Bet> {
    let start = Instant::now();

    let result = self.inner_place_bet(req).await;

    let duration = start.elapsed().as_secs_f64();
    let status = if result.is_ok() { "success" } else { "error" };

    // Метрики
    counter!("betting_bets_placed_total", "status" => status, 
             "bet_type" => req.bet_type.to_string());
    histogram!("betting_bet_placement_duration_seconds", duration,
               "status" => status);

    if let Ok(ref bet) = result {
        histogram!("betting_bet_stake_amount", bet.stake as f64,
                   "currency" => bet.currency.clone());
        gauge!("betting_active_bets_count", 1.0, "increment" => "true");
    }

    result
}
HTTP MIDDLEWARE METRICS
Rust

// Rust — Axum middleware для HTTP метрик
pub async fn metrics_middleware(
    request: Request,
    next: Next,
) -> Response {
    let method = request.method().to_string();
    let path = request.uri().path().to_string();
    // Нормализация пути (убрать ID)
    let route = normalize_path(&path);

    let start = Instant::now();
    let response = next.run(request).await;
    let duration = start.elapsed().as_secs_f64();
    let status = response.status().as_u16().to_string();

    counter!(
        "betting_http_requests_total",
        "method" => method.clone(),
        "route" => route.clone(),
        "status" => status.clone()
    );

    histogram!(
        "betting_http_request_duration_seconds",
        duration,
        "method" => method,
        "route" => route,
        "status" => status
    );

    response
}

fn normalize_path(path: &str) -> String {
    // /api/v1/bets/12345 → /api/v1/bets/:id
    // /api/v1/users/67890/sessions → /api/v1/users/:id/sessions
    let re = regex::Regex::new(r"/\d+").unwrap();
    re.replace_all(path, "/:id").to_string()
}
GO METRICS
Go

// Go — prometheus metrics
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "auth",
            Subsystem: "http",
            Name:      "requests_total",
            Help:      "Total HTTP requests",
        },
        []string{"method", "route", "status"},
    )

    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "auth",
            Subsystem: "http",
            Name:      "request_duration_seconds",
            Help:      "HTTP request duration",
            Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5},
        },
        []string{"method", "route", "status"},
    )

    loginAttemptsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "auth",
            Name:      "login_attempts_total",
            Help:      "Total login attempts",
        },
        []string{"status", "method"},  // success/failure, password/2fa
    )

    activeSessionsGauge = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "auth",
            Name:      "active_sessions_count",
            Help:      "Currently active user sessions",
        },
    )

    dbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "auth",
            Subsystem: "db",
            Name:      "query_duration_seconds",
            Help:      "Database query duration",
            Buckets:   []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
        },
        []string{"query", "status"},
    )
)

// Middleware
func MetricsMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        start := time.Now()

        err := c.Next()

        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Response().StatusCode())
        method := c.Method()
        route := c.Route().Path  // уже нормализован Fiber

        httpRequestsTotal.WithLabelValues(method, route, status).Inc()
        httpRequestDuration.WithLabelValues(method, route, status).Observe(duration)

        return err
    }
}

// Metrics endpoint
app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
BUSINESS METRICS
Rust

// Бизнес-метрики — самое ценное для платформы
describe_counter!("platform_deposits_total", "Total deposits");
describe_counter!("platform_withdrawals_total", "Total withdrawals");
describe_histogram!("platform_deposit_amount", "Deposit amounts");
describe_gauge!("platform_active_users", "Currently active users");
describe_counter!("platform_revenue_total", "Gross Gaming Revenue");
describe_gauge!("platform_withdrawal_queue_depth", "Pending withdrawals");
describe_counter!("platform_fraud_blocks_total", "Fraud-blocked actions");
describe_histogram!("platform_casino_rtp", "Casino RTP deviation");

// Пример: трекинг GGR
pub fn track_bet_settled(bet: &Bet) {
    let sport = bet.sport.as_deref().unwrap_or("unknown");

    counter!("betting_bets_settled_total",
        "result" => bet.status.to_string(),
        "sport" => sport.to_string()
    );

    if bet.status == BetStatus::Lost {
        // Проигрыш пользователя = доход платформы
        counter!("platform_revenue_total",
            bet.stake as f64,
            "source" => "sports",
            "sport" => sport.to_string()
        );
    } else if bet.status == BetStatus::Won {
        let payout = bet.actual_win - bet.stake;
        if payout > 0.0 {
            // Большой выигрыш — counter отрицательный дохода
        }
    }

    gauge!("betting_active_bets_count", -1.0);
}
HISTOGRAM BUCKETS
Rust

// Выбор buckets зависит от SLO

// HTTP latency (цель: p99 < 100ms)
Buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0]
//        1ms    5ms   10ms  25ms   50ms  100ms 250ms 500ms 1s    5s

// DB query latency (цель: p99 < 10ms)
Buckets: [0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5]
//        0.5ms   1ms   2.5ms   5ms   10ms  25ms   50ms  100ms 500ms

// Bet amount (business metric)
Buckets: [1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000, 50000]

// WebSocket message latency (цель: < 20ms)
Buckets: [0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1]
GOLDEN SIGNALS
text

Для каждого сервиса мониторим 4 golden signals:

1. LATENCY (время ответа)
   metric: {service}_http_request_duration_seconds
   alert:  p99 > SLO target для > 5 минут

2. TRAFFIC (throughput)
   metric: {service}_http_requests_total (rate)
   alert:  резкое падение > 50% → что-то сломалось

3. ERRORS (ошибки)
   metric: {service}_http_requests_total{status=~"5.."}
   alert:  error_rate > 1% для > 2 минут

4. SATURATION (насыщенность)
   metrics: CPU usage, memory usage, connection pool, queue depth
   alert:  > 80% для > 10 минут
CARDINALITY RULES
text

Labels (tags) должны иметь LOW cardinality:

✅ ПРАВИЛЬНО (low cardinality):
  method:  GET, POST, PUT, DELETE       (4 значения)
  status:  200, 201, 400, 404, 500      (~10 значений)
  sport:   football, basketball, ...     (~50 значений)
  country: US, GB, DE, ...              (~200 значений)

❌ ПЛОХО (high cardinality):
  user_id:  12345, 67890, ...           (миллионы!)
  bet_id:   1, 2, 3, ...               (миллиарды!)
  email:    user@example.com            (миллионы!)
  ip:       1.2.3.4                     (миллионы!)

High cardinality labels → VictoriaMetrics OOM или extreme cost
Правило: < 1000 уникальных значений на label
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: user_id как label
counter!("bets_total", "user_id" => user_id.to_string());
// Миллионы time series → OOM

// ✅ ПРАВИЛЬНО: user_id в логах, не в метриках
counter!("bets_total", "sport" => sport, "status" => status);
tracing::info!(user_id = user_id, "Bet placed");

// ❌ ПЛОХО: метрики внутри tight loop
for item in &items {
    histogram!("processing_time", item.duration);
}
// 1M записей → 1M histogram.observe() за раз

// ✅ ПРАВИЛЬНО: batch метрика
histogram!("batch_processing_time", total_duration);
counter!("batch_items_processed_total", items.len() as f64);

// ❌ ПЛОХО: нет unit suffix
"request_duration"      // секунды? миллисекунды?

// ✅ ПРАВИЛЬНО:
"request_duration_seconds"

// ❌ ПЛОХО: snake_case нарушение
"requestDuration"       // camelCase
"Request-Duration"      // kebab-case

// ✅ ПРАВИЛЬНО:
"request_duration_seconds"  // snake_case всегда