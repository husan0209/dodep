import {
  addMoney,
  compareMoney,
  formatMoney,
  isNegative,
  isPositive,
  isZero,
  multiplyMoney,
  parseMoney,
  subtractMoney,
} from './helpers';
import { Money } from './types';

const usd = (amount: string): Money => ({ amount, currency: 'USD' });

describe('money minor-unit arithmetic (NEVER-6)', () => {
  it('adds exactly, without float drift', () => {
    expect(addMoney(usd('0.10'), usd('0.20')).amount).toBe('0.30');
  });

  it('accumulates repeated small amounts exactly', () => {
    // 0.07 + 0.07 + 0.07 is 0.21000000000000002 in binary floating point.
    let total = usd('0.00');
    total = addMoney(total, usd('0.07'));
    total = addMoney(total, usd('0.07'));
    total = addMoney(total, usd('0.07'));
    expect(total.amount).toBe('0.21');
  });

  // With valid 2-decimal inputs add/subtract happen to agree with float
  // arithmetic, so these pin the boundary rather than a known regression.
  it.each([
    ['0.01', '0.02'],
    ['99.99', '100.00'],
    ['0.00', '0.01'],
  ])('adds %s without drift', (input, expected) => {
    expect(addMoney(usd(input), usd('0.01')).amount).toBe(expected);
  });

  it('keeps precision above 2^53 where float parsing loses digits', () => {
    // parseFloat("9007199254740993.00") === 9007199254740992
    const big = usd('9007199254740993.00');
    expect(addMoney(big, usd('0.00')).amount).toBe('9007199254740993.00');
    expect(subtractMoney(big, usd('0.01')).amount).toBe('9007199254740992.99');
  });

  it('preserves a negative zero-free representation', () => {
    expect(subtractMoney(usd('1.00'), usd('1.00')).amount).toBe('0.00');
  });

  it('allows a negative result from subtraction (pure arithmetic)', () => {
    expect(subtractMoney(usd('10.00'), usd('25.00')).amount).toBe('-15.00');
  });

  it('normalises single-digit fractions', () => {
    expect(addMoney(usd('0.5'), usd('0.5')).amount).toBe('1.00');
  });

  it('handles amounts with no fraction part', () => {
    expect(addMoney(usd('10'), usd('0.01')).amount).toBe('10.01');
  });

  it('rejects malformed amounts instead of coercing them', () => {
    for (const bad of ['', 'abc', '1.234', '1e5', '1,00', ' 1.00', '+1.00', 'NaN', 'Infinity', '.50']) {
      expect(() => addMoney(usd(bad), usd('1.00'))).toThrow(/Invalid money amount/);
    }
  });

  it('rejects a leading zero such as 007.00', () => {
    expect(() => addMoney(usd('007.00'), usd('0'))).toThrow(/Invalid money amount/);
  });
});

describe('currency handling', () => {
  it('refuses to mix currencies on add, subtract and compare', () => {
    const eur: Money = { amount: '1.00', currency: 'EUR' };
    expect(() => addMoney(usd('1.00'), eur)).toThrow(/Currency mismatch/);
    expect(() => subtractMoney(usd('1.00'), eur)).toThrow(/Currency mismatch/);
    expect(() => compareMoney(usd('1.00'), eur)).toThrow(/Currency mismatch/);
  });

  it('uppercases the currency in parseMoney', () => {
    expect(parseMoney('1.00', 'usd').currency).toBe('USD');
  });

  it('normalises the amount in parseMoney', () => {
    expect(parseMoney('1.5', 'USD').amount).toBe('1.50');
    expect(parseMoney('7', 'USD').amount).toBe('7.00');
  });

  it('refuses a number input so no float can reach the money path', () => {
    // @ts-expect-error deliberately wrong: parseFloat must not be accepted here
    expect(() => parseMoney(0.1 + 0.2, 'USD')).toThrow(/Invalid money amount/);
  });
});

