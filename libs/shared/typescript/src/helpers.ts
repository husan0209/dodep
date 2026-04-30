/**
 * Helper functions for Opus Casino platform
 */

import { Money } from './types';

/**
 * Format money for display
 */
export function formatMoney(money: Money, locale = 'en-US'): string {
  const formatter = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: money.currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  
  return formatter.format(parseFloat(money.amount));
}

/**
 * Parse money string to Money object
 */
export function parseMoney(amount: number | string, currency: string): Money {
  const amountStr = typeof amount === 'number' 
    ? amount.toFixed(2) 
    : amount;
  
  return {
    amount: amountStr,
    currency,
  };
}

/**
 * Add two money amounts (must be same currency)
 */
export function addMoney(a: Money, b: Money): Money {
  if (a.currency !== b.currency) {
    throw new Error(`Currency mismatch: ${a.currency} !== ${b.currency}`);
  }
  
  const sum = parseFloat(a.amount) + parseFloat(b.amount);
  return {
    amount: sum.toFixed(2),
    currency: a.currency,
  };
}

/**
 * Subtract two money amounts (must be same currency)
 */
export function subtractMoney(a: Money, b: Money): Money {
  if (a.currency !== b.currency) {
    throw new Error(`Currency mismatch: ${a.currency} !== ${b.currency}`);
  }
  
  const diff = parseFloat(a.amount) - parseFloat(b.amount);
  return {
    amount: diff.toFixed(2),
    currency: a.currency,
  };
}

/**
 * Multiply money by a scalar
 */
export function multiplyMoney(money: Money, scalar: number): Money {
  const result = parseFloat(money.amount) * scalar;
  return {
    amount: Math.abs(result).toFixed(2),
    currency: money.currency,
  };
}

/**
 * Compare two money amounts
 * Returns: -1 if a < b, 0 if a === b, 1 if a > b
 */
export function compareMoney(a: Money, b: Money): number {
  if (a.currency !== b.currency) {
    throw new Error(`Currency mismatch: ${a.currency} !== ${b.currency}`);
  }
  
  const aAmount = parseFloat(a.amount);
  const bAmount = parseFloat(b.amount);
  
  if (aAmount < bAmount) return -1;
  if (aAmount > bAmount) return 1;
  return 0;
}

/**
 * Check if money amount is zero
 */
export function isZero(money: Money): boolean {
  return parseFloat(money.amount) === 0;
}

/**
 * Check if money amount is positive
 */
export function isPositive(money: Money): boolean {
  return parseFloat(money.amount) > 0;
}

/**
 * Check if money amount is negative
 */
export function isNegative(money: Money): boolean {
  return parseFloat(money.amount) < 0;
}

/**
 * Generate UUID v4
 */
export function generateUuid(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  
  // Fallback for environments without crypto.randomUUID
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
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
 * Debounce a function
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
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
 */
export function throttle<T extends (...args: unknown[]) => unknown>(
  fn: T,
  limit: number
): (...args: Parameters<T>) => void {
  let inThrottle: boolean;
  
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
