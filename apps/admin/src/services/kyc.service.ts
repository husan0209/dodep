import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  KycQueueFilters,
  KycReviewPayload,
  KycReviewItem,
  KycDocument,
  ScreeningResult,
  SofRequest,
  RgAlert,
  RgPlayerLimits,
  ExpiryStats,
  ExpiringDocument,
  KycTeamStats,
} from "@/types/kyc";

export const kycService = {
  // KYC Queue
  async getQueue(filters: KycQueueFilters): Promise<PaginatedResponse<KycReviewItem>> {
    const response = await apiClient.get<PaginatedResponse<KycReviewItem>>("/admin/kyc/queue", {
      params: filters,
    });
    return response.data;
  },

  async assignReview(reviewId: string, adminId: string): Promise<void> {
    await apiClient.post(`/admin/kyc/reviews/${reviewId}/assign`, {
      assigned_to: adminId,
    });
  },

  async submitReview(
    reviewId: string,
    payload: KycReviewPayload,
  ): Promise<void> {
    await apiClient.post(`/admin/kyc/reviews/${reviewId}/decision`, payload);
  },

  // KYC Documents
  async getDocument(documentId: string): Promise<KycDocument> {
    const response = await apiClient.get<ApiResponse<KycDocument>>(
      `/admin/kyc/documents/${documentId}`,
    );
    return response.data.data;
  },

  async getPlayerDocuments(playerId: string): Promise<KycDocument[]> {
    const response = await apiClient.get<ApiResponse<KycDocument[]>>(
      `/admin/kyc/players/${playerId}/documents`,
    );
    return response.data.data;
  },

  // SOF Requests
  async getSofRequests(
    params?: { status?: string; page?: number; page_size?: number },
  ): Promise<PaginatedResponse<SofRequest>> {
    const response = await apiClient.get<PaginatedResponse<SofRequest>>(
      "/admin/kyc/sof/requests",
      { params },
    );
    return response.data;
  },

  async reviewSof(
    requestId: string,
    payload: { decision: "approve" | "reject"; notes?: string },
  ): Promise<void> {
    await apiClient.post(`/admin/kyc/sof/requests/${requestId}/review`, payload);
  },

  // Screening
  async getScreenings(
    params?: { status?: string; page?: number; page_size?: number },
  ): Promise<PaginatedResponse<ScreeningResult>> {
    const response = await apiClient.get<PaginatedResponse<ScreeningResult>>(
      "/admin/kyc/screenings",
      { params },
    );
    return response.data;
  },

  async getPlayerScreening(playerId: string): Promise<ScreeningResult | null> {
    const response = await apiClient.get<ApiResponse<ScreeningResult | null>>(
      `/admin/kyc/screenings/player/${playerId}`,
    );
    return response.data.data;
  },

  async rescreenPlayer(playerId: string): Promise<void> {
    await apiClient.post(`/admin/kyc/screenings/player/${playerId}/rescreen`);
  },

  async reviewScreening(
    screeningId: string,
    payload: { decision: ScreeningResult["status"]; notes: string },
  ): Promise<void> {
    await apiClient.post(
      `/admin/kyc/screenings/${screeningId}/review`,
      payload,
    );
  },

  // Expiry Tracking
  async getExpiryStats(): Promise<ExpiryStats> {
    const response = await apiClient.get<ApiResponse<ExpiryStats>>(
      "/admin/kyc/expiry-stats",
    );
    return response.data.data;
  },

  async getExpiringDocuments(
    days: number,
    page?: number,
  ): Promise<PaginatedResponse<ExpiringDocument>> {
    const response = await apiClient.get<PaginatedResponse<ExpiringDocument>>(
      "/admin/kyc/expiring",
      { params: { days, page } },
    );
    return response.data;
  },

  // Team Metrics
  async getTeamStats(period: "today" | "week" | "month"): Promise<KycTeamStats> {
    const response = await apiClient.get<ApiResponse<KycTeamStats>>(
      "/admin/kyc/team-stats",
      { params: { period } },
    );
    return response.data.data;
  },

  // RG Dashboard
  async getRgAlerts(
    params?: { severity?: string; acknowledged?: boolean; page?: number },
  ): Promise<PaginatedResponse<RgAlert>> {
    const response = await apiClient.get<PaginatedResponse<RgAlert>>(
      "/admin/kyc/rg/alerts",
      { params },
    );
    return response.data;
  },

  async acknowledgeRgAlert(alertId: string): Promise<void> {
    await apiClient.post(`/admin/kyc/rg/alerts/${alertId}/acknowledge`);
  },

  async getRgPlayerLimits(playerId: string): Promise<RgPlayerLimits> {
    const response = await apiClient.get<ApiResponse<RgPlayerLimits>>(
      `/admin/kyc/rg/players/${playerId}/limits`,
    );
    return response.data.data;
  },

  async updateRgPlayerLimits(
    playerId: string,
    payload: Partial<RgPlayerLimits>,
  ): Promise<RgPlayerLimits> {
    const response = await apiClient.put<ApiResponse<RgPlayerLimits>>(
      `/admin/kyc/rg/players/${playerId}/limits`,
      payload,
    );
    return response.data.data;
  },
};
