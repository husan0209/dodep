# Affiliate Service

## Purpose

`affiliate-service` owns the affiliate program domain for Opus Casino.

It manages:

- affiliate enrollment and approval flow;
- affiliate profiles and commission plans;
- referral links, clicks, and player attribution;
- NGR-based commission accrual and hold release;
- affiliate payout methods and payout requests;
- affiliate fraud flags and payout gating;
- affiliate-facing dashboard aggregates and admin workflows.

This service follows the platform rule that affiliate earnings are a separate
financial domain and must not be mixed with the player's gaming balance.

## Core Business Model

The MVP commission model is `revshare from NGR`.

```text
casino_ggr = total_bets - total_wins
sports_ggr = settled_stakes - payouts - refunds - voids - cashout_adjustments

ngr = ggr
    - bonuses
    - payment_fees
    - chargebacks
    - fraud_writeoffs
    - taxes
    - manual_adjustments

affiliate_commission = max(0, ngr * commission_rate)
```

MVP rules:

- use `max(0, ...)`;
- do not introduce negative carryover on the first release;
- accrue only from settled and finalized upstream data;
- one referred player can belong to only one affiliate;
- self-referral is forbidden.

## Service Boundaries

`affiliate-service` owns its PostgreSQL schema and publishes its own events.

Synchronous dependencies:

- `user-service`: user profile, affiliate eligibility, referred user binding;
- `payment-service`: affiliate payout execution through a dedicated payout flow;
- `wallet-core`: separate affiliate ledger or accounting integration;
- `kyc-service` and `fraud-service`: payout gates and review inputs.

Asynchronous dependencies:

- `users.user.registered`
- `payments.deposit.completed`
- `bets.bet.settled`
- `casino.game.round.completed`
- `bonus.wagering.progress_updated`
- `fraud.alert.created`

The service publishes affiliate-specific topics through the outbox pattern.

## gRPC Contract

The internal contract is defined in:

- `libs/proto/affiliate/v1/affiliate.proto`

The contract keeps the required task methods:

- `EnrollAffiliate`
- `GetAffiliateProfile`
- `GetAffiliateDashboard`
- `CreateAffiliateLink`
- `ListAffiliateLinks`
- `TrackAffiliateClick`
- `BindReferredUser`
- `CalculateCommission`
- `ListAffiliateEarnings`
- `RequestAffiliatePayout`
- `ApproveAffiliatePayout`
- `RejectAffiliatePayout`
- `FlagAffiliateFraud`

Notable contract decisions:

- payout request is explicit: `amount + idempotency_key`;
- commission plan settings are snapshotted onto the affiliate profile;
- payout methods and fraud flags are first-class domain objects;
- dashboard stays aggregate-oriented rather than exposing raw ledger rows.

## PostgreSQL Model

The schema is defined in:

- `libs/migrations/postgresql/021_affiliates.sql` (centralised)

Main tables:

- `affiliate_enrollment_requests`
- `affiliate_commission_plans`
- `affiliate_profiles`
- `affiliate_links`
- `affiliate_clicks`
- `affiliate_attributions`
- `affiliate_daily_aggregates`
- `affiliate_earnings`
- `affiliate_adjustments`
- `affiliate_payout_methods`
- `affiliate_payouts`
- `affiliate_fraud_flags`
- `affiliate_ledger_accounts`
- `affiliate_ledger_entries`
- `affiliate_outbox`

Important consistency rules:

- pending enrollment is unique per `user_id` via partial unique index;
- affiliate profile is unique per `user_id`;
- affiliate code is globally unique;
- payout and earning writes use `idempotency_key`;
- earnings default to `accrued`, then move to `available` after hold release;
- all event publication is backed by `affiliate_outbox`.

## Ledger Model

Affiliate money is tracked separately from the player wallet.

Ledger accounts per affiliate and currency:

- `pending`
- `available`
- `paid`
- `reversed`
- `adjusted`

Typical flow:

```text
commission accrued:
  credit pending

hold release:
  debit pending
  credit available

payout approved and paid:
  debit available
  credit paid

reversal:
  debit pending|available
  credit reversed

manual adjustment:
  debit|credit adjusted
  offset pending|available based on action
```

Every ledger mutation must:

- use decimal values;
- be idempotent;
- write audit-friendly reference fields;
- be published through outbox after commit;
- be reconcilable against earnings and payouts.

