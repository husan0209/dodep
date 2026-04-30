# Tasks: NOWPayments Integration

## Overview

Этот документ содержит задачи для реализации интеграции NOWPayments в Payment Service. Задачи разбиты на фазы и могут выполняться параллельно в рамках каждой фазы.

---

## Phase 1: Foundation

### 1.1 Setup Payment Service Project Structure
- [ ] Create `services/payment-service/` directory structure
- [ ] Initialize Go module with `go.mod`
- [ ] Create `cmd/server/main.go` entry point
- [ ] Create `internal/config/config.go` with configuration loading
- [ ] Create `Dockerfile` and `Makefile`
- [ ] Setup `config/default.yaml` and `config/production.yaml`

### 1.2 Database Migrations
- [ ] Create `migrations/001_create_payments.up.sql` and `.down.sql`
- [ ] Create `migrations/002_create_withdrawals.up.sql` and `.down.sql`
- [ ] Create `migrations/003_create_audit_logs.up.sql` and `.down.sql`
- [ ] Add indexes for common query patterns
- [ ] Test migrations with testcontainers

### 1.3 Domain Types
- [ ] Create `internal/domain/payment.go` with Payment entity and PaymentStatus enum
- [ ] Create `internal/domain/withdrawal.go` with Withdrawal entity and WithdrawalStatus enum
- [ ] Create `internal/domain/crypto_currency.go` with supported currencies
- [ ] Create `internal/domain/errors.go` with domain errors and error codes
- [ ] Add status transition validation methods

---

## Phase 2: Repository Layer

### 2.1 Repository Interfaces
- [ ] Create `internal/repository/interfaces.go` with all repository interfaces
- [ ] Define PaymentRepository interface
- [ ] Define WithdrawalRepository interface
- [ ] Define IdempotencyRepository interface
- [ ] Define ExchangeRateRepository interface
- [ ] Define DailyLimitsRepository interface

### 2.2 PostgreSQL Repositories
- [ ] Create `internal/repository/payment_repo.go` with GORM implementation
- [ ] Create `internal/repository/withdrawal_repo.go` with GORM implementation
- [ ] Create `internal/repository/audit_log_repo.go` for audit logging
- [ ] Add optimistic locking for status updates
- [ ] Add cursor-based pagination for list queries

### 2.3 DragonflyDB Repositories
- [ ] Create `internal/repository/idempotency_repo.go` with Redis client
- [ ] Create `internal/repository/exchange_rate_repo.go` for rate caching
- [ ] Create `internal/repository/daily_limits_repo.go` for limit tracking
- [ ] Implement TTL-based expiration for all cached data

---

## Phase 3: External Clients

### 3.1 NOWPayments Client
- [ ] Create `internal/client/nowpayments_client.go`
- [ ] Implement `CreatePayment` method with retry logic
- [ ] Implement `CreatePayout` method with retry logic
- [ ] Implement `GetEstimatedPrice` for exchange rates
- [ ] Implement `GetCurrencies` for supported currencies
- [ ] Implement `VerifyWebhookSignature` with HMAC-SHA512
- [ ] Add exponential backoff for 5xx errors
- [ ] Add request/response logging (sanitized)

### 3.2 Wallet Service gRPC Client
- [ ] Create `internal/client/wallet_client.go`
- [ ] Implement `GetBalance` method
- [ ] Implement `CreditWallet` method
- [ ] Implement `LockFunds` method
- [ ] Implement `UnlockFunds` method
- [ ] Implement `FinalizeDebit` method
- [ ] Add gRPC interceptors for tracing and logging
- [ ] Add timeout configuration for all calls

### 3.3 User Service gRPC Client
- [ ] Create `internal/client/user_client.go`
- [ ] Implement `GetKYCLevel` method
- [ ] Implement `GetUserStatus` method
- [ ] Add caching for KYC level lookups

---

## Phase 4: Service Layer

### 4.1 Payment Service
- [ ] Create `internal/service/payment_service.go`
- [ ] Implement `InitiateDeposit` with KYC validation
- [ ] Implement `GetPayment` by ID
- [ ] Implement `ListPayments` with pagination
- [ ] Add daily limit checking logic
- [ ] Add idempotency handling

### 4.2 Withdrawal Service
- [ ] Create `internal/service/withdrawal_service.go`
- [ ] Implement `InitiateWithdrawal` with KYC level 2 check
- [ ] Implement balance validation
- [ ] Implement fund locking before payout
- [ ] Implement compensation on payout failure
- [ ] Add daily withdrawal limit checking

### 4.3 Webhook Service
- [ ] Create `internal/service/webhook_service.go`
- [ ] Implement `ProcessDepositWebhook` with signature verification
- [ ] Implement `ProcessWithdrawalWebhook` with signature verification
- [ ] Add idempotency for webhook processing
- [ ] Add dead-letter queue for failed webhooks
- [ ] Log all webhook payloads (sanitized)

### 4.4 Exchange Rate Service
- [ ] Create `internal/service/exchange_rate_service.go`
- [ ] Implement `GetExchangeRate` with caching
- [ ] Add cache refresh on stale data
- [ ] Implement fiat amount conversion

### 4.5 KYC Limits Service
- [ ] Create `internal/service/kyc_limits_service.go`
- [ ] Define KYC level limits (0-3)
- [ ] Implement `CheckDepositLimit` method
- [ ] Implement `CheckWithdrawalLimit` method
- [ ] Track daily cumulative amounts

### 4.6 Idempotency Service
- [ ] Create `internal/service/idempotency_service.go`
- [ ] Implement `CheckOrSet` for DragonflyDB
- [ ] Add PostgreSQL UNIQUE constraint fallback
- [ ] Handle race conditions gracefully

