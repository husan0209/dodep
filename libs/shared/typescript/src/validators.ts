/**
 * Validators for Opus Casino platform
 */

// Type-only import: these are erased at compile time, so importing them as
// values breaks ESM/bundler consumers.
import type { CountryCode, CurrencyCode, Money } from './types';

/**
 * Validate UUID v4
 */
export function isValidUuid(uuid: string): boolean {
  const uuidV4Regex = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  return uuidV4Regex.test(uuid);
}

/**
 * Validate email address
 */
export function isValidEmail(email: string): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(email);
}

/**
 * Validate country code (ISO 3166-1 alpha-2)
 */
export function isValidCountryCode(code: CountryCode): boolean {
  const countryCodeRegex = /^[A-Z]{2}$/;
  return countryCodeRegex.test(code);
}

/**
 * Validate currency code (ISO 4217)
 */
export function isValidCurrencyCode(code: CurrencyCode): boolean {
  const currencyCodeRegex = /^[A-Z]{3}$/;
  return currencyCodeRegex.test(code);
}

/**
 * Platform money contract: amounts are decimal strings matching PostgreSQL
 * NUMERIC(18,8) — up to 10 integer digits and 8 fractional digits.
 *
 * Floats, exponent notation and thousands separators are rejected: money is
 * never a JS number on this platform (CONVENTIONS NEVER-6).
 */
const MONEY_RE = /^-?\d{1,10}(\.\d{1,8})?$/;

/** Validate money amount */
export function isValidMoney(money: Money): boolean {
  if (!isValidCurrencyCode(money.currency)) {
    return false;
  }

  if (typeof money.amount !== 'string' || !MONEY_RE.test(money.amount)) {
    return false;
  }

  // Exact sign check: "0.00000000" with a minus sign is still zero, but a
  // negative amount is never a valid *balance* value.
  return !money.amount.startsWith('-');
}

/**
 * Validate password strength
 * Requirements:
 * - At least 8 characters
 * - At least one uppercase letter
 * - At least one lowercase letter
 * - At least one number
 * - At least one special character
 */
export function isValidPassword(password: string): boolean {
  const passwordRegex = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$/;
  return passwordRegex.test(password);
}

/**
 * Validate phone number (E.164 format)
 */
export function isValidPhone(phone: string): boolean {
  const phoneRegex = /^\+[1-9]\d{1,14}$/;
  return phoneRegex.test(phone);
}

/** Odds are decimal strings with up to 4 fractional digits (e.g. "1.85"). */
const ODDS_RE = /^\d{1,6}(\.\d{1,4})?$/;

/**
 * Validate odds format (decimal string, no floats).
 *
 * Accepted range is [1.01, 1000] — checked with exact string comparison,
 * never via parseFloat, so no rounding decision is made in binary floating
 * point.
 */
export function isValidOdds(odds: string): boolean {
  if (typeof odds !== 'string' || !ODDS_RE.test(odds)) {
    return false;
  }

  // Pad to 2 decimals so "1.01" and "1.0100" compare equal, and so the
  // lower bound can be checked lexicographically on a fixed-width string.
  const [intPart, fracPart = ''] = odds.split('.');
  const normalized = `${intPart.padStart(4, '0')}.${fracPart.padEnd(4, '0')}`;
  return normalized >= '0001.0100' && normalized <= '1000.0000';
}

/** Percentages accept a decimal string or a finite number, in range [0, 100]. */
const PERCENTAGE_RE = /^\d{1,3}(\.\d{1,8})?$/;

/**
 * Validate percentage (0-100).
 *
 * A decimal string is preferred and validated exactly; a number is still
 * accepted for UI-internal use but must be finite and in range.
 */
export function isValidPercentage(value: string | number): boolean {
  if (typeof value === 'string') {
    return PERCENTAGE_RE.test(value) && Number(value) <= 100;
  }
  return Number.isFinite(value) && value >= 0 && value <= 100;
}

const HTML_ENTITIES: Record<string, string> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#x27;': "'",
  '&#39;': "'",
  '&#x60;': '`',
  '&#x3D;': '=',
};

/**
 * Escape text for safe interpolation into HTML text nodes and quoted
 * attribute values.
 *
 * Idempotent: already-escaped input is decoded first, so escaping twice
 * cannot produce `&amp;amp;` (a common source of display corruption).
 * Backtick and `=` are included so the result is safe for both quoted and
 * unquoted attribute contexts.
 *
 * Note: this is output encoding, not sanitization. Prefer escaping at the
 * point of rendering; this helper exists for template strings that cannot
 * use framework escaping.
 */
export function sanitizeString(str: string): string {
  if (typeof str !== 'string') {
    return '';
  }

  // Decode first so repeated calls are stable.
  const decoded = str.replace(
    /&(?:amp|lt|gt|quot|#x27|#39|#x60|#x3D);/gi,
    (entity) => HTML_ENTITIES[entity.toLowerCase()] ?? entity,
  );

  return decoded
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;')
    .replace(/`/g, '&#x60;')
    .replace(/=/g, '&#x3D;');
}

const IPV4_RE =
  /^(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

function isValidIpv6(ip: string): boolean {
  // At most one "::" compression allowed.
  if ((ip.match(/::/g) ?? []).length > 1) {
    return false;
  }

  const [headPart, tailPart] = ip.split('::');
  const head = headPart ? headPart.split(':') : [];
  const tail = tailPart !== undefined ? (tailPart ? tailPart.split(':') : []) : null;

  if (headPart === '' && tailPart === undefined) {
    return false; // bare "::" is not a valid address string here
  }

  const groups = tail === null ? [...head] : [...head, ...tail];
  if (groups.some((g) => !/^[0-9A-F]{1,4}$/i.test(g))) {
    return false;
  }

  if (tail === null) {
    // No compression: all 8 groups must be present.
    return groups.length === 8;
  }

  // With "::" at least one group must be elided.
  return groups.length <= 7;
}

/**
 * Validate IP address (IPv4 or IPv6, including compressed forms like `::1`)
 */
export function isValidIp(ip: string): boolean {
  if (typeof ip !== 'string' || ip.length === 0) {
    return false;
  }
  return IPV4_RE.test(ip) || isValidIpv6(ip);
}

/**
 * Validate date string (ISO 8601 calendar date, `YYYY-MM-DD`).
 *
 * Rejects dates the `Date` constructor would silently roll over, such as
 * `2026-02-31`.
 */
export function isValidDate(date: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date ?? '');
  if (!match) {
    return false;
  }

  const [, yearText, monthText, dayText] = match;
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);

  if (month < 1 || month > 12 || day < 1) {
    return false;
  }

  // Days per month, accounting for leap years.
  const daysInMonth = [
    31,
    (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0 ? 29 : 28,
    31,
    30,
    31,
    30,
    31,
    31,
    30,
    31,
    30,
    31,
  ];

  return day <= daysInMonth[month - 1];
}

/**
 * Validate datetime string (ISO 8601, e.g. `2026-03-28T10:15:30Z`).
 */
export function isValidDateTime(dateTime: string): boolean {
  const isoPattern =
    /^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}(\.\d{1,9})?)?(Z|[+-]\d{2}:\d{2})?$/;
  if (!isoPattern.test(dateTime ?? '')) {
    return false;
  }
  if (!isValidDate(dateTime.slice(0, 10))) {
    return false;
  }
  return !Number.isNaN(new Date(dateTime.replace(' ', 'T')).getTime());
}
