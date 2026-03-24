import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type { Game, GameSession, Provider } from "@/types/casino";

export const casinoService = {
  async getGames(params?: {
    category?: string;
    provider?: string;
    enabled?: boolean;
    search?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<Game>> {
    const response = await apiClient.get<PaginatedResponse<Game>>(
      "/admin/casino/games",
      { params },
    );
    return response.data;
  },

  async getGame(gameId: string): Promise<Game> {
    const response = await apiClient.get<ApiResponse<Game>>(
      `/admin/casino/games/${gameId}`,
    );
    return response.data.data;
  },

  async updateGame(gameId: string, data: Partial<Game>): Promise<Game> {
    const response = await apiClient.put<ApiResponse<Game>>(
      `/admin/casino/games/${gameId}`,
      data,
    );
    return response.data.data;
  },

  async getProviders(): Promise<Provider[]> {
    const response = await apiClient.get<ApiResponse<Provider[]>>(
      "/admin/casino/providers",
    );
    return response.data.data;
  },

  async toggleProvider(providerId: string, enabled: boolean): Promise<void> {
    await apiClient.put(`/admin/casino/providers/${providerId}`, { enabled });
  },

  async getGameSessions(params?: {
    user_id?: string;
    game_id?: string;
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<GameSession>> {
    const response = await apiClient.get<PaginatedResponse<GameSession>>(
      "/admin/casino/sessions",
      { params },
    );
    return response.data;
  },

  async getRtpReport(params?: {
    provider?: string;
    date_from?: string;
    date_to?: string;
  }): Promise<unknown> {
    const response = await apiClient.get("/admin/casino/rtp-report", {
      params,
    });
    return response.data.data;
  },
};
