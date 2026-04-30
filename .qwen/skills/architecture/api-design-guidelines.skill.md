# SKILL #5 — api-design-guidelines.skill.md

```markdown
# api-design-guidelines.skill.md
# GAMBLING PLATFORM — API DESIGN GUIDELINES
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: All Backend Agents, Frontend Agents

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Two API types:
  REST + JSON — external (client-facing, through Kong gateway)
  gRPC + Protobuf — internal (service-to-service)

REST is consumed by: Web (Next.js), Mobile (Flutter), Admin (React).
gRPC is consumed by: other microservices only.

# ============================================================
# SECTION 2: REST API CONVENTIONS
# ============================================================

```text
BASE URL: https://api.platform.com/api/v1

VERSIONING:
  Path-based: /api/v1, /api/v2
  v1 maintained 12+ months after v2 release
  Breaking changes ONLY in new version

URL NAMING:
  ✅ kebab-case, plural nouns, lowercase
  ✅ GET  /api/v1/sports-events
  ✅ POST /api/v1/bets
  ✅ GET  /api/v1/users/me/bet-history
  ❌ GET  /api/v1/getSportsEvents (verb in URL)
  ❌ POST /api/v1/bet/place (singular + verb)
  ❌ GET  /api/v1/Users (uppercase)

BODY/QUERY NAMING:
  ✅ snake_case everywhere
  ✅ { "user_id": 123, "created_at": "..." }
  ❌ { "userId": 123, "createdAt": "..." }

HTTP METHODS:
  GET     — read only (NEVER modifies state)
  POST    — create resource or execute action
  PUT     — full replacement of resource
  PATCH   — partial update
  DELETE  — remove (soft delete in our case)

STATUS CODES:
  200  Success (with body)
  201  Created (POST that creates, include Location header)
  204  No Content (DELETE success, no body)
  400  Bad request (malformed input)
  401  Unauthorized (missing/invalid token)
  403  Forbidden (valid token, insufficient permission)
  404  Not found
  409  Conflict (duplicate, state conflict)
  422  Unprocessable (valid format, business rule violation)
  429  Too many requests (include Retry-After header)
  500  Internal error (never expose details)
  503  Service unavailable (maintenance/overload)
============================================================
SECTION 3: RESPONSE FORMAT
============================================================
JSON

