import apiClient from "./api";
import type { PaginatedResponse } from "@/types/api";

export interface AffiliateDashboard {
  earnings_today?: string;
  earnings_this_month?: string;
  pending_amount?: string;
  available_amount?: string;
  paid_amount?: string;
  next_payout_date?: string | null;
  clicks?: number;
  registrations?: number;
  ftd_count?: number;
  active_players?: number;
  ggr_amount?: string;
  ngr_amount?: string;
  commission_amount?: string;
  commission_reversed_amount?: string;
  commission_adjusted_amount?: string;
  currency?: string;
}

export interface Affiliate {
  id: string;
  user_id: number;
  status: string;
  affiliate_code: string;
  commission_rate: string;
  hold_period_days: number;
  min_payout_amount: string;
  currency: string;
  kyc_required: boolean;
  approved_by: string;
  approved_at: string | null;
  created_at: string;
  // Dashboard fields (returned in detail view)
  profile?: any;
  dashboard?: AffiliateDashboard;
}

export interface Payout {
  id: string;
  affiliate_id: string;
  amount: string;
  currency: string;
  status: string;
  period_start?: string;
  period_end?: string;
  provider_reference?: string | null;
  rejection_reason?: string | null;
  created_at: string;
}

export interface FraudFlag {
  id: string;
  affiliate_id: string;
  referred_user_id: number;
  flag_type: string;
  status: string;
  details?: Record<string, unknown> | null;
  flagged_by?: string | null;
  resolved_by?: string | null;
  resolved_at?: string | null;
  notes?: string | null;
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
    const response = await apiClient.get<Affiliate>(
      `/admin/affiliates/${affiliateId}`,
    );
    return response.data;
  },

  async approveAffiliate(
    userId: string,
    data?: {
      commission_rate?: string;
      hold_period_days?: number;
      min_payout_amount?: string;
      currency?: string;
    },
  ): Promise<void> {
    await apiClient.post(`/admin/affiliates/${userId}/approve`, data || {});
  },

  async rejectAffiliate(userId: string, reviewNotes: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/${userId}/reject`, {
      review_notes: reviewNotes,
    });
  },

  async suspendAffiliate(affiliateId: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/${affiliateId}/suspend`);
  },

  async updateCommissionRate(
    affiliateId: string,
    rate: number,
  ): Promise<void> {
    await apiClient.put(`/admin/affiliates/${affiliateId}/commission-rate`, {
      commission_rate: String(rate),
    });
  },

  async createAdjustment(
    affiliateId: string,
    data: { adjustment_type: string; amount: string; reason: string },
  ): Promise<void> {
    await apiClient.post(
      `/admin/affiliates/${affiliateId}/adjustments`,
      data,
    );
  },

  async getPayouts(params?: {
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<Record<string, unknown>>> {
    const response = await apiClient.get<PaginatedResponse<Record<string, unknown>>>(
      "/admin/affiliates/payouts",
      { params },
    );
    return response.data;
  },

  async approvePayout(
    payoutId: string,
    providerReference?: string,
  ): Promise<void> {
    await apiClient.post(`/admin/affiliates/payouts/${payoutId}/approve`, {
      provider_reference: providerReference || "",
    });
  },

  async rejectPayout(payoutId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/payouts/${payoutId}/reject`, {
      rejection_reason: reason,
    });
  },

  async getFraudFlags(params?: {
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<Record<string, unknown>>> {
    const response = await apiClient.get<PaginatedResponse<Record<string, unknown>>>(
      "/admin/affiliates/fraud-flags",
      { params },
    );
    return response.data;
  },
};
