import dayjs from "dayjs";

export function formatMoney(amount: string | number, currency = "USD"): string {
  const num = typeof amount === "string" ? parseFloat(amount) : amount;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(num);
}

export function formatDate(
  date: string,
  format = "YYYY-MM-DD HH:mm:ss",
): string {
  return dayjs(date).format(format);
}

export function formatRelativeTime(date: string): string {
  const now = dayjs();
  const d = dayjs(date);
  const diffMinutes = now.diff(d, "minute");
  if (diffMinutes < 1) return "just now";
  if (diffMinutes < 60) return `${diffMinutes}m ago`;
  const diffHours = now.diff(d, "hour");
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = now.diff(d, "day");
  if (diffDays < 30) return `${diffDays}d ago`;
  return d.format("YYYY-MM-DD");
}

export function formatPercent(value: number, decimals = 1): string {
  return `${(value * 100).toFixed(decimals)}%`;
}

export function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength - 3) + "...";
}

export function maskEmail(email: string): string {
  const [local, domain] = email.split("@");
  if (local.length <= 2) return email;
  return `${local[0]}${"*".repeat(local.length - 2)}${local[local.length - 1]}@${domain}`;
}

export function maskPhone(phone: string): string {
  if (phone.length <= 4) return phone;
  return "*".repeat(phone.length - 4) + phone.slice(-4);
}