describe('multiplyMoney', () => {
  it('multiplies by a fractional scalar', () => {
    expect(multiplyMoney(usd('100.00'), 0.5).amount).toBe('50.00');
    expect(multiplyMoney(usd('100.00'), 1.5).amount).toBe('150.00');
  });

  // These are the float regressions: every one of them was a cent LOW with
  // `parseFloat(amount) * scalar`, because the product lands just under the
  // midpoint in binary and toFixed then rounds down.
  it.each([
    ['0.01', 1.5, '0.02'],   // float: 0.01 (0.015 under)
    ['0.03', 1.5, '0.05'],   // float: 0.04 (0.045 under)
    ['0.05', 0.3, '0.02'],   // float: 0.01 (0.015 under)
    ['0.09', 2.5, '0.23'],   // float: 0.22 (0.225 under)
    ['0.10', 1.15, '0.12'],  // float: 0.11 (0.115 under)
    ['0.15', 1.5, '0.23'],   // float: 0.22 (0.225 under)
  ])('multiplies %s by %s as %s, not the float result', (amount, scalar, expected) => {
    expect(multiplyMoney(usd(amount), scalar).amount).toBe(expected);
  });

  it('takes the absolute value of a negative scalar', () => {
    // A negative scalar must not produce a negative, creditable amount.
    expect(multiplyMoney(usd('100.00'), -1.5).amount).toBe('150.00');
  });

  it('preserves the sign of an already negative amount', () => {
    expect(multiplyMoney(usd('-100.00'), 2).amount).toBe('-200.00');
  });

  it('rejects non-finite scalars', () => {
    expect(() => multiplyMoney(usd('100.00'), Number.NaN)).toThrow(/Invalid scalar/);
    expect(() => multiplyMoney(usd('100.00'), Number.POSITIVE_INFINITY)).toThrow(/Invalid scalar/);
  });
});

describe('compareMoney', () => {
  it('orders by value', () => {
    expect(compareMoney(usd('100.00'), usd('50.00'))).toBe(1);
    expect(compareMoney(usd('50.00'), usd('100.00'))).toBe(-1);
    expect(compareMoney(usd('100.00'), usd('100.00'))).toBe(0);
  });

  it('does not confuse values that a float would render identically', () => {
    // 0.1 + 0.2 !== 0.3 as floats; in minor units they are exactly equal.
    const third = subtractMoney(addMoney(usd('0.10'), usd('0.20')), usd('0.00'));
    expect(compareMoney(third, usd('0.30'))).toBe(0);
  });

  it('orders negative amounts correctly', () => {
    expect(compareMoney(usd('-5.00'), usd('0.00'))).toBe(-1);
  });
});

describe('predicates', () => {
  it('classifies zero, positive and negative', () => {
    expect(isZero(usd('0.00'))).toBe(true);
    expect(isZero(usd('0.01'))).toBe(false);
    expect(isPositive(usd('0.01'))).toBe(true);
    expect(isPositive(usd('0.00'))).toBe(false);
    expect(isNegative(usd('-0.01'))).toBe(true);
    expect(isNegative(usd('0.00'))).toBe(false);
  });

  it('treats a bare "0" as zero', () => {
    expect(isZero(usd('0'))).toBe(true);
  });

  it('rejects a sub-cent amount rather than truncating it to zero', () => {
    // 0.001 has three fraction digits. A float implementation rendered it as
    // "0.00" and then reported it as neither positive nor zero, which is a
    // silent loss of value. It is now a hard error.
    expect(() => isPositive(usd('0.001'))).toThrow(/Invalid money amount/);
    expect(() => isZero(usd('0.001'))).toThrow(/Invalid money amount/);
  });
});

describe('formatMoney', () => {
  it('formats a known currency', () => {
    expect(formatMoney(usd('1234.56'), 'en-US')).toBe('$1,234.56');
  });

  it('always shows two fraction digits', () => {
    expect(formatMoney(usd('5'), 'en-US')).toBe('$5.00');
  });

  it('omits decimals for a zero-decimal currency', () => {
    // JPY has ISO 4217 exponent 0, so "1234" is 1234 yen, not 1234.00 yen.
    // Forcing two fraction digits invented decimals the currency lacks.
    expect(formatMoney({ amount: '1234', currency: 'JPY' }, 'en-US')).toBe('¥1,234');
  });

  it('shows two decimals for a two-decimal currency', () => {
    expect(formatMoney(usd('1234.5'), 'en-US')).toBe('$1,234.50');
  });

  it('defaults to en-US', () => {
    expect(formatMoney(usd('1.00'))).toBe('$1.00');
  });
});