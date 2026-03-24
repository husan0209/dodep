export type UserStatus =
  | "active"
  | "blocked"
  | "pending"
  | "self_excluded"
  | "suspended";
export type KycLevel = 0 | 1 | 2 | 3 | 4;

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
  page?: number;
  page_size?: number;
}

export interface UserListResponse {
  items: UserProfile[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}