// Success — single object
{
  "data": {
    "id": 12345,
    "status": "active",
    "balance": "150.00"
  },
  "meta": {
    "request_id": "req_abc123",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}

// Success — list with pagination
{
  "data": [
    { "id": 1, "name": "..." },
    { "id": 2, "name": "..." }
  ],
  "pagination": {
    "cursor": "eyJpZCI6MTAwfQ==",
    "has_more": true,
    "total_count": 1500
  },
  "meta": {
    "request_id": "req_abc123",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}

// Error
{
  "error": {
    "code": "WALLET_INSUFFICIENT_BALANCE",
    "message": "Insufficient balance for this operation",
    "details": {
      "required": "100.00",
      "available": "50.00"
    }
  },
  "meta": {
    "request_id": "req_abc123",
    "timestamp": "2025-01-15T10:30:00.000Z"
  }
}
text

RULES:
  1. ALWAYS wrap in { "data": ... } or { "error": ... }
  2. ALWAYS include "meta" with request_id and timestamp
  3. NEVER return bare arrays (always in "data" wrapper)
  4. NEVER return null for "data" — use empty object {} or empty array []
  5. Money values as STRINGS ("100.00"), never floats
  6. Dates as ISO 8601 with timezone ("2025-01-15T10:30:00.000Z")
  7. IDs as integers in body, as path params in URL
  8. Enum values as snake_case strings ("pending", "in_progress")
============================================================
SECTION 4: PAGINATION
============================================================
text

TWO TYPES:

CURSOR-BASED (preferred for infinite scroll):
  GET /api/v1/bets?cursor=eyJpZCI6MTAwfQ==&page_size=20
  Response: { "pagination": { "cursor": "...", "has_more": true } }
  
  ✅ Consistent results when data changes
  ✅ Efficient for large datasets
  ❌ Can't jump to page N
  
  USE FOR: bet history, transaction history, game history

OFFSET-BASED (for admin tables):
  GET /api/v1/admin/users?page=3&page_size=50
  Response: { "pagination": { "page": 3, "total_pages": 20, "total_count": 1000 } }
  
  ✅ Can jump to any page
  ❌ Inconsistent if data changes between pages
  
  USE FOR: admin panel tables only

DEFAULTS:
  page_size default: 20
  page_size max: 100
  ALWAYS enforce max (never let client request 10000 items)
============================================================
SECTION 5: FILTERING AND SORTING
============================================================
text

FILTERING:
  GET /api/v1/bets?status=won&sport=football&date_from=2025-01-01
  
  Multiple values: ?status=won,lost (comma-separated)
  Date ranges: ?date_from=...&date_to=...
  
  RULES:
  - Unknown filter params → ignore (don't error)
  - Invalid filter values → 400 error
  - Filters are AND logic (all must match)

SORTING:
  GET /api/v1/bets?sort_by=placed_at&sort_order=desc
  
  Allowed sort fields: explicitly defined per endpoint
  Default sort: created_at DESC (newest first)
  
  RULES:
  - Only whitelist sortable fields (prevent SQL injection via sort)
  - Default direction: DESC for dates, ASC for names
============================================================
SECTION 6: gRPC CONVENTIONS
============================================================
text

PACKAGE: platform.{service}.v1
  platform.wallet.v1
  platform.betting.v1
  platform.auth.v1

METHOD NAMING: VerbNoun (PascalCase)
  ✅ GetUser, PlaceBet, CreditWallet, ValidateToken
  ❌ UserGet, Bet, Credit

FIELD NAMING: snake_case
  ✅ user_id, created_at, currency_code
  ❌ userId, createdAt

MONEY FIELDS: string (decimal as string)
  ✅ string amount = 1; // "100.50"
  ❌ double amount = 1; // precision loss
  ❌ int64 amount_cents = 1; // not flexible for crypto

TIMESTAMPS: google.protobuf.Timestamp
  ✅ google.protobuf.Timestamp created_at = 5;
  ❌ string created_at = 5;
  ❌ int64 created_at_unix = 5;

OPTIONAL FIELDS: use optional keyword or wrapper
  ✅ optional string phone = 3;
  ✅ google.protobuf.StringValue phone = 3;

ENUMS: UPPER_SNAKE_CASE with 0 = UNSPECIFIED
  enum BetStatus {
    BET_STATUS_UNSPECIFIED = 0;
    BET_STATUS_PENDING = 1;
    BET_STATUS_ACTIVE = 2;
    BET_STATUS_WON = 3;
    BET_STATUS_LOST = 4;
  }
protobuf

// Example: well-structured proto service
syntax = "proto3";
package platform.wallet.v1;

import "google/protobuf/timestamp.proto";

service WalletService {
  rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
  rpc Debit(DebitRequest) returns (DebitResponse);
  rpc Credit(CreditRequest) returns (CreditResponse);
  rpc Lock(LockRequest) returns (LockResponse);
  rpc Unlock(UnlockRequest) returns (UnlockResponse);
  rpc Settle(SettleRequest) returns (SettleResponse);
  rpc GetTransactions(GetTransactionsRequest) returns (GetTransactionsResponse);
}

message DebitRequest {
  int64 user_id = 1;
  string currency_code = 2;
  string amount = 3;            // "50.00"
  string idempotency_key = 4;   // UUID
  string reference_type = 5;    // "bet", "withdrawal"
  int64 reference_id = 6;
  string reason = 7;
}

message DebitResponse {
  int64 transaction_id = 1;
  string new_balance = 2;       // "100.00"
  google.protobuf.Timestamp processed_at = 3;
}
============================================================
SECTION 7: ERROR CODES
============================================================
text

NAMESPACED by service, numeric ranges:

AUTH_*:     1000-1999
  1001  AUTH_INVALID_CREDENTIALS
  1002  AUTH_TOKEN_EXPIRED
  1003  AUTH_TOKEN_INVALID
  1004  AUTH_2FA_REQUIRED
  1005  AUTH_2FA_INVALID
  1006  AUTH_ACCOUNT_LOCKED
  1007  AUTH_ACCOUNT_SUSPENDED
  1008  AUTH_SESSION_LIMIT

USER_*:     2000-2999
  2001  USER_NOT_FOUND
  2002  USER_EMAIL_EXISTS
  2003  USER_PHONE_EXISTS
  2004  USER_COUNTRY_BLOCKED
  2005  USER_SELF_EXCLUDED

WALLET_*:   3000-3999
  3001  WALLET_INSUFFICIENT_BALANCE
  3002  WALLET_CURRENCY_MISMATCH
  3003  WALLET_LIMIT_EXCEEDED
  3004  WALLET_LOCKED

BET_*:      4000-4999
  4001  BET_EVENT_SUSPENDED
  4002  BET_MARKET_CLOSED
  4003  BET_ODDS_CHANGED
  4004  BET_STAKE_TOO_LOW
  4005  BET_STAKE_TOO_HIGH
  4006  BET_MAX_PAYOUT_EXCEEDED
  4007  BET_CASHOUT_UNAVAILABLE

PAYMENT_*:  5000-5999
CASINO_*:   6000-6999
KYC_*:      7000-7999
BONUS_*:    8000-8999
SYSTEM_*:   9000-9999

RULES:
  - Code is string in response ("BET_ODDS_CHANGED"), not numeric
  - Message is human-readable, localizable
  - Details object has machine-parseable fields
  - New codes can be added without breaking clients
  - Client should handle unknown codes gracefully
============================================================
SECTION 8: RATE LIMITING
============================================================
text

PER ENDPOINT, configured in Kong API Gateway:

  POST /auth/login:       10 req/min per IP
  POST /auth/register:    5 req/hour per IP
  POST /bets:             60 req/min per user
  POST /payments/deposit: 10 req/hour per user
  GET  /* (general):      100 req/min per user

RESPONSE when limited:
  HTTP 429 Too Many Requests
  Header: Retry-After: 60 (seconds)
  Body: { "error": { "code": "RATE_LIMITED", "message": "..." } }

ALGORITHM: Token Bucket (implemented in Kong + DragonflyDB)
============================================================
SECTION 9: ANTI-PATTERNS
============================================================
text

❌ NEVER return different formats for success vs error (always use envelope)
❌ NEVER use float/double for money in API (always string)
❌ NEVER expose internal IDs in URLs where UUID should be used
❌ NEVER return 200 with error body (use proper status codes)
❌ NEVER accept unbounded page_size (always enforce max)
❌ NEVER put sensitive data in URL query params (tokens, passwords)
❌ NEVER use GET for operations that modify state
❌ NEVER return stack traces in production error responses
❌ NEVER ignore Accept-Language header (use for error message localization)
❌ NEVER break backward compatibility in same API version