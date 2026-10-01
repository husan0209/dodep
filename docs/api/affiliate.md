# Affiliate Service API

Partner program on the `revshare from NGR` model. Implementation:
`services/go/affiliate/`. Proto: `libs/proto/affiliate/v1/affiliate.proto`.
Migration: `libs/migrations/postgresql/021_affiliates.sql`.

Ports: HTTP `8090`, gRPC `50061` (see `services/go/affiliate/internal/config/config.go`).

## Financial model

```text
casino_ggr = total_bets - total_wins
sports_ggr = settled_stakes - payouts - refunds - voids - cashout_adjustments
ngr        = ggr - bonuses - payment_fees - chargebacks - fraud_writeoffs - taxes - manual_adjustments
commission = max(0, ngr * commission_rate)
```

MVP rules: no negative carryover, accrual only from settled/finalized data,
one referred player maps to exactly one affiliate, self-referral forbidden.
Affiliate money lives in a separate ledger and never mixes with the player
gaming balance. RTP is not an affiliate control parameter.

## Auth

- Partner endpoints: `Authorization: Bearer <JWT>`, `user_id` is taken from
  the token only (`middleware.go`), never from the body.
- Admin endpoints: admin JWT (`AdminMiddleware`).
- Mutating payout endpoints expect `X-Idempotency-Key: <uuid>`.

## Partner REST (`/api/v1/affiliate`)

| Method | Path | Description |
| ------ | ---- | ----------- |
| POST | `/enroll` | Request affiliate enrollment (`{reason}`) → 201 enrollment request |
| GET | `/profile` | Own affiliate profile (404 if none) |
| GET | `/dashboard` | Earnings/funnel aggregates (see below) |
| GET | `/links` | List referral links → `{links: [...]}` |
| POST | `/links` | Create link (`campaign_name`, `landing_page`, `utm_*`) → 201 |
| GET | `/earnings?status=&limit=` | Commission earnings |
| GET | `/reports` | `{summary, earnings}` for the cabinet reports tab |
| GET | `/payouts?status=&page=&page_size=` | Own payouts, paginated `{data, pagination}` |
| GET | `/payout-methods` | List payout methods → `{methods: [...]}` |
| POST | `/payout-methods` | Add method (`method_type`, `display_name`, `details_masked`, `is_default`) → 201 |
| PUT | `/payout-methods/:id` | Update method |
| POST | `/payouts/request` | Request payout (`method_id`, `amount` decimal string, `idempotency_key`) → 201. KYC gate via `X-KYC-Approved` header |

## Tracking (no auth)

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/r/:affiliate_code` | Track click, set `aff_click` cookie (30d, HttpOnly/Secure/Lax), 302 to `?landing=` |
| GET | `/r/:affiliate_code/:campaign` | Same with campaign attribution |

Tracking never blocks the user: on error it silently 302-redirects.
IP/User-Agent are stored as hashes, device fingerprint from
`X-Device-Fingerprint`, country from `CF-IPCountry`.

## Admin REST (`/admin/affiliates`)

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/` | List profiles (`?status=&page=&page_size=`) |
| GET | `/:id` | Profile + dashboard |
| POST | `/:user_id/approve` | Approve enrollment (plan/rate/hold/min-payout/currency override) → 201 profile |
| POST | `/:user_id/reject` | Reject enrollment (`review_notes`) |
| POST | `/:id/suspend` | Suspend affiliate |
| PUT | `/:id/commission-rate` | Override rate (`commission_rate` decimal string) |
| POST | `/:id/adjustments` | Manual credit/debit (`adjustment_type`, `amount`, `reason`) → 201 |
| GET | `/payouts?status=&page=&page_size=` | Global payout queue |
| POST | `/payouts/:id/approve` | Approve payout (`provider_reference`) |
| POST | `/payouts/:id/reject` | Reject payout (`rejection_reason`) |
| GET | `/fraud-flags?status=&page=&page_size=` | Fraud flag queue |
| POST | `/:id/fraud-flag` | Manual flag (`referred_user_id`, `flag_type`, `severity`, `details`) → 201 |

## gRPC (`affiliate.v1.AffiliateService`)

`EnrollAffiliate`, `GetAffiliateProfile`, `GetAffiliateDashboard`,
`CreateAffiliateLink`, `ListAffiliateLinks`, `TrackAffiliateClick`,
`BindReferredUser`, `CalculateCommission`, `ListAffiliateEarnings`,
`RequestAffiliatePayout`, `ApproveAffiliatePayout`, `RejectAffiliatePayout`,
`FlagAffiliateFraud`. Handler: `services/go/affiliate/internal/grpc/`.

## Status models

- Profile: `pending | active | suspended | rejected | closed`.
- Earning: `accrued → pending → available → paid`, or `reversed`.
  Hold release moves `pending → available` after `hold_period_days`.
- Payout: `requested → reviewing → approved → processing → paid`,
  or `rejected | failed`.
- Defaults: rate `20%`, hold `14d`, min payout `100 USD`, manual approval,
  monthly schedule.

## Error mapping (REST)

`409` duplicate/pending enrollment, attribution conflict; `404` unknown
profile/link/method/payout; `400` self-referral, bad amount, min-payout,
bad status, bad commission; `403` KYC required, fraud block, inactive
affiliate; `500` anything else. See `handleDomainError` in `routes.go`.

## Events (Redpanda, via outbox)

Published: `affiliate.click.tracked`, `affiliate.attribution.created`,
`affiliate.player.ftd`, `affiliate.commission.accrued`,
`affiliate.commission.released`, `affiliate.commission.reversed`,
`affiliate.payout.requested`, `affiliate.payout.paid`,
`affiliate.fraud.flagged`, plus enrollment requested/approved/rejected and
payout approved/rejected. Outbox worker polls `affiliate_outbox`
(`OUTBOX_POLL_INTERVAL`, `OUTBOX_BATCH_SIZE`); default publisher logs,
production must implement `internal/event.Publisher` with Redpanda.

## Consumers of this API

- Web cabinet: `apps/web/src/components/pages/affiliate.tsx`, client
  `apps/web/src/lib/api/affiliate.ts`.
- Mobile cabinet: `apps/mobile/lib/features/affiliate/`.
- Admin: `apps/admin/src/pages/affiliates/`, service
  `apps/admin/src/services/affiliates.service.ts`.
