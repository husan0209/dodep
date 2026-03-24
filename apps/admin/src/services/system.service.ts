import apiClient from "./api";
import type { ApiResponse } from "@/types/api";
import type { DashboardStats } from "@/types/admin";

export const systemService = {
  async getDashboardStats(): Promise<DashboardStats> {
    const response = await apiClient.get<ApiResponse<DashboardStats>>(
      "/admin/system/dashboard",
    );
    return response.data.data;
  },

  async getHealthStatus(): Promise<
    Record<string, { status: string; latency_ms: number }>
  > {
    const response = await apiClient.get<
      ApiResponse<Record<string, { status: string; latency_ms: number }>>
    >("/admin/system/health");
    return response.data.data;
  },

  async getFeatureFlags(): Promise<
    Array<{ key: string; enabled: boolean; description: string }>
  > {
    const response = await apiClient.get("/admin/system/feature-flags");
    return response.data.data;
  },

  async toggleFeatureFlag(key: string, enabled: boolean): Promise<void> {
    await apiClient.put(`/admin/system/feature-flags/${key}`, { enabled });
  },

  async getConfig(): Promise<Record<string, unknown>> {
    const response = await apiClient.get("/admin/system/config");
    return response.data.data;
  },

  async updateConfig(key: string, value: unknown): Promise<void> {
    await apiClient.put(`/admin/system/config/${key}`, { value });
  },
};