---

## Phase 5: HTTP Handlers

### 5.1 Payment Handler
- [ ] Create `internal/handler/payment_handler.go`
- [ ] Implement `POST /api/v1/payments/deposit` endpoint
- [ ] Implement `GET /api/v1/payments/:id` endpoint
- [ ] Implement `GET /api/v1/payments/history` endpoint
- [ ] Implement `GET /api/v1/payments/methods` endpoint
- [ ] Add request validation with go-playground/validator
- [ ] Add error mapping to HTTP status codes

### 5.2 Withdrawal Handler
- [ ] Create `internal/handler/withdrawal_handler.go`
- [ ] Implement `POST /api/v1/payments/withdraw` endpoint
- [ ] Implement `GET /api/v1/payments/withdrawals/:id` endpoint
- [ ] Implement `GET /api/v1/payments/withdrawals/history` endpoint
- [ ] Add crypto address validation

### 5.3 Webhook Handler
- [ ] Create `internal/handler/webhook_handler.go`
- [ ] Implement `POST /api/v1/payments/webhooks/nowpayments` endpoint
- [ ] Add HMAC signature verification
- [ ] Return 401 for invalid signatures
- [ ] Return 200 for already processed webhooks

### 5.4 Response Helpers
- [ ] Create `internal/handler/response.go`
- [ ] Implement `respondSuccess` helper
- [ ] Implement `respondError` helper
- [ ] Implement `respondPaginated` helper
- [ ] Implement `mapError` for domain to HTTP mapping

### 5.5 Middleware
- [ ] Create `internal/handler/middleware.go`
- [ ] Add authentication middleware
- [ ] Add request ID middleware
- [ ] Add rate limiting middleware
- [ ] Add logging middleware

---

## Phase 6: Event Publishing

### 6.1 Redpanda Producer
- [ ] Create `internal/event/producer.go`
- [ ] Configure producer with acks=all
- [ ] Add tracing headers to all events
- [ ] Implement graceful shutdown

### 6.2 Event Types
- [ ] Create `internal/event/events.go`
- [ ] Define `DepositCompletedEvent`
- [ ] Define `DepositFailedEvent`
- [ ] Define `WithdrawalCompletedEvent`
- [ ] Define `WithdrawalFailedEvent`
- [ ] Define `PaymentAuditEvent`

---

## Phase 7: gRPC Server

### 7.1 gRPC Service
- [ ] Create `internal/grpc/server.go`
- [ ] Implement health check service
- [ ] Add reflection for development
- [ ] Configure interceptors

---

## Phase 8: Testing

### 8.1 Unit Tests
- [ ] Create `internal/service/payment_service_test.go`
- [ ] Create `internal/service/withdrawal_service_test.go`
- [ ] Create `internal/service/webhook_service_test.go`
- [ ] Create `internal/service/kyc_limits_service_test.go`
- [ ] Create `internal/client/nowpayments_client_test.go`
- [ ] Mock all external dependencies

### 8.2 Integration Tests
- [ ] Create `tests/integration/deposit_flow_test.go`
- [ ] Create `tests/integration/withdrawal_flow_test.go`
- [ ] Create `tests/integration/webhook_processing_test.go`
- [ ] Use testcontainers for PostgreSQL and DragonflyDB
- [ ] Test complete flows end-to-end

### 8.3 Property-Based Tests
- [ ] Create `internal/service/payment_service_property_test.go`
- [ ] Test idempotency property (Property 1)
- [ ] Test webhook idempotency (Property 3)
- [ ] Test KYC limits enforcement (Property 10)
- [ ] Test daily limits tracking (Property 13)
- [ ] Configure with min_successes: 100

---

## Phase 9: Observability

### 9.1 Metrics
- [ ] Add Prometheus metrics for all endpoints
- [ ] Add business metrics (deposits, withdrawals, amounts)
- [ ] Add provider latency metrics
- [ ] Add error rate metrics

### 9.2 Logging
- [ ] Configure structured logging with zerolog
- [ ] Add request ID to all logs
- [ ] Add trace ID to all logs
- [ ] Mask sensitive data in logs

### 9.3 Tracing
- [ ] Add OpenTelemetry spans for all operations
- [ ] Trace NOWPayments API calls
- [ ] Trace Wallet Service gRPC calls
- [ ] Trace database operations

---

## Phase 10: Documentation

### 10.1 API Documentation
- [ ] Create OpenAPI spec for all endpoints
- [ ] Document error codes and responses
- [ ] Add examples for all requests

### 10.2 Runbook
- [ ] Create operational runbook
- [ ] Document common error scenarios
- [ ] Document webhook debugging
- [ ] Document manual reconciliation steps

---

## Dependencies

### External
- NOWPayments API (v1)
- Wallet Service (gRPC)
- User Service (gRPC)

### Internal
- PostgreSQL 16
- DragonflyDB
- Redpanda

### Libraries
- gofiber/fiber/v2
- gorm.io/gorm
- redis/go-redis/v9
- twmb/franz-go
- shopspring/decimal
- go-playground/validator/v10
- leanovate/gopter (property testing)
- testcontainers/testcontainers-go

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| NOWPayments API downtime | Retry with exponential backoff, mark as requires_review |
| Webhook replay attacks | HMAC verification, idempotency keys |
| Race conditions on balance | Optimistic locking, DB constraints |
| Exchange rate volatility | Cache with 60s TTL, log differences |
| KYC limit bypass | Check on every request, track daily cumulative |
