# User Service — API Documentation

> **Source of truth (REST):** `services/go/user/routes.go` (Fiber, `package main`).
> **Source of truth (gRPC):** `libs/proto/user/v1/user.proto` (package `user.v1`, buf v2).
> This document describes the **actual implemented behavior**, including known gaps.
> Backend implementation: `services/go/user/` (HTTP + gRPC + pgx repository).

## Base URLs

```
HTTP: user.platform:8082 (code default PORT) — CONVENTIONS.md prescribes 8085, helm chart exposes 8080.
gRPC: user.platform:9092 (code default GRPC_PORT) — CONVENTIONS.md prescribes 50052.
```

> ⚠️ **Port mismatch (open):** code defaults (`8082`/`9092`) disagree with both
> `CONVENTIONS.md` (`8085`/`50052`) and `infra/helm/charts/user/values.yaml`
> (`8080`). Deployments must set `PORT`/`GRPC_PORT` explicitly until the owner
> aligns the defaults. See `services/go/user/internal/config/config.go`.

---

## Authentication

> ⚠️ **Auth gap (open, NEVER-7):** HTTP routes take `user_id` from the URL path
> with **no auth middleware** — any caller can read/modify any user.
> gRPC callers are expected to be internal services (already authenticated).
> Fix (require JWT, take `user_id` from claims) is a product decision for the
> service owner + Security Engineer. Frontend/mobile must not rely on these
> routes being private.

---

## REST Endpoints

Base: `/api/v1/users`.

| Method | Path | Success | Errors | Notes |
|--------|------|---------|--------|-------|
| `GET` | `/:id` | `200` user JSON | `400` non-numeric id, `404` unknown id | |
| `PUT` | `/:id` | `200` updated user JSON | `400` bad body / service error | `req.user_id` is **overwritten from path**. ⚠️ Updating a nonexistent user returns `200 null` (repo UPDATE affects 0 rows, then re-read yields nil,nil — open bug, see flags) |
| `GET` | `/:id/preferences` | `200` preferences JSON | `500` on DB error | Unknown user → `200` with defaults (`en`/`UTC`), not 404 |
| `PUT` | `/:id/preferences` | `200` preferences JSON | `400` bad body / service error | Upsert + re-read |
| `GET` | `/:id/limits` | `200` limits JSON | `500` on DB error | Unknown user → `200` with empty `UserLimits{user_id}` |
| `PUT` | `/:id/limits` | `200` limits JSON | `400` bad body / service error | Upsert + re-read; `self_exclusion: true` is logged server-side |

Health: `GET /health` → `ok`, `GET /ready` → `ready` (no dependency checks).

> ⚠️ **Missing routes (open):** `DeleteUser` and `GetActivity` exist in
> `UserService` and gRPC but have **no HTTP route**. Add only if web/mobile
> need them over REST.

### Money/limits encoding

- Limit amounts are **decimal strings** (`"100.00"`), never float
  (`domain.SetLimitsRequest.* *string`).
- `session_time_minutes` is an integer number of minutes.

---

## gRPC Service `user.v1.UserService`

Proto package: `user.v1` · Go package: `github.com/opus-casino/proto/gen/go/user/v1`.

| RPC | Request | Response | Error mapping |
|-----|---------|----------|---------------|
| `GetUser` | `GetUserRequest{user_id}` | `GetUserResponse{user}` | gRPC `NotFound` when missing (incl. non-numeric id → `0` → not found) |
| `GetUserByEmail` | `GetUserByEmailRequest{email}` | `GetUserByEmailResponse{user, error}` | Error **in-body**, never gRPC error |
| `UpdateUser` | `UpdateUserRequest{user_id, username?, first_name?, ...}` | `UpdateUserResponse{user, error}` | Error **in-body**; only set fields are updated (`COALESCE`) |
| `DeleteUser` | `DeleteUserRequest{user_id, reason}` | `DeleteUserResponse{success, error}` | Soft delete (`status='deleted'`, `deleted_at`); error **in-body** |
| `GetPreferences` | `GetPreferencesRequest{user_id}` | `GetPreferencesResponse{preferences}` | gRPC `Internal` on DB error |
| `UpdatePreferences` | `UpdatePreferencesRequest{user_id, language?, timezone?, ...}` | `UpdatePreferencesResponse{preferences, error}` | Error **in-body** |
| `GetLimits` | `GetLimitsRequest{user_id}` | `GetLimitsResponse{limits}` | gRPC `Internal` on DB error |
| `SetLimits` | `SetLimitsRequest{user_id, daily_deposit_limit?{amount}, ..., session_time_limit?{minutes}, self_exclusion?, self_exclusion_until?}` | `SetLimitsResponse{limits, error}` | Only `daily/monthly_deposit`, `daily_loss`, `session_time`, `self_exclusion*` are mapped (weekly/bet/weekly-loss/monthly-loss proto fields are **ignored** — open gap) |
| `GetActivity` | `GetActivityRequest{user_id, pagination{page_size}}` | `GetActivityResponse{activities[{id, user_id, description}], pagination{total_count}}` | `offset` is always `0` (no cursor support — open gap); reads `audit_log` where `table_name='users'` |

### Type mappings (`toProto*` in `grpc_handler.go`)

- `User.id` → string (`"42"`); `status` via `UserStatus_value`; `kyc_level` cast to `pb.KycLevel`.
- `UserLimits.session_time_limit` → `nil` when unset (not an empty object).
- `ActivityEntry.description` ← `audit_log.action`; missing keys render as `<nil>`-free `%v` strings.

---

## Data & Idempotency

- Reads are plain `SELECT`s (pgx, parameterized — no string concatenation).
- `UpsertPreferences` / `SetLimits` use `INSERT ... ON CONFLICT (user_id) DO UPDATE` — safe to retry.
- No `X-Idempotency-Key` handling in this service (upserts are naturally idempotent; no money moves here).

---

## Tests

- Unit (no DB): `internal/service/user_service_test.go` (8 tests, fake repo),
  `internal/handlers/grpc_handler_test.go` (7 tests),
  `internal/repository/user_repository_test.go` (11 tests, pgxmock — SQL-level,
  no database needed), `routes_test.go` (4 tests, real Fiber app via
  `app.Test()`), `internal/config/config_test.go` (3 tests).
- Coverage: service 97.1%, handlers 79.5%, repository 94.3%, config 100%,
  routes package 53.5% (`main()` bootstrap excluded).
- Load: `tools/testing/k6/scenarios/user-service.js` (reads always; writes gated behind `ALLOW_WRITES=true` + `TEST_USER_ID`).
- DB seam: `UserRepository` depends on the `DBPool` interface (not `*pgxpool.Pool`);
  `*pgxpool.Pool` satisfies it implicitly, pgxmock in tests.
