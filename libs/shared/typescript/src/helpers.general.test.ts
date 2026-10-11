import {
  debounce,
  deepClone,
  generateUuid,
  isEmpty,
  now,
  nowIso,
  omit,
  pick,
  retry,
  sleep,
  throttle,
} from './helpers';
import { isValidUuid } from './validators';

describe('generateUuid', () => {
  it('produces a valid v4 uuid', () => {
    expect(isValidUuid(generateUuid())).toBe(true);
  });

  it('produces distinct values', () => {
    const seen = new Set(Array.from({ length: 500 }, () => generateUuid()));
    expect(seen.size).toBe(500);
  });

  // The fallback path runs on HTTP origins and older runtimes where
  // crypto.randomUUID is unavailable. It must still produce v4 values.
  it('produces a valid uuid on the fallback path', () => {
    const original = globalThis.crypto;
    Object.defineProperty(globalThis, 'crypto', {
      value: { getRandomValues: original?.getRandomValues?.bind(original) },
      configurable: true,
    });

    try {
      const seen = new Set<string>();
      for (let i = 0; i < 200; i += 1) {
        const uuid = generateUuid();
        expect(isValidUuid(uuid)).toBe(true);
        seen.add(uuid);
      }
      expect(seen.size).toBe(200);
    } finally {
      Object.defineProperty(globalThis, 'crypto', {
        value: original,
        configurable: true,
      });
    }
  });

  it('produces a valid uuid with no Web Crypto at all', () => {
    const original = globalThis.crypto;
    Object.defineProperty(globalThis, 'crypto', {
      value: undefined,
      configurable: true,
    });

    try {
      expect(isValidUuid(generateUuid())).toBe(true);
    } finally {
      Object.defineProperty(globalThis, 'crypto', {
        value: original,
        configurable: true,
      });
    }
  });
});

describe('timestamps', () => {
  it('now returns non-decreasing milliseconds', () => {
    const a = now();
    const b = now();
    expect(b).toBeGreaterThanOrEqual(a);
  });

  it('nowIso returns a parseable ISO 8601 timestamp', () => {
    expect(Number.isNaN(Date.parse(nowIso()))).toBe(false);
  });

  it('sleep resolves after the delay', async () => {
    const started = Date.now();
    await sleep(30);
    expect(Date.now() - started).toBeGreaterThanOrEqual(25);
  });
});

describe('retry', () => {
  it('returns the first successful result', async () => {
    let calls = 0;
    const result = await retry(async () => {
      calls += 1;
      return 'ok';
    });
    expect(result).toBe('ok');
    expect(calls).toBe(1);
  });

  it('recovers after transient failures', async () => {
    let calls = 0;
    const result = await retry(
      async () => {
        calls += 1;
        if (calls < 3) {
          throw new Error('flaky');
        }
        return 'ok';
      },
      { maxRetries: 5, initialDelay: 1 },
    );
    expect(result).toBe('ok');
    expect(calls).toBe(3);
  });

  it('rethrows the last error once retries are exhausted', async () => {
    let calls = 0;
    await expect(
      retry(
        async () => {
          calls += 1;
          throw new Error('always');
        },
        { maxRetries: 2, initialDelay: 1 },
      ),
    ).rejects.toThrow('always');
    // One initial attempt plus two retries.
    expect(calls).toBe(3);
  });

  it('respects maxDelay when the multiplier would exceed it', async () => {
    const started = Date.now();
    await expect(
      retry(async () => { throw new Error('nope'); }, {
        maxRetries: 4,
        initialDelay: 20,
        maxDelay: 25,
        multiplier: 10,
      }),
    ).rejects.toThrow('nope');
    // Delays would be 20, 25, 25, 25 = 95ms without the cap, far more with it.
    expect(Date.now() - started).toBeLessThan(1000);
  });
});

describe('debounce', () => {
  it('invokes once for a burst of calls', async () => {
    let calls = 0;
    const fn = debounce(() => { calls += 1; }, 20);

    fn();
    fn();
    fn();

    expect(calls).toBe(0);
    await sleep(50);
    expect(calls).toBe(1);
  });

  it('invokes again after the quiet period', async () => {
    let calls = 0;
    const fn = debounce(() => { calls += 1; }, 20);

    fn();
    await sleep(40);
    fn();
    await sleep(40);

    expect(calls).toBe(2);
  });

  it('passes the latest arguments through', async () => {
    const seen: string[] = [];
    const fn = debounce((value: string) => { seen.push(value); }, 20);

    fn('first');
    fn('second');

    await sleep(50);
    expect(seen).toEqual(['second']);
  });
});

describe('throttle', () => {
  it('allows the leading call and blocks within the window', async () => {
    let calls = 0;
    const fn = throttle(() => { calls += 1; }, 40);

    fn();
    fn();
    fn();

    expect(calls).toBe(1);
    await sleep(60);
    fn();
    expect(calls).toBe(2);
  });
});

describe('deepClone', () => {
  it('produces an independent copy', () => {
    const source = { nested: { tags: ['a', 'b'] }, count: 1 };
    const clone = deepClone(source);

    clone.nested.tags[0] = 'mutated';
    clone.count = 99;

    expect(source.nested.tags[0]).toBe('a');
    expect(source.count).toBe(1);
  });

  it('handles primitives and arrays', () => {
    expect(deepClone(5)).toBe(5);
    expect(deepClone('text')).toBe('text');
    expect(deepClone(null)).toBeNull();
    expect(deepClone([1, 2, 3])).toEqual([1, 2, 3]);
  });
});

describe('isEmpty', () => {
  it('detects empty and non-empty objects', () => {
    expect(isEmpty({})).toBe(true);
    expect(isEmpty({ a: 1 })).toBe(false);
    // A key present with an undefined value still counts as present.
    expect(isEmpty({ a: undefined })).toBe(false);
  });
});

describe('pick', () => {
  it('selects only the requested keys', () => {
    expect(pick({ a: 1, b: 2, c: 3 }, ['a', 'c'])).toEqual({ a: 1, c: 3 });
  });

  it('skips keys that are absent', () => {
    expect(pick({ a: 1 } as Record<string, unknown>, ['a', 'missing'])).toEqual({ a: 1 });
  });

  it('returns an empty object when nothing matches', () => {
    const source = { a: 1 } as Record<string, unknown>;
    expect(pick(source, ['zzz'])).toEqual({});
  });
});

describe('omit', () => {
  it('removes only the requested keys', () => {
    expect(omit({ a: 1, b: 2, c: 3 }, ['b'])).toEqual({ a: 1, c: 3 });
  });

  it('ignores keys that are absent', () => {
    expect(omit({ a: 1 }, ['zzz'] as never)).toEqual({ a: 1 });
  });

  it('does not mutate the source', () => {
    const source = { a: 1, b: 2 };
    omit(source, ['a']);
    expect(source).toEqual({ a: 1, b: 2 });
  });
});