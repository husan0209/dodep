export type UserStatus =
  | "active"
  | "blocked"
  | "pending"
  | "self_excluded"
  | "suspended";
export type KycLevel = 0 | 1 | 2 | 3 | 4;
export type PlayerGroup = "standard" | "vip" | "vvip" | "whale";

export interface User {
  id: string;
  uuid: string;
  email: string;
  phone: string | null;
  status: UserStatus;
  kyc_level: KycLevel;
  country_code: string;
  currency_code: string;
  created_at: string;
  updated_at: string;
  last_login_at: string | null;
  metadata: Record<string, unknown>;
}

export interface UserProfile extends User {
  first_name: string | null;
  last_name: string | null;
  avatar_url: string | null;
  date_of_birth: string | null;
  address: string | null;
  city: string | null;
  postal_code: string | null;
  username: string | null;
  group: PlayerGroup;
  tags: string[];
  balance: string;
  bonus_balance: string;
  deposit_total: string;
  withdrawal_total: string;
  ggr: string;
  risk_score: number;
  affiliate_id: string | null;
}

export interface UserSession {
  id: string;
  device_fingerprint: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
  last_activity: string;
  is_current: boolean;
}

export interface UserLimits {
  deposit_limit_daily: string | null;
  deposit_limit_weekly: string | null;
  deposit_limit_monthly: string | null;
  loss_limit: string | null;
  session_time_limit_minutes: number | null;
  wager_limit_daily: string | null;
  self_exclusion_until: string | null;
}

export interface UserSearchParams {
  search?: string;
  status?: UserStatus;
  kyc_level?: KycLevel;
  country_code?: string;
  created_from?: string;
  created_to?: string;
  last_login_from?: string;
  last_login_to?: string;
  tags?: string[];
  player_group?: PlayerGroup;
  affiliate_id?: string;
  deposit_min?: string;
  deposit_max?: string;
  ggr_min?: string;
  ggr_max?: string;
  balance_min?: string;
  balance_max?: string;
  risk_score_min?: number;
  risk_score_max?: number;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: "asc" | "desc";
}

export interface UserListResponse {
  items: UserProfile[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

// Player Actions
export interface BlockUserPayload {
  type: "full" | "casino" | "sports" | "temporary";
  duration_hours?: number;
  reason: string;
}

export interface AdjustBalancePayload {
  amount: string;
  currency: string;
  type: "credit" | "debit";
  reason: string;
}

export interface UpdateTagsPayload {
  add?: string[];
  remove?: string[];
  reason: string;
}

export interface UpdateGroupPayload {
  group: PlayerGroup;
  reason: string;
}

export interface AddNotePayload {
  text: string;
}

export interface GiveBonusPayload {
  bonus_id: string;
  reason: string;
}

export interface RequestKycPayload {
  type: "identity" | "address" | "source_of_funds";
  message?: string;
}

export interface SendMessagePayload {
  channel: "email" | "sms" | "push";
  subject?: string;
  body: string;
}

export interface UpdateLimitsPayload {
  max_deposit_daily?: string;
  max_deposit_weekly?: string;
  max_withdrawal_daily?: string;
  max_bet?: string;
  max_loss_daily?: string;
  reason: string;
}

// Merge
export interface MergePreviewResponse {
  primary: UserProfile;
  secondary: UserProfile;
  conflicts: string[];
  final_state: Partial<UserProfile>;
}

export interface MergeRequestPayload {
  primary_id: string;
  secondary_id: string;
  reason: string;
  totp_code: string;
  confirmed_by: string;
}

// Communication Timeline
export interface CommunicationEntry {
  id: string;
  type: "email" | "sms" | "push" | "chat" | "call";
  date: string;
  subject: string;
  preview: string;
  channel: string;
  status: "sent" | "delivered" | "opened" | "clicked" | "failed" | "suppressed";
  campaign?: string;
}

// Linked Account
export interface LinkedAccount {
  player_id: string;
  username: string;
  link_type: "device" | "ip" | "payment_method";
  link_value: string;
  confidence: number;
}

// Admin Note
export interface AdminNote {
  id: string;
  author_id: string;
  author_name: string;
  text: string;
  created_at: string;
}

// KYC Document
export type KycDocumentType = "identity" | "address" | "source_of_funds" | "selfie";
export type KycDocumentStatus = "pending" | "verified" | "rejected" | "expired";

export interface KycDocument {
  id: string;
  user_id: string;
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
}

// Support Chat
export interface SupportChat {
  id: string;
  user_id: string;
  subject: string;
  status: "open" | "resolved" | "closed";
  agent_name: string | null;
  last_message: string;
  last_message_at: string;
  created_at: string;
}
