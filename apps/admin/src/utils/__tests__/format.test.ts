import { describe, it, expect, afterEach, vi } from "vitest";
import {
  formatMoney,
  formatDate,
  formatRelativeTime,
  formatPercent,
  truncateText,
  maskEmail,
  maskPhone,
} from "@/utils/format";

describe("formatMoney", () => {
  it("renders a numeric amount as a 2-decimal USD string", () => {
    expect(formatMoney(1234.5)).toBe("$1,234.50");
    expect(formatMoney(0)).toBe("$0.00");
    expect(formatMoney(-42)).toBe("-$42.00");
  });

  it("parses decimal-string amounts (API money fields arrive as strings)", () => {
    expect(formatMoney("1234.5")).toBe("$1,234.50");
    expect(formatMoney("0")).toBe("$0.00");
  });

  it("always shows two fraction digits, never more", () => {
    expect(formatMoney("10.005")).toBe("$10.01");
    expect(formatMoney(10.129)).toBe("$10.13");
    expect(formatMoney(10)).toBe("$10.00");
  });

  it("honours the currency argument instead of hard-coding USD", () => {
    expect(formatMoney(100, "EUR")).toBe("€100.00");
    expect(formatMoney(100, "USD")).toBe("$100.00");
  });
});

describe("formatPercent", () => {
  it("converts a 0..1 fraction into a percentage string", () => {
    expect(formatPercent(0.1234)).toBe("12.3%");
    expect(formatPercent(1)).toBe("100.0%");
    expect(formatPercent(0)).toBe("0.0%");
  });

  it("respects the requested number of decimals", () => {
    expect(formatPercent(0.4567, 0)).toBe("46%");
    expect(formatPercent(0.4567, 2)).toBe("45.67%");
    expect(formatPercent(0.4567, 3)).toBe("45.670%");
  });
});

describe("formatDate", () => {
  it("formats with the default dayjs-style pattern", () => {
    expect(formatDate("2026-03-04T05:06:07")).toBe("2026-03-04 05:06:07");
  });

  it("honours a custom pattern", () => {
    expect(formatDate("2026-03-04T05:06:07", "DD/MM/YYYY")).toBe("04/03/2026");
    expect(formatDate("2026-03-04T05:06:07", "YYYY-MM")).toBe("2026-03");
  });
});

describe("formatRelativeTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function freezeNow() {
    // Local-time constructor so dayjs() and the fixture agree on the zone.
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 12, 0, 0));
  }

  it("collapses sub-minute gaps to 'just now'", () => {
    freezeNow();
    expect(formatRelativeTime("2026-01-15T11:59:31")).toBe("just now");
  });

  it("uses minutes below one hour, hours below one day, days below a month", () => {
    freezeNow();
    expect(formatRelativeTime("2026-01-15T11:30:00")).toBe("30m ago");
    expect(formatRelativeTime("2026-01-15T09:00:00")).toBe("3h ago");
    expect(formatRelativeTime("2026-01-13T12:00:00")).toBe("2d ago");
    expect(formatRelativeTime("2026-01-01T12:00:00")).toBe("14d ago");
  });

  it("falls back to an absolute date once the gap exceeds 30 days", () => {
    freezeNow();
    expect(formatRelativeTime("2025-11-20T12:00:00")).toBe("2025-11-20");
  });
});

describe("truncateText", () => {
  it("leaves short text untouched", () => {
    expect(truncateText("short", 10)).toBe("short");
    expect(truncateText("exactlyten", 10)).toBe("exactlyten");
  });

  it("elides with '...' and never exceeds maxLength", () => {
    const result = truncateText("abcdefghijklmnop", 10);
    expect(result).toBe("abcdefg...");
    expect(result).toHaveLength(10);
  });
});

describe("maskEmail", () => {
  it("keeps the first and last local character", () => {
    expect(maskEmail("player@example.com")).toBe("p****r@example.com");
    expect(maskEmail("abc@x.io")).toBe("a*c@x.io");
  });

  it("returns very short local parts unchanged", () => {
    expect(maskEmail("ab@example.com")).toBe("ab@example.com");
    expect(maskEmail("a@example.com")).toBe("a@example.com");
  });
});

describe("maskPhone", () => {
  it("reveals only the last four digits", () => {
    expect(maskPhone("1234567890")).toBe("******7890");
    expect(maskPhone("+15551234567")).toBe("********4567");
  });

  it("returns short numbers unchanged", () => {
    expect(maskPhone("1234")).toBe("1234");
  });
});
