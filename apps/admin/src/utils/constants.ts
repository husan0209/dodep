import type { StatusConfig } from "@/components/common/StatusTag";

export const API_BASE_URL = import.meta.env.VITE_API_URL || "/api/v1";

export const PAGE_SIZE_OPTIONS = ["10", "20", "50", "100"];

export const USER_STATUSES: Record<string, StatusConfig> = {
  active: { label: "Active", color: "green" },
  blocked: { label: "Blocked", color: "red" },
  pending: { label: "Pending", color: "orange" },
  self_excluded: { label: "Self-Excluded", color: "volcano" },
  suspended: { label: "Suspended", color: "red" },
};

export const KYC_LEVELS: Record<string, StatusConfig> = {
  0: { label: "Unverified", color: "default" },
  1: { label: "Level 1 - Email", color: "blue" },
  2: { label: "Level 2 - ID", color: "cyan" },
  3: { label: "Level 3 - Address", color: "green" },
  4: { label: "Level 4 - EDD", color: "purple" },
};

export const BET_STATUSES: Record<string, StatusConfig> = {
  pending: { label: "Pending", color: "orange" },
  active: { label: "Active", color: "processing" },
  won: { label: "Won", color: "green" },
  lost: { label: "Lost", color: "red" },
  void: { label: "Void", color: "default" },
  cashout: { label: "Cashout", color: "purple" },
};

export const TRANSACTION_STATUSES: Record<string, StatusConfig> = {
  pending: { label: "Pending", color: "orange" },
  processing: { label: "Processing", color: "processing" },
  completed: { label: "Completed", color: "green" },
  failed: { label: "Failed", color: "red" },
  cancelled: { label: "Cancelled", color: "default" },
  requires_review: { label: "Requires Review", color: "volcano" },
};

export const WITHDRAWAL_STATUSES: Record<string, StatusConfig> = {
  pending: { label: "Pending", color: "orange" },
  approved: { label: "Approved", color: "processing" },
  processing: { label: "Processing", color: "processing" },
  completed: { label: "Completed", color: "green" },
  rejected: { label: "Rejected", color: "red" },
  cancelled: { label: "Cancelled", color: "default" },
};

export const ALERT_SEVERITIES: Record<string, StatusConfig> = {
  low: { label: "Low", color: "blue" },
  medium: { label: "Medium", color: "orange" },
  high: { label: "High", color: "volcano" },
  critical: { label: "Critical", color: "red" },
};

export const ALERT_STATUSES: Record<string, StatusConfig> = {
  open: { label: "Open", color: "red" },
  in_review: { label: "In Review", color: "processing" },
  resolved: { label: "Resolved", color: "green" },
  false_positive: { label: "False Positive", color: "default" },
};

export const BONUS_TYPES: Record<string, StatusConfig> = {
  welcome: { label: "Welcome Bonus", color: "gold" },
  reload: { label: "Reload", color: "blue" },
  free_spins: { label: "Free Spins", color: "purple" },
  free_bet: { label: "Free Bet", color: "cyan" },
  cashback: { label: "Cashback", color: "green" },
  no_deposit: { label: "No Deposit", color: "magenta" },
  loyalty: { label: "Loyalty", color: "orange" },
};
