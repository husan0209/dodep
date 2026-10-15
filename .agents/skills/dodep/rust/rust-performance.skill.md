# SKILL #14 — rust-performance.skill.md

```markdown
# rust-performance.skill.md
# GAMBLING PLATFORM — RUST PERFORMANCE OPTIMIZATION
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Rust services handle 80% of platform traffic.
Performance targets:
  p50: < 2ms, p95: < 10ms, p99: < 50ms
  Throughput: 10K+ req/sec per instance
  Memory: 150-500 MB per instance (stable, no growth)

Profile BEFORE optimizing. Premature optimization is the root of all evil.

# ============================================================
# SECTION 2: ALLOCATION AWARENESS
# ============================================================

```rust
// ── Avoid unnecessary allocations in hot paths ──

// ❌ BAD: Allocates String every call
fn format_cache_key(event_id: i64, market_id: i64) -> String {
    format!("odds:{}:{}", event_id, market_id) // heap allocation
}

// ✅ GOOD: Use write! into pre-allocated buffer
fn format_cache_key(buf: &mut String, event_id: i64, market_id: i64) {
    buf.clear();
    write!(buf, "odds:{}:{}", event_id, market_id).unwrap();
}

// ✅ GOOD: Use SmallVec for small collections
use smallvec::SmallVec;
type Selections = SmallVec<[Selection; 4]>; // stack-allocated up to 4

// ── Prefer &str over String in function parameters ──

// ❌ BAD
fn get_by_email(email: String) -> ... // forces caller to allocate

// ✅ GOOD
fn get_by_email(email: &str) -> ...   // borrows, no allocation

// ── Use Cow for conditional ownership ──
use std::borrow::Cow;

fn normalize_email(email: &str) -> Cow<'_, str> {
    if email.contains(char::is_uppercase) {
        Cow::Owned(email.to_lowercase()) // allocates only when needed
    } else {
        Cow::Borrowed(email) // zero-cost
    }
}

// ── Object pooling for frequently allocated types ──
use crossbeam::queue::ArrayQueue;

static BUFFER_POOL: Lazy<ArrayQueue<Vec<u8>>> = Lazy::new(|| {
    let queue = ArrayQueue::new(256);
    for _ in 0..256 {
        queue.push(Vec::with_capacity(4096)).ok();
    }
    queue
});

fn get_buffer() -> Vec<u8> {
    BUFFER_POOL.pop().unwrap_or_else(|| Vec::with_capacity(4096))
}

fn return_buffer(mut buf: Vec<u8>) {
    buf.clear();
    let _ = BUFFER_POOL.push(buf); // silently drops if pool full
}
============================================================
SECTION 3: ZERO-COPY PATTERNS
============================================================
Rust

// ── Use bytes::Bytes for shared immutable buffers ──
use bytes::Bytes;

// WebSocket broadcast: serialize once, share with all subscribers
pub async fn broadcast(&self, topic: &Topic, data: &impl Serialize) {
    let bytes = Bytes::from(serde_json::to_vec(data).unwrap());
    // Bytes is reference-counted — cloning is O(1)
    for sender in self.get_subscribers(topic) {
        let _ = sender.try_send(Message::Binary(bytes.clone())); // no copy
    }
}

// ── Use rkyv for zero-copy deserialization (cache) ──
use rkyv::{Archive, Deserialize, Serialize};

#[derive(Archive, Deserialize, Serialize)]
pub struct CachedOdds {
    pub event_id: i64,
    pub odds: Vec<(i64, i64)>, // (outcome_id, odds_as_i64)
}

// Read from cache without deserialization
let bytes = cache.get_bytes("odds:123").await?;
let archived = rkyv::check_archived_root::<CachedOdds>(&bytes)?;
let odds = archived.odds[0]; // access directly from bytes, no copy

// ── Use str instead of String where lifetime allows ──

// In request processing (short-lived):
struct BetContext<'a> {
    currency: &'a str,  // borrowed from request, no allocation
    ip: &'a str,
}
============================================================
SECTION 4: CONCURRENCY PATTERNS
============================================================
Rust

// ── DashMap for concurrent read-heavy maps ──
use dashmap::DashMap;

// Subscription manager: millions of reads, few writes
let subscriptions: DashMap<Topic, Vec<Sender>> = DashMap::new();
// Lock-free reads, per-shard write locks

// ── Atomic counters for metrics ──
use std::sync::atomic::{AtomicU64, Ordering};

static BETS_PLACED: AtomicU64 = AtomicU64::new(0);

fn record_bet() {
    BETS_PLACED.fetch_add(1, Ordering::Relaxed); // no lock, no contention
}

// ── Read-write lock for config that rarely changes ──
use tokio::sync::RwLock;

let config: Arc<RwLock<OddsConfig>> = Arc::new(RwLock::new(load_config()));

