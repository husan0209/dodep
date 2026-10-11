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

### Money arithmetic

Money is never represented as a JavaScript number. Amounts are decimal strings
and every helper here converts them to integer minor units (cents) before doing
any arithmetic, so no value ever passes through a binary float
(`CONVENTIONS.md` NEVER-6).

```typescript
import { addMoney, subtractMoney, compareMoney, parseMoney } from '@opus-casino/shared';

const total = addMoney({ amount: '0.10', currency: 'USD' }, { amount: '0.20', currency: 'USD' });
total.amount; // '0.30' — exactly, not 0.30000000000000004

const change = subtractMoney({ amount: '10.00', currency: 'USD' }, { amount: '2.50', currency: 'USD' });
change.amount; // '7.50'

compareMoney({ amount: '100.00', currency: 'USD' }, { amount: '50.00', currency: 'USD' }); // 1

parseMoney('1.5', 'usd'); // { amount: '1.50', currency: 'USD' }
```

Points worth knowing:

- **Never pass a `number`.** `parseMoney` takes a string only. A float input is
  a compile error, and a malformed string throws rather than being coerced.
- **Mixing currencies throws.** `addMoney`, `subtractMoney` and `compareMoney`
  reject a currency mismatch instead of adding euros to dollars.
- **Rounding is half away from zero**, matching the Go
  (`shopspring/decimal.Round`) and Rust
  (`RoundingStrategy::MidpointAwayFromZero`) helpers. So
  `multiplyMoney({ amount: '0.01', currency: 'USD' }, 1.5).amount` is `'0.02'`,
  where float arithmetic produced `'0.01'`.
- **Precision holds past 2^53.** A `number` cannot represent
  `9007199254740993`; these helpers can.
- **Sub-cent input is an error**, not a truncated value: `'0.001'` throws
  instead of silently becoming `'0.00'`.

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

# Test with coverage
npm test -- --coverage
```

### Tests

`npm test` runs Jest through `ts-jest`, type-checking each test file under the
package's own strict tsconfig, so a test cannot pass against a type error that
`npm run build` would reject. Coverage thresholds are enforced in
`jest.config.js` (85% statements, 80% branches, 90% functions); current coverage
is above 99%.

| File | Covers |
|------|--------|
| `src/helpers.test.ts` | Money arithmetic, currency guards, comparison, predicates, formatting |
| `src/helpers.general.test.ts` | UUID generation, timestamps, retry, debounce/throttle, clone, pick/omit |
| `src/validators.test.ts` | Every exported validator, including IPv6 forms and calendar edge cases |
| `src/constants.test.ts` | Constant invariants: ordered bounds, unique codes, money-shaped limits |

### Known inconsistency

`CURRENCIES` mixes fiat ISO 4217 codes with crypto tickers. `USDT` is four
letters, so `isValidCurrencyCode('USDT')` and therefore `isValidMoney` reject
any USDT amount, and `Intl.NumberFormat` throws a `RangeError` for `USDT`, `BTC`
and `ETH`. This is asserted in `src/constants.test.ts` so the mismatch stays
visible; resolving it (drop the tickers, or teach the validator about them) is a
call for the constants owner.

## License

Proprietary — все права защищены
