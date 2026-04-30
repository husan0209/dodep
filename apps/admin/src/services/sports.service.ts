import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type { Bet, BetSearchParams, SportEvent, EventSearchParams } from "@/types/bet";

export const sportsService = {
  async getBets(params: BetSearchParams): Promise<PaginatedResponse<Bet>> {
    const response = await apiClient.get<PaginatedResponse<Bet>>(
      "/admin/sports/bets",
      { params },
    );
    return response.data;
  },

  async getBet(betId: string): Promise<Bet> {
    const response = await apiClient.get<ApiResponse<Bet>>(
      `/admin/sports/bets/${betId}`,
    );
    return response.data.data;
  },

  async voidBet(betId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/sports/bets/${betId}/void`, { reason });
  },

  async resettleBet(betId: string, result: string): Promise<void> {
    await apiClient.post(`/admin/sports/bets/${betId}/resettle`, { result });
  },

  async getEvents(params?: EventSearchParams): Promise<PaginatedResponse<SportEvent>> {
    const response = await apiClient.get<PaginatedResponse<SportEvent>>(
      "/admin/sports/events",
      { params },
    );
    return response.data;
  },

  async updateEvent(eventId: string, payload: Partial<SportEvent>): Promise<SportEvent> {
    const response = await apiClient.patch<ApiResponse<SportEvent>>(
      `/admin/sports/events/${eventId}`,
      payload,
    );
    return response.data.data;
  },

  async getOddsSnapshot(sport: string): Promise<{ data: Array<{ event_id: string; home_odds: number; draw_odds?: number; away_odds: number; timestamp: string }> }> {
    const response = await apiClient.get("/admin/sports/odds/snapshot", { params: { sport } });
    return response.data;
  },

  async suspendEvent(eventId: number, reason: string): Promise<void> {
    await apiClient.post(`/admin/sports/events/${eventId}/suspend`, { reason });
  },

  async getLiability(params?: {
    sport_id?: number;
    event_id?: number;
  }): Promise<unknown> {
    const response = await apiClient.get("/admin/sports/liability", { params });
    return response.data.data;
  },
};
