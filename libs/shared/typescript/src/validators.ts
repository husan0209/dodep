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
 */
export function isValidMoney(money: Money): boolean {
  if (!isValidCurrencyCode(money.currency)) {
    return false;
  }
  
  // Validate amount format (decimal string)
  const amountRegex = /^\d+(\.\d{1,2})?$/;
  return amountRegex.test(money.amount) && parseFloat(money.amount) >= 0;
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
 */
export function isValidIp(ip: string): boolean {
  const ipv4Regex = /^(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;
  const ipv6Regex = /^(?:[A-F0-9]{1,4}:){7}[A-F0-9]{1,4}$/i;
  return ipv4Regex.test(ip) || ipv6Regex.test(ip);
}

/**
 * Validate date string (ISO 8601)
 */
export function isValidDate(date: string): boolean {
  const isoDateRegex = /^\d{4}-\d{2}-\d{2}$/;
  if (!isoDateRegex.test(date)) {
    return false;
  }
  const parsed = new Date(date);
  return !Number.isNaN(parsed.getTime());
}

/**
 * Validate datetime string (ISO 8601)
 */
export function isValidDateTime(dateTime: string): boolean {
  const parsed = new Date(dateTime);
  return !Number.isNaN(parsed.getTime());
}
