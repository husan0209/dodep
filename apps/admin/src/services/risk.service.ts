import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  FraudAlert,
  AuditLogEntry,
  AlertSearchParams,
} from "@/types/risk";

export const riskService = {
  async getAlerts(
    params: AlertSearchParams,
  ): Promise<PaginatedResponse<FraudAlert>> {
    const response = await apiClient.get<PaginatedResponse<FraudAlert>>(
      "/admin/risk/alerts",
      { params },
    );
    return response.data;
  },

  async getAlert(alertId: string): Promise<FraudAlert> {
    const response = await apiClient.get<ApiResponse<FraudAlert>>(
      `/admin/risk/alerts/${alertId}`,
    );
    return response.data.data;
  },

  async updateAlertStatus(
    alertId: string,
    status: string,
    resolution?: string,
  ): Promise<void> {
    await apiClient.put(`/admin/risk/alerts/${alertId}/status`, {
      status,
      resolution,
    });
  },

  async assignAlert(alertId: string, adminId: string): Promise<void> {
    await apiClient.put(`/admin/risk/alerts/${alertId}/assign`, {
      admin_id: adminId,
    });
  },

  async getAuditLog(params?: {
    admin_id?: string;
    action?: string;
    resource_type?: string;
    date_from?: string;
    date_to?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<AuditLogEntry>> {
    const response = await apiClient.get<PaginatedResponse<AuditLogEntry>>(
      "/admin/risk/audit-log",
      { params },
    );
    return response.data;
  },

  async getUserRiskProfile(userId: string): Promise<unknown> {
    const response = await apiClient.get(`/admin/risk/users/${userId}/profile`);
    return response.data.data;
  },
};
