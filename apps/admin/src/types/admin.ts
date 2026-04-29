export type AdminRole =
  | "SUPER_ADMIN"
  | "FINANCE_MANAGER"
  | "RISK_MANAGER"
  | "CRM_MANAGER"
  | "SPORTS_TRADER"
  | "SUPPORT_AGENT"
  | "KYC_OFFICER"
  | "AFFILIATE_MANAGER"
  | "CONTENT_MANAGER"
  | "VIEWER"
  | "COMPLIANCE_OFFICER";

export type Permission =
  // Player
  | "user.view"
  | "user.edit"
  | "user.block"
  | "user.delete"
  | "user.merge"
  // Finance
  | "transaction.view"
  | "transaction.adjust"
  | "withdrawal.approve_small"
  | "withdrawal.approve_large"
  | "finance.balance_sheet"
  | "finance.crypto_wallet"
  // Risk
  | "fraud.review"
  | "fraud.rule_builder"
  | "fraud.screening"
  // Bonus
  | "bonus.create"
  | "bonus.edit"
  | "bonus.grant"
  // Sportsbook
  | "bet.view"
  | "bet.void"
  | "sportsbook.manage"
  | "sportsbook.trading_terminal"
  // Casino
  | "casino.manage"
  | "casino.rtp_config"
  // Affiliate
  | "affiliate.manage"
  // CRM
  | "content.manage"
  | "crm.campaign"
  | "crm.segment"
  | "communication.manage"
  // KYC
  | "kyc.review"
  | "kyc.sof_review"
  // Reports
  | "reports.view"
  | "reports.export"
  // System
  | "system.config"
  | "system.maintenance"
  | "admin.manage"
  | "audit.view";

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: AdminRole;
  permissions: Permission[];
  last_login: string | null;
  last_login_at?: string | null;
  created_at: string;
  locked?: boolean;
  totp_enabled?: boolean;
}

export interface DashboardStats {
  total_users: number;
  active_users_today: number;
  new_users_today: number;
  total_deposits_today: string;
  total_withdrawals_today: string;
  bets_placed_today: number;
  ggr_today: string;
  open_fraud_alerts: number;
  pending_withdrawals: number;
  pending_kyc_reviews: number;

  // Live metrics (from WS)
  online_casino: number;
  online_sports: number;
  ftd_count_today: number;
  ftd_amount_today: string;
  open_support_tickets: number;
}

export interface ProviderHealth {
  name: string;
  status: "online" | "degraded" | "down";
  latency_p99_ms: number;
  error_rate_pct: number;
  ggr_today: string;
}

export interface GatewayHealth {
  name: string;
  success_rate_pct: number;
  avg_latency_ms: number;
}

export interface TopItem {
  id: string;
  name: string;
  value: number; // GGR for games/events, count for countries
  currency?: string;
}

export interface ChartPoint {
  date: string;
  ggr: number;
  ngr: number;
  deposits: number;
  withdrawals: number;
}

export interface ConversionFunnel {
  visits: number;
  registrations: number;
  ftd: number;
  second_deposit: number;
}

export interface LiveMetrics {
  online: { casino: number; sports: number; total: number };
  ggr_today: { casino: number; sports: number; live_casino: number };
  deposits_today: number;
  withdrawals_today: number;
  ftd_today: { count: number; amount: number };
  pending_withdrawals: { count: number; amount: number };
  pending_kyc: number;
  open_tickets: number;
}
