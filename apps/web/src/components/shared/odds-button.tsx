"use client";

import * as React from "react";
import { cn } from "@/lib/cn";
import { formatOdds } from "@/lib/format";

export interface OddsButtonProps {
  eventId: number;
  marketId: number;
  outcomeId: number;
  outcomeName: string;
  odds: number;
  previousOdds?: number;
  suspended?: boolean;
  isSelected?: boolean;
  oddsFormat?: "decimal" | "fractional" | "american";
  onClick?: () => void;
  className?: string;
}

export function OddsButton({
  eventId,
  marketId,
  outcomeId,
  outcomeName,
  odds,
  previousOdds,
  suspended = false,
  isSelected = false,
  oddsFormat = "decimal",
  onClick,
  className,
}: OddsButtonProps) {
  const [flash, setFlash] = React.useState<"up" | "down" | null>(null);
  const prevOddsRef = React.useRef(odds);

  React.useEffect(() => {
    if (prevOddsRef.current !== odds) {
      setFlash(odds > prevOddsRef.current ? "up" : "down");
      prevOddsRef.current = odds;
      const timer = setTimeout(() => setFlash(null), 2000);
      return () => clearTimeout(timer);
    }
  }, [odds]);

  return (
    <button
      onClick={onClick}
      disabled={suspended}
      aria-pressed={isSelected}
      aria-label={`${outcomeName} at ${formatOdds(odds, oddsFormat)}`}
      className={cn(
        "flex flex-col items-center justify-center px-3 py-2 rounded-lg border min-w-[80px]",
        "transition-all duration-200",
        isSelected && "bg-primary text-white border-primary",
        !isSelected && "bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700",
        suspended && "opacity-50 cursor-not-allowed",
        flash === "up" && "animate-flash-green bg-green-100 dark:bg-green-900/20",
        flash === "down" && "animate-flash-red bg-red-100 dark:bg-red-900/20",
        className
      )}
    >
      <span className="text-xs text-muted-foreground truncate max-w-full">
        {outcomeName}
      </span>
      <span className={cn(
        "text-sm font-bold tabular-nums",
        flash === "up" && "text-green-600 dark:text-green-400",
        flash === "down" && "text-red-600 dark:text-red-400"
      )}>
        {formatOdds(odds, oddsFormat)}
      </span>
    </button>
  );
}
