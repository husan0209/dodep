import { afterEach, describe, expect, it, vi } from "vitest";

import {
  formatDate,
  formatMoney,
  formatPercent,
  formatRelativeTime,
  maskEmail,
  maskPhone,
  truncateText,
} from "./format";

describe("formatMoney", () => {
  it("renders a numeric amount with two fraction digits", () => {
    expect(formatMoney(1234.5)).toBe("$1,234.50");
    expect(formatMoney(0)).toBe("$0.00");
  });

  it("renders a decimal string amount", () => {
    expect(formatMoney("1234.5")).toBe("$1,234.50");
    expect(formatMoney("0")).toBe("$0.00");
  });

  it("honours the currency", () => {
    expect(formatMoney(10, "EUR")).toContain("10.00");
    expect(formatMoney(10, "EUR")).toContain("€");
  });

  it("rounds to two fraction digits", () => {
    expect(formatMoney("10.005")).toBe("$10.01");
    expect(formatMoney("10.004")).toBe("$10.00");
  });

  it("groups thousands", () => {
    expect(formatMoney("1234567.89")).toBe("$1,234,567.89");
  });
});

describe("formatPercent", () => {
  it("multiplies by 100 and keeps one decimal by default", () => {
    expect(formatPercent(0.1234)).toBe("12.3%");
    expect(formatPercent(1)).toBe("100.0%");
    expect(formatPercent(0)).toBe("0.0%");
  });

  it("honours the requested precision", () => {
    expect(formatPercent(0.1234, 0)).toBe("12%");
    expect(formatPercent(0.1234, 3)).toBe("12.340%");
  });
});

describe("truncateText", () => {
  it("leaves short text untouched", () => {
    expect(truncateText("short", 10)).toBe("short");
    expect(truncateText("exactly-10", 10)).toBe("exactly-10");
  });

  it("cuts long text and appends an ellipsis", () => {
    expect(truncateText("abcdefghij", 5)).toBe("ab...");
    expect(truncateText("abcdefghij", 5)).toHaveLength(5);
  });
});

describe("maskEmail", () => {
  it("keeps the first and last character of the local part", () => {
    expect(maskEmail("admin@example.com")).toBe("a***n@example.com");
  });

  it("returns the address unchanged when the local part is too short", () => {
    expect(maskEmail("ab@example.com")).toBe("ab@example.com");
  });
});

describe("maskPhone", () => {
  it("keeps only the last four digits", () => {
    // 13 characters in, 9 masked + the last 4 kept.
    expect(maskPhone("+380501234567")).toBe("*********4567");
  });

  it("returns short numbers unchanged", () => {
    expect(maskPhone("123")).toBe("123");
  });
});

describe("formatDate", () => {
  // dayjs formats in the runner's local zone, so the inputs below carry no
  // trailing Z; otherwise the expected string would depend on the CI machine's
  // offset.
  it("uses the default pattern", () => {
    expect(formatDate("2026-02-03T04:05:06")).toBe("2026-02-03 04:05:06");
  });

  it("honours a custom pattern", () => {
    expect(formatDate("2026-02-03T04:05:06", "YYYY-MM-DD")).toBe("2026-02-03");
  });
});

describe("formatRelativeTime", () => {
  // Noon UTC keeps the calendar date identical in every plausible CI zone.
  const base = new Date("2026-02-03T12:00:00Z").getTime();

  const minutesAgo = (n: number) => new Date(base - n * 60_000).toISOString();
  const hoursAgo = (n: number) => new Date(base - n * 3_600_000).toISOString();
  const daysAgo = (n: number) => new Date(base - n * 86_400_000).toISOString();

  // formatRelativeTime reads the wall clock through dayjs(), which uses
  // new Date(); freezing the clock keeps the assertions independent of when the
  // suite happens to run.
  const freeze = () => {
    vi.useFakeTimers();
    vi.setSystemTime(base);
  };

  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports sub-minute differences as 'just now'", () => {
    freeze();
    expect(formatRelativeTime(minutesAgo(0))).toBe("just now");
  });

  it("uses minutes, then hours, then days", () => {
    freeze();
    expect(formatRelativeTime(minutesAgo(5))).toBe("5m ago");
    expect(formatRelativeTime(hoursAgo(3))).toBe("3h ago");
    expect(formatRelativeTime(daysAgo(4))).toBe("4d ago");
  });

  it("falls back to an absolute date beyond 30 days", () => {
    freeze();
    // 2026-02-03 minus 45 days.
    expect(formatRelativeTime(daysAgo(45))).toBe("2025-12-20");
  });
});