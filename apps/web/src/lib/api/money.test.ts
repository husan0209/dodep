import {
  MONEY_SCALE,
  MoneyError,
  addMoney,
  assertMoney,
  assertPositiveMoney,
  compareMoney,
  formatMoney,
  isMoneyString,
  isZeroMoney,
  subtractMoney,
  sumMoney,
  toWireMoney,
} from "./money";

describe("money validation", () => {
  it("accepts NUMERIC(18,8)-compatible decimal strings", () => {
    expect(isMoneyString("0")).toBe(true);
    expect(isMoneyString("100")).toBe(true);
    expect(isMoneyString("0.00000001")).toBe(true);
    expect(isMoneyString("9999999999.99999999")).toBe(true);
    expect(isMoneyString("-15.5")).toBe(true);
  });

  it("rejects floats, exponents and out-of-range values", () => {
    expect(isMoneyString(100)).toBe(false);
    expect(isMoneyString(0.1)).toBe(false);
    expect(isMoneyString("1e3")).toBe(false);
    expect(isMoneyString("1,000")).toBe(false);
    expect(isMoneyString(" 100")).toBe(false);
    expect(isMoneyString("")).toBe(false);
    expect(isMoneyString(null)).toBe(false);
    expect(isMoneyString("0.000000001")).toBe(false); // 9 fractional digits
    expect(isMoneyString("12345678901")).toBe(false); // 11 integer digits
  });

  it("throws MoneyError with the offending field name", () => {
    expect(() => assertMoney(10.5, "bet.stake")).toThrow(MoneyError);
    expect(() => assertMoney(10.5, "bet.stake")).toThrow(/bet\.stake/);
  });

  it("requires a strictly positive amount for money movement", () => {
    expect(assertPositiveMoney("0.01")).toBe("0.01");
    expect(() => assertPositiveMoney("0")).toThrow(MoneyError);
    expect(() => assertPositiveMoney("-5")).toThrow(MoneyError);
  });
});

describe("exact decimal arithmetic", () => {
  it("adds without float drift", () => {
    expect(addMoney("0.1", "0.2")).toBe("0.30000000");
    expect(addMoney("1.005", "2.005")).toBe("3.01000000");
    expect(addMoney("9999999999.99999999", "0.00000001")).toBe(
      "10000000000.00000000",
    );
  });

  it("subtracts and handles sign changes", () => {
    expect(subtractMoney("0.3", "0.1")).toBe("0.20000000");
    expect(subtractMoney("1", "3")).toBe("-2.00000000");
    expect(subtractMoney("-2.5", "-1")).toBe("-1.50000000");
  });

  it("never produces a negative zero", () => {
    expect(addMoney("1.5", "-1.5")).toBe("0.00000000");
    expect(subtractMoney("1", "1")).toBe("0.00000000");
  });

  it("compares exactly", () => {
    expect(compareMoney("1.10", "1.1")).toBe(0);
    expect(compareMoney("0.30000000", "0.3")).toBe(0);
    expect(compareMoney("1.00000001", "1")).toBe(1);
    expect(compareMoney("0.99999999", "1")).toBe(-1);
    expect(compareMoney("-5", "-4")).toBe(-1);
  });

  it("sums lists exactly and treats an empty list as zero", () => {
    expect(sumMoney(["0.1", "0.2", "0.3"])).toBe("0.60000000");
    expect(sumMoney([])).toBe("0.00000000");
    expect(sumMoney(["999.99", "0.01"])).toBe("1000.00000000");
  });

  it("reports zero amounts", () => {
    expect(isZeroMoney("0")).toBe(true);
    expect(isZeroMoney("0.00000000")).toBe(true);
    expect(isZeroMoney("0.00000001")).toBe(false);
  });

  it("normalizes to the wire format with 8 decimals", () => {
    expect(toWireMoney("5")).toBe("5.00000000");
    expect(toWireMoney("-0")).toBe("0.00000000");
    expect(MONEY_SCALE).toBe(8);
  });
});

describe("formatting", () => {
  it("groups thousands without float conversion", () => {
    expect(formatMoney("1234567.891", { display: "none" })).toBe("1,234,567.89");
    expect(formatMoney("9999999999.99999999", { display: "none" })).toBe(
      "10,000,000,000",
    );
  });

  it("rounds half-up on the exact value", () => {
    expect(formatMoney("0.125", { display: "none" })).toBe("0.13");
    expect(formatMoney("0.124", { display: "none" })).toBe("0.12");
  });

  it("supports currency display modes", () => {
    expect(formatMoney("1500", { currency: "USD" })).toBe("1,500 USD");
    expect(formatMoney("1500", { currency: "USD", display: "symbol" })).toBe(
      "1,500 $",
    );
    expect(formatMoney("1500.25", { currency: "RUB", display: "symbol" })).toBe(
      "1,500.25 ₽",
    );
  });

  it("keeps negative signs and trims trailing zeros", () => {
    expect(formatMoney("-42.5", { display: "none" })).toBe("-42.5");
    expect(formatMoney("1000", { display: "none" })).toBe("1,000");
  });
});