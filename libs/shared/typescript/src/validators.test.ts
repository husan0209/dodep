import {
  isValidCountryCode,
  isValidCurrencyCode,
  isValidDate,
  isValidDateTime,
  isValidEmail,
  isValidIp,
  isValidMoney,
  isValidOdds,
  isValidPassword,
  isValidPercentage,
  isValidPhone,
  isValidUuid,
  sanitizeString,
} from './validators';

describe('isValidMoney (NUMERIC(18,8))', () => {
  it('accepts decimal strings up to 8 fractional digits', () => {
    expect(isValidMoney({ amount: '0', currency: 'USD' })).toBe(true);
    expect(isValidMoney({ amount: '1500.75', currency: 'USD' })).toBe(true);
    expect(isValidMoney({ amount: '0.00000001', currency: 'BTC' })).toBe(true);
    expect(isValidMoney({ amount: '9999999999.99999999', currency: 'USD' })).toBe(true);
  });

  it('rejects floats, exponents and out-of-range precision', () => {
    expect(isValidMoney({ amount: 1500.75 as unknown as string, currency: 'USD' })).toBe(false);
    expect(isValidMoney({ amount: '1e3', currency: 'USD' })).toBe(false);
    expect(isValidMoney({ amount: '0.000000001', currency: 'USD' })).toBe(false);
    expect(isValidMoney({ amount: '12345678901', currency: 'USD' })).toBe(false);
    expect(isValidMoney({ amount: '1,500.00', currency: 'USD' })).toBe(false);
    expect(isValidMoney({ amount: '-10.00', currency: 'USD' })).toBe(false);
  });

  it('requires a valid ISO 4217 currency code', () => {
    expect(isValidMoney({ amount: '10.00', currency: 'usd' } as never)).toBe(false);
    expect(isValidMoney({ amount: '10.00', currency: 'US' } as never)).toBe(false);
  });
});

describe('isValidOdds', () => {
  it('accepts the platform range', () => {
    expect(isValidOdds('1.01')).toBe(true);
    expect(isValidOdds('1.85')).toBe(true);
    expect(isValidOdds('1000')).toBe(true);
    expect(isValidOdds('2')).toBe(true);
  });

  it('rejects values outside [1.01, 1000] without float rounding', () => {
    expect(isValidOdds('1')).toBe(false);
    expect(isValidOdds('1.009')).toBe(false);
    expect(isValidOdds('1.0')).toBe(false);
    expect(isValidOdds('1000.0001')).toBe(false);
    expect(isValidOdds('1001')).toBe(false);
  });

  it('rejects non-string and malformed input', () => {
    expect(isValidOdds(1.85 as unknown as string)).toBe(false);
    expect(isValidOdds('')).toBe(false);
    expect(isValidOdds('-1.85')).toBe(false);
    expect(isValidOdds('1.85000')).toBe(false); // 5 fractional digits
  });
});

describe('isValidPercentage', () => {
  it('accepts decimal strings in [0, 100]', () => {
    expect(isValidPercentage('0')).toBe(true);
    expect(isValidPercentage('100')).toBe(true);
    expect(isValidPercentage('20.5')).toBe(true);
    expect(isValidPercentage('99.99999999')).toBe(true);
  });

  it('rejects out-of-range and malformed values', () => {
    expect(isValidPercentage('100.00000001')).toBe(false);
    expect(isValidPercentage('101')).toBe(false);
    expect(isValidPercentage('-1')).toBe(false);
    expect(isValidPercentage('abc')).toBe(false);
  });

  it('still guards finite numbers', () => {
    expect(isValidPercentage(0)).toBe(true);
    expect(isValidPercentage(100)).toBe(true);
    expect(isValidPercentage(Number.NaN)).toBe(false);
    expect(isValidPercentage(Number.POSITIVE_INFINITY)).toBe(false);
    expect(isValidPercentage(101)).toBe(false);
  });
});

