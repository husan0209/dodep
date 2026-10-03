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
} from '../validators'
import type { CurrencyCode } from '../types'

describe('isValidUuid', () => {
  it('accepts a v4 UUID', () => {
    expect(isValidUuid('3f2504e0-4f89-41d3-9a0c-0305e82c3301')).toBe(true)
  })

  it('rejects a v1 UUID (version nibble must be 4)', () => {
    expect(isValidUuid('3f2504e0-4f89-11d3-9a0c-0305e82c3301')).toBe(false)
  })

  it('rejects malformed input', () => {
    expect(isValidUuid('not-a-uuid')).toBe(false)
    expect(isValidUuid('')).toBe(false)
  })
})

describe('isValidEmail', () => {
  it.each(['user@example.com', 'first.last+tag@sub.example.co.uk'])('accepts %s', (email) => {
    expect(isValidEmail(email)).toBe(true)
  })

  it.each(['plainaddress', 'no@tld', 'two@@example.com', 'has space@example.com'])(
    'rejects %s',
    (email) => {
      expect(isValidEmail(email)).toBe(false)
    },
  )
})

describe('isValidCountryCode', () => {
  it('accepts ISO 3166-1 alpha-2', () => {
    expect(isValidCountryCode('US')).toBe(true)
  })

  it('rejects wrong length or casing', () => {
    expect(isValidCountryCode('USA')).toBe(false)
    expect(isValidCountryCode('us')).toBe(false)
  })
})

describe('isValidCurrencyCode', () => {
  it('accepts ISO 4217 alphabetic codes', () => {
    expect(isValidCurrencyCode('USD' as CurrencyCode)).toBe(true)
    expect(isValidCurrencyCode('EUR' as CurrencyCode)).toBe(true)
  })

  it('rejects non-alphabetic codes', () => {
    expect(isValidCurrencyCode('US' as CurrencyCode)).toBe(false)
    expect(isValidCurrencyCode('123' as CurrencyCode)).toBe(false)
  })
})

describe('isValidMoney', () => {
  it('accepts a decimal amount with a valid currency', () => {
    expect(isValidMoney({ amount: '100.00', currency: 'USD' as CurrencyCode })).toBe(true)
    expect(isValidMoney({ amount: '0', currency: 'USD' as CurrencyCode })).toBe(true)
  })

  it('rejects more than two decimal places', () => {
    expect(isValidMoney({ amount: '1.234', currency: 'USD' as CurrencyCode })).toBe(false)
  })

  it('rejects a bad currency regardless of amount', () => {
    expect(isValidMoney({ amount: '10.00', currency: 'US' as CurrencyCode })).toBe(false)
  })
})

describe('isValidPassword', () => {
  it('accepts a password meeting every rule', () => {
    expect(isValidPassword('Passw0rd!')).toBe(true)
  })

  it.each([
    ['short', 'Pw0!'],
    ['no uppercase', 'passw0rd!'],
    ['no lowercase', 'PASSW0RD!'],
    ['no digit', 'Password!'],
    ['no special character', 'Passw0rdd'],
  ])('rejects a password with %s', (_case, password) => {
    expect(isValidPassword(password)).toBe(false)
  })
})

describe('isValidPhone', () => {
  it('accepts E.164', () => {
    expect(isValidPhone('+14155552671')).toBe(true)
  })

  it('rejects missing + and invalid length', () => {
    expect(isValidPhone('14155552671')).toBe(false)
    expect(isValidPhone('+0415555267')).toBe(false)
  })
})

describe('isValidOdds', () => {
  it('accepts decimal odds inside the allowed band', () => {
    expect(isValidOdds('1.01')).toBe(true)
    expect(isValidOdds('1000')).toBe(true)
  })

  it('rejects odds at or below 1.00 and above 1000', () => {
    expect(isValidOdds('1.00')).toBe(false)
    expect(isValidOdds('1000.01')).toBe(false)
  })

  it('rejects non-numeric input', () => {
    expect(isValidOdds('abc')).toBe(false)
  })
})

describe('isValidPercentage', () => {
  it('accepts the inclusive 0-100 range', () => {
    expect(isValidPercentage(0)).toBe(true)
    expect(isValidPercentage(100)).toBe(true)
  })

  it('rejects out-of-range and non-finite values', () => {
    expect(isValidPercentage(-1)).toBe(false)
    expect(isValidPercentage(101)).toBe(false)
    expect(isValidPercentage(Number.NaN)).toBe(false)
  })
})

describe('sanitizeString', () => {
  it('escapes HTML-significant characters', () => {
    expect(sanitizeString('<script>alert("x")</script>')).toBe(
      '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;',
    )
  })

  it('escapes ampersands before the other entities', () => {
    expect(sanitizeString('a & <b>')).toBe('a &amp; &lt;b&gt;')
  })

  it('escapes single quotes', () => {
    expect(sanitizeString("it's")).toBe('it&#x27;s')
  })
})

describe('isValidIp', () => {
  it('accepts IPv4', () => {
    expect(isValidIp('192.168.1.1')).toBe(true)
  })

  it('rejects out-of-range octets', () => {
    expect(isValidIp('256.1.1.1')).toBe(false)
  })

  it('accepts the full IPv6 form', () => {
    expect(isValidIp('2001:0db8:85a3:0000:0000:8a2e:0370:7334')).toBe(true)
  })
})

describe('isValidDate', () => {
  it('accepts an ISO date', () => {
    expect(isValidDate('2026-10-03')).toBe(true)
  })

  it('rejects a non-date and a malformed string', () => {
    expect(isValidDate('2026-13-01')).toBe(false)
    expect(isValidDate('03/10/2026')).toBe(false)
  })
})

describe('isValidDateTime', () => {
  it('accepts a parseable timestamp', () => {
    expect(isValidDateTime('2026-10-03T10:27:00Z')).toBe(true)
  })

  it('rejects unparseable input', () => {
    expect(isValidDateTime('not a date')).toBe(false)
  })
})
