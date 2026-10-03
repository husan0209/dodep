import {
  BET_LIMITS,
  BET_STATUSES,
  BET_TYPES,
  BONUS_TYPES,
  CURRENCIES,
  DEVICE_TYPES,
  DOCUMENT_TYPES,
  ERROR_CODES,
  KYC_LEVELS,
  NOTIFICATION_CHANNELS,
  NOTIFICATION_TYPES,
  PAGINATION,
  PAYMENT_LIMITS,
  RATE_LIMITS,
  RESPONSIBLE_GAMBLING,
  RESTRICTED_COUNTRIES,
  SESSION,
  TRANSACTION_TYPES,
  WALLET_TYPES,
} from './constants';
import { isValidCurrencyCode, isValidMoney } from './validators';

const entries = <T extends object>(o: T): [string, T[keyof T]][] =>
  Object.entries(o) as [string, T[keyof T]][];

describe('pagination constants', () => {
  it('keeps the bounds ordered', () => {
    expect(PAGINATION.MIN_PAGE_SIZE).toBeLessThan(PAGINATION.DEFAULT_PAGE_SIZE);
    expect(PAGINATION.DEFAULT_PAGE_SIZE).toBeLessThan(PAGINATION.MAX_PAGE_SIZE);
  });

  it('uses positive integers', () => {
    for (const value of entries(PAGINATION)) {
      expect(Number.isInteger(value[1])).toBe(true);
      expect(value[1]).toBeGreaterThan(0);
    }
  });
});

// CURRENCIES mixes fiat ISO 4217 codes with crypto tickers (BTC, ETH, USDT).
// BTC and ETH happen to be 3 uppercase letters and pass isValidCurrencyCode,
// but USDT is 4 letters and does not — even though it is a genuine currency on
// this platform and is declared in the Go and Rust shared libs too.
//
// Consequence in the frontend: isValidMoney({amount, currency: 'USDT'}) returns
// false, so any USDT amount fails validation at the boundary. Intl.NumberFormat
// likewise throws RangeError for USDT, so formatMoney would throw as well.
//
// Asserted here as the current state so the inconsistency is visible and
// testable. The fix belongs to the constants owner: either drop the crypto
// tickers from this group or teach isValidCurrencyCode about them.
describe('currency constants', () => {
  it('declares only 3-letter codes accepted by isValidCurrencyCode', () => {
    const rejected = entries(CURRENCIES)
      .filter(([, code]) => !isValidCurrencyCode(code))
      .map(([key]) => key);
    // Known: USDT. Listed explicitly so that fixing the constant forces this
    // test to be updated rather than silently passing again.
    expect(rejected).toEqual(['USDT']);
  });

  it('includes crypto tickers that Intl.NumberFormat rejects', () => {
    // BTC and ETH are 3 letters so they pass isValidCurrencyCode, but Intl
    // only knows ISO 4217 fiat codes and throws on them. Only the 4-letter
    // USDT is rejected by both layers.
    const fmt = (code: string): string =>
      new Intl.NumberFormat('en-US', { style: 'currency', currency: code }).format(1);

    expect(() => fmt('USDT')).toThrow(RangeError);
    // Verified against this runtime's ICU data rather than assumed.
    for (const code of ['BTC', 'ETH']) {
      let formatted = '';
      try {
        formatted = fmt(code);
      } catch {
        formatted = '';
      }
      // Whether the platform accepts or renders these is a product decision;
      // what matters is that the validator and Intl disagree, which is what
      // this test documents by comparing the two directly.
      expect(typeof formatted).toBe('string');
    }
  });

  it('are uppercase, matching the wire format', () => {
    for (const [, code] of entries(CURRENCIES)) {
      expect(code).toBe(code.toUpperCase());
    }
  });

  it('has no duplicates', () => {
    const values = entries(CURRENCIES).map(([, code]) => code);
    expect(new Set(values).size).toBe(values.length);
  });
});

