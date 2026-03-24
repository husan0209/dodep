export type AdminRole =
  | "support_l1"
  | "support_l2"
  | "risk_manager"
  | "finance"
  | "marketing"
  | "admin"
  | "super_admin";

export type Permission =
  | "user.view"
  | "user.edit"
  | "user.block"
  | "user.delete"
  | "bet.view"
  | "bet.void"
  | "transaction.view"
  | "transaction.adjust"
  | "withdrawal.approve_small"
  | "withdrawal.approve_large"
  | "fraud.review"
  | "bonus.create"
  | "bonus.edit"
  | "bonus.grant"
  | "content.manage"
  | "affiliate.manage"
  | "reports.view"
  | "system.config";

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: AdminRole;
  permissions: Permission[];
  last_login: string | null;
  created_at: string;
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
}
