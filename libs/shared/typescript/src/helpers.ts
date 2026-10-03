/**
 * Helper functions for Opus Casino platform
 */

import { Money } from './types';

/**
 * Money arithmetic in minor units (cents).
 *
 * CONVENTIONS.md NEVER-6 forbids representing money as a float. The previous
 * implementation parsed every amount with `parseFloat`, added the floats, and
 * formatted with `toFixed(2)`. That is incorrect in ways a real payout hits:
 *
 *   2.675.toFixed(2) === "2.67"   // banker's rounding on a binary float
 *   1.005.toFixed(2) === "1.00"   // same
 *   0.07 + 0.07 + 0.07 === 0.21000000000000002
 *   parseFloat("9007199254740993.00") === 9007199254740992  // past 2^53
 *
 * So an amount could be stored a cent below what was requested, and amounts
 * above 2^53 lost precision entirely. Here every value is converted to an
 * integer number of minor units first; all arithmetic is exact integer math and
 * formatting happens once, at the end.
 */

/** Fraction digits for the currencies this platform handles. */
const MINOR_UNITS = 2;

const MONEY_PATTERN = /^-?(0|[1-9]\d*)(\.\d{1,2})?$/;

/**
 * Convert a decimal string to an integer number of minor units.
 *
 * Parsing is done with string slicing rather than parseFloat so that no value
 * ever passes through a binary float. Rejects exponent notation, thousands
 * separators and more than two fraction digits, which are all ways a malformed
 * amount could otherwise silently become a different number.
 */
function toMinorUnits(amount: string): bigint {
  if (!MONEY_PATTERN.test(amount)) {
    throw new Error(`Invalid money amount: ${amount}`);
  }

  const negative = amount.startsWith('-');
  const digits = negative ? amount.slice(1) : amount;
  const dot = digits.indexOf('.');
  const whole = dot === -1 ? digits : digits.slice(0, dot);
  const fraction = dot === -1 ? '' : digits.slice(dot + 1);

  const padded = fraction.padEnd(MINOR_UNITS, '0');
  let units = BigInt(`${whole}${padded}`);

  if (negative) {
    units = -units;
  }
  return units;
}

/** Render integer minor units back to a fixed 2-decimal string. */
function fromMinorUnits(units: bigint): string {
  const negative = units < 0n;
  const magnitude = negative ? -units : units;

  const base = 10n ** BigInt(MINOR_UNITS);
  const whole = magnitude / base;
  const fraction = magnitude % base;

  const rendered = `${whole}.${fraction.toString().padStart(MINOR_UNITS, '0')}`;
  return negative && magnitude !== 0n ? `-${rendered}` : rendered;
}

/** Convert a decimal string to a number for display only (never arithmetic). */
function toDisplayNumber(amount: string): number {
  return Number(amount);
}

/** Require matching currencies before any arithmetic. */
function assertSameCurrency(a: Money, b: Money): void {
  if (a.currency !== b.currency) {
    throw new Error(`Currency mismatch: ${a.currency} !== ${b.currency}`);
  }
}

/**
 * Format money for display
 *
 * Fraction digits are left to Intl.NumberFormat, which already knows the ISO
 * 4217 exponent of each currency. Forcing two digits, as the previous version
 * did, rendered JPY — a zero-decimal currency — as "¥1,234.00", inventing two
 * decimal places the currency does not have.
 */
export function formatMoney(money: Money, locale = 'en-US'): string {
  const formatter = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: money.currency,
  });

  // Number() is acceptable here: the value goes straight into Intl, never back
  // into arithmetic. Values beyond 2^53 may display rounded, which is a
  // presentation limit of the platform rather than a data error.
  return formatter.format(toDisplayNumber(money.amount));
}

/**
 * Parse money into a Money object.
 *
 * A number input is rejected: accepting one would let a float reach the money
 * path in the first place, which is what NEVER-6 exists to prevent. Callers
 * must pass a decimal string.
 */
export function parseMoney(amount: string, currency: string): Money {
  const normalized = fromMinorUnits(toMinorUnits(amount));

  return {
    amount: normalized,
    currency: currency.toUpperCase(),
  };
}

/**
 * Add two money amounts (must be same currency)
 */
export function addMoney(a: Money, b: Money): Money {
  assertSameCurrency(a, b);

  return {
    amount: fromMinorUnits(toMinorUnits(a.amount) + toMinorUnits(b.amount)),
    currency: a.currency,
  };
}

/**
 * Subtract two money amounts (must be same currency)
 */
export function subtractMoney(a: Money, b: Money): Money {
  assertSameCurrency(a, b);

  return {
    amount: fromMinorUnits(toMinorUnits(a.amount) - toMinorUnits(b.amount)),
    currency: a.currency,
  };
}

/**
 * Multiply money by a scalar.
 *
 * Kept in major units because a scalar such as an odds multiplier or a
 * percentage is not money. The product is rounded half away from zero, matching
 * the Go and Rust implementations, so the three services agree on the cent.
 *
 * A negative scalar yields the absolute value: this mirrors `Money::multiply`
 * in libs/shared/rust, and prevents a negative (creditable) amount from being
 * constructed by sign error.
 */
