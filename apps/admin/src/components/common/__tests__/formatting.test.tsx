import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { USER_STATUSES, TRANSACTION_STATUSES, KYC_LEVELS } from "@/utils/constants";

describe("StatusTag", () => {
  it("renders the mapped label instead of the raw status key", () => {
    render(<StatusTag status="active" config={USER_STATUSES} />);

    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("active")).not.toBeInTheDocument();
  });

  it("renders the label defined for a numeric status key (KYC levels)", () => {
    render(<StatusTag status="2" config={KYC_LEVELS} />);

    expect(screen.getByText("Level 2 - ID")).toBeInTheDocument();
    expect(screen.queryByText("2")).not.toBeInTheDocument();
  });

  it("falls back to the raw status text for an unknown status", () => {
    render(<StatusTag status="weird_state" config={TRANSACTION_STATUSES} />);

    expect(screen.getByText("weird_state")).toBeInTheDocument();
  });

  it("falls back to the raw status text when no config is supplied", () => {
    render(<StatusTag status="active" />);

    expect(screen.getByText("active")).toBeInTheDocument();
  });

  it("every status in the shared constants has a label and a colour", () => {
    for (const config of [USER_STATUSES, TRANSACTION_STATUSES]) {
      for (const [key, value] of Object.entries(config)) {
        expect(value.label.trim().length).toBeGreaterThan(0);
        expect(value.color.trim().length).toBeGreaterThan(0);
        expect(key.length).toBeGreaterThan(0);
      }
    }
  });
});

describe("MoneyDisplay", () => {
  it("formats a numeric amount", () => {
    render(<MoneyDisplay amount={1234.5} />);

    expect(screen.getByText("$1,234.50")).toBeInTheDocument();
  });

  it("formats a string amount and honours the currency prop", () => {
    render(<MoneyDisplay amount="99.9" currency="EUR" />);

    expect(screen.getByText("€99.90")).toBeInTheDocument();
  });

  it("colours positive green and negative red, leaving neutral uncoloured", () => {
    const { rerender } = render(<MoneyDisplay amount={10} type="positive" />);
    expect(screen.getByText("$10.00")).toHaveStyle({ color: "#52c41a" });

    rerender(<MoneyDisplay amount={-10} type="negative" />);
    expect(screen.getByText("-$10.00")).toHaveStyle({ color: "#ff4d4f" });

    rerender(<MoneyDisplay amount={10} type="neutral" />);
    expect(screen.getByText("$10.00")).not.toHaveStyle({ color: "#52c41a" });
  });

  it("uses a monospace font so decimal columns line up in tables", () => {
    render(<MoneyDisplay amount={1} />);

    expect(screen.getByText("$1.00")).toHaveStyle({
      fontFamily: "monospace",
    });
  });

  it("defaults to non-bold and bolds only when asked", () => {
    const { rerender } = render(<MoneyDisplay amount={5} />);
    expect(screen.getByText("$5.00")).not.toHaveStyle({
      fontWeight: "600",
    });

    rerender(<MoneyDisplay amount={5} bold />);
    expect(screen.getByText("$5.00")).toHaveStyle({ fontWeight: "600" });
  });
});
