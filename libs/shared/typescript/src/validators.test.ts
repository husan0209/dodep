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

describe('isValidUuid', () => {
  it('accepts a v4 uuid in either case', () => {
    expect(isValidUuid('550e8400-e29b-41d4-a716-446655440000')).toBe(true);
    expect(isValidUuid('550E8400-E29B-41D4-A716-446655440000')).toBe(true);
  });

  it('rejects anything that is not a v4 uuid', () => {
    for (const bad of [
      '',
      'not-a-uuid',
      '550e8400-e29b-41d4-a716',
      // Version nibble is 1, not 4.
      '550e8400-e29b-11d4-a716-446655440000',
      // Variant nibble out of range.
      '550e8400-e29b-41d4-0716-446655440000',
      '550e8400e29b41d4a716446655440000',
    ]) {
      expect(isValidUuid(bad)).toBe(false);
    }
  });
});

describe('isValidEmail', () => {
  it('accepts ordinary addresses', () => {
    expect(isValidEmail('user@example.com')).toBe(true);
    expect(isValidEmail('test.user+tag@domain.co.uk')).toBe(true);
  });

  it('rejects malformed addresses', () => {
    for (const bad of [
      '',
      'invalid',
      '@example.com',
      'user@',
      'a b@c.com',
      'user@example',
      'user@@example.com',
    ]) {
      expect(isValidEmail(bad)).toBe(false);
    }
  });
});

describe('code validators', () => {
  it('validates country codes', () => {
    for (const good of ['US', 'UA', 'DE', 'GB']) {
      expect(isValidCountryCode(good as never)).toBe(true);
    }
    for (const bad of ['', 'U', 'USA', 'us', 'U1', '12']) {
      expect(isValidCountryCode(bad as never)).toBe(false);
    }
  });

  it('validates currency codes', () => {
    for (const good of ['USD', 'EUR', 'JPY', 'BTC']) {
      expect(isValidCurrencyCode(good as never)).toBe(true);
    }
    for (const bad of ['', 'US', 'USDD', 'usd', 'U1D']) {
      expect(isValidCurrencyCode(bad as never)).toBe(false);
    }
  });
});

describe('isValidMoney', () => {
  it('accepts non-negative amounts with up to two decimals', () => {
    for (const amount of ['0', '0.00', '10', '10.5', '100.00', '999999999.99']) {
      expect(isValidMoney({ amount, currency: 'USD' })).toBe(true);
    }
  });

  it('accepts a lowercase currency, matching parseMoney normalisation', () => {
    expect(isValidMoney({ amount: '10.00', currency: 'usd' })).toBe(true);
  });

  it('rejects negative and malformed amounts', () => {
    for (const amount of ['-1', '-0.01', '', 'abc', '1e5', '1.234', ' 1.00', '+1.00', '1,00', '007.00']) {
      expect(isValidMoney({ amount, currency: 'USD' })).toBe(false);
    }
  });

  it('rejects an invalid currency', () => {
    expect(isValidMoney({ amount: '10.00', currency: 'US' })).toBe(false);
    expect(isValidMoney({ amount: '10.00', currency: '' })).toBe(false);
  });
});

describe('isValidPassword', () => {
  it('accepts passwords meeting every rule', () => {
    expect(isValidPassword('SecureP@ss123')).toBe(true);
    expect(isValidPassword('MyP@ssw0rd!')).toBe(true);
  });

  it('rejects passwords missing a class or too short', () => {
    for (const bad of [
      '',
      'weak',
      'nouppercase1!',
      'NOLOWERCASE1!',
      'NoSpecial1',
      'NoDigit@!',
      'Sh0rt!@',
    ]) {
      expect(isValidPassword(bad)).toBe(false);
    }
  });
});

describe('isValidPhone', () => {
  it('accepts E.164 numbers', () => {
    expect(isValidPhone('+14155552671')).toBe(true);
    expect(isValidPhone('+380501234567')).toBe(true);
  });

  it('rejects non-E.164 input', () => {
    for (const bad of ['', '4155552671', '+0415552671', '+1', '+1415555267123456', '+1a55552671', '+ 14155552671']) {
      expect(isValidPhone(bad)).toBe(false);
    }
  });
});

describe('isValidOdds', () => {
  it('accepts decimal odds inside the platform bounds', () => {
    for (const good of ['1.01', '1.50', '2.00', '100.00', '1000', '1000.00']) {
      expect(isValidOdds(good)).toBe(true);
    }
  });

  it('rejects odds outside the bounds or malformed', () => {
    for (const bad of ['1.00', '0.99', '1000.01', '1001', '-2', '', 'abc', '1.5.5', '1e2']) {
      expect(isValidOdds(bad)).toBe(false);
    }
  });
});

describe('isValidPercentage', () => {
  it('accepts 0 through 100 inclusive', () => {
    expect(isValidPercentage(0)).toBe(true);
    expect(isValidPercentage(50.5)).toBe(true);
    expect(isValidPercentage(100)).toBe(true);
  });

  it('rejects out-of-range and non-finite values', () => {
    for (const bad of [-0.1, 100.1, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY]) {
      expect(isValidPercentage(bad)).toBe(false);
    }
  });
});