export function multiplyMoney(money: Money, scalar: number): Money {
  if (!Number.isFinite(scalar)) {
    throw new Error(`Invalid scalar: ${scalar}`);
  }

  const units = toMinorUnits(money.amount);
  const absUnits = units < 0n ? -units : units;

  // Scale by 10^6 so that up to six decimal places of scalar keep sub-cent
  // precision before a single deliberate rounding step.
  const SCALE_DECIMALS = 6;
  const scale = 10n ** BigInt(SCALE_DECIMALS);
  const scaledScalar = BigInt(Math.round(Math.abs(scalar) * 1_000_000));

  const scaledUnits = (absUnits * scaledScalar + scale / 2n) / scale;
  const sign = units < 0n ? -1n : 1n;

  return {
    amount: fromMinorUnits(sign * scaledUnits),
    currency: money.currency,
  };
}

/**
 * Compare two money amounts
 * Returns: -1 if a < b, 0 if a === b, 1 if a > b
 */
export function compareMoney(a: Money, b: Money): number {
  assertSameCurrency(a, b);

  const left = toMinorUnits(a.amount);
  const right = toMinorUnits(b.amount);

  if (left < right) return -1;
  if (left > right) return 1;
  return 0;
}

/**
 * Check if money amount is zero
 */
export function isZero(money: Money): boolean {
  return toMinorUnits(money.amount) === 0n;
}

/**
 * Check if money amount is positive
 */
export function isPositive(money: Money): boolean {
  return toMinorUnits(money.amount) > 0n;
}

/**
 * Check if money amount is negative
 */
export function isNegative(money: Money): boolean {
  return toMinorUnits(money.amount) < 0n;
}

/**
 * Generate UUID v4
 */
export function generateUuid(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }

  // Fallback for environments without crypto.randomUUID (HTTP origins, older
  // runtimes). getRandomValues is used rather than Math.random so the value is
  // still unpredictable: on a Math.random fallback, a UUID generated in a
  // browser could be predicted by an attacker sharing the page's realm.
  const randomBytes = new Uint8Array(16);
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(randomBytes);
  } else {
    for (let i = 0; i < randomBytes.length; i += 1) {
      randomBytes[i] = (Math.random() * 256) | 0;
    }
  }

  // Set the version (4) and variant (RFC 4122) bits, then format.
  randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40;
  randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80;

  const hex: string[] = [];
  for (let i = 0; i < randomBytes.length; i += 1) {
    hex.push(randomBytes[i].toString(16).padStart(2, '0'));
  }

  return [
    hex.slice(0, 4).join(''),
    hex.slice(4, 6).join(''),
    hex.slice(6, 8).join(''),
    hex.slice(8, 10).join(''),
    hex.slice(10, 16).join(''),
  ].join('-');
}

/**
 * Get current timestamp in milliseconds
 */
export function now(): number {
  return Date.now();
}

/**
 * Get current ISO 8601 timestamp
 */
export function nowIso(): string {
  return new Date().toISOString();
}

/**
 * Sleep for specified milliseconds
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Retry a function with exponential backoff
 */
export async function retry<T>(
  fn: () => Promise<T>,
  options: {
    maxRetries?: number;
    initialDelay?: number;
    maxDelay?: number;
    multiplier?: number;
  } = {}
): Promise<T> {
  const {
    maxRetries = 3,
    initialDelay = 100,
    maxDelay = 10000,
    multiplier = 2,
  } = options;
  
  let lastError: Error;
  let delay = initialDelay;
  
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error as Error;
      
      if (attempt === maxRetries) {
        break;
      }
      
      await sleep(delay);
      delay = Math.min(delay * multiplier, maxDelay);
    }
  }
  
  throw lastError!;
}

/**
 * Any callable, preserving the caller's signature.
 *
 * The constraint deliberately uses `any[]` rather than `unknown[]`. With
 * `unknown[]`, TypeScript rejects every function that declares concrete
 * parameter types — `debounce((value: string) => ...)` fails with
 * "unknown is not assignable to string", which made both helpers unusable
 * without a cast.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyFunction = (...args: any[]) => any;

/**
 * Debounce a function
 */
export function debounce<T extends AnyFunction>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: ReturnType<typeof setTimeout>;

  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  };
}

/**
 * Throttle a function
 *
 * Leading-edge: the first call in each window runs immediately and subsequent
 * calls are dropped until the window closes. Trailing calls are not replayed.
 */
export function throttle<T extends AnyFunction>(
  fn: T,
  limit: number
): (...args: Parameters<T>) => void {
  let inThrottle = false;

  return (...args: Parameters<T>) => {
    if (!inThrottle) {
      fn(...args);
      inThrottle = true;
      setTimeout(() => (inThrottle = false), limit);
    }
  };
}

/**
 * Deep clone an object
 */
export function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

/**
 * Check if object is empty
 */
export function isEmpty(obj: Record<string, unknown>): boolean {
  return Object.keys(obj).length === 0;
}

/**
 * Pick specific keys from an object
 */
export function pick<T extends Record<string, unknown>, K extends keyof T>(
  obj: T,
  keys: K[]
): Pick<T, K> {
  const result = {} as Pick<T, K>;
  for (const key of keys) {
    if (key in obj) {
      result[key] = obj[key];
    }
  }
  return result;
}

/**
 * Omit specific keys from an object
 */
export function omit<T extends Record<string, unknown>, K extends keyof T>(
  obj: T,
  keys: K[]
): Omit<T, K> {
  const result = { ...obj };
  for (const key of keys) {
    delete result[key];
  }
  return result as Omit<T, K>;
}
