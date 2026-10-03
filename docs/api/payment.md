# Payment Service API

PSP integration layer (fiat + crypto). Implementation:
`services/go/payment/` (Go + Fiber + GORM). Proto:
`libs/proto/payment/v1/payment.proto`.

Ports: HTTP `8084`, gRPC `50055` (see `CONVENTIONS.md`).

## Principles

- Payment Service orchestrates; Wallet Core owns the money (credit/debit
  only via wallet gRPC, never local balance math).
- Every initiation carries `idempotency_key` (UUID); duplicate posts return
  the original result.
- `completed` is final — reversals are new refund transactions, never
  in-place state rewrites.
- Webhooks verify PSP signatures before touching state.

## REST (`/api/v1/payments`, JWT required unless noted)

| Method | Path | Description |
| ------ | ---- | ----------- |
| POST | `/deposit` | Initiate deposit (`amount` decimal string, `currency`, `method`, `return_url`) |
| GET | `/:id` | Deposit/payment status |
| GET | `/history` | Deposit history (filters + pagination) |
| POST | `/withdraw` | Request withdrawal (`amount`, `currency`, `method`, `destination`) |
| GET | `/withdrawals/history` | Withdrawal history |
| GET | `/withdrawals/:id` | Withdrawal status |
| POST | `/webhooks/nowpayments` | NOWPayments webhook (public, signature-verified) |

Health: `GET /healthz`, `GET /readyz`.

## gRPC (`payment.v1.PaymentService`)

`GetPaymentMethods`, `CreateDeposit`, `GetDeposit`, `ListDeposits`,
`RequestWithdrawal`, `GetWithdrawal`, `ListWithdrawals`,
`CancelWithdrawal`, `GetPaymentMethod`, `SavePaymentMethod`,
`DeletePaymentMethod`.

## State machines

Deposit: `initiated → processing → completed | failed`, or
`processing → requires_review → completed | rejected`.
Withdrawal: requested → reviewing → approved → processing → paid, or
rejected/failed. Admin approve/reject lives in the Admin Panel finance
queue (`apps/admin` withdrawals workflow).

## KYC gating

Withdrawals above the KYC level limits are blocked (`PAYMENT_KYC_REQUIRED`).
Level limits are enforced against the User/KYC services, not locally.

## Error codes

`PAYMENT_METHOD_UNAVAILABLE`, `PAYMENT_AMOUNT_TOO_LOW/HIGH`,
`PAYMENT_DAILY_LIMIT_EXCEEDED`, `PAYMENT_KYC_REQUIRED`,
`PAYMENT_PROVIDER_ERROR`, `PAYMENT_DECLINED`,
`PAYMENT_WAGERING_INCOMPLETE`.

## Consumers

- Web wallet (`apps/web` deposit/withdraw forms + transaction history).
- Mobile wallet (`apps/mobile` wallet feature).
- Admin BFF finance handlers + Admin Panel finance pages.
- Affiliate payouts reuse PSP integrations through a separate payout flow
  (affiliate earnings never touch the player wallet).
