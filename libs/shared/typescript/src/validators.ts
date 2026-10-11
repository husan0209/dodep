/**
 * Validators for Opus Casino platform
 */

import { CountryCode, CurrencyCode, Money } from './types';

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
 * Validate money amount
 *
 * Currency codes are compared case-insensitively: ISO 4217 codes are
 * canonically uppercase, but `Money` values arrive from JSON payloads and API
 * responses where "usd" is a plausible spelling. Rejecting it here would make
 * the validator disagree with parseMoney, which normalises the currency to
 * uppercase, so a value parseMoney produced would fail its own validation.
 */
export function isValidMoney(money: Money): boolean {
  if (!isValidCurrencyCode(money.currency.toUpperCase())) {
    return false;
  }

  // Shape only: a non-negative decimal with at most two fraction digits. No
  // parseFloat here — the amount is a string precisely so that it never has to
  // pass through a binary float, and this predicate must not reintroduce that.
  const amountRegex = /^(?:0|[1-9]\d*)(?:\.\d{1,2})?$/;
  return amountRegex.test(money.amount);
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

/**
 * Validate odds format (decimal)
 */
export function isValidOdds(odds: string): boolean {
  const oddsRegex = /^\d+(\.\d+)?$/;
  if (!oddsRegex.test(odds)) {
    return false;
  }
  const oddsValue = parseFloat(odds);
  return oddsValue >= 1.01 && oddsValue <= 1000;
}

/**
 * Validate percentage (0-100)
 */
export function isValidPercentage(value: number): boolean {
  return Number.isFinite(value) && value >= 0 && value <= 100;
}

/**
 * Sanitize string to prevent XSS
 */
export function sanitizeString(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;');
}

/**
 * Validate IP address (IPv4 or IPv6)
 *
 * IPv6 uses a structural validator rather than the previous single regex,
 * which only accepted the fully expanded eight-group form and therefore
 * rejected `::1`, `fe80::1` and `2001:db8::1` — every address a real client
 * arrives with. The Go (net.ParseIP) and Rust (std::net::IpAddr) validators
 * accept them, so the three implementations disagreed on the same input.
 *
 * Hand-rolled rather than delegated because the platform targets browsers and
 * Node without assuming a Node-only API surface.
 */
export function isValidIp(ip: string): boolean {
  return isValidIpv4(ip) || isValidIpv6(ip);
}

const IPV4_OCTET = /^(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/;

function isValidIpv4(ip: string): boolean {
  const parts = ip.split('.');
  if (parts.length !== 4) {
    return false;
  }
  return parts.every((part) => IPV4_OCTET.test(part));
}

const IPV6_GROUP = /^[0-9A-Fa-f]{1,4}$/;

/**
 * Count the 16-bit groups in one side of a possible "::" split.
 *
 * A trailing IPv4 quad is legal in the last 32 bits and counts as two groups.
 * Returns null when any group is malformed.
 */
function countIpv6Groups(segment: string): number | null {
  if (segment === '') {
    return 0;
  }

  const parts = segment.split(':');
  let groups = 0;

  for (let i = 0; i < parts.length; i += 1) {
    const part = parts[i];

    // The IPv4 form may only appear as the very last group.
    if (part.includes('.')) {
      if (i !== parts.length - 1 || !isValidIpv4(part)) {
        return null;
      }
      groups += 2;
      continue;
    }

    if (!IPV6_GROUP.test(part)) {
      return null;
    }
    groups += 1;
  }

  return groups;
}

function isValidIpv6(ip: string): boolean {
  // Zone index such as fe80::1%eth0 is valid syntax for a scoped address.
  const parts = ip.split('%');
  const address = parts[0];
  if (parts.length > 2 || (parts.length === 2 && parts[1].length === 0)) {
    return false;
  }

  const halves = address.split('::');
  if (halves.length > 2) {
    // More than one "::" is invalid.
    return false;
  }

  if (halves.length === 2) {
    const left = countIpv6Groups(halves[0]);
    const right = countIpv6Groups(halves[1]);
    if (left === null || right === null) {
      return false;
    }
    // "::" must stand for at least one all-zero group, so the explicit part
    // can never already fill all eight.
    return left + right < 8;
  }

  if (!address.includes(':')) {
    return false;
  }

  const groups = countIpv6Groups(address);
  return groups !== null && groups === 8;
}

/**
 * Validate date string (ISO 8601 calendar date: YYYY-MM-DD)
 *
 * The previous implementation parsed the string with `new Date(date)` and only
 * checked for NaN. JavaScript normalises overflowing dates instead of rejecting
 * them, so `2026-02-30` parsed successfully as `2026-03-02` and validated. A
 * date of birth or a responsible-gambling limit boundary could therefore be set
 * to a day that does not exist.
 *
 * Now the round trip is verified: the formatted result must equal the input.
 */
export function isValidDate(date: string): boolean {
  const isoDateRegex = /^(\d{4})-(\d{2})-(\d{2})$/;
  const match = isoDateRegex.exec(date);
  if (!match) {
    return false;
  }

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);

  if (month < 1 || month > 12) {
    return false;
  }

  // Days in month, accounting for leap years.
  const daysInMonth = [31, isLeapYear(year) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][month - 1];
  if (day < 1 || day > daysInMonth) {
    return false;
  }

  // A constructed Date rolls over for day/month 0, so bounds are re-checked
  // via the round trip below.
  const parsed = new Date(Date.UTC(year, month - 1, day));
  if (Number.isNaN(parsed.getTime())) {
    return false;
  }
  const roundTrip =
    `${parsed.getUTCFullYear()}-` +
    `${String(parsed.getUTCMonth() + 1).padStart(2, '0')}-` +
    `${String(parsed.getUTCDate()).padStart(2, '0')}`;
  return roundTrip === date;
}

function isLeapYear(year: number): boolean {
  return (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
}

/**
 * Validate datetime string (ISO 8601)
 */
export function isValidDateTime(dateTime: string): boolean {
  const parsed = new Date(dateTime);
  return !Number.isNaN(parsed.getTime());
}
