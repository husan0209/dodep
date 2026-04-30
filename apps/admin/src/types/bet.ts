export type BetStatus =
  | "pending"
  | "active"
  | "won"
  | "lost"
  | "void"
  | "cashout";
export type BetType = "single" | "accumulator" | "system";

export interface BetSelection {
  id: string;
  event_id: number;
  market_id: number;
  outcome_id: number;
  odds: string;
  result: "pending" | "won" | "lost" | "void" | null;
}

export interface Bet {
  id: string;
  user_id: string;
  bet_type: BetType;
  status: BetStatus;
  stake: string;
  potential_win: string;
  actual_win: string;
  odds: string;
  currency_code: string;
  sport_id: number | null;
  event_id: number | null;
  selections: BetSelection[];
  placed_at: string;
  settled_at: string | null;
  ip_address: string | null;
  device_fingerprint: string | null;
  metadata: Record<string, unknown>;
}

export interface BetSearchParams {
  user_id?: string;
  status?: BetStatus;
  bet_type?: BetType;
  sport_id?: number;
  stake_min?: string;
  stake_max?: string;
  placed_from?: string;
  placed_to?: string;
  page?: number;
  page_size?: number;
}

export interface SportEvent {
  id: string;
  sport: string;
  league?: string;
  home_team: string;
  away_team: string;
  starts_at: string;
  status: string;
  live: boolean;
}

export interface EventSearchParams {
  sport_id?: number;
  status?: string;
  search?: string;
  page?: number;
  page_size?: number;
}
