import apiClient from "./api";
import type { ApiResponse } from "@/types/api";
import type {
  DashboardStats,
  ProviderHealth,
  GatewayHealth,
  TopItem,
  ChartPoint,
  ConversionFunnel,
} from "@/types/admin";

export const systemService = {
  async getDashboardStats(): Promise<DashboardStats> {
    const response = await apiClient.get<ApiResponse<DashboardStats>>(
      "/admin/system/dashboard",
    );
    return response.data.data;
  },

  async getProviderHealth(): Promise<ProviderHealth[]> {
    const response = await apiClient.get<ApiResponse<ProviderHealth[]>>(
      "/admin/dashboard/provider-health",
    );
    return response.data.data;
  },

  async getGatewayHealth(): Promise<GatewayHealth[]> {
    const response = await apiClient.get<ApiResponse<GatewayHealth[]>>(
      "/admin/dashboard/gateway-health",
    );
    return response.data.data;
  },

  async getGGRChart(period: string = "30d"): Promise<ChartPoint[]> {
    const response = await apiClient.get<ApiResponse<ChartPoint[]>>(
      `/admin/dashboard/charts/ggr?period=${period}`,
    );
    return response.data.data;
  },

  async getDepositsVsWithdrawalsChart(): Promise<ChartPoint[]> {
    const response = await apiClient.get<ApiResponse<ChartPoint[]>>(
      "/admin/dashboard/charts/deposits-vs-withdrawals",
    );
    return response.data.data;
  },

  async getConversionFunnel(): Promise<ConversionFunnel> {
    const response = await apiClient.get<ApiResponse<ConversionFunnel>>(
      "/admin/dashboard/conversion-funnel",
    );
    return response.data.data;
  },

  async getTopGames(limit: number = 5): Promise<TopItem[]> {
    const response = await apiClient.get<ApiResponse<TopItem[]>>(
      `/admin/dashboard/top-games?limit=${limit}&period=today`,
    );
    return response.data.data;
  },

  async getTopEvents(limit: number = 5): Promise<TopItem[]> {
    const response = await apiClient.get<ApiResponse<TopItem[]>>(
      `/admin/dashboard/top-events?limit=${limit}&period=today`,
    );
    return response.data.data;
  },

  async getTopCountries(limit: number = 5): Promise<TopItem[]> {
    const response = await apiClient.get<ApiResponse<TopItem[]>>(
      `/admin/dashboard/top-countries?limit=${limit}&period=today`,
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

  async getMaintenanceStatus(): Promise<Record<string, unknown>> {
    const response = await apiClient.get<ApiResponse<Record<string, unknown>>>("/admin/system/maintenance");
    return response.data.data;
  },

  async setMaintenanceMode(enabled: boolean): Promise<void> {
    await apiClient.post("/admin/system/maintenance", { enabled });
  },

  async scheduleMaintenance(params: { message: string; start?: string; end?: string }): Promise<void> {
    await apiClient.post("/admin/system/maintenance/schedule", params);
  },

  async getSettings(): Promise<Record<string, unknown>> {
    const response = await apiClient.get<ApiResponse<Record<string, unknown>>>("/admin/system/settings");
    return response.data.data;
  },

  async updateSettings(values: Record<string, unknown>): Promise<Record<string, unknown>> {
    const response = await apiClient.put<ApiResponse<Record<string, unknown>>>("/admin/system/settings", values);
    return response.data.data;
  },

  async getAuditLogs(params?: { from?: string; to?: string; action?: string; search?: string }): Promise<unknown[]> {
    const response = await apiClient.get<ApiResponse<unknown[]>>("/admin/system/audit-logs", { params });
    return response.data.data;
  },
};
