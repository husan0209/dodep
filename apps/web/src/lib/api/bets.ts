import { api } from "./client";
import { assertMoney, assertPositiveMoney } from "./money";

export interface Selection {
  event_id: number;
  market_id: number;
  outcome_id: number;
  /** Decimal string (e.g. "1.85"), never a float. */
  odds: string;
  outcome_name: string;
}

export interface PlaceBetRequest {
  bet_type: "single" | "accumulator" | "system";
  selections: Selection[];
  /** Stake as decimal string (NUMERIC(18,8)) — CONVENTIONS NEVER-6. */
  stake: string;
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
  placeBet: async (data: PlaceBetRequest) =>
    api.post<Bet>("/api/v1/bets", {
      ...data,
      // Guards: stake must be a positive decimal string and every odds
      // value must be a decimal string — floats never reach the wire.
      stake: assertPositiveMoney(data.stake, "bet.stake"),
      selections: data.selections.map((selection, index) => ({
        ...selection,
        odds: assertMoney(selection.odds, `bet.selections[${index}].odds`),
      })),
    }),

  getActive: () =>
    api.get<Bet[]>("/api/v1/bets/active"),

  getHistory: (filters?: BetFilters) =>
    api.get<Bet[]>("/api/v1/bets/history", filters as Record<string, string>),

  getById: (betId: number) =>
    api.get<Bet>(`/api/v1/bets/${betId}`),

  cashout: (betId: number) =>
    api.post<Bet>(`/api/v1/bets/${betId}/cashout`),

  getCashoutValue: (betId: number) =>
    api.get<{ amount: string }>(`/api/v1/bets/${betId}/cashout-value`),
};
