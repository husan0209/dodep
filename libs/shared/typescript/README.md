# @opus-casino/shared — TypeScript Shared Library

Shared types, validators, constants, and utilities for Opus Casino platform.

## Installation

```bash
npm install @opus-casino/shared
```

Or with yarn:

```bash
yarn add @opus-casino/shared
```

## Usage

### Import types

```typescript
import { UserId, Money, PaginationResult } from '@opus-casino/shared';

const userId: UserId = '550e8400-e29b-41d4-a716-446655440000';
const balance: Money = { amount: '100.00', currency: 'USD' };
```

### Use validators

```typescript
import { isValidEmail, isValidUuid, isValidMoney, isValidPassword } from '@opus-casino/shared';

isValidEmail('user@example.com');  // true
isValidUuid('550e8400-e29b-41d4-a716-446655440000');  // true
isValidMoney({ amount: '100.00', currency: 'USD' });  // true
isValidPassword('SecureP@ss123');  // true
```

### Use constants

```typescript
import { CURRENCIES, RATE_LIMITS, BET_STATUSES, ERROR_CODES } from '@opus-casino/shared';

console.log(CURRENCIES.USD);  // 'USD'
console.log(RATE_LIMITS.API_REQUESTS_PER_MINUTE);  // 100
console.log(BET_STATUSES.PENDING);  // 'pending'
```

### Use helpers

```typescript
import { 
  formatMoney, 
  generateUuid, 
  retry, 
  debounce 
} from '@opus-casino/shared';

const formatted = formatMoney({ amount: '100.50', currency: 'USD' });  // '$100.50'
const id = generateUuid();  // '550e8400-e29b-41d4-a716-446655440000'

// Retry with exponential backoff
const result = await retry(
  () => fetch('/api/data'),
  { maxRetries: 3, initialDelay: 100 }
);

// Debounced search
const search = debounce((query) => api.search(query), 300);
```

## API Reference

### Types

- `UserId` — User identifier (UUID v4)
- `BetId`, `TransactionId`, `GameId`, `SessionId` — Entity identifiers
- `Money` — Monetary amount with currency
- `PaginationParams`, `PaginationResult<T>` — Pagination types
- `DateRange` — Date range filter
- `ErrorDetails`, `FieldError` — Error types
- `ApiResponse<T>` — API response wrapper
- `HealthCheckResponse` — Health check response

### Validators

- `isValidUuid(uuid)` — Validate UUID v4
- `isValidEmail(email)` — Validate email
- `isValidCountryCode(code)` — Validate ISO 3166-1 alpha-2
- `isValidCurrencyCode(code)` — Validate ISO 4217
- `isValidMoney(money)` — Validate money amount
- `isValidPassword(password)` — Validate password strength
- `isValidPhone(phone)` — Validate E.164 phone
- `isValidOdds(odds)` — Validate decimal odds
- `isValidPercentage(value)` — Validate percentage
- `isValidIp(ip)` — Validate IPv4/IPv6
- `isValidDate(date)` — Validate ISO date
- `isValidDateTime(dateTime)` — Validate ISO datetime

### Constants

- `CURRENCIES` — Supported currencies
- `RESTRICTED_COUNTRIES` — Restricted jurisdictions
- `WALLET_TYPES` — Wallet type constants
- `TRANSACTION_TYPES` — Transaction type constants
- `BET_TYPES`, `BET_STATUSES` — Bet constants
- `BONUS_TYPES` — Bonus type constants
- `KYC_LEVELS` — KYC level constants
- `NOTIFICATION_CHANNELS`, `NOTIFICATION_TYPES` — Notification constants
- `RATE_LIMITS` — Rate limit constants
- `BET_LIMITS`, `PAYMENT_LIMITS` — Limit constants
- `SESSION` — Session settings
- `RESPONSIBLE_GAMBLING` — Responsible gambling constants
- `ERROR_CODES` — Error code constants

### Helpers

- `formatMoney(money, locale)` — Format money for display
- `parseMoney(amount, currency)` — Parse to Money object
- `addMoney(a, b)`, `subtractMoney(a, b)` — Money arithmetic
- `multiplyMoney(money, scalar)` — Multiply money
- `compareMoney(a, b)` — Compare money amounts
- `generateUuid()` — Generate UUID v4
- `now()`, `nowIso()` — Current timestamp
- `sleep(ms)` — Sleep promise
- `retry(fn, options)` — Retry with backoff
- `debounce(fn, delay)` — Debounce function
- `throttle(fn, limit)` — Throttle function
- `deepClone(obj)` — Deep clone object
- `pick(obj, keys)`, `omit(obj, keys)` — Object utilities

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Watch mode
npm run dev

# Lint
npm run lint

# Format
npm run format

# Test
npm test
```

## License

Proprietary — все права защищены
