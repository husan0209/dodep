# Requirements Document

## Introduction

Интеграция NOWPayments для приёма крипто-платежей (депозиты и выводы) на гемблинг-платформе с 10M+ пользователей. Интеграция включает обработку webhooks, конвертацию курсов, проверку KYC лимитов и полную audit-трейсабилити всех операций.

## Glossary

- **Payment_Service**: Go-сервис, отвечающий за интеграцию с платежными провайдерами и оркестрацию платежных операций
- **Wallet_Service**: Rust-сервис, управляющий балансами пользователей и ledger-записями
- **NOWPayments**: Крипто-платёжный провайдер (PSP) для приёма BTC, ETH, USDT и других криптовалют
- **IPN**: Instant Payment Notification — webhook-уведомления от NOWPayments о статусе платежа
- **Idempotency_Key**: Уникальный идентификатор операции для обеспечения идемпотентности
- **KYC_Level**: Уровень верификации пользователя (0-3), определяющий лимиты депозитов/выводов
- **Ledger_Entry**: Запись в double-entry bookkeeping системе для каждой финансовой операции
- **Fiat_Currency**: Фиатная валюта (USD, EUR) для отображения балансов
- **Crypto_Currency**: Криптовалюта (BTC, ETH, USDT, и т.д.) для проведения платежей

## Requirements

### Requirement 1: Create Deposit Payment Request

**User Story:** As a player, I want to create a crypto deposit request, so that I can fund my account with cryptocurrency.

#### Acceptance Criteria

1. WHEN a user initiates a deposit, THE Payment_Service SHALL validate the user's KYC level and applicable deposit limits
2. WHEN a valid deposit request is received, THE Payment_Service SHALL call NOWPayments API to create a payment with unique payment_id
3. WHEN NOWPayments returns a payment address, THE Payment_Service SHALL store the payment record with status "pending" and return the deposit address to the user
4. THE Payment_Service SHALL generate a unique idempotency_key for each deposit request and store it in the payment record
5. IF the same idempotency_key is received, THE Payment_Service SHALL return the existing payment record without creating a new one

### Requirement 2: Process Deposit Webhook (IPN)

**User Story:** As a platform operator, I want deposit webhooks processed reliably, so that user balances are updated correctly when crypto payments are confirmed.

#### Acceptance Criteria

1. WHEN an IPN webhook is received, THE Payment_Service SHALL verify the HMAC signature using the IPN secret
2. WHEN the signature is invalid, THE Payment_Service SHALL reject the webhook with HTTP 401 and log a security alert
3. WHEN a valid webhook indicates payment status "finished", THE Payment_Service SHALL credit the user's wallet via Wallet_Service gRPC call
4. WHEN crediting the wallet, THE Payment_Service SHALL use the NOWPayments payment_id as the idempotency_key
5. IF the same payment_id is processed again, THE Payment_Service SHALL return success without duplicating the credit operation
6. WHEN the wallet credit completes, THE Payment_Service SHALL update the payment record status to "completed" and publish a payments.completed event to Redpanda
7. WHEN a webhook indicates payment status "failed" or "expired", THE Payment_Service SHALL update the payment record status accordingly and publish a payments.failed event

### Requirement 3: Create Withdrawal Request

**User Story:** As a player, I want to withdraw funds to my crypto wallet, so that I can access my winnings.

#### Acceptance Criteria

1. WHEN a user initiates a withdrawal, THE Payment_Service SHALL verify the user's KYC level is at least 2
2. IF the user's KYC level is below 2, THE Payment_Service SHALL reject the withdrawal with error PAYMENT_KYC_REQUIRED
3. WHEN KYC validation passes, THE Payment_Service SHALL check the user's available balance via Wallet_Service
4. IF the available balance is insufficient, THE Payment_Service SHALL reject the withdrawal with error WALLET_INSUFFICIENT_BALANCE
5. WHEN balance validation passes, THE Payment_Service SHALL lock the withdrawal amount in the user's wallet
6. WHEN funds are locked, THE Payment_Service SHALL create a withdrawal request in NOWPayments API
7. WHEN NOWPayments returns a withdrawal_id, THE Payment_Service SHALL store the withdrawal record with status "processing" and the generated idempotency_key
8. THE Payment_Service SHALL enforce daily withdrawal limits based on KYC level

### Requirement 4: Process Withdrawal Webhook (IPN)

**User Story:** As a platform operator, I want withdrawal webhooks processed reliably, so that user balances are correctly debited when withdrawals complete.

#### Acceptance Criteria

1. WHEN an IPN webhook for withdrawal is received, THE Payment_Service SHALL verify the HMAC signature using the IPN secret
2. WHEN a valid webhook indicates withdrawal status "finished", THE Payment_Service SHALL finalize the wallet debit via Wallet_Service gRPC call
3. WHEN finalizing the debit, THE Payment_Service SHALL use the NOWPayments withdrawal_id as the idempotency_key
4. IF the same withdrawal_id is processed again, THE Payment_Service SHALL return success without duplicating the debit operation
5. WHEN the wallet debit finalizes, THE Payment_Service SHALL update the withdrawal record status to "completed" and publish a payments.withdrawal.completed event
6. WHEN a webhook indicates withdrawal status "failed", THE Payment_Service SHALL unlock the locked funds and update the withdrawal record status to "failed"
7. WHEN funds are unlocked after failure, THE Payment_Service SHALL notify the user and publish a payments.withdrawal.failed event

### Requirement 5: Crypto-to-Fiat Currency Conversion