## Event Topics

Affiliate event payloads live in:

- `libs/proto/events/v1/events.proto`

Topics used by this domain:

- `affiliate.click.tracked`
- `affiliate.attribution.created`
- `affiliate.player.ftd`
- `affiliate.commission.accrued`
- `affiliate.commission.released`
- `affiliate.commission.reversed`
- `affiliate.payout.requested`
- `affiliate.payout.paid`
- `affiliate.fraud.flagged`

Recommended additional operational topics during implementation:

- `affiliate.enrollment.requested`
- `affiliate.enrollment.approved`
- `affiliate.enrollment.rejected`
- `affiliate.payout.approved`
- `affiliate.payout.rejected`

Publishing rules:

- write domain state and outbox row in one DB transaction;
- publish asynchronously from outbox worker;
- use affiliate or aggregate identifier as message key;
- consumers must be idempotent.

## Runtime Outbox Worker

`affiliate-service` now starts an internal outbox worker on boot.

Behavior:

- polls `affiliate_outbox` for unpublished events;
- publishes events via the configured `Publisher` implementation;
- marks successful events as published;
- increments retry counter on publish failure.

Current default publisher:

- `LogPublisher` (safe fallback): logs event payload and marks delivered.

For production you should replace it with a real broker publisher
(Redpanda/Kafka) by implementing `internal/event.Publisher`.

Worker env configuration:

- `OUTBOX_POLL_INTERVAL` (default: `2s`)
- `OUTBOX_BATCH_SIZE` (default: `100`)

## Backend Plan

Phase 1: contracts and domain

- finalize `affiliate.proto`;
- define domain status transitions;
- define commission, payout, and fraud invariants;
- add explicit idempotency on payout and accrual flows.

Phase 2: persistence and consistency

- finalize PostgreSQL schema;
- implement repositories;
- add outbox worker;
- add ledger reconciliation job hooks.

Phase 3: tracking and attribution

- resolve referral links;
- track clicks and campaign metadata;
- bind referred players on registration;
- mark FTD qualification on first successful deposit.

Phase 4: commission engine

- consume finalized activity events;
- calculate GGR and NGR inputs per referred player;
- accrue commissions with hold;
- release held earnings on schedule;
- support reversals and manual adjustments.

Phase 5: payout orchestration

- manage payout methods;
- validate KYC and fraud gates;
- request payout with idempotency;
- approve or reject via admin flow;
- trigger dedicated affiliate payout execution.

Phase 6: observability and controls

- add structured logs and metrics;
- add dashboard aggregates;
- add reconciliation and alerting;
- add audit trail for admin actions.

## Web Pages

Affiliate cabinet pages for `apps/web`:

- `/affiliate`
- `/affiliate/dashboard`
- `/affiliate/links`
- `/affiliate/reports`
- `/affiliate/earnings`
- `/affiliate/payouts`
- `/affiliate/settings`

Key widgets:

- earnings summary;
- funnel conversion;
- GGR, NGR, commission trend;
- payout status timeline;
- referral link builder and UTM presets.

## Admin Pages

Admin panel pages for `apps/admin`:

- `Affiliates / List`
- `Affiliates / Detail`
- `Affiliates / Commission Plans`
- `Affiliates / Payout Queue`
- `Affiliates / Fraud Flags`
- `Affiliates / Adjustments`

Admin actions:

- approve, reject, suspend affiliate;
- change plan or override commission rate;
- review and approve payouts;
- add manual adjustments;
- resolve fraud flags;
- inspect audit history.

## Anti-Fraud Rules

Mandatory affiliate checks:

- self-referral detection;
- same KYC identity between affiliate and referred player;
- same payment destination between source traffic and affiliate payout target;
- device fingerprint overlap;
- suspicious IP cluster overlap;
- abnormal click-to-registration ratio;
- chargeback-heavy referred traffic;
- bonus abuse concentration among referred players.

System actions:

- create fraud flag;
- block payout request or approval;
- keep earnings in hold or reverse them;
- move affiliate to review or suspended state;
- emit audit and affiliate fraud events.

## Implementation Notes

Do not use:

- RTP as an affiliate control parameter;
- player wallet as the source of truth for affiliate money;
- auto-approve for MVP;
- instant payout for MVP;
- sub-affiliate or multi-level structures in MVP.
