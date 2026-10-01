# KYC Service — API Documentation

> **Source of truth:** `libs/proto/kyc/v1/kyc.proto` (gRPC contract, buf v2).
> This document maps the proto contract to the HTTP layer.
> Proto section is normative; REST mapping is the agreed contract for
> frontend/admin clients and the HTTP handler checklist.
> Backend implementation: `services/go/kyc/` (in progress).

## Base URLs

```
HTTP: kyc.platform:8089
gRPC: kyc.platform:50059
```

Ports per `CONVENTIONS.md` (kyc: 8089 HTTP / 50059 gRPC).

---

## Authentication

- All `/api/v1/kyc/*` endpoints require `Authorization: Bearer <access_token>`
  (JWT Ed25519, issued by auth-service, TTL 15 min).
- `user_id` is **always taken from the JWT claims, never from the request body**
  (`CONVENTIONS.md` NEVER-7).
- Admin endpoints (`/api/v1/kyc/admin/*`) additionally require one of the roles:
  `super_admin`, `risk_manager`, `kyc_officer`, `support_l2`.
- Provider webhooks (`POST /api/v1/kyc/webhook/:provider`) use HMAC-SHA256
  signature over the raw body (`X-Webhook-Signature` header, hex),
  key from `KYC_WEBHOOK_SECRET` env. No JWT.

---

## KYC Levels & Limits

| Level | Name | Requirements | Deposit/day | Withdrawal/day |
|-------|------|--------------|-------------|----------------|
| 0 | none | registration only | $0 | $0 |
| 1 | basic | email + phone verified | $200 | $0 |
| 2 | identity | government ID + selfie | $5,000 | $2,000 |
| 3 | enhanced | + proof of address + source of funds | $50,000 | $20,000 |
| 4 | vip | manual compliance review | custom | custom |

- Money values are decimal strings (`"5000.00"`), never float.
- Level upgrades require approved documents (see `UpgradeVerificationLevel`).
- Level 4 is never auto-approved.

---

## gRPC Service `kyc.v1.KYCService`

Proto package: `kyc.v1` · Go package: `github.com/opus-casino/proto/gen/go/kyc/v1`

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `GetKYCStatus` | `GetKYCStatusRequest{user_id}` | `GetKYCStatusResponse{status, current_level, required_documents, submitted_documents}` | Aggregate status |
| `SubmitDocument` | `SubmitDocumentRequest{user_id, document_type, document_side, file_id, document_number, issuing_country, expiry_date, metadata}` | `SubmitDocumentResponse{document, error}` | Creates `submitted` document |
| `GetDocument` | `GetDocumentRequest{user_id, document_id}` | `GetDocumentResponse{document}` | Owner or admin only |
| `ListDocuments` | `ListDocumentsRequest{user_id, document_type?, status?, pagination}` | `ListDocumentsResponse{documents, pagination}` | Cursor pagination |
| `DeleteDocument` | `DeleteDocumentRequest{user_id, document_id}` | `DeleteDocumentResponse{success, error}` | `draft` only |
| `VerifyDocument` | `VerifyDocumentRequest{document_id, verified_by, verification_notes, extracted_data}` | `VerifyDocumentResponse{success, error}` | Admin |
| `RejectDocument` | `RejectDocumentRequest{document_id, rejected_by, rejection_reason, required_actions}` | `RejectDocumentResponse{success, error}` | Admin |
| `GetVerificationLevel` | `GetVerificationLevelRequest{user_id}` | `GetVerificationLevelResponse{level, limits, pending_requirements}` | Level + money limits |
| `UpgradeVerificationLevel` | `UpgradeVerificationLevelRequest{user_id, target_level}` | `UpgradeVerificationLevelResponse{new_level, required_documents, error}` | Returns missing docs if not eligible |

`user_id` is `common.v1.UserId{value}` (UUID string).

---

## REST Mapping

| Method & Path | gRPC equivalent | Auth |
|---------------|-----------------|------|
| `GET /api/v1/kyc/status` | `GetKYCStatus` | user |
| `POST /api/v1/kyc/documents` | `SubmitDocument` | user |
| `GET /api/v1/kyc/documents?type=&status=&page_size=&cursor=` | `ListDocuments` | user |
| `GET /api/v1/kyc/documents/:id` | `GetDocument` | user |
| `DELETE /api/v1/kyc/documents/:id` | `DeleteDocument` | user |
| `GET /api/v1/kyc/level` | `GetVerificationLevel` | user |
| `POST /api/v1/kyc/upgrade` | `UpgradeVerificationLevel` | user |
| `POST /api/v1/kyc/admin/documents/:id/verify` | `VerifyDocument` | admin |
| `POST /api/v1/kyc/admin/documents/:id/reject` | `RejectDocument` | admin |
| `GET /api/v1/kyc/admin/expiring?days=30` | — (ops monitor) | admin |
| `POST /api/v1/kyc/webhook/:provider` | — (provider callback) | HMAC |

### Submit Document

**POST** `/api/v1/kyc/documents`

