import apiClient from "./api";
import type { ApiResponse } from "@/types/api";

export const reportsService = {
  async getGameAnalytics(params: { from?: string; to?: string; provider?: string }): Promise<unknown> {
    const response = await apiClient.get<ApiResponse<unknown>>("/admin/games/analytics", { params });
    return response.data.data;
  },
};
