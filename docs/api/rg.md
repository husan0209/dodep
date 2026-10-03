# RG (Responsible Gambling) Service — API Documentation

> **Status:** implemented end-to-end (HTTP + service + repository + tests).
> **Source of truth (gRPC contract):** `libs/proto/rg/v1/rg.proto` (package `rg.v1`).
> **Source of truth (REST):** `services/go/rg/routes.go` (Fiber, `package main`).
> **Implementation:** `services/go/rg/`.

## Why this service exists

Platform RULE 5 (architecture-overview): *KYC levels enforce limits, all actions are
audit-logged, regulatory compliance is non-negotiable*. The concrete risk:

- a self-excluded player placing a bet is a **licence-revocation** event, not a bug;
- limit increases must not take effect instantly (protects against impulsive changes
  during a session);
- withdrawals must **never** be blocked by RG — a blocked withdrawal is a regulatory
  and goodwill problem.

Therefore RG is a **gate**, not a report: betting-engine, casino and payment call
`CheckPlayAllowed` before acting, and must treat `allowed=false` as a hard stop.

## Regulatory grounding

| Requirement | Source | Implementation |
|-------------|--------|----------------|
| Self-exclusion periods, min 6 months for multi-operator sync | UKGC LCCP SR 3.5 + GAMSTOP | `Exclusion6M` period; operator sync is an outbound integration point |
| Permanent exclusion cannot be lifted by the operator | UKGC / GAMSTOP | `ExclusionStatus.CanTransitionTo` returns false for permanent rows; `RevokeSelfExclusion` returns `ErrPermanentExclusion` |
| Limit decreases immediate, increases cooling | Gambling-industry standard (24-72h) | `SetLimits`: decrease → immediate, increase → `PendingChange.EffectiveAt = now + cooling` |
| Reality checks | UKGC social-responsibility guidance | `RealityCheckMinutes` exposed on `/status`, default 60 |

Websearch was unavailable in this environment; the UKGC and GAMSTOP pages were fetched
directly and are cited above as the grounding documents. Values not fixed by the
regulator (24h cooling default) are marked as project defaults in config, not law.

## Ports

```
HTTP: 8091   (code default; CONVENTIONS.md has no entry yet — see Open items)
gRPC: 50062
```

Chosen as the next free pair after affiliate (8090/50061). `CONVENTIONS.md` and the
Helm chart need an entry — flagged, not edited (those files are owned elsewhere).

## Authentication

- `/api/v1/rg/*` requires `Authorization: Bearer <access_token>`; `user_id` is taken
  **from the token only** (NEVER-7), never from the body.
- `/admin/rg/*` requires `RG_ADMIN_TOKEN` (fail-secure: unset token rejects everything).
- Token verification: HS256 when `JWT_SECRET_KEY` is configured; otherwise claims are
  parsed but the request is rejected outside `APP_ENV=development`. Production should
  terminate user auth at the API gateway / auth-service `ValidateToken` and forward a
  trusted identity — see `README`-level note in the service code.

---

## REST Endpoints

Base: `/api/v1/rg`.

| Method | Path | Success | Notes |
|--------|------|---------|-------|
| `POST` | `/check` | `200 allowed:true` | The enforcement gate. `403` for self-excluded/cool-off, `422` for limit denials (with `remaining_amount`). |
| `PUT` | `/limits` | `200 {limits, pending}` | Decrease applies now; increase is staged in `pending`. |
| `GET` | `/limits` | `200 {limits, pending}` | Effective limits + cooled-down-but-not-applied increases. |
| `POST` | `/self-exclusion` | `201` | `period` ∈ `24h 7d 30d 6m 1y permanent`. `409` if one is already active. |
| `POST` | `/self-exclusion/revoke` | `200` | Requires `confirm:true`, an **expired** temporary exclusion and a passed revocation cooling window. `422` otherwise. |
| `POST` | `/timeout` | `201` | Short cooling-off (`24h 48h 7d 30d`). `409` if already active. |
| `GET` | `/status` | `200` | Aggregate posture: `gambling_allowed`, `blocked_reason`, `limits`, `pending`, `reality_check_minutes`, optional `exclusion`/`timeout`. |

Operator endpoints (`RG_ADMIN_TOKEN`):

| Method | Path | Success | Notes |
|--------|------|---------|-------|
| `POST` | `/admin/rg/exclusions` | `201` | `user_id`, `period`, `type` ∈ `operator regulatory`. |
| `POST` | `/admin/rg/apply-due` | `200 {applied:N}` | Scheduler trigger applying cooled-down increases. |

Health: `GET /health` (liveness), `GET /ready` (readiness incl. DB ping).

### `/check` semantics

`channel` ∈ `bet | game_launch | deposit | withdrawal`.

Check order (deliberate, matches the skill):

1. `withdrawal` → **always allowed** (no further checks);
2. active self-exclusion → `RG_SELF_EXCLUDED`;
3. active time-out → `RG_COOL_OFF`;
4. channel limits:
   - `deposit` → daily/weekly/monthly deposit limits (`spent + amount > limit`);
   - `bet` → loss limits (net loss = wagers − payouts ≥ limit) then wager limits
     (`wagered + stake > limit`);
   - `game_launch` → no money check (session duration enforcement is owned by the
     session tracker, not implemented here — see Open items).

### Limit model

