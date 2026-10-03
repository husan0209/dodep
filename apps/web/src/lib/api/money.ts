/**
 * Exact decimal money primitives for the API boundary.
 *
 * Platform rule (CONVENTIONS NEVER-6): money is NEVER a JS number.
 * Amounts travel as decimal strings matching PostgreSQL NUMERIC(18,8),
 * and all arithmetic here is exact (BigInt), so `0.1 + 0.2` problems and
 * float rounding can never reach a deposit, withdrawal or bet.
 *
 * Wire format: `[-]<up to 10 digits>[.<up to 8 digits>]`
 * (18 total precision, 8 fractional digits — same as NUMERIC(18,8)).
 */

export const MONEY_SCALE = 8;
export const MONEY_TOTAL_PRECISION = 18;

/** Strict decimal string: no exponent, no thousands separators, no spaces. */
const MONEY_RE = /^(-?)(\d{1,10})(?:\.(\d{1,8}))?$/;

export class MoneyError extends Error {
  constructor(
    message: string,
    readonly field?: string,
  ) {
    super(field ? `${field}: ${message}` : message);
    this.name = "MoneyError";
  }
}

/** A parsed money value: sign + unscaled integer of 10^8 minor units. */
export interface ParsedMoney {
  negative: boolean;
  /** Absolute value scaled by 10^MONEY_SCALE. */
  minor: bigint;
}

function parse(value: string, field?: string): ParsedMoney {
  if (typeof value !== "string") {
    throw new MoneyError(
      `money must be a decimal string, received ${typeof value}`,
      field,
    );
  }
  const match = MONEY_RE.exec(value);
  if (!match) {
    throw new MoneyError(
      `"${value}" is not a valid decimal amount ` +
        `(max ${MONEY_TOTAL_PRECISION - MONEY_SCALE} integer digits, ` +
        `${MONEY_SCALE} fractional digits, no exponent)`,
      field,
    );
  }
  const [, sign, intPart, fracPart = ""] = match;
  const padded = fracPart.padEnd(MONEY_SCALE, "0");
  return {
    negative: sign === "-",
    minor: BigInt(intPart + padded),
  };
}

function unparse({ negative, minor }: ParsedMoney): string {
  const digits = minor.toString().padStart(MONEY_SCALE + 1, "0");
  const intPart = digits.slice(0, digits.length - MONEY_SCALE);
  const fracPart = digits.slice(digits.length - MONEY_SCALE);
  return `${negative && minor !== 0n ? "-" : ""}${intPart}.${fracPart}`;
}

/** Type guard for wire-format money strings. */
export function isMoneyString(value: unknown): value is string {
  return typeof value === "string" && MONEY_RE.test(value);
}

/** Validates and returns the value, throwing {@link MoneyError} if invalid. */
export function assertMoney(value: unknown, field?: string): string {
  if (!isMoneyString(value)) {
    throw new MoneyError(
      `"${String(value)}" is not a valid decimal amount`,
      field,
    );
  }
  return value;
}

/** Exact sum, always `${int}.${8 decimals}`. */
export function addMoney(a: string, b: string, field?: string): string {
  const left = parse(a, field);
  const right = parse(b, field);
  const total =
    (left.negative ? -left.minor : left.minor) +
    (right.negative ? -right.minor : right.minor);
  const negative = total < 0n;
  return unparse({ negative, minor: negative ? -total : total });
}

/** Exact subtraction, always `${int}.${8 decimals}`. */
export function subtractMoney(a: string, b: string, field?: string): string {
  const left = parse(a, field);
  const right = parse(b, field);
  const total =
    (left.negative ? -left.minor : left.minor) -
    (right.negative ? -right.minor : right.minor);
  const negative = total < 0n;
  return unparse({ negative, minor: negative ? -total : total });
}

/** -1 | 0 | 1 — exact comparison, no floats. */
export function compareMoney(a: string, b: string, field?: string): -1 | 0 | 1 {
  const left = parse(a, field);
  const right = parse(b, field);
  const l = left.negative ? -left.minor : left.minor;
  const r = right.negative ? -right.minor : right.minor;
  if (l < r) return -1;
  if (l > r) return 1;
  return 0;
}

export function isZeroMoney(value: string, field?: string): boolean {
  return parse(value, field).minor === 0n;
}

/** Exact sum of many amounts; empty list is `0.00000000`. */
export function sumMoney(values: string[], field?: string): string {
  const total = values.reduce<string>(
    (acc, v) => addMoney(acc, v, field),
    `0.${"0".repeat(MONEY_SCALE)}`,
  );
  return toWireMoney(total, field);
}

/** Canonical wire form: exactly 8 fractional digits, no `-0`. */
export function toWireMoney(value: string, field?: string): string {
  return unparse(parse(value, field));
}

/** Requires a strictly positive amount (deposits, withdrawals, stakes). */
export function assertPositiveMoney(value: unknown, field?: string): string {
  const money = assertMoney(value, field);
  if (compareMoney(money, "0") !== 1) {
    throw new MoneyError("amount must be greater than zero", field);
  }
  return money;
}

const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: "$",
  EUR: "€",
  GBP: "£",
  RUB: "₽",
  TRY: "₺",
  BRL: "R$",
  KZT: "₸",
  UAH: "₴",
};

export interface FormatMoneyOptions {
  currency?: string;
  /** Show the currency symbol/code after the amount. Default: currency code. */
  display?: "symbol" | "code" | "none";
  /** Trim trailing zeros down to this many decimals. Default: 2. */
  fractionDigits?: number;
}

/**
 * Human-readable amount without ever converting to a float:
 * grouping is done on the integer part, and rounding is half-up on the
 * exact decimal string (not via `Number`/`toFixed`).
 */
export function formatMoney(
  value: string,
  options: FormatMoneyOptions = {},
): string {
  const { currency, display = "code", fractionDigits = 2 } = options;
  const { negative, minor } = parse(value);
  const factor = 10n ** BigInt(MONEY_SCALE - fractionDigits);

  // Half-up rounding on the exact value.
  const scaled = (minor + factor / 2n) / factor;
  const roundedNegative = negative && scaled !== 0n;

  const digits = scaled.toString().padStart(fractionDigits + 1, "0");
  const intPart = digits.slice(0, digits.length - fractionDigits) || "0";
  const trimmedFrac =
    fractionDigits > 0
      ? digits.slice(digits.length - fractionDigits).replace(/0+$/, "")
      : "";
  const fracPart = trimmedFrac ? `.${trimmedFrac}` : "";

  const grouped = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const sign = roundedNegative ? "-" : "";
  const suffix = currency
    ? display === "symbol"
      ? ` ${CURRENCY_SYMBOLS[currency] ?? currency}`
      : display === "code"
        ? ` ${currency}`
        : ""
    : "";
  return `${sign}${grouped}${fracPart}${suffix}`;
}