describe('sanitizeString', () => {
  it('escapes HTML metacharacters', () => {
    expect(sanitizeString('<script>alert("x")</script>')).toBe(
      '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;',
    );
    expect(sanitizeString("it's")).toBe('it&#x27;s');
    expect(sanitizeString('`back`')).toBe('&#x60;back&#x60;');
    expect(sanitizeString('a=b')).toBe('a&#x3D;b');
  });

  it('is idempotent (no double escaping)', () => {
    const once = sanitizeString('<b>a & b</b>');
    expect(sanitizeString(once)).toBe(once);
  });

  it('neutralizes ampersand injection through an existing entity', () => {
    expect(sanitizeString('&lt;')).toBe('&lt;');
    expect(sanitizeString('&')).toBe('&amp;');
  });

  it('handles non-string input safely', () => {
    expect(sanitizeString(undefined as unknown as string)).toBe('');
  });
});

describe('isValidDate', () => {
  it('accepts real calendar dates', () => {
    expect(isValidDate('2026-03-28')).toBe(true);
    expect(isValidDate('2024-02-29')).toBe(true); // leap year
  });

  it('rejects dates the Date constructor would roll over', () => {
    expect(isValidDate('2026-02-31')).toBe(false);
    expect(isValidDate('2025-02-29')).toBe(false);
    expect(isValidDate('2026-04-31')).toBe(false);
    expect(isValidDate('2026-13-01')).toBe(false);
    expect(isValidDate('2026-00-10')).toBe(false);
  });

  it('rejects other formats', () => {
    expect(isValidDate('28.03.2026')).toBe(false);
    expect(isValidDate('2026-3-8')).toBe(false);
    expect(isValidDate('')).toBe(false);
  });
});

describe('isValidDateTime', () => {
  it('accepts ISO 8601 datetimes', () => {
    expect(isValidDateTime('2026-03-28T10:15:30Z')).toBe(true);
    expect(isValidDateTime('2026-03-28T10:15:30')).toBe(true);
    expect(isValidDateTime('2026-03-28T10:15:30.123Z')).toBe(true);
    expect(isValidDateTime('2026-03-28T10:15:30+02:00')).toBe(true);
  });

  it('rejects loose date formats and rolled-over dates', () => {
    expect(isValidDateTime('01/02/2026')).toBe(false);
    expect(isValidDateTime('2026-03-28')).toBe(false);
    expect(isValidDateTime('2026-02-31T00:00:00Z')).toBe(false);
    expect(isValidDateTime('not a date')).toBe(false);
  });
});

describe('isValidIp', () => {
  it('accepts IPv4', () => {
    expect(isValidIp('127.0.0.1')).toBe(true);
    expect(isValidIp('255.255.255.255')).toBe(true);
    expect(isValidIp('0.0.0.0')).toBe(true);
  });

  it('rejects invalid IPv4', () => {
    expect(isValidIp('256.1.1.1')).toBe(false);
    expect(isValidIp('1.1.1')).toBe(false);
    expect(isValidIp('1.1.1.1.1')).toBe(false);
  });

  it('accepts compressed and full IPv6', () => {
    expect(isValidIp('::1')).toBe(true);
    expect(isValidIp('::')).toBe(true);
    expect(isValidIp('2001:db8::8a2e:370:7334')).toBe(true);
    expect(isValidIp('fe80:0:0:0:0:0:0:1')).toBe(true);
  });

  it('rejects malformed IPv6', () => {
    expect(isValidIp('::::')).toBe(false);
    expect(isValidIp('2001::db8::1')).toBe(false); // two compressions
    expect(isValidIp('2001:db8:::1')).toBe(false);
    expect(isValidIp('gggg::1')).toBe(false);
    expect(isValidIp('12345::1')).toBe(false);
  });

  it('rejects empty input', () => {
    expect(isValidIp('')).toBe(false);
  });
});

describe('unchanged validators still behave', () => {
  it('uuid, email, phone, password, currency, country', () => {
    expect(isValidUuid('11111111-1111-4111-8111-111111111111')).toBe(true);
    expect(isValidUuid('11111111-1111-1111-1111-111111111111')).toBe(false);
    expect(isValidEmail('player@example.com')).toBe(true);
    expect(isValidEmail('player@')).toBe(false);
    expect(isValidPhone('+79961234567')).toBe(true);
    expect(isValidPhone('79961234567')).toBe(false);
    expect(isValidPassword('Str0ng!pass')).toBe(true);
    expect(isValidPassword('weakpass')).toBe(false);
    expect(isValidCurrencyCode('USD')).toBe(true);
    expect(isValidCountryCode('DE')).toBe(true);
    expect(isValidCountryCode('de')).toBe(false);
  });
});