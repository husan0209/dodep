export type AlertSeverity = "low" | "medium" | "high" | "critical";
export type AlertStatus = "open" | "in_review" | "resolved" | "false_positive";
export type AlertCategory =
  | "velocity"
  | "amount_anomaly"
  | "pattern"
  | "multi_account"
  | "bonus_abuse"
  | "payment_fraud"
  | "collusion";

export interface FraudAlert {
  id: string;
  user_id: string;
  category: AlertCategory;
  severity: AlertSeverity;
  status: AlertStatus;
  risk_score: number;
  title: string;
  description: string;
  evidence: Record<string, unknown>;
  assigned_to: string | null;
  resolution: string | null;
  created_at: string;
  updated_at: string;
  resolved_at: string | null;
}

export interface AuditLogEntry {
  id: string;
  admin_id: string;
  admin_email: string;
  action: string;
  resource_type: string;
  resource_id: string;
  old_value: Record<string, unknown> | null;
  new_value: Record<string, unknown> | null;
  ip_address: string;
  created_at: string;
}

export interface AlertSearchParams {
  category?: AlertCategory;
  severity?: AlertSeverity;
  status?: AlertStatus;
  user_id?: string;
  assigned_to?: string;
  created_from?: string;
  created_to?: string;
  page?: number;
  page_size?: number;
}
