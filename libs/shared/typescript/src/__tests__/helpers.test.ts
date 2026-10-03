import {
  addMoney,
  compareMoney,
  deepClone,
  debounce,
  formatMoney,
  generateUuid,
  isEmpty,
  isNegative,
  isPositive,
  isZero,
  multiplyMoney,
  omit,
  parseMoney,
  pick,
  retry,
  subtractMoney,
} from '../helpers'
import type { CurrencyCode } from '../types'
import { isValidUuid } from '../validators'

const USD = 'USD' as CurrencyCode
const EUR = 'EUR' as CurrencyCode

describe('parseMoney', () => {
  it('normalises a number to two decimals', () => {
    expect(parseMoney(10, USD)).toEqual({ amount: '10.00', currency: 'USD' })
    expect(parseMoney(10.005, USD)).toEqual({ amount: '10.01', currency: 'USD' })
  })

  it('passes a string amount through', () => {
    expect(parseMoney('10.50', USD)).toEqual({ amount: '10.50', currency: 'USD' })
  })
})

describe('formatMoney', () => {
  it('formats using the currency of the Money object', () => {
    expect(formatMoney({ amount: '1234.5', currency: USD })).toBe('$1,234.50')
  })
})

describe('addMoney / subtractMoney', () => {
  it('adds and subtracts within the same currency', () => {
    expect(addMoney({ amount: '10.25', currency: USD }, { amount: '5.75', currency: USD })).toEqual(
      {
        amount: '16.00',
        currency: 'USD',
      },
    )
    expect(
      subtractMoney({ amount: '10.25', currency: USD }, { amount: '0.25', currency: USD }),
    ).toEqual({ amount: '10.00', currency: 'USD' })
  })

  it('refuses to mix currencies', () => {
    expect(() =>
      addMoney({ amount: '1.00', currency: USD }, { amount: '1.00', currency: EUR }),
    ).toThrow(/Currency mismatch/)
    expect(() =>
      subtractMoney({ amount: '1.00', currency: USD }, { amount: '1.00', currency: EUR }),
    ).toThrow(/Currency mismatch/)
  })
})

describe('multiplyMoney', () => {
  it('scales the amount and keeps two decimals', () => {
    expect(multiplyMoney({ amount: '10.00', currency: USD }, 3)).toEqual({
      amount: '30.00',
      currency: 'USD',
    })
  })

  it('returns the absolute value for a negative scalar', () => {
    expect(multiplyMoney({ amount: '10.00', currency: USD }, -2)).toEqual({
      amount: '20.00',
      currency: 'USD',
    })
  })
})

describe('compareMoney', () => {
  it('orders by amount', () => {
    const one = { amount: '1.00', currency: USD }
    const two = { amount: '2.00', currency: USD }
    expect(compareMoney(one, two)).toBe(-1)
    expect(compareMoney(two, one)).toBe(1)
    expect(compareMoney(one, one)).toBe(0)
  })

  it('refuses to compare different currencies', () => {
    expect(() =>
      compareMoney({ amount: '1.00', currency: USD }, { amount: '1.00', currency: EUR }),
    ).toThrow(/Currency mismatch/)
  })
})

describe('sign predicates', () => {
  it('classifies zero, positive and negative amounts', () => {
    expect(isZero({ amount: '0.00', currency: USD })).toBe(true)
    expect(isPositive({ amount: '0.01', currency: USD })).toBe(true)
    expect(isNegative({ amount: '-0.01', currency: USD })).toBe(true)
    expect(isZero({ amount: '0.01', currency: USD })).toBe(false)
  })
})

describe('generateUuid', () => {
  it('produces a v4 UUID the validators accept', () => {
    expect(isValidUuid(generateUuid())).toBe(true)
  })

  it('produces distinct values', () => {
    expect(generateUuid()).not.toBe(generateUuid())
  })
})

describe('retry', () => {
  it('returns the value once the function succeeds', async () => {
    const fn = jest
      .fn<Promise<string>, []>()
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValue('ok')

    await expect(retry(fn, { initialDelay: 1 })).resolves.toBe('ok')
    expect(fn).toHaveBeenCalledTimes(2)
  })

  it('rethrows the last error after exhausting retries', async () => {
    const fn = jest.fn<Promise<string>, []>().mockRejectedValue(new Error('always'))

    await expect(retry(fn, { maxRetries: 2, initialDelay: 1 })).rejects.toThrow('always')
    expect(fn).toHaveBeenCalledTimes(3) // initial attempt + 2 retries
  })
})

describe('debounce', () => {
  it('invokes the function once after the delay', async () => {
    jest.useFakeTimers()
    const fn = jest.fn()
    const debounced = debounce(fn, 50)

    debounced()
    debounced()
    debounced()
    expect(fn).not.toHaveBeenCalled()

    jest.advanceTimersByTime(50)
    expect(fn).toHaveBeenCalledTimes(1)
    jest.useRealTimers()
  })
})

describe('object utilities', () => {
  it('deepClone produces an independent copy', () => {
    const original = { a: { b: [1, 2] } }
    const clone = deepClone(original)
    expect(clone).toEqual(original)
    expect(clone.a).not.toBe(original.a)
  })

  it('isEmpty detects empty objects only', () => {
    expect(isEmpty({})).toBe(true)
    expect(isEmpty({ a: 1 })).toBe(false)
  })

  it('pick keeps only the requested present keys', () => {
    expect(pick({ a: 1, b: 2, c: 3 }, ['a', 'c'])).toEqual({ a: 1, c: 3 })
    expect(pick({ a: 1 }, ['a', 'missing' as keyof { a: number }])).toEqual({ a: 1 })
  })

  it('omit drops the requested keys', () => {
    expect(omit({ a: 1, b: 2, c: 3 }, ['b'])).toEqual({ a: 1, c: 3 })
  })
})