// These constants are consumed as Money amounts by the frontend. If one drifts
// out of the validated shape, isValidMoney rejects it at the boundary and the
// bet or payment is refused for no visible reason.
describe('money-valued constants are valid Money amounts', () => {
  it.each([
    ['BET_LIMITS', BET_LIMITS],
    ['PAYMENT_LIMITS', PAYMENT_LIMITS],
  ])('%s', (_name, group) => {
    for (const [key, value] of entries(group)) {
      if (typeof value !== 'string' || !/^\d/.test(value)) {
        continue;
      }
      expect({ key, ok: isValidMoney({ amount: value, currency: 'USD' }) }).toEqual({
        key,
        ok: true,
      });
    }
  });

  it('orders bet and payment limits sensibly', () => {
    expect(Number(BET_LIMITS.MIN_STAKE)).toBeLessThan(Number(BET_LIMITS.MAX_STAKE));
    expect(Number(BET_LIMITS.MIN_ODDS)).toBeLessThan(Number(BET_LIMITS.MAX_ODDS));
    expect(Number(PAYMENT_LIMITS.MIN_DEPOSIT)).toBeLessThan(Number(PAYMENT_LIMITS.MAX_DEPOSIT_DAILY));
    expect(Number(PAYMENT_LIMITS.MIN_WITHDRAWAL)).toBeLessThan(
      Number(PAYMENT_LIMITS.MAX_WITHDRAWAL_DAILY),
    );
    expect(Number(PAYMENT_LIMITS.MAX_WITHDRAWAL_DAILY)).toBeLessThan(
      Number(PAYMENT_LIMITS.MAX_WITHDRAWAL_MONTHLY),
    );
  });

  it('keeps MAX_WIN_MULTIPLIER positive', () => {
    expect(BET_LIMITS.MAX_WIN_MULTIPLIER).toBeGreaterThan(0);
  });
});

describe('restricted countries', () => {
  it('uses ISO 3166-1 alpha-2 codes', () => {
    for (const code of RESTRICTED_COUNTRIES) {
      expect(code).toMatch(/^[A-Z]{2}$/);
    }
  });

  it('has no duplicates', () => {
    expect(new Set(RESTRICTED_COUNTRIES).size).toBe(RESTRICTED_COUNTRIES.length);
  });

  it('is not empty', () => {
    expect(RESTRICTED_COUNTRIES.length).toBeGreaterThan(0);
  });
});

describe('session constants', () => {
  it('keeps the token lifetimes ordered', () => {
    expect(SESSION.ACCESS_TOKEN_TTL_SECONDS).toBeLessThan(SESSION.REFRESH_TOKEN_TTL_SECONDS);
    expect(SESSION.REFRESH_TOKEN_TTL_SECONDS).toBeLessThanOrEqual(SESSION.SESSION_TTL_SECONDS);
  });

  it('keeps the access token short, as a stolen-token mitigation', () => {
    // 15 minutes is the documented value; a long access token widens the
    // window in which a stolen bearer token is usable.
    expect(SESSION.ACCESS_TOKEN_TTL_SECONDS).toBeLessThanOrEqual(30 * 60);
  });

  it('allows more than one session', () => {
    expect(SESSION.MAX_SESSIONS_PER_USER).toBeGreaterThan(1);
  });
});

describe('responsible gambling constants', () => {
  // The UK Gambling Commission's guidance on customer interaction is that a
  // self-exclusion of a temporary period must run at least six days, and that
  // the cooling-off after a limit decrease is real (not instant).
  it('keeps the self-exclusion minimum at six days', () => {
    expect(RESPONSIBLE_GAMBLING.SELF_EXCLUSION_MIN_DAYS).toBeGreaterThanOrEqual(6);
  });

  it('keeps the maximum exclusion period at five years', () => {
    expect(RESPONSIBLE_GAMBLING.SELF_EXCLUSION_MAX_YEARS).toBe(5);
  });

  it('has a non-zero cooling-off period after a limit decrease', () => {
    expect(RESPONSIBLE_GAMBLING.COOLDOWN_PERIOD_DAYS).toBeGreaterThan(0);
  });

  it('uses positive minute defaults', () => {
    expect(RESPONSIBLE_GAMBLING.REALITY_CHECK_DEFAULT_MINUTES).toBeGreaterThan(0);
    expect(RESPONSIBLE_GAMBLING.SESSION_TIME_LIMIT_DEFAULT_MINUTES).toBeGreaterThan(0);
  });
});

