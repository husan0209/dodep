import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  UserProfile,
  UserSession,
  UserLimits,
  UserSearchParams,
  PlayerGroup,
  BlockUserPayload,
  AdjustBalancePayload,
  UpdateTagsPayload,
  UpdateGroupPayload,
  AddNotePayload,
  GiveBonusPayload,
  RequestKycPayload,
  SendMessagePayload,
  UpdateLimitsPayload,
  MergePreviewResponse,
  MergeRequestPayload,
  CommunicationEntry,
  LinkedAccount,
  AdminNote,
  KycDocument,
  SupportChat,
} from "@/types/user";

export const usersService = {
  async list(
    params: UserSearchParams,
  ): Promise<PaginatedResponse<UserProfile>> {
    const response = await apiClient.get<PaginatedResponse<UserProfile>>(
      "/admin/users",
      { params },
    );
    return response.data;
  },

  async export(params: UserSearchParams): Promise<{ download_url: string }> {
    const response = await apiClient.post<ApiResponse<{ download_url: string }>>(
      "/admin/users/export",
      params,
    );
    return response.data.data;
  },

  async get(userId: string): Promise<UserProfile> {
    const response = await apiClient.get<ApiResponse<UserProfile>>(
      `/admin/users/${userId}`,
    );
    return response.data.data;
  },

  async update(
    userId: string,
    data: Partial<UserProfile>,
  ): Promise<UserProfile> {
    const response = await apiClient.put<ApiResponse<UserProfile>>(
      `/admin/users/${userId}`,
      data,
    );
    return response.data.data;
  },

  async block(userId: string, payload: BlockUserPayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/block`, payload);
  },

  async unblock(userId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/unblock`, { reason });
  },

  async getSessions(userId: string): Promise<UserSession[]> {
    const response = await apiClient.get<ApiResponse<UserSession[]>>(
      `/admin/users/${userId}/sessions`,
    );
    return response.data.data;
  },

  async revokeSession(userId: string, sessionId: string): Promise<void> {
    await apiClient.delete(`/admin/users/${userId}/sessions/${sessionId}`);
  },

  async getLimits(userId: string): Promise<UserLimits> {
    const response = await apiClient.get<ApiResponse<UserLimits>>(
      `/admin/users/${userId}/limits`,
    );
    return response.data.data;
  },

  async updateLimits(
    userId: string,
    payload: UpdateLimitsPayload,
  ): Promise<UserLimits> {
    const response = await apiClient.put<ApiResponse<UserLimits>>(
      `/admin/users/${userId}/limits`,
      payload,
    );
    return response.data.data;
  },

  async getActivity(
    userId: string,
    params?: { page?: number; page_size?: number },
  ): Promise<PaginatedResponse<unknown>> {
    const response = await apiClient.get<PaginatedResponse<unknown>>(
      `/admin/users/${userId}/activity`,
      { params },
    );
    return response.data;
  },

  // Player Actions
  async adjustBalance(
    userId: string,
    payload: AdjustBalancePayload,
  ): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/adjust-balance`, payload);
  },

  async updateTags(userId: string, payload: UpdateTagsPayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/tags`, payload);
  },

  async updateGroup(userId: string, payload: UpdateGroupPayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/group`, payload);
  },

  async addNote(userId: string, payload: AddNotePayload): Promise<AdminNote> {
    const response = await apiClient.post<ApiResponse<AdminNote>>(
      `/admin/users/${userId}/notes`,
      payload,
    );
    return response.data.data;
  },

  async getNotes(userId: string): Promise<AdminNote[]> {
    const response = await apiClient.get<ApiResponse<AdminNote[]>>(
      `/admin/users/${userId}/notes`,
    );
    return response.data.data;
  },

  async giveBonus(userId: string, payload: GiveBonusPayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/give-bonus`, payload);
  },

  async requestKyc(userId: string, payload: RequestKycPayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/request-kyc`, payload);
  },

  async sendMessage(userId: string, payload: SendMessagePayload): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/send-message`, payload);
  },

  // Merge
  async getMergePreview(
    primaryId: string,
    secondaryId: string,
  ): Promise<MergePreviewResponse> {
    const response = await apiClient.get<ApiResponse<MergePreviewResponse>>(
      "/admin/players/merge/preview",
      { params: { primary: primaryId, secondary: secondaryId } },
    );
    return response.data.data;
  },

  async mergePlayers(payload: MergeRequestPayload): Promise<void> {
    await apiClient.post("/admin/players/merge", payload);
  },

  // Communications
  async getCommunications(
    userId: string,
    params?: { page?: number; page_size?: number },
  ): Promise<PaginatedResponse<CommunicationEntry>> {
    const response = await apiClient.get<PaginatedResponse<CommunicationEntry>>(
      `/admin/users/${userId}/communications`,
      { params },
    );
    return response.data;
  },

  // Linked Accounts
  async getLinkedAccounts(userId: string): Promise<LinkedAccount[]> {
    const response = await apiClient.get<ApiResponse<LinkedAccount[]>>(
      `/admin/users/${userId}/linked-accounts`,
    );
    return response.data.data;
  },

  // KYC Documents
  async getKycDocuments(userId: string): Promise<KycDocument[]> {
    const response = await apiClient.get<ApiResponse<KycDocument[]>>(
      `/admin/users/${userId}/kyc-documents`,
    );
    return response.data.data;
  },

  // Support Chats
  async getSupportChats(
    userId: string,
    params?: { page?: number; page_size?: number },
  ): Promise<PaginatedResponse<SupportChat>> {
    const response = await apiClient.get<PaginatedResponse<SupportChat>>(
      `/admin/users/${userId}/support-chats`,
      { params },
    );
    return response.data;
  },

  // Bulk Actions
  async bulkAddTags(
    userIds: string[],
    payload: { tags: string[]; reason: string },
  ): Promise<void> {
    await apiClient.post("/admin/users/bulk/tags", {
      user_ids: userIds,
      ...payload,
    });
  },

  async bulkUpdateGroup(
    userIds: string[],
    payload: { group: PlayerGroup; reason: string },
  ): Promise<void> {
    await apiClient.post("/admin/users/bulk/group", {
      user_ids: userIds,
      ...payload,
    });
  },

  // Segments
  async getSegments(params?: { search?: string }): Promise<unknown[]> {
    const response = await apiClient.get<ApiResponse<unknown[]>>("/admin/users/segments", { params });
    return response.data.data;
  },

  async createSegment(data: unknown): Promise<unknown> {
    const response = await apiClient.post<ApiResponse<unknown>>("/admin/users/segments", data);
    return response.data.data;
  },

  async updateSegment(segmentId: string, data: unknown): Promise<unknown> {
    const response = await apiClient.put<ApiResponse<unknown>>(`/admin/users/segments/${segmentId}`, data);
    return response.data.data;
  },

  async getPlayerAnalytics(params: { from?: string; to?: string; metric?: string }): Promise<unknown> {
    const response = await apiClient.get<ApiResponse<unknown>>("/admin/users/analytics", { params });
    return response.data.data;
  },

  async getAdminUsers(params?: { search?: string; role?: string }): Promise<unknown[]> {
    const response = await apiClient.get<ApiResponse<unknown[]>>("/admin/users/admin", { params });
    return response.data.data;
  },

  async createAdminUser(data: unknown): Promise<unknown> {
    const response = await apiClient.post<ApiResponse<unknown>>("/admin/users/admin", data);
    return response.data.data;
  },

  async updateAdminUser(userId: string, data: unknown): Promise<unknown> {
    const response = await apiClient.put<ApiResponse<unknown>>(`/admin/users/admin/${userId}`, data);
    return response.data.data;
  },
};
