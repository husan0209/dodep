import { api } from "./client";

export interface Selection {
  event_id: number;
  market_id: number;
  outcome_id: number;
  odds: number;
  outcome_name: string;
}

export interface PlaceBetRequest {
  bet_type: "single" | "accumulator" | "system";
  selections: Selection[];
  stake: number;
  currency: string;
  accept_odds_changes: "none" | "higher" | "any";
  idempotency_key: string;
}

export interface Bet {
  id: number;
  user_id: number;
  bet_type: "single" | "accumulator" | "system";
  status: "pending" | "active" | "won" | "lost" | "void" | "cashout";
  stake: string;
  potential_win: string;
  actual_win?: string;
  odds: string;
  selections: Selection[];
  placed_at: string;
  settled_at?: string;
}

export interface BetFilters {
  status?: string;
  bet_type?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export const betsApi = {
  placeBet: (data: PlaceBetRequest) =>
    api.post<Bet>("/api/v1/bets", data).then((r) => r.data),

  getActive: () =>
    api.get<Bet[]>("/api/v1/bets/active").then((r) => r.data),

  getHistory: (filters?: BetFilters) =>
    api.get<Bet[]>("/api/v1/bets/history", filters as Record<string, string>).then((r) => r.data),

  getById: (betId: number) =>
    api.get<Bet>(`/api/v1/bets/${betId}`).then((r) => r.data),

  cashout: (betId: number) =>
    api.post<Bet>(`/api/v1/bets/${betId}/cashout`).then((r) => r.data),

  getCashoutValue: (betId: number) =>
    api.get<{ amount: string }>("/api/v1/bets/{betId}/cashout-value".replace("{betId}", betId.toString())).then((r) => r.data),
};
