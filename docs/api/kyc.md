# KYC Service — API Documentation

> **Source of truth:** `libs/proto/kyc/v1/kyc.proto` (gRPC contract, buf v2).
> This document maps the proto contract to the HTTP layer.
> Backend implementation: `services/go/kyc/` (Fiber + gRPC, GORM, Redis).

## Base URLs

```
HTTP: kyc.platform:8089
gRPC: kyc.platform:50059
```

Ports per `CONVENTIONS.md` (kyc: 8089 HTTP / 50059 gRPC).

---

## Authentication

- All `/api/v1/kyc/*` endpoints (except webhooks and probes) require
  `Authorization: Bearer <access_token>` (JWT, issued by auth-service).
- `user_id` is **always taken from the JWT claims, never from the request body**
  (`CONVENTIONS.md` NEVER-7).
- Mutating endpoints additionally require `X-Idempotency-Key: <uuid>`.
- Admin endpoints (`/api/v1/kyc/admin/*`) require the JWT to carry
  `role=admin` or `is_admin=true`.
- Webhooks (`POST /api/v1/kyc/webhooks/:provider`) carry no JWT; they are
  authenticated with HMAC-SHA256 (`X-Webhook-Signature`) over the raw body.

---

## KYC lifecycle

```
Levels (stepwise, never skip, never downgrade silently):
  none → basic → identity → enhanced → vip
  basic     = email + phone (verified externally, recorded from trusted caller)
  identity  = photo ID (passport/drivers_license/national_id/residence_permit) + selfie
  enhanced  = identity + proof_of_address/bank_statement (fresh ≤ 90 days)
  vip       = enhanced + source_of_funds (+ manual compliance review of screening)

Record status:
  not_started → in_progress → pending_review → verified
                                        ↘ rejected (reason stored)
  verified → expired (load-bearing document expired; re-verification required)

Document status:
  draft → submitted → in_review → approved → expired
                       ↘ rejected (immutable; re-submit as a new document)
```

- `RequestUpgrade` registers intent one level up and returns the missing
  documents; the tier itself changes only when approvals satisfy the
  requirements (or immediately for `basic`).
- Every `VerifyDocument` that completes a tier runs PEP/sanctions screening
  **before** promotion: `clear` → promote; `potential_match` → hold at
  `pending_review` for compliance; `true_match` → `rejected` (SAR path).
- Screening failures fail **closed**: promotion deferred, never granted.
- Approved documents are immutable (delete refused); rejected documents are
  never rewritten — submit a new one (audit trail stays intact).

---

## gRPC Service `kyc.v1.KYCService`

Proto package: `kyc.v1` · Go package: `github.com/opus-casino/proto/gen/go/kyc/v1`

| RPC | Notes |
|-----|-------|
| `GetKYCStatus` | Status + current level + required/submitted docs + rejection reason |
| `SubmitDocument` | Validation failures ride in `Error` field; duplicates → `Error` |
| `GetDocument` | Owner only (else `NotFound`); PII (`document_number`) admin-only, otherwise empty |
| `ListDocuments` | Cursor pagination (`page_size`, `cursor` = last document id) + type/status filters |
| `DeleteDocument` | `submitted`/`rejected` only; approved docs → `Error` (immutable) |
| `VerifyDocument` | Admin; `verified_by` required; triggers level recompute + screening |
| `RejectDocument` | Admin; `rejection_reason` required |
| `GetVerificationLevel` | Tier + decimal-string limits + pending requirements |
| `UpgradeVerificationLevel` | Stepwise intent; `basic` completes synchronously |

Money/limits are decimal **strings** (`"5000"`), never float (`CONVENTIONS.md` NEVER-6).

Limits per tier (USD):

| Tier | Daily dep / with | Monthly dep / with | Max bet | Crypto | Bonus |
|------|------------------|--------------------|---------|--------|-------|
| none | 0 / 0 | 0 / 0 | 0 | no | no |
| basic | 200 / 0 | 1000 / 0 | 50 | no | no |
| identity | 5000 / 2000 | 20000 / 10000 | 500 | no | yes |
| enhanced | 50000 / 20000 | 500000 / 200000 | 5000 | yes | yes |
| vip | 500000 / 200000 | 5000000 / 2000000 | 50000 | yes | yes |

---

## REST Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health`, `/ready` | Liveness / readiness (ready checks DB) |
| `POST` | `/api/v1/kyc/documents` | Submit document (idempotency key required) |
| `GET` | `/api/v1/kyc/documents?type=&status=` | List own documents |
| `GET` | `/api/v1/kyc/documents/:id` | Document detail (owner only; no PII) |
| `DELETE` | `/api/v1/kyc/documents/:id` | Delete submitted/rejected document |
| `GET` | `/api/v1/kyc/status` | Verification snapshot + limits |
| `GET` | `/api/v1/kyc/level` | Tier + limits + pending requirements |
| `POST` | `/api/v1/kyc/upgrade` | `{"target_level": 1..4}` → missing documents |
| `POST` | `/api/v1/kyc/admin/documents/:id/verify` | Approve (PII in response: admin surface only) |
| `POST` | `/api/v1/kyc/admin/documents/:id/reject` | Reject with reason + required actions |
| `GET` | `/api/v1/kyc/admin/screenings` | Unresolved flagged screenings |
| `POST` | `/api/v1/kyc/admin/screenings/:id/resolve` | `{"approve": bool, "note": str}` |
| `POST` | `/api/v1/kyc/admin/sweep-expired` | Cron trigger: mark expired docs (bounded batch) |
| `POST` | `/api/v1/kyc/webhooks/:provider` | Provider callback (HMAC, idempotent) |

