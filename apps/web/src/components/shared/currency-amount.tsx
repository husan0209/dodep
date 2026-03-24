import * as React from "react";
import { cn } from "@/lib/cn";

export interface CurrencyAmountProps {
  amount: number | string;
  currency: string;
  showSign?: boolean;
  colorize?: boolean;
  compact?: boolean;
  className?: string;
}

export function CurrencyAmount({
  amount,
  currency,
  showSign,
  colorize,
  compact,
  className,
}: CurrencyAmountProps) {
  const num = typeof amount === "string" ? parseFloat(amount) : amount;
  const absAmount = Math.abs(num);

  const formatted = new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: currency.toUpperCase(),
    notation: compact ? "compact" : "standard",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(absAmount);

  const sign = num > 0 && showSign ? "+" : num < 0 ? "-" : "";

  return (
    <span
      className={cn(
        "tabular-nums font-medium",
        colorize && num > 0 && "text-green-500",
        colorize && num < 0 && "text-red-500",
        className
      )}
    >
      {sign}{formatted}
    </span>
  );
}
