import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  UserProfile,
  UserSession,
  UserLimits,
  UserSearchParams,
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

  async block(userId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/block`, { reason });
  },

  async unblock(userId: string): Promise<void> {
    await apiClient.post(`/admin/users/${userId}/unblock`);
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
    limits: Partial<UserLimits>,
  ): Promise<UserLimits> {
    const response = await apiClient.put<ApiResponse<UserLimits>>(
      `/admin/users/${userId}/limits`,
      limits,
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
};
