export type KycDocumentType = "identity" | "address" | "source_of_funds" | "selfie";
export type KycDocumentStatus = "pending" | "verified" | "rejected" | "expired";
export type KycPriority = "low" | "medium" | "high";
export type KycReviewStatus = "pending" | "in_review" | "approved" | "rejected" | "resubmission_requested";
export type ScreeningStatus = "clear" | "pep_match" | "sanctions_hit" | "review_required";
export type SofStatus = "open" | "submitted" | "under_review" | "approved" | "rejected" | "expired";
export type RgAlertType = "chasing_losses" | "late_night_session" | "rapid_deposit_increase" | "long_session" | "limit_breach";

export interface KycDocument {
  id: string;
  player_id: string;
  type: KycDocumentType;
  status: KycDocumentStatus;
  file_url: string;
  uploaded_at: string;
  reviewed_by: string | null;
  reviewed_by_name: string | null;
  reviewed_at: string | null;
  rejection_reason: string | null;
  notes: string | null;
  expires_at: string | null;
  ocr_data: Record<string, unknown> | null;
}

export interface KycReviewItem {
  id: string;
  document_id: string;
  player_id: string;
  player_email: string;
  player_username: string;
  player_group: string;
  document_type: KycDocumentType;
  priority: KycPriority;
  status: KycReviewStatus;
  assigned_to: string | null;
  assigned_to_name: string | null;
  wait_time_minutes: number;
  reviewed_by: string | null;
  reviewed_at: string | null;
  decision_reason: string | null;
  created_at: string;
}

export interface KycQueueFilters {
  status?: KycReviewStatus;
  priority?: KycPriority;
  assigned_to?: string;
  document_type?: KycDocumentType;
  page?: number;
  page_size?: number;
}

export interface KycReviewPayload {
  decision: "approve" | "reject" | "resubmission";
  reason?: string;
  notes?: string;
}

export interface ScreeningResult {
  id: string;
  player_id: string;
  player_email: string;
  status: ScreeningStatus;
  matched_lists: string[];
  match_score: number;
  screened_at: string;
  next_screen_at: string | null;
  screened_by: string;
  reviewed_by: string | null;
  reviewed_at: string | null;
  review_notes: string | null;
}

export interface SofRequest {
  id: string;
  player_id: string;
  player_email: string;
  trigger_type: "threshold" | "single_tx" | "manual";
  threshold_amount: string | null;
  period_days: number | null;
  status: SofStatus;
  deadline_at: string;
  documents: SofDocument[];
  reviewed_by: string | null;
  reviewed_at: string | null;
  notes: string | null;
  created_at: string;
}

export interface SofDocument {
  type: "bank_statement" | "salary_slip" | "inheritance" | "property_sale" | "crypto_portfolio";
  file_url: string;
  uploaded_at: string;
}

export interface RgAlert {
  id: string;
  player_id: string;
  player_email: string;
  alert_type: RgAlertType;
  severity: "low" | "medium" | "high" | "critical";
  details: Record<string, unknown>;
  acknowledged_by: string | null;
  acknowledged_at: string | null;
  created_at: string;
}

export interface RgPlayerLimits {
  player_id: string;
  deposit_limit_daily: string | null;
  deposit_limit_weekly: string | null;
  deposit_limit_monthly: string | null;
  loss_limit: string | null;
  wager_limit_daily: string | null;
  session_time_limit_minutes: number | null;
  reality_check_frequency_minutes: number | null;
  self_exclusion_until: string | null;
  cool_off_until: string | null;
}

export interface ExpiryStats {
  expiring_30d: number;
  expiring_7d: number;
  expired: number;
}

export interface ExpiringDocument extends KycDocument {
  player_email: string;
  days_until_expiry: number;
}

export interface KycTeamMetric {
  officer_id: string;
  officer_name: string;
  metric_date: string;
  reviewed_count: number;
  avg_review_time_minutes: number;
  approve_count: number;
  reject_count: number;
  sla_breach_count: number;
}

export interface KycTeamStats {
  today: {
    queue_depth: number;
    avg_review_minutes: number;
    sla_breaches: number;
  };
  officers: KycTeamMetric[];
}