```json
{
  "document_type": "passport",
  "document_side": "front",
  "file_id": "s3:kyc/<user_uuid>/passport/<file_uuid>.enc",
  "document_number": "AB1234567",
  "issuing_country": "DE",
  "expiry_date": "2030-05-01T00:00:00Z",
  "metadata": {}
}
```

Rules: `document_number` is AES-256-GCM encrypted at rest, never logged
in full; `expiry_date` must be in the future; one active document per
type+side (resubmit only after `rejected`/`expired`); idempotent via
`X-Idempotency-Key: <uuid>` header.

**Response** `201 Created` — `Document` object (see below).

### Verify Document (admin)

**POST** `/api/v1/kyc/admin/documents/:id/verify`

```json
{
  "verification_notes": "MRZ matches, selfie liveness OK",
  "extracted_data": {"first_name": "Max", "dob": "1990-01-01"}
}
```

### Reject Document (admin)

**POST** `/api/v1/kyc/admin/documents/:id/reject`

```json
{
  "rejection_reason": "Document is blurred, corners not visible",
  "required_actions": ["reupload_front", "reupload_back"]
}
```

---

## Document Lifecycle

```
draft → submitted → in_review → approved
draft → submitted → in_review → rejected  (user resubmits → new document)
draft → submitted → approved              (admin single-step approval)
any non-final → expired                   (expiry job, past expiry_date)
```

- `DELETE` allowed only for `draft`.
- `approved`/`rejected`/`expired` are final (no transitions out).
- Invalid transitions return `409` / gRPC `FailedPrecondition`.

---

## Enums

**DocumentType:** `passport`, `drivers_license`, `national_id`,
`residence_permit`, `proof_of_address`, `bank_statement`, `selfie`,
`source_of_funds`

**DocumentSide:** `front`, `back`, `single`

**DocumentStatus:** `draft`, `submitted`, `in_review`, `approved`,
`rejected`, `expired`

**KYCLevel:** `none(1)`, `basic(2)`, `identity(3)`, `enhanced(4)`, `vip(5)`
(proto `KYC_LEVEL_*`; `0` = unspecified)

**VerificationStatus:** `not_started`, `in_progress`, `pending_review`,
`verified`, `rejected`, `expired`

---

## Document Object

```json
{
  "id": "uuid",
  "user_id": "uuid",
  "document_type": "passport",
  "document_side": "front",
  "status": "in_review",
  "file_id": "s3:kyc/...",
  "issuing_country": "DE",
  "expiry_date": "2030-05-01T00:00:00Z",
  "submitted_at": "2026-09-30T08:00:00Z",
  "reviewed_at": null,
  "reviewed_by": null,
  "rejection_reason": null,
  "required_actions": [],
  "extracted_data": {},
  "created_at": "2026-09-30T08:00:00Z",
  "updated_at": "2026-09-30T08:00:00Z"
}
```

`document_number` is never returned. Admin file access is via short-lived
pre-signed URLs, audit-logged.

---

## Errors

Standard envelope: `{ "error": { "code", "message", "details" }, "meta": {...} }`.
KYC codes live in the `KYC_*` range (7000–7999):

| Code | HTTP | gRPC | Meaning |
|------|------|------|---------|
| `KYC_DOCUMENT_NOT_FOUND` | 404 | NotFound | id unknown or not owned by caller |
| `KYC_DOCUMENT_NOT_DRAFT` | 409 | FailedPrecondition | delete/modify non-draft |
| `KYC_DOCUMENT_EXISTS` | 409 | AlreadyExists | active doc of type+side exists |
| `KYC_INVALID_TRANSITION` | 409 | FailedPrecondition | illegal status change |
| `KYC_DOCUMENT_EXPIRED` | 422 | FailedPrecondition | past `expiry_date` |
| `KYC_ALREADY_VERIFIED` | 409 | AlreadyExists | upgrade at/below current level |
| `KYC_WEBHOOK_INVALID_SIGNATURE` | 401 | Unauthenticated | HMAC mismatch |
| `VALIDATION_FAILED` | 400 | InvalidArgument | field errors in `details` |

---

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| `POST /api/v1/kyc/documents` | 10/hour per user |
| `POST /api/v1/kyc/admin/*` | 100/hour per admin |
| `POST /api/v1/kyc/webhook/*` | 300/min per provider IP |

`429` with `Retry-After` header on breach.

---

## Events (Redpanda, published by KYC)

- `users.kyc_verified` — `{user_id, level}` (consumed by payment limits, affiliate payout gates, notification)
- `kyc.document_rejected` — `{user_id, document_id, reason}` (user notification)

Consumed: provider review callbacks via webhook endpoint (idempotent by provider event id).

---

## Related

- Proto: `libs/proto/kyc/v1/kyc.proto`
- Schema: `libs/migrations/postgresql/010_kyc_rg.sql` (central tables, DATA_ENGINEER-owned)
- Service: `services/go/kyc/` (Go, Fiber, GORM, ports 8089/50059)
- Admin UI: KYC queue, expiry monitor, screening queue (`apps/admin`)
- Compliance: `docs/compliance/gambling-compliance.md`
