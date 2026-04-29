import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";

export interface RegulatoryReport {
  id: string;
  jurisdiction: string;
  report_type: string;
  period_start: string;
  period_end: string;
  status: string;
  generated_at?: string;
  submitted_at?: string;
  submitted_by?: string;
  regulator_ref?: string;
  file_url?: string;
  created_at: string;
}

export interface SARReport {
  id: string;
  jurisdiction: string;
  player_id: number;
  trigger_type: string;
  status: string;
  amount_involved?: string;
  currency?: string;
  description: string;
  assigned_to?: string;
  tipping_off_lock: boolean;
  created_at: string;
}

export interface PlayerComplaint {
  id: string;
  player_id: number;
  ticket_id?: string;
  category: string;
  description: string;
  status: string;
  adr_ref?: string;
  resolution?: string;
  resolved_at?: string;
  assigned_to?: string;
  created_at: string;
}

export interface TaxConfig {
  id: string;
  jurisdiction: string;
  tax_type: string;
  tax_base: string;
  rate: string;
  currency: string;
  effective_from: string;
  effective_to?: string;
}

export const regulatoryService = {
  // Reports
  async getReports(params?: { status?: string; jurisdiction?: string; page?: number }): Promise<PaginatedResponse<RegulatoryReport>> {
    const response = await apiClient.get<PaginatedResponse<RegulatoryReport>>("/admin/regulatory/reports", { params });
    return response.data;
  },

  async createReport(data: Partial<RegulatoryReport>): Promise<RegulatoryReport> {
    const response = await apiClient.post<ApiResponse<RegulatoryReport>>("/admin/regulatory/reports", data);
    return response.data.data;
  },

  async updateReportStatus(id: string, status: string, regulatorRef?: string): Promise<RegulatoryReport> {
    const response = await apiClient.put<ApiResponse<RegulatoryReport>>(`/admin/regulatory/reports/${id}/status`, { status, regulator_ref: regulatorRef });
    return response.data.data;
  },

  // SAR
  async getSARs(params?: { status?: string; page?: number }): Promise<PaginatedResponse<SARReport>> {
    const response = await apiClient.get<PaginatedResponse<SARReport>>("/admin/regulatory/sar", { params });
    return response.data;
  },

  async createSAR(data: Partial<SARReport>): Promise<SARReport> {
    const response = await apiClient.post<ApiResponse<SARReport>>("/admin/regulatory/sar", data);
    return response.data.data;
  },

  async updateSARStatus(id: string, status: string): Promise<SARReport> {
    const response = await apiClient.put<ApiResponse<SARReport>>(`/admin/regulatory/sar/${id}/status`, { status });
    return response.data.data;
  },

  // Complaints
  async getComplaints(params?: { status?: string; page?: number }): Promise<PaginatedResponse<PlayerComplaint>> {
    const response = await apiClient.get<PaginatedResponse<PlayerComplaint>>("/admin/regulatory/complaints", { params });
    return response.data;
  },

  async createComplaint(data: Partial<PlayerComplaint>): Promise<PlayerComplaint> {
    const response = await apiClient.post<ApiResponse<PlayerComplaint>>("/admin/regulatory/complaints", data);
    return response.data.data;
  },

  async updateComplaintStatus(id: string, status: string, resolution?: string, adrRef?: string): Promise<PlayerComplaint> {
    const response = await apiClient.put<ApiResponse<PlayerComplaint>>(`/admin/regulatory/complaints/${id}/status`, { status, resolution, adr_ref: adrRef });
    return response.data.data;
  },

  // Tax Config
  async getTaxConfigs(): Promise<TaxConfig[]> {
    const response = await apiClient.get<ApiResponse<TaxConfig[]>>("/admin/regulatory/tax-config");
    return response.data.data;
  },

  async saveTaxConfig(data: Partial<TaxConfig>): Promise<TaxConfig> {
    const response = await apiClient.put<ApiResponse<TaxConfig>>("/admin/regulatory/tax-config", data);
    return response.data.data;
  },

  // Player Funds
  async getPlayerFunds(): Promise<{
    total_player_balances: string;
    funds_in_segregated: string;
    segregation_ratio: number;
    liabilities_total: string;
  }> {
    const response = await apiClient.get<ApiResponse<any>>("/admin/regulatory/player-funds");
    return response.data.data;
  },
};
