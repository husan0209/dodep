import { describe, expect, it } from "vitest";

import {
  formatMoney,
  formatPercent,
  maskEmail,
  maskPhone,
  truncateText,
} from "./format";

describe("formatMoney", () => {
  it("renders numbers with two fraction digits", () => {
    expect(formatMoney(1234.5)).toBe("$1,234.50");
  });

  it("parses string amounts before formatting", () => {
    expect(formatMoney("99.9")).toBe("$99.90");
  });

  it("honours the requested currency", () => {
    expect(formatMoney(10, "EUR")).toBe("€10.00");
  });
});

describe("formatPercent", () => {
  it("scales a ratio to a percentage", () => {
    expect(formatPercent(0.1234)).toBe("12.3%");
  });

  it("respects the requested precision", () => {
    expect(formatPercent(0.1234, 2)).toBe("12.34%");
  });
});

describe("truncateText", () => {
  it("leaves short text untouched", () => {
    expect(truncateText("abc", 5)).toBe("abc");
  });

  it("cuts long text and appends an ellipsis", () => {
    expect(truncateText("abcdefghij", 5)).toBe("ab...");
  });
});

describe("maskEmail", () => {
  it("keeps very short local parts readable", () => {
    expect(maskEmail("ab@example.com")).toBe("ab@example.com");
  });

  it("masks the middle of the local part", () => {
    expect(maskEmail("player@example.com")).toBe("p****r@example.com");
  });
});

describe("maskPhone", () => {
  it("keeps very short numbers untouched", () => {
    expect(maskPhone("1234")).toBe("1234");
  });

  it("masks everything but the last four digits", () => {
    expect(maskPhone("1234567890")).toBe("******7890");
  });
});