// The previous regexes accepted only the fully expanded eight-group IPv6 form,
// so ::1 and 2001:db8::1 — the addresses real clients arrive with — were
// rejected, while Go (net.ParseIP) and Rust (std::net::IpAddr) accepted them.
describe('isValidIp', () => {
  it('accepts IPv4', () => {
    for (const ip of ['127.0.0.1', '0.0.0.0', '192.168.1.1', '255.255.255.255', '8.8.8.8']) {
      expect(isValidIp(ip)).toBe(true);
    }
  });

  it('rejects malformed IPv4', () => {
    for (const ip of ['', '256.1.1.1', '999.1.1.1', '1.2.3', '1.2.3.4.5', '01.2.3.4', '1.2.3.-1', 'abc']) {
      expect(isValidIp(ip)).toBe(false);
    }
  });

  it('accepts fully expanded IPv6', () => {
    expect(isValidIp('2001:0db8:85a3:0000:0000:8a2e:0370:7334')).toBe(true);
    expect(isValidIp('2001:db8:85a3:0:0:8a2e:370:7334')).toBe(true);
  });

  it('accepts compressed IPv6, which the old regex rejected', () => {
    expect(isValidIp('::1')).toBe(true);
    expect(isValidIp('::')).toBe(true);
    expect(isValidIp('fe80::1')).toBe(true);
    expect(isValidIp('2001:db8::1')).toBe(true);
    expect(isValidIp('2001:db8:0:1:1:1:1:1')).toBe(true);
  });

  it('accepts an IPv4-mapped IPv6 address', () => {
    expect(isValidIp('::ffff:192.168.1.1')).toBe(true);
    expect(isValidIp('64:ff9b::192.0.2.33')).toBe(true);
  });

  it('accepts a zone index', () => {
    expect(isValidIp('fe80::1%eth0')).toBe(true);
    expect(isValidIp('fe80::1%')).toBe(false);
    expect(isValidIp('fe80::1%a%b')).toBe(false);
  });

  it('rejects structurally invalid IPv6', () => {
    for (const ip of [
      '2001:db8::1::2',   // two "::"
      '2001:db8:::1',
      '2001:db8::1::',
      '12345::1',         // group too long
      'gggg::1',          // non-hex
      '1:2:3:4:5:6:7',    // too few groups without "::"
      '1:2:3:4:5:6:7:8:9',
      '::ffff:999.1.1.1', // bad embedded IPv4
      ':::',
    ]) {
      expect(isValidIp(ip)).toBe(false);
    }
  });
});

// Regression: `new Date("2026-02-30")` does not produce NaN, it rolls over to
// 2026-03-02, so the previous implementation validated a date that does not
// exist. A date of birth or an RG limit boundary could be set to it.
describe('isValidDate', () => {
  it('accepts real calendar dates', () => {
    for (const date of ['2026-09-30', '2026-01-01', '2026-12-31', '2024-02-29', '2026-10-02']) {
      expect(isValidDate(date)).toBe(true);
    }
  });

  it('rejects impossible days, which JS used to roll over silently', () => {
    for (const date of ['2026-02-30', '2026-04-31', '2025-02-29', '2026-06-31', '2026-01-32', '2026-01-00']) {
      expect(isValidDate(date)).toBe(false);
    }
  });

  it('rejects impossible months', () => {
    for (const date of ['2026-13-01', '2026-00-10', '2026-99-01']) {
      expect(isValidDate(date)).toBe(false);
    }
  });

  it('applies the full leap-year rule', () => {
    expect(isValidDate('2024-02-29')).toBe(true);  // divisible by 4
    expect(isValidDate('2025-02-29')).toBe(false); // not a leap year
    expect(isValidDate('2000-02-29')).toBe(true);  // divisible by 400
    expect(isValidDate('1900-02-29')).toBe(false); // divisible by 100 only
  });

  it('rejects non-ISO shapes', () => {
    for (const date of ['', '30-09-2026', '2026-9-3', '2026/09/30', '2026-09-30T00:00:00Z', '20260930', 'abcd-ef-gh']) {
      expect(isValidDate(date)).toBe(false);
    }
  });
});

describe('isValidDateTime', () => {
  it('accepts parseable datetimes', () => {
    expect(isValidDateTime('2026-10-02T15:04:05Z')).toBe(true);
    expect(isValidDateTime('2026-10-02T15:04:05.123Z')).toBe(true);
  });

  it('rejects unparseable input', () => {
    expect(isValidDateTime('')).toBe(false);
    expect(isValidDateTime('not-a-date')).toBe(false);
  });
});

describe('sanitizeString', () => {
  it('escapes HTML metacharacters', () => {
    expect(sanitizeString('<script>alert("x")</script>')).toBe(
      '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;',
    );
  });

  it('escapes ampersands first so entities are not double-broken', () => {
    expect(sanitizeString('a & b')).toBe('a &amp; b');
    expect(sanitizeString('&lt;')).toBe('&amp;lt;');
  });

  it('escapes single quotes', () => {
    expect(sanitizeString("it's")).toBe('it&#x27;s');
  });

  it('leaves plain text untouched', () => {
    expect(sanitizeString('player 42')).toBe('player 42');
    expect(sanitizeString('')).toBe('');
  });
});