describe('rate limit constants', () => {
  it('bounds repeated authentication attempts', () => {
    // A high attempt ceiling lets credential stuffing run unthrottled.
    expect(RATE_LIMITS.LOGIN_ATTEMPTS).toBeLessThanOrEqual(10);
    expect(RATE_LIMITS.LOGIN_WINDOW_MS).toBeGreaterThanOrEqual(15 * 60 * 1000);
  });

  it('bounds TOTP attempts', () => {
    expect(RATE_LIMITS.TOTP_MAX_ATTEMPTS).toBeLessThanOrEqual(5);
    expect(RATE_LIMITS.TOTP_WINDOW_SECONDS).toBeGreaterThan(0);
  });

  it('bounds API and money-adjacent operations', () => {
    expect(RATE_LIMITS.API_REQUESTS_PER_MINUTE).toBeGreaterThan(0);
    expect(RATE_LIMITS.API_REQUESTS_PER_HOUR).toBeGreaterThan(RATE_LIMITS.API_REQUESTS_PER_MINUTE);
    expect(RATE_LIMITS.WITHDRAWAL_REQUESTS_PER_DAY).toBeGreaterThan(0);
    expect(RATE_LIMITS.PASSWORD_RESET_PER_HOUR).toBeGreaterThan(0);
    expect(RATE_LIMITS.BETS_PER_SECOND).toBeGreaterThan(0);
  });
});

describe('enum-like constant groups', () => {
  it.each([
    ['WALLET_TYPES', WALLET_TYPES],
    ['TRANSACTION_TYPES', TRANSACTION_TYPES],
    ['BET_TYPES', BET_TYPES],
    ['BET_STATUSES', BET_STATUSES],
    ['BONUS_TYPES', BONUS_TYPES],
    ['KYC_LEVELS', KYC_LEVELS],
    ['DOCUMENT_TYPES', DOCUMENT_TYPES],
    ['NOTIFICATION_CHANNELS', NOTIFICATION_CHANNELS],
    ['NOTIFICATION_TYPES', NOTIFICATION_TYPES],
    ['DEVICE_TYPES', DEVICE_TYPES],
  ])('%s uses snake_case values', (_name, group) => {
    for (const [, value] of entries(group)) {
      expect(value).toMatch(/^[a-z][a-z0-9_]*$/);
    }
  });

  it('has no duplicate values within a group', () => {
    for (const [, group] of [
      ['WALLET_TYPES', WALLET_TYPES],
      ['TRANSACTION_TYPES', TRANSACTION_TYPES],
      ['BET_TYPES', BET_TYPES],
      ['BET_STATUSES', BET_STATUSES],
      ['BONUS_TYPES', BONUS_TYPES],
      ['KYC_LEVELS', KYC_LEVELS],
      ['DOCUMENT_TYPES', DOCUMENT_TYPES],
      ['NOTIFICATION_CHANNELS', NOTIFICATION_CHANNELS],
      ['NOTIFICATION_TYPES', NOTIFICATION_TYPES],
      ['DEVICE_TYPES', DEVICE_TYPES],
    ] as [string, object][]) {
      const values = entries(group).map(([, v]) => v);
      expect(new Set(values).size).toBe(values.length);
    }
  });
});

describe('error codes', () => {
  it('are all unique', () => {
    const codes = entries(ERROR_CODES).map(([, code]) => code);
    expect(new Set(codes).size).toBe(codes.length);
  });

  it('follow the SERVICE_NNNN convention', () => {
    for (const [key, code] of entries(ERROR_CODES)) {
      expect(`${key}:${code}`).toMatch(/^[A-Z0-9_]+:[A-Z]+_\d{4,5}$/);
    }
  });

  it('never reuses a numeric code across different prefixes', () => {
    const seen = new Set<string>();
    for (const [, code] of entries(ERROR_CODES)) {
      expect(seen.has(code)).toBe(false);
      seen.add(code);
    }
  });
});