**User Story:** As a player, I want to see deposit and withdrawal amounts in fiat currency, so that I understand the value of my transactions.

#### Acceptance Criteria

1. WHEN displaying a deposit amount, THE Payment_Service SHALL convert the crypto amount to fiat using NOWPayments estimated exchange rate
2. WHEN displaying a withdrawal amount, THE Payment_Service SHALL show the equivalent fiat value at the current exchange rate
3. THE Payment_Service SHALL cache exchange rates in DragonflyDB with a TTL of 60 seconds
4. IF the exchange rate cache is stale or missing, THE Payment_Service SHALL fetch fresh rates from NOWPayments API
5. THE Payment_Service SHALL store both the crypto amount and fiat equivalent in the payment record
6. WHEN the actual received crypto amount differs from the requested amount, THE Payment_Service SHALL credit the actual received amount and log the difference

### Requirement 6: KYC Level Limits Enforcement

**User Story:** As a compliance officer, I want deposit and withdrawal limits enforced by KYC level, so that the platform complies with AML regulations.

#### Acceptance Criteria

1. WHEN processing a deposit, THE Payment_Service SHALL check the user's KYC level and apply the corresponding daily deposit limit
2. WHEN processing a withdrawal, THE Payment_Service SHALL check the user's KYC level and apply the corresponding daily withdrawal limit
3. THE Payment_Service SHALL maintain the following limits by KYC level:
   - KYC Level 0: Deposit $500/day, Withdrawal $0/day
   - KYC Level 1: Deposit $2,000/day, Withdrawal $500/day
   - KYC Level 2: Deposit $10,000/day, Withdrawal $5,000/day
   - KYC Level 3: Deposit $50,000/day, Withdrawal $25,000/day
4. IF a transaction would exceed the daily limit, THE Payment_Service SHALL reject it with error PAYMENT_DAILY_LIMIT_EXCEEDED
5. THE Payment_Service SHALL track daily cumulative amounts per user per operation type in DragonflyDB with expiry at midnight UTC

### Requirement 7: Idempotency for All Write Operations

**User Story:** As a platform operator, I want all payment operations to be idempotent, so that retries and duplicate requests do not cause financial errors.

#### Acceptance Criteria

1. WHEN any write operation is received, THE Payment_Service SHALL check DragonflyDB for an existing idempotency_key
2. IF an existing result is found, THE Payment_Service SHALL return the cached response without re-executing the operation
3. WHEN a new operation completes successfully, THE Payment_Service SHALL store the result in DragonflyDB with the idempotency_key and a 24-hour TTL
4. THE Payment_Service SHALL also enforce a UNIQUE constraint on idempotency_key in PostgreSQL as a safety net
5. IF a database unique constraint violation occurs, THE Payment_Service SHALL fetch and return the existing record

### Requirement 8: Audit Logging

**User Story:** As a compliance officer, I want all payment operations logged, so that the platform can demonstrate regulatory compliance and investigate issues.

#### Acceptance Criteria

1. WHEN any payment operation is initiated, THE Payment_Service SHALL create an audit log entry with user_id, operation_type, amount, currency, and timestamp
2. WHEN a payment status changes, THE Payment_Service SHALL log the transition with previous_status, new_status, and reason
3. WHEN a webhook is received, THE Payment_Service SHALL log the raw payload (excluding sensitive data), signature validation result, and processing outcome
4. THE Payment_Service SHALL write audit logs to ClickHouse via Redpanda topic payments.audit
5. THE Payment_Service SHALL include trace_id and correlation_id in all audit log entries for distributed tracing
6. THE Payment_Service SHALL mask sensitive data in logs: wallet addresses truncated to first 8 and last 4 characters

### Requirement 9: Error Handling and Retry Logic

**User Story:** As a platform operator, I want payment errors handled gracefully, so that users receive clear feedback and operations can be retried safely.

#### Acceptance Criteria

1. WHEN NOWPayments API returns a 5xx error, THE Payment_Service SHALL retry the request with exponential backoff (max 3 retries)
2. WHEN NOWPayments API returns a 4xx error, THE Payment_Service SHALL NOT retry and shall return an appropriate error to the user
3. WHEN a webhook processing fails after all retries, THE Payment_Service SHALL store the webhook in a dead-letter queue for manual review
4. WHEN Wallet_Service gRPC call fails, THE Payment_Service SHALL retry with backoff and log the failure
5. IF Wallet_Service is unavailable after all retries, THE Payment_Service SHALL mark the payment as "requires_review" and alert operations team
6. THE Payment_Service SHALL return error codes in the PAYMENT_* namespace (5000-5999) as defined in API standards

### Requirement 10: Supported Cryptocurrencies

**User Story:** As a player, I want to deposit and withdraw in multiple cryptocurrencies, so that I can use my preferred crypto asset.

#### Acceptance Criteria

1. THE Payment_Service SHALL support the following cryptocurrencies for deposits: BTC, ETH, USDT (ERC-20, TRC-20), USDC, LTC, BCH
2. THE Payment_Service SHALL support the following cryptocurrencies for withdrawals: BTC, ETH, USDT (ERC-20, TRC-20), USDC, LTC, BCH
3. WHEN a user requests available currencies, THE Payment_Service SHALL fetch the list from NOWPayments API and cache it for 10 minutes
4. IF a currency is temporarily disabled by NOWPayments, THE Payment_Service SHALL exclude it from the available list
5. THE Payment_Service SHALL validate that the withdrawal address matches the expected format for the selected cryptocurrency
