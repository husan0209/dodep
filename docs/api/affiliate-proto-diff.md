# Affiliate Proto Diff — ReverseCommission + MarkAffiliatePayoutPaid

**File:** `libs/proto/affiliate/v1/affiliate.proto`
**Change type:** additive only (no breaking changes)
**Package:** `affiliate.v1` — preserved
**Codegen:** NOT regenerated (`gen/` untouched — backend regenerates on its side)

## Why

The Go affiliate-service already implements both operations in the service
layer (`internal/service/commission_engine.go`):

- `ReverseEarning(ReverseEarningInput{EarningID, Reason})` — flips an
  accrued/pending/available earning to `reversed`, emits
  `affiliate.commission.reversed`, mirrors the move in the affiliate ledger
  (`earning_reversal` entries, `reverse:<id>:out/in` idempotency keys).
  Paid earnings are rejected with `ErrEarningNotReversible`.
- `MarkPayoutPaid(MarkPayoutPaidInput{PayoutID, ProviderReference})` — moves
  an approved/processing payout to `paid` (ledger `available -> paid`,
  `payout-paid:<id>:out/in` idempotency keys, outbox topic
  `affiliate.payout.paid`).

The gRPC contract was missing both methods, so admin/finance tooling had no
typed internal API for them. This diff closes that gap.

## Diff (additive)

```proto
service AffiliateService {
  // ... 13 existing RPCs unchanged ...
  rpc ReverseCommission(ReverseCommissionRequest) returns (ReverseCommissionResponse);
  rpc MarkAffiliatePayoutPaid(MarkAffiliatePayoutPaidRequest) returns (MarkAffiliatePayoutPaidResponse);
}

message ReverseCommissionRequest {
  string earning_id = 1;  // earning UUID
  string reason = 2;      // mandatory audit reason
}
message ReverseCommissionResponse {
  AffiliateEarning earning = 1;  // status = REVERSED
}

message MarkAffiliatePayoutPaidRequest {
  string payout_id = 1;           // payout UUID (approved/processing)
  string provider_reference = 2;  // PSP settlement reference, optional
}
message MarkAffiliatePayoutPaidResponse {
  AffiliatePayout payout = 1;  // status = PAID
}
```

## Backward-compatibility analysis (buf breaking = FILE)

| Rule | Status |
| ---- | ------ |
| Package `affiliate.v1` unchanged | ✅ |
| No field removed / renumbered / retyped | ✅ (only new messages) |
| No RPC removed or signature-changed | ✅ (only new RPCs appended) |
| No enum value removed | ✅ (no enum touched) |
| New field numbers start at 1 inside new messages | ✅ (no collisions possible) |
| Money stays `common.v1.Money` (decimal strings, NEVER-6) | ✅ (no new money fields; amounts ride inside `AffiliateEarning` / `AffiliatePayout`) |
| Timestamps stay `google.protobuf.Timestamp` | ✅ (inside reused messages) |

Safe for old clients: unknown RPCs are never called by them; unknown
messages are never sent to them.

## buf lint compatibility (`libs/proto/buf.yaml`: DEFAULT + COMMENTS)

- Every new RPC, message, and field carries a `//` doc comment
  (COMMENTS category: `COMMENT_RPC`, `COMMENT_MESSAGE`, `COMMENT_FIELD`) —
  verified: `buf lint` reports zero violations on the added lines
  (service block ~30–35, messages ~314–351).
- Pre-existing debt (not touched): the committed file already fails
  COMMENTS on all original RPCs/messages/fields and reports one
  `IMPORT_USED` notice on `common/v1/money.proto`; fixing those is a
  separate cleanup and would touch lines outside this diff.
- Naming: `PascalCase` RPC/messages, `snake_case` fields,
  per-RPC `Request`/`Response` suffix. Enums untouched, so the
  `*_UNSPECIFIED = 0` rule is unaffected.

## Examples

### ReverseCommission

```jsonc
// ReverseCommissionRequest
{
  "earning_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "reason": "chargeback on referred player 4815162342, PSP case CB-9921"
}
```

```jsonc
// ReverseCommissionResponse (earning.status = EARNING_STATUS_REVERSED = 5)
{
  "earning": {
    "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "affiliate_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "referred_user_id": "4815162342",
    "commission_amount": { "amount": "12.50000000", "currency": "USD" },
    "status": "EARNING_STATUS_REVERSED"
  }
}
```

Error cases (backend service errors, unchanged by this diff):

- unknown `earning_id` → `EarningNotFound`
- earning already `paid`/`reversed` → `EarningNotReversible`
- empty `reason` → validation error (reason is mandatory for audit)

### MarkAffiliatePayoutPaid

```jsonc
// MarkAffiliatePayoutPaidRequest
{
  "payout_id": "9b2d3f1a-4c5e-4f6a-8b7c-9d0e1f2a3b4c",
  "provider_reference": "psp_settle_2026_04_88123"
}
```

```jsonc
// MarkAffiliatePayoutPaidResponse (payout.status = PAYOUT_STATUS_PAID = 5)
{
  "payout": {
    "id": "9b2d3f1a-4c5e-4f6a-8b7c-9d0e1f2a3b4c",
    "affiliate_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "amount": { "amount": "250.00000000", "currency": "USD" },
    "status": "PAYOUT_STATUS_PAID",
    "provider_reference": "psp_settle_2026_04_88123"
  }
}
```

Error cases:

- unknown `payout_id` → `PayoutNotFound`
- payout in `requested`/`reviewing`/`rejected`/`failed` → `InvalidPayoutStatus`
  (must be `approved` or `processing`)
- empty `provider_reference` is allowed (stored as-is, reconciliation degrades
  to payout-id matching)

## Follow-ups (NOT in this diff, backend scope)

1. gRPC handler wiring: affiliate-service currently serves REST (Fiber) and
   has no `internal/handler` gRPC server; map the two new methods to
   `ReverseEarning` / `MarkPayoutPaid` when the gRPC server lands.
2. REST parity (optional): `POST /admin/affiliates/earnings/:id/reverse`
   `{reason}` and `POST /admin/affiliates/payouts/:id/paid`
   `{provider_reference}` mirroring the existing approve/reject routes in
   `services/go/affiliate/admin_routes.go`.
3. Regenerate `gen/` stubs (`buf generate`) after this proto lands.
