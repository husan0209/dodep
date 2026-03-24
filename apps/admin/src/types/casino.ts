export type GameCategory =
  | "slots"
  | "table"
  | "live"
  | "crash"
  | "instant"
  | "card";
export type GameSessionStatus = "active" | "completed" | "abandoned";

export interface Game {
  id: string;
  external_id: string;
  provider: string;
  name: string;
  category: GameCategory;
  rtp: number;
  volatility: "low" | "medium" | "high";
  min_bet: string;
  max_bet: string;
  thumbnail_url: string;
  enabled: boolean;
  featured: boolean;
}

export interface GameSession {
  id: string;
  user_id: string;
  game_id: string;
  status: GameSessionStatus;
  total_bet: string;
  total_win: string;
  rounds_played: number;
  started_at: string;
  ended_at: string | null;
  device: string;
  ip_address: string;
}

export interface Provider {
  id: string;
  name: string;
  external_id: string;
  enabled: boolean;
  games_count: number;
}
