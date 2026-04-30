import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  BonusCampaign,
  UserBonus,
  BonusSearchParams,
} from "@/types/bonus";

export const bonusesService = {
  async getCampaigns(
    params: BonusSearchParams,
  ): Promise<PaginatedResponse<BonusCampaign>> {
    const response = await apiClient.get<PaginatedResponse<BonusCampaign>>(
      "/admin/bonuses/campaigns",
      { params },
    );
    return response.data;
  },

  async getCampaign(campaignId: string): Promise<BonusCampaign> {
    const response = await apiClient.get<ApiResponse<BonusCampaign>>(
      `/admin/bonuses/campaigns/${campaignId}`,
    );
    return response.data.data;
  },

  async createCampaign(data: Partial<BonusCampaign>): Promise<BonusCampaign> {
    const response = await apiClient.post<ApiResponse<BonusCampaign>>(
      "/admin/bonuses/campaigns",
      data,
    );
    return response.data.data;
  },

  async updateCampaign(
    campaignId: string,
    data: Partial<BonusCampaign>,
  ): Promise<BonusCampaign> {
    const response = await apiClient.put<ApiResponse<BonusCampaign>>(
      `/admin/bonuses/campaigns/${campaignId}`,
      data,
    );
    return response.data.data;
  },

  async deleteCampaign(campaignId: string): Promise<void> {
    await apiClient.delete(`/admin/bonuses/campaigns/${campaignId}`);
  },

  async toggleCampaign(
    campaignId: string,
    status: "active" | "paused",
  ): Promise<void> {
    await apiClient.put(`/admin/bonuses/campaigns/${campaignId}/status`, {
      status,
    });
  },

  async grantBonus(userId: string, campaignId: string): Promise<void> {
    await apiClient.post(`/admin/bonuses/grant`, {
      user_id: userId,
      campaign_id: campaignId,
    });
  },

  async getUserBonuses(params?: {
    user_id?: string;
    campaign_id?: string;
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<UserBonus>> {
    const response = await apiClient.get<PaginatedResponse<UserBonus>>(
      "/admin/bonuses/user-bonuses",
      { params },
    );
    return response.data;
  },
};
