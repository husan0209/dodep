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

All `/api/v1/**` routes require `Authorization: Bearer <access-token>` issued by
the auth service. The user id is taken **exclusively from the token's `user_id`
claim** and is never read from the URL, query string or request body (satisfies
`CONVENTIONS.md` NEVER-7). Health/readiness probes are unauthenticated.

Token requirements (see `middleware.go`):

| Check | Rationale |
|-------|-----------|
| `iss == opus-casino-auth` | Rejects correctly-signed tokens from any other issuer |
| `token_type == access` | The auth service signs **access and refresh with the same secret**, differing only in `token_type`/TTL. Without this check a 30-day refresh token passes as an API credential |
| `exp` present | A token without expiry would be a permanent credential |
| `sub == user_id` | Auth enforces this invariant; a mismatch means a tampered claim set |
| `user_id` numeric and `> 0` | `users.id` is `BIGSERIAL`, carried in the token as its decimal string |
| `alg ∈ {HS256, EdDSA}` | Pinning the algorithm blocks `alg: none` and RSA/HMAC confusion. HS384/HS512 are rejected: the auth service only issues HS256 |

Verification keys, in order of precedence:

- `JWT_SECRET_KEY` — HS256 shared secret. Must be ≥32 bytes and **not** the
  `change-me-in-production` placeholder.
- `JWT_ED25519_PUBLIC_KEY` — base64 Ed25519 public key, used when the auth
  service runs in EdDSA mode.

If neither is usable the verifier **fails closed**: every authenticated request
returns `401` rather than verifying against a weak key. `config.Validate()`
also stops the process at startup when `APP_ENV != development` and no usable
key is configured, matching the auth service's behaviour.

### Authorization

There is no role claim in the auth token, so there is no path to cross-user
access over HTTP: a token can only ever act as its own subject. Reads of another
player belong on the gRPC surface (internal service callers), not on these
routes.

---

## REST Endpoints

Base: `/api/v1/users`. **No endpoint takes a user id in the path.**

| Method | Path | Success | Errors | Notes |
|--------|------|---------|--------|-------|
| `GET` | `/me` | `200` user JSON | `404` unknown user, `500` datastore error | Identity from token |
| `PUT` | `/me` | `200` updated user JSON | `400` bad body, `404` no row updated, `500` datastore error | `req.user_id` from the token; a body `user_id` is ignored |
| `GET` | `/me/preferences` | `200` preferences JSON | `500` on DB error | Unknown user → `200` with defaults (`en`/`UTC`), not 404 |
| `PUT` | `/me/preferences` | `200` preferences JSON | `400` bad body, `500` service error | Upsert + re-read; `user_id` from token |
| `GET` | `/me/limits` | `200` limits JSON | `500` on DB error | Unknown user → `200` with empty `UserLimits{user_id}` |
| `PUT` | `/me/limits` | `200` limits JSON | `400` validation failure, `500` service error | See limits validation below |

Health: `GET /health` → `ok`, `GET /ready` → `ready` (no dependency checks).

The previous `/api/v1/users/:id` surface was removed rather than gated: nothing
in the repository consumed it (no service, frontend, mobile app or Helm chart
calls this HTTP API), and keeping an id-in-path route would preserve the IDOR
surface regardless of any equality check.

> ⚠️ **Missing routes (open):** `DeleteUser` and `GetActivity` exist in
> `UserService` and gRPC but have **no HTTP route**. Add only if web/mobile
> need them over REST.

### Money/limits encoding

- Limit amounts are **decimal strings** (`"100.00"`), never float
  (`domain.SetLimitsRequest.* *string`).
- `session_time_minutes` is an integer number of minutes.

### Limits validation (`PUT /me/limits`)

The limit fields are `*string`, so the values are attacker-controlled. They are
validated in `internal/domain/limits_validation.go` before anything is stored:

1. Every present amount must match `^[0-9]{1,15}(\.[0-9]{1,2})?$` — non-negative,
   at most two decimals. This rejects the dangerous shapes: a **negative limit is
   not a restriction** (a `-5000` deposit ceiling would defeat the control), and
   `1e9`, `NaN`, `Inf`, `+10.00`, `10,00` and `$100` are not amounts at all.
2. `daily ≤ weekly ≤ monthly` within each of the deposit/bet/loss families. A
   daily ceiling above the weekly one is self-contradictory and would leave the
   weaker bound governing in practice.
3. `session_time_minutes ∈ [1, 1440]`.
4. `self_exclusion: false` is **rejected**. Self-exclusion is revocable only
   through the Responsible Gambling service, after a cooling-off period; it must
   not be clearable from a generic profile endpoint.

Comparisons run on integer minor units (cents) derived by string manipulation,
never through a binary float (CONVENTIONS NEVER-6).

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

- Unit (no DB): `internal/service/user_service_test.go` (fake repo),
  `internal/handlers/grpc_handler_test.go`,
  `internal/repository/user_repository_test.go` (pgxmock — SQL-level, no database
  needed), `internal/domain/limits_validation_test.go` (exhaustive matrix of
  rejected/accepted amount shapes), `middleware_test.go` (JWT: header parsing,
  signature confusion, refresh-token rejection, claim validation, Ed25519 path,
  fail-closed verifier), `routes_test.go` (real Fiber app via `app.Test()`),
  `internal/config/config_test.go`.
- Security-relevant tests are regression tests for concrete findings:
  `TestRoutesRejectAnonymous` (the IDOR), `TestLegacyUserIDRoutesGone`,
  `TestUpdateMeIgnoresBodyUserID` (mass assignment),
  `TestAuthRejectsRefreshTokens`, `TestUpdateMeUnknownUserIs404` (the old `200 null`).
- Coverage: service 88.4%, handlers 79.5%, repository 94.3%, config 100%,
  domain 94.6%, routes package 70.9% (`main()` bootstrap excluded).
- Load: `tools/testing/k6/scenarios/user-service.js` (reads always; writes gated behind `ALLOW_WRITES=true` + `TEST_USER_ID`).
- DB seam: `UserRepository` depends on the `DBPool` interface (not `*pgxpool.Pool`);
  `*pgxpool.Pool` satisfies it implicitly, pgxmock in tests.
