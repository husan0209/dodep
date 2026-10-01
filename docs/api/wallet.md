# Wallet Core API

Single source of truth for all money. Implementation: `services/rust/wallet-core/`
(Rust + Axum + SQLx). Proto: `libs/proto/wallet/v1/wallet.proto`.

Ports: HTTP `8081`, gRPC `50053` (see `CONVENTIONS.md`).

## Principles

- Double-entry bookkeeping: every movement debits one account and credits
  another; `SUM(all ledger entries) = 0`, reconciled hourly (P1 on mismatch).
- Money is `NUMERIC(18,8)` / `Decimal` — never float.
- Balances never go negative (`CHECK(balance >= 0)` + optimistic locking
  via `version` field, retried, plus `SELECT FOR UPDATE` on hot paths).
- Every financial mutation is idempotent: `idempotency_key` (UUID) cached in
  DragonflyDB for 24h and backed by a `UNIQUE` constraint (race safety).
- `transactions` table is append-only (no UPDATE/DELETE), partitioned monthly.

## Accounts

Per `(user_id, currency)`: `available`, `locked` (in bets), `bonus`.
House accounts: `house`, `bonus_pool`, `payment_gateway_transit`,
`tax_reserve`, `revenue`.

## HTTP

Only operational endpoints (`src/api/http.rs`): `GET /health`, `/ready`,
`/live`, `/metrics`. All business operations are gRPC-only.

## gRPC (`wallet.v1.WalletService`)

| Method | Input | Effect |
| ------ | ----- | ------ |
| `GetBalance` | `user_id`, `currency_code` | Returns `available`, `locked`, `bonus`, `total` (< 2ms) |
| `GetWallets` | `user_id` | All currencies |
| `Credit` | `user_id`, `currency`, `amount`, `idempotency_key`, `reference_type/id`, `reason` | Deposit/win/bonus (< 5ms p99) |
| `Debit` | same shape | Bet/withdrawal; fails on insufficient balance |
| `Lock` | `user_id`, `currency`, `amount`, `idempotency_key`, `reference_*` | Freeze for pending bet → `lock_id` |
| `Unlock` | `lock_id`, `idempotency_key` | Release on bet cancel |
| `Settle` | `lock_id`, `settlement_amount`, `idempotency_key` | Atomic unlock + credit/debit on bet result |
| `Transfer` | from/to wallets, `amount`, `idempotency_key` | Internal moves |
| `GetTransactions` | `user_id`, filters, pagination | Audit history |
| `GetTransaction` | `transaction_id` | Single entry |

`reference_type` is one of `bet | payment | bonus | adjustment`;
`reference_id` links the source entity. Every call should set a deadline
(2s budget on the critical path).

## Typical flows

Bet placement: `Lock(stake)` → on result `Settle(lock_id, payout or 0)`.
Cancel: `Unlock(lock_id)`. Deposit webhook: `Credit` with PSP
`idempotency_key`. Withdrawal approve: `Debit`.

## Consumers

- Betting Engine (Rust) — lock/settle.
- Payment Service (Go) — credit/debit on PSP webhooks.
- Bonus Service (Go) — bonus credits.
- Admin BFF — finance views via `payment`/`user` services (no direct wallet
  access from admin handlers; balance adjust TODO pending Wallet gRPC).