| Control | Type | Enforced on |
|---------|------|-------------|
| `deposit_daily/weekly/monthly` | money | deposit |
| `loss_daily/weekly/monthly` | money | bet |
| `wager_daily/weekly` | money | bet |
| `session_minutes` | minutes | game launch (consumer-side, not enforced here yet) |
| `reality_check_minutes` | minutes | exposed for client popups (default 60) |

- All amounts are **decimal strings** (`decimal.Decimal` end to end), never float.
- First-time configuration of a control applies immediately (setting protection is
  never "relaxing" an existing control); only increases relative to an existing value
  cool down.
- Windows are rolling (24h / 7d / 30d) over daily aggregates
  (`rg_service_spend_daily`, keyed by user + UTC day + currency).

---

## gRPC `rg.v1.RGService`

Proto: `libs/proto/rg/v1/rg.proto` · Go package `github.com/opus-casino/proto/gen/go/rg/v1`.

| RPC | Purpose |
|-----|---------|
| `CheckPlayAllowed` | the enforcement gate (bet/game/deposit) |
| `SetLimits` / `GetLimits` | limit management |
| `StartSelfExclusion` / `RevokeSelfExclusion` | exclusion lifecycle |
| `StartTimeout` | short cooling-off |
| `GetStatus` | aggregate posture |
| `RecordSpend` | report settled deposit/wager/payout for limit accounting |

> Go stubs land with `buf generate` (PROTOBUF_CONTRACTS). Until then
> `internal/handlers/grpc_handler.go` implements the same method surface with plain Go
> types so the service builds and ships; swapping in the generated adapter requires no
> business-logic change. gRPC server currently serves health + reflection only.

## Data Model (service-owned tables)

| Table | Purpose |
|-------|---------|
| `rg_service_limits` | one row per user, effective limits |
| `rg_service_pending_changes` | cooled-down increases (status `pending|applied|cancelled`) |
| `rg_service_exclusions` | immutable exclusion audit trail (`active|expired|revoked`) |
| `rg_service_timeouts` | cooling-off records |
| `rg_service_spend_daily` | per (user, day, currency) deposits/wagers/payouts |
| `rg_service_outbox` | transactional outbox for domain events |

Tables are prefixed `rg_service_` because the legacy `rg_player_limits` in
`libs/migrations/postgresql/010_kyc_rg.sql` keys players by UUID with an FK that cannot
reference `users(BIGSERIAL id)`. This service keys by BIGINT `user_id` and holds no
cross-service FK (microservice boundary; consistency via events).

Every state change and its event are written in **one transaction**
(`Repository.Transact` + outbox), so events cannot be lost on partial failure.

## Events (Redpanda topics)

| Topic | When |
|-------|------|
| `users.self_excluded` | exclusion started (existing platform topic — session kill, status change) |
| `rg.exclusion.started` | exclusion row created (type/period/permanent) |
| `rg.exclusion.revoked` | temporary exclusion lifted |
| `rg.timeout.started` | cooling-off started |
| `rg.limit.set` | limits changed |
| `rg.limit.increase_applied` | cooled-down increase became effective |

Denials (per-bet) are **not** published as events — at bet volume that would flood the
topic; they are logged and visible through `/status` and audit log.

## Tests

| Package | Tests | Coverage |
|---------|-------|----------|
| `main` (`middleware_test.go`, `routes_test.go`) | JWT claim extraction + expiry/tamper, fail-closed outside dev, auth/admin middleware, request-id, error envelope, full route wiring incl. auth boundaries | 55.0% |
| `internal/domain` | periods, state machines, limit get/set, activation | 65.4% |
| `internal/service` | enforcement order, limits + cooling, exclusion lifecycle, revocation rules, status | 72.7% |
| `internal/repository` | GORM SQL via `go-sqlmock` (real emitted SQL, no DB) | 79.1% |
| `internal/handlers` | HTTP status/JSON mapping for every endpoint incl. admin | 71.3% |
| `internal/config` | defaults, env, regulatory clamping | 100% |

`main` is 55% because `main.go` itself (bootstrap: DB connect, AutoMigrate, server
start) is not unit-testable; every middleware and route it wires is.

Key invariants pinned by tests:

- self-excluded player cannot bet / launch / deposit, **can** withdraw;
- decrease immediate, increase only after cooling; superseded increase is replaced,
  never stacked;
- permanent exclusion is unrevokable through any code path;
- revoke requires explicit confirm **and** an elapsed revocation cooling window;
- per-bet denials return `422` with `remaining_amount`; hard blocks return `403`;
- player routes 401 without a valid token; operator routes are unreachable with a
  player token, and an unset `RG_ADMIN_TOKEN` forbids everything (fail-secure);
- token verification fails closed outside `APP_ENV=development` when no shared secret
  is configured.

## Open items (need other owners)

1. **Wire the gate**: betting-engine / casino / payment must call `CheckPlayAllowed`
   before every bet, game launch and deposit (owner: Rust + payment agents).
2. **`RecordSpend` callers**: wallet/payment must report settled deposits, wagers and
   payouts, or limits cannot be enforced accurately.
3. **Session duration enforcement** (`session_minutes`) needs a session tracker; this
   service exposes the setting only.
4. **Cross-service FK/reconciliation**: `users.kyc_level` sync and RG status are
   eventually consistent by design; a reconciliation job is not implemented.
5. **CONVENTIONS.md + Helm**: no entry for rg ports yet (8091/50062).
6. **Multi-operator / GAMSTOP sync**: `Exclusion6M` exists, but the outbound
   registration API to the national scheme is not implemented.