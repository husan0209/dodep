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

export interface AffiliateStats {
  clicks: number;
  registrations: number;
  ftd_count: number;
  players: number;
  active_players: number;
  ggr: string;
  ngr: string;
  commission_accrued: string;
  commission_reversed: string;
  commission_pending: string;
  commission_available: string;
  commission_paid: string;
  owed: string;
  open_fraud_flags: number;
  click_to_reg_rate: number;
  reg_to_ftd_rate: number;
}

/** Referred player is anonymised: `player_ref` is a stable pseudonymous id. */
export interface AffiliatePlayerSummary {
  player_ref: string;
  attributed_at: string;
  ftd_qualified: boolean;
  ftd_at?: string | null;
  ngr_amount: string;
  commission_amount: string;
}

export type PostbackEvent =
  | "registration"
  | "ftd"
  | "deposit"
  | "redeposit";

export type PostbackMethod = "GET" | "POST";

export interface PostbackConfig {
  event: PostbackEvent;
  url: string;
  method: PostbackMethod;
  variables?: Record<string, string>;
  retry_count?: number;
  retry_backoff?: string;
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

  async getAffiliateStats(affiliateId: string, days?: number): Promise<AffiliateStats> {
    const response = await apiClient.get<{ data: AffiliateStats }>(
      `/admin/affiliates/${affiliateId}/stats`,
      { params: days && days > 0 ? { days } : undefined },
    );
    return response.data.data;
  },

  async getAffiliatePlayers(
    affiliateId: string,
    params?: { page?: number; page_size?: number },
  ): Promise<PaginatedResponse<AffiliatePlayerSummary>> {
    const response = await apiClient.get<
      PaginatedResponse<AffiliatePlayerSummary>
    >(`/admin/affiliates/${affiliateId}/players`, { params });
    return response.data;
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

  /** Releases an approved payout to the partner (money movement step). */
  async markPayoutPaid(payoutId: string, providerReference?: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/payouts/${payoutId}/paid`, {
      provider_reference: providerReference || "",
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

  /** Confirmed fraud — notes are mandatory. */
  async resolveFraudFlag(flagId: string, notes: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/fraud-flags/${flagId}/resolve`, {
      notes,
    });
  },

  /** False positive — notes optional. */
  async dismissFraudFlag(flagId: string, notes?: string): Promise<void> {
    await apiClient.post(`/admin/affiliates/fraud-flags/${flagId}/dismiss`, {
      notes: notes || "",
    });
  },

  async getPostbackConfigs(affiliateId: string): Promise<PostbackConfig[]> {
    const response = await apiClient.get<{ data: PostbackConfig[] }>(
      `/admin/affiliates/${affiliateId}/postback-config`,
    );
    return response.data.data ?? [];
  },

  async updatePostbackConfigs(
    affiliateId: string,
    configs: PostbackConfig[],
  ): Promise<void> {
    await apiClient.put(`/admin/affiliates/${affiliateId}/postback-config`, configs);
  },
};