// Hot path: read lock (concurrent, fast)
let cfg = config.read().await;

// Cold path: write lock (exclusive, rare)
let mut cfg = config.write().await;
*cfg = new_config;

// ── Connection pooling (reuse, don't create per request) ──
// SQLx pool, Redis pool, HTTP client — all in AppState
// NEVER create pool/client inside a handler or service method
============================================================
SECTION 5: DATABASE PERFORMANCE
============================================================
text

1. USE INDEXES for every WHERE clause pattern
   CREATE INDEX idx_bets_user_status ON bets(user_id, status);

2. USE prepared statements (SQLx does this automatically with query!)

3. AVOID N+1 queries — use JOIN or IN clause
   ❌ for bet in bets { get_selections(bet.id) }  // N+1
   ✅ get_selections_for_bets(&bet_ids)            // 1 query

4. USE cursor pagination, not OFFSET for large tables
   ❌ OFFSET 100000 LIMIT 20  // scans 100K rows
   ✅ WHERE id > $cursor ORDER BY id LIMIT 20

5. BATCH inserts for bulk operations
   Use QueryBuilder::push_values() for multi-row INSERT

6. MINIMIZE transaction scope
   ❌ Begin tx → call external API → commit  // holds lock during API call
   ✅ Call external API → begin tx → write → commit

7. USE connection pool wisely
   max_connections = 2 × CPU cores (not hundreds)
   Monitor pool wait time → alert if > 100ms
============================================================
SECTION 6: CACHE PERFORMANCE
============================================================
text

1. USE pipeline for multiple cache operations
   ❌ get(key1).await; get(key2).await; get(key3).await;  // 3 round trips
   ✅ pipeline { get(key1); get(key2); get(key3); }.exec() // 1 round trip

2. USE appropriate serialization
   JSON:     easy debug, ~500 bytes for odds
   Protobuf: smaller (~200 bytes), faster parse
   rkyv:     zero-copy read, fastest for hot path

3. SET correct TTL
   Live odds:    3-5 seconds (stale odds = wrong bets)
   Pre-match:    30 seconds
   User profile: 5 minutes
   Game catalog:  10 minutes

4. WARM cache on service startup
   Load frequently accessed data before accepting traffic

5. USE cache-aside pattern (not write-through for most cases)
   Read: cache miss → DB → set cache
   Write: update DB → invalidate cache (not update)
============================================================
SECTION 7: PROFILING TOOLS
============================================================
text

CONTINUOUS PROFILING (production):
  Pyroscope agent integrated into each service
  CPU + memory flame graphs 24/7
  Overhead < 2%

DEVELOPMENT PROFILING:
  cargo flamegraph — generate flame graph from binary
  tokio-console — inspect async tasks and resources
  DHAT (via dhat crate) — heap allocation profiling

BENCHMARKING:
  criterion — statistical micro-benchmarks
  k6 — HTTP/WebSocket load testing (system level)

METRICS TO WATCH:
  p99 latency per endpoint (VictoriaMetrics)
  Memory RSS over time (should be flat, not growing)
  DB query duration (pg_stat_statements)
  Cache hit rate (should be > 95%)
  GC pauses: N/A (Rust has no GC — this is the advantage)
  Allocation rate (Pyroscope memory profile)
============================================================
SECTION 8: RELEASE BUILD OPTIMIZATION
============================================================
toml

# Cargo.toml [profile.release]
[profile.release]
lto = "thin"         # link-time optimization (10-20% faster)
codegen-units = 1    # better optimization, slower compile
strip = "symbols"    # smaller binary
panic = "abort"      # smaller binary, no unwinding overhead
opt-level = 3        # maximum optimization

# For even smaller binary:
# opt-level = "z"    # optimize for size over speed
Dockerfile

# Multi-stage build with distroless
FROM rust:1.80-bookworm AS builder
# ... build ...

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=builder /app/target/release/service /app/service
# Final image: ~30MB instead of ~1GB
============================================================
SECTION 9: ANTI-PATTERNS
============================================================
text

❌ NEVER optimize without profiling first (measure, don't guess)
❌ NEVER clone() in hot paths without checking if borrow works
❌ NEVER use String where &str suffices
❌ NEVER create Vec when SmallVec or ArrayVec fits
❌ NEVER allocate inside a loop when buffer can be reused
❌ NEVER use Mutex where AtomicU64 or DashMap works
❌ NEVER hold locks across .await points
❌ NEVER use OFFSET pagination for large tables
❌ NEVER skip connection pooling (pool in AppState, not per-request)
❌ NEVER ignore cache hit rate metrics (< 90% = something wrong)
❌ NEVER use debug build for benchmarks (release only)
❌ NEVER block tokio runtime with CPU-heavy work (use spawn_blocking)