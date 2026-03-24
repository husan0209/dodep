export type BonusType =
  | "welcome"
  | "reload"
  | "free_spins"
  | "free_bet"
  | "cashback"
  | "no_deposit"
  | "loyalty";
export type BonusStatus = "active" | "paused" | "expired" | "draft";
export type UserBonusStatus = "active" | "completed" | "forfeited" | "expired";

export interface BonusCampaign {
  id: string;
  name: string;
  type: BonusType;
  status: BonusStatus;
  match_percent: number;
  max_amount: string;
  min_deposit: string;
  wagering_multiplier: number;
  expiry_days: number;
  max_bet: string;
  start_date: string;
  end_date: string | null;
  target_countries: string[];
  target_segments: string[];
  total_claims: number;
  created_at: string;
}

export interface UserBonus {
  id: string;
  user_id: string;
  campaign_id: string;
  campaign_name: string;
  bonus_amount: string;
  wagering_requirement: string;
  wagering_progress: string;
  status: UserBonusStatus;
  claimed_at: string;
  expires_at: string;
  completed_at: string | null;
}

export interface BonusSearchParams {
  type?: BonusType;
  status?: BonusStatus;
  search?: string;
  page?: number;
  page_size?: number;
}
