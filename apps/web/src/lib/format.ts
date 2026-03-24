/**
 * Format money amount with currency symbol
 */
export function formatMoney(amount: number | string, currency: string): string {
  const num = typeof amount === "string" ? parseFloat(amount) : amount;
  
  const symbols: Record<string, string> = {
    RUB: "₽",
    USD: "$",
    EUR: "€",
    GBP: "£",
    KZT: "₸",
  };

  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: currency.toUpperCase(),
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(num).replace(symbols[currency.toUpperCase()] || currency, ` ${symbols[currency.toUpperCase()] || currency}`);
}

/**
 * Format odds in different formats
 */
export function formatOdds(odds: number, format: "decimal" | "fractional" | "american" = "decimal"): string {
  switch (format) {
    case "decimal":
      return odds.toFixed(2);

    case "fractional": {
      if (odds <= 1) return "0/1";
      const profit = odds - 1;
      const denominator = 100;
      const numerator = Math.round(profit * denominator);
      const gcd = (a: number, b: number): number => (b ? gcd(b, a % b) : a);
      const d = gcd(numerator, denominator);
      return `${numerator / d}/${denominator / d}`;
    }

    case "american":
      if (odds >= 2.0) {
        return `+${Math.round((odds - 1) * 100)}`;
      }
      return `-${Math.round(100 / (odds - 1))}`;

    default:
      return odds.toFixed(2);
  }
}

/**
 * Format date
 */
export function formatDate(date: string | Date, options?: Intl.DateTimeFormatOptions): string {
  const d = typeof date === "string" ? new Date(date) : date;
  
  const defaultOptions: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  };

  return new Intl.DateTimeFormat("ru-RU", options || defaultOptions).format(d);
}

/**
 * Format relative time (e.g., "5 min ago")
 */
export function formatRelativeTime(date: string | Date): string {
  const d = typeof date === "string" ? new Date(date) : date;
  const now = new Date();
  const diff = now.getTime() - d.getTime();

  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (minutes < 1) return "Только что";
  if (minutes < 60) return `${minutes} мин назад`;
  if (hours < 24) return `${hours} ч назад`;
  if (days < 7) return `${days} дн назад`;
  
  return formatDate(d, { day: "numeric", month: "short", year: "numeric" });
}

/**
 * Parse ISO 8601 date string
 */
export function parseDateTime(isoString?: string | null): Date | null {
  if (!isoString) return null;
  const d = new Date(isoString);
  return isNaN(d.getTime()) ? null : d;
}