Error codes (`KYC_*`, 7000–7999 range): `KYC_VALIDATION_FAILED` (400),
`KYC_NOT_FOUND` (404), `KYC_DOCUMENT_CONFLICT` (409),
`KYC_ALREADY_VERIFIED` (409), `KYC_DOCUMENT_LOCKED` (409),
`KYC_COMPLIANCE_HOLD` (403/422), `KYC_INVALID_SIGNATURE` (401),
`KYC_INTERNAL_ERROR` (500, cause logged server-side only).

Deviation from proto comment: `DeleteDocument` also accepts `submitted`
documents, not only `draft` — submissions go straight to `submitted`
(there is no draft API), and unreviewed submissions must stay retractable.

---

## Screening (PEP / sanctions)

Default provider is the offline **stub** (`complyadvantage-stub`):
name contains `sanction` → `true_match`; contains `pep` → `potential_match`;
otherwise `clear`. Every stub verdict is labelled with the provider name in
`kyc_screenings`, so test data can never be mistaken for a real check.

Production path: `ComplyAdvantageClient` is wired but fails closed until
`COMPLYADVANTAGE_API_KEY` is set — screening errors never silently clear a user.

---

## Webhooks (Sumsub-style)

1. Provider POSTs JSON `{event_id, external_user_id, review_answer, level, reject_labels}`.
2. Service verifies `X-Webhook-Signature = hex(HMAC-SHA256(SUMSUB_WEBHOOK_SECRET, body))`.
3. Idempotency: processed `event_id`s are marked in Redis for 24h; duplicate
   deliveries return `200` without re-execution.
4. `GREEN` → record verified at provider level; `RED` → rejected with labels.
5. Every webhook appends to `kyc_audit_log`.

---

## Data & security

- Tables: `kyc_records` (1 row/user), `kyc_documents`, `kyc_screenings`,
  `kyc_audit_log` (append-only; no UPDATE/DELETE in the repository).
- `document_number` is AES-256-GCM encrypted (`KYC_FIELD_KEY`, 32 bytes as
  base64/hex/raw); `number_hash` (SHA-256) enables cross-user duplicate
  detection without decryption.
- Document numbers are masked in logs (`AB***4567`) and omitted from
  user-facing responses; admin review surfaces are the only PII path.
- Status cache (`kyc:status:{user_id}`) TTL is **60s** — never longer.
- All queries are parameterised (GORM); user input validated at handler
  (format), service (business rules) and DB (constraints).

---

## Rate limits & hardening

| Protection | Value | Notes |
|------------|-------|-------|
| Read budget | 300 req/min per user (or IP) | Status/list/level reads |
| Write budget | 30 req/min per user (or IP) | Submit/upgrade/admin review |
| Webhook budget | 60 req/min per IP | Provider retries |
| Body size | 1 MiB | Rejected before parsing |
| Headers | `X-RateLimit-Limit/Remaining/Reset`, `Retry-After` | On 429 |

Security headers on every response: `X-Content-Type-Options: nosniff`,
`X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`,
`Permissions-Policy: camera=(), microphone=(), geolocation=()`, HSTS,
`X-DNS-Prefetch-Control: off`. Limiter state is per pod (edge/mesh own
global quotas).

---

## Configuration (env)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8089` | HTTP port |
| `GRPC_PORT` | `50059` | gRPC port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable` | PostgreSQL DSN |
| `REDIS_ADDR` | `localhost:6379` | DragonflyDB/Redis (memory fallback when unreachable) |
| `REDIS_DB` | `0` | Redis DB index |
| `JWT_SECRET_KEY` | `change-me-in-production` | JWT HMAC secret (enforced outside dev) |
| `KYC_FIELD_KEY` | _(ephemeral in dev)_ | 32-byte PII field key (enforced outside dev) |
| `SUMSUB_API_KEY` / `SUMSUB_API_SECRET` | _(empty)_ | Verification provider credentials |
| `SUMSUB_WEBHOOK_SECRET` | _(empty)_ | Webhook HMAC secret (empty → webhooks rejected) |
| `SUMSUB_BASE_URL` | `https://api.sumsub.com` | Provider base URL |
| `COMPLYADVANTAGE_API_KEY` | _(empty)_ | Empty → stub screening provider |
| `COMPLYADVANTAGE_URL` | `https://api.complyadvantage.com` | Screening base URL |
| `CORS_ORIGINS` | `http://localhost:3000` | CSV allowlist (`*` stripped) |

---

## Remaining work (not in this change)

- PSP credentials: real Sumsub applicant/token flow and ComplyAdvantage search
  mapping, retries and response parsing (`internal/screening` fails closed).
- Object storage: KYC file bytes live with the uploader today (`file_id` is a
  reference); S3 SSE-KMS upload + 5-min pre-signed admin URLs.
- Schedulers: cron for `POST /admin/sweep-expired` and daily batch
  re-screening of active users.
- PITR/backup verification for the new tables; `libs/migrations` SQL to match
  `AutoMigrate` (dev parity only at present).
- Redpanda events `kyc.verified` / `kyc.rejected` for payment/notification
  consumers (audit log is the interim source of truth).
- Integration tests with testcontainers (Postgres + Redis); current suite uses
  in-memory fakes (`internal/testutil`).
- Custom per-user VIP limits negotiation surface.
