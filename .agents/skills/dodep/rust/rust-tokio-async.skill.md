# SKILL #8 — rust-tokio-async.skill.md

```markdown
# rust-tokio-async.skill.md
# GAMBLING PLATFORM — RUST TOKIO ASYNC PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

All Rust services run on Tokio multi-threaded runtime.
Target: 10K+ concurrent connections per instance.
RAM budget: 150-500 MB (no room for bloat).

# ============================================================
# SECTION 2: RUNTIME CONFIGURATION
# ============================================================

```rust
// main.rs — Tokio runtime setup
#[tokio::main]  // defaults: multi-threaded, worker_threads = CPU cores
async fn main() -> anyhow::Result<()> {
    // ...
}

// For fine-tuned control:
fn main() {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(4)              // match CPU cores
        .enable_all()
        .thread_name("platform-worker")
        .thread_stack_size(2 * 1024 * 1024) // 2MB stack
        .build()
        .expect("Failed to build runtime");
    
    runtime.block_on(async_main());
}
============================================================
SECTION 3: TASK SPAWNING RULES
============================================================
text

RULES:
  1. NEVER use std::thread::spawn (wrong runtime)
  2. ALWAYS use tokio::spawn for concurrent async work
  3. ALWAYS handle JoinHandle result (don't ignore panics)
  4. ALWAYS use structured concurrency where possible
  5. Use tokio::spawn_blocking for CPU-heavy or blocking I/O
  6. NEVER hold a MutexGuard across an .await point
  7. Set limits on concurrent spawned tasks (prevent unbounded growth)
Rust

// ✅ GOOD: Structured concurrency with tokio::join!
pub async fn get_bet_details(
    bet_repo: &BetRepo,
    user_repo: &UserRepo,
    bet_id: BetId,
    user_id: UserId,
) -> Result<BetDetails, AppError> {
    // Parallel fetches — both run concurrently
    let (bet, user) = tokio::try_join!(
        bet_repo.get_by_id(bet_id),
        user_repo.get_by_id(user_id),
    )?;
    
    Ok(BetDetails { bet, user })
}

// ✅ GOOD: Fan-out with concurrency limit
use futures::stream::{self, StreamExt};

pub async fn settle_bets(bets: Vec<Bet>) -> Vec<Result<(), SettlementError>> {
    stream::iter(bets)
        .map(|bet| async move {
            settle_single_bet(bet).await
        })
        .buffer_unordered(10)  // max 10 concurrent settlements
        .collect()
        .await
}

// ✅ GOOD: Spawn for fire-and-forget with error handling
pub async fn place_bet(&self, req: PlaceBetRequest) -> Result<Bet, AppError> {
    let bet = self.create_bet(req).await?;
    
    // Fire-and-forget: publish event (don't block response)
    let producer = self.producer.clone();
    let event = BetPlacedEvent::from(&bet);
    tokio::spawn(async move {
        if let Err(e) = producer.publish("bets.placed", &event).await {
            tracing::error!(error = %e, "Failed to publish bet event");
        }
    });
    
    Ok(bet)
}

// ✅ GOOD: CPU-heavy work on blocking thread pool
pub async fn calculate_risk_score(data: &RiskData) -> Result<f64, AppError> {
    let data = data.clone();
    tokio::task::spawn_blocking(move || {
        // CPU-intensive ML inference
        run_onnx_model(&data)
    })
    .await
    .map_err(|e| AppError::Internal(e.into()))?
}
============================================================
SECTION 4: CANCELLATION AND TIMEOUTS
============================================================
Rust

use tokio::time::{timeout, Duration};

// ✅ GOOD: Always set timeout on external calls
pub async fn call_risk_engine(&self, req: CheckBetRequest) -> Result<RiskResult, AppError> {
    timeout(Duration::from_secs(3), self.risk_client.check_bet(req))
        .await
        .map_err(|_| AppError::ServiceUnavailable("risk-engine timeout".into()))?
        .map_err(|e| AppError::ServiceUnavailable(format!("risk-engine: {e}")))
}

// ✅ GOOD: Graceful shutdown with drain
pub async fn run_server(listener: TcpListener, app: Router) {
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
        .expect("Server failed");
}

async fn shutdown_signal() {
    let ctrl_c = tokio::signal::ctrl_c();
    let mut sigterm = tokio::signal::unix::signal(
        tokio::signal::unix::SignalKind::terminate()
    ).expect("SIGTERM handler");
    
    tokio::select! {
        _ = ctrl_c => tracing::info!("Received Ctrl+C"),
        _ = sigterm.recv() => tracing::info!("Received SIGTERM"),
    }
    
    tracing::info!("Starting graceful shutdown (10s drain)...");
    // Axum will stop accepting new connections and drain existing ones
}

// ✅ GOOD: Select for racing operations
pub async fn get_odds_with_fallback(
    cache: &CacheClient,
    db: &PgPool,
    key: &str,
) -> Result<Odds, AppError> {
    tokio::select! {
        result = cache.get::<Odds>(key) => {
            match result {
                Ok(Some(odds)) => Ok(odds),
                _ => db_fallback(db, key).await,
            }
        }
        _ = tokio::time::sleep(Duration::from_millis(50)) => {
            // Cache too slow, fall back to DB
            tracing::warn!("Cache timeout, falling back to DB");
            db_fallback(db, key).await
        }
    }
}
============================================================
SECTION 5: CHANNELS AND SYNCHRONIZATION
============================================================
Rust

use tokio::sync::{mpsc, oneshot, watch, Semaphore};

// ── mpsc: Multiple producers, single consumer ──
// USE FOR: event processing pipeline, work queue

pub async fn start_settlement_worker(mut rx: mpsc::Receiver<SettlementJob>) {
    while let Some(job) = rx.recv().await {
        if let Err(e) = process_settlement(job).await {
            tracing::error!(error = %e, "Settlement failed");
        }
    }
}

// Sender side (bounded channel — backpressure)
let (tx, rx) = mpsc::channel::<SettlementJob>(1000); // max 1000 queued
tokio::spawn(start_settlement_worker(rx));
tx.send(job).await.map_err(|_| AppError::Internal("worker died".into()))?;

// ── Semaphore: Limit concurrent operations ──
// USE FOR: limiting concurrent DB connections, external API calls

static EXTERNAL_API_SEMAPHORE: Lazy<Semaphore> = Lazy::new(|| Semaphore::new(20));

pub async fn call_sportradar(&self, req: Request) -> Result<Response, AppError> {
    let _permit = EXTERNAL_API_SEMAPHORE
        .acquire()
        .await
        .map_err(|_| AppError::Internal("semaphore closed".into()))?;
    
    // Max 20 concurrent calls to Sportradar
    self.http_client.send(req).await
}

// ── watch: Single value broadcast ──
// USE FOR: config updates, feature flags

let (tx, rx) = watch::channel(AppConfig::default());
// Producer: tx.send(new_config)
// Consumer: let config = rx.borrow().clone();
============================================================
SECTION 6: COMMON ASYNC ANTI-PATTERNS
============================================================
Rust

// ❌ BAD: Holding MutexGuard across .await
let guard = mutex.lock().await;
do_async_work().await;  // ❌ guard held during await!
drop(guard);

// ✅ GOOD: Release before await
{
    let guard = mutex.lock().await;
    let data = guard.clone();
} // guard dropped here
do_async_work_with(data).await;

// ❌ BAD: std::thread::sleep in async context
std::thread::sleep(Duration::from_secs(1)); // ❌ blocks entire thread!

// ✅ GOOD: tokio::time::sleep
tokio::time::sleep(Duration::from_secs(1)).await;

// ❌ BAD: Unbounded channel (memory leak risk)
let (tx, rx) = mpsc::unbounded_channel(); // ❌ can grow forever

// ✅ GOOD: Bounded channel with backpressure
let (tx, rx) = mpsc::channel(1000); // blocks sender when full

// ❌ BAD: Spawning without tracking
for item in items {
    tokio::spawn(process(item)); // ❌ no error handling, no limit
}

// ✅ GOOD: Spawn with collection and limit
let semaphore = Arc::new(Semaphore::new(50));
let mut handles = vec![];
for item in items {
    let permit = semaphore.clone().acquire_owned().await?;
    handles.push(tokio::spawn(async move {
        let _permit = permit;
        process(item).await
    }));
}
for handle in handles {
    handle.await??; // propagate errors
}

// ❌ BAD: Blocking in async context
let hash = argon2::hash_password(password); // ❌ CPU-heavy blocks runtime

// ✅ GOOD: spawn_blocking for CPU work
let hash = tokio::task::spawn_blocking(move || {
    argon2::hash_password(password)
}).await??;
============================================================
SECTION 7: PERFORMANCE TIPS
============================================================
text

1. Prefer tokio::try_join! over sequential awaits
   Sequential: fetch_a().await?; fetch_b().await?;  // 10ms + 10ms = 20ms
   Parallel:   try_join!(fetch_a(), fetch_b());       // max(10,10) = 10ms

2. Use .buffer_unordered() for fan-out with concurrency limit
   Not .for_each_concurrent() which doesn't collect results

3. Reuse connections (pools, not per-request connections)
   HTTP client: reqwest::Client (reuse via AppState)
   DB: sqlx::PgPool (shared pool)
   Cache: fred::RedisClient (multiplexed connection)

4. Avoid unnecessary clones in spawned tasks
   Move ownership into spawned task, not clone + move

5. Use bytes::Bytes for zero-copy buffer sharing

6. Profile with tokio-console for task scheduling issues
   RUSTFLAGS="--cfg tokio_unstable" cargo build
============================================================
SECTION 8: TESTING ASYNC CODE
============================================================
Rust

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_concurrent_balance_check() {
        let service = create_test_service().await;
        
        // Spawn multiple concurrent operations
        let mut handles = vec![];
        for _ in 0..100 {
            let svc = service.clone();
            handles.push(tokio::spawn(async move {
                svc.get_balance(UserId(1)).await
            }));
        }
        
        // All should succeed
        for handle in handles {
            assert!(handle.await.unwrap().is_ok());
        }
    }
    
    #[tokio::test]
    async fn test_timeout_on_slow_service() {
        let service = create_test_service_with_delay(Duration::from_secs(10)).await;
        
        let result = timeout(
            Duration::from_secs(1),
            service.call_external()
        ).await;
        
        assert!(result.is_err()); // should timeout
    }
}