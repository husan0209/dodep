import { api } from "./client";

export interface Game {
  id: string;
  name: string;
  provider_id: string;
  provider_name: string;
  category: string;
  tags: string[];
  description: string;
  image_url: string;
  thumbnail_url: string;
  supported_currencies: string[];
  is_active: boolean;
  is_demo_available: boolean;
  popularity_score: number;
  rtp?: number;
  volatility?: string;
}

export interface Provider {
  id: string;
  name: string;
  logo_url: string;
  description: string;
  is_active: boolean;
  games_count: number;
}

export interface GameSession {
  id: string;
  user_id: number;
  game_id: string;
  provider_id: string;
  status: "active" | "paused" | "ended";
  balance_at_start: string;
  started_at: string;
  last_activity: string;
  ended_at?: string;
  launch_url: string;
  token: string;
}

export interface GameFilters {
  provider_id?: string;
  category?: string;
  tags?: string[];
  search?: string;
  page?: number;
  page_size?: number;
}

export const casinoApi = {
  getGames: (filters?: GameFilters) =>
    api.get<Game[]>("/api/v1/casino/games", filters as Record<string, string>),

  getGame: (gameId: string) =>
    api.get<Game>(`/api/v1/casino/games/${gameId}`),

  launchGame: (gameId: string, deviceType: string = "web") =>
    api.post<GameSession>("/api/v1/casino/games/launch", { game_id: gameId, device_type: deviceType }),

  getGameSession: (sessionId: string) =>
    api.get<GameSession>(`/api/v1/casino/sessions/${sessionId}`),

  endGameSession: (sessionId: string) =>
    api.post<{ success: boolean }>(`/api/v1/casino/sessions/${sessionId}/end`),

  getGameHistory: (filters?: { game_id?: string; date_from?: string; date_to?: string; page?: number; page_size?: number }) =>
    api.get<GameSession[]>("/api/v1/casino/history", filters as Record<string, string>),

  getProviders: () =>
    api.get<Provider[]>("/api/v1/casino/providers"),
};
