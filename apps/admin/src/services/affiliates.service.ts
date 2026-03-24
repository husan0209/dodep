import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";

export interface Affiliate {
  id: string;
  name: string;
  email: string;
  status: "pending" | "active" | "suspended" | "rejected";
  commission_type: "revenue_share" | "cpa" | "hybrid";
  commission_rate: number;
  total_referrals: number;
  total_revenue: string;
  total_commission: string;
  created_at: string;
}

export const affiliatesService = {
  async getAffiliates(params?: {
    status?: string;
    search?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<Affiliate>> {
    const response = await apiClient.get<PaginatedResponse<Affiliate>>(
      "/admin/affiliates",
      { params },
    );
    return response.data;
  },

  async getAffiliate(affiliateId: string): Promise<Affiliate> {
    const response = await apiClient.get<ApiResponse<Affiliate>>(
      `/admin/affiliates/${affiliateId}`,
    );
    return response.data.data;
  },

  async approveAffiliate(affiliateId: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/${affiliateId}/approve`);
  },

  async rejectAffiliate(affiliateId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/${affiliateId}/reject`, { reason });
  },

  async updateCommission(
    affiliateId: string,
    data: { commission_type: string; commission_rate: number },
  ): Promise<void> {
    await apiClient.put(`/admin/affiliates/${affiliateId}/commission`, data);
  },

  async getPayments(params?: {
    affiliate_id?: string;
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<unknown>> {
    const response = await apiClient.get("/admin/affiliates/payments", {
      params,
    });
    return response.data;
  },
};
