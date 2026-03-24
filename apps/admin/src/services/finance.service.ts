import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  Deposit,
  Withdrawal,
  Transaction,
  FinanceSearchParams,
} from "@/types/finance";

export const financeService = {
  async getDeposits(
    params: FinanceSearchParams,
  ): Promise<PaginatedResponse<Deposit>> {
    const response = await apiClient.get<PaginatedResponse<Deposit>>(
      "/admin/finance/deposits",
      { params },
    );
    return response.data;
  },

  async getDeposit(depositId: string): Promise<Deposit> {
    const response = await apiClient.get<ApiResponse<Deposit>>(
      `/admin/finance/deposits/${depositId}`,
    );
    return response.data.data;
  },

  async getWithdrawals(
    params: FinanceSearchParams,
  ): Promise<PaginatedResponse<Withdrawal>> {
    const response = await apiClient.get<PaginatedResponse<Withdrawal>>(
      "/admin/finance/withdrawals",
      { params },
    );
    return response.data;
  },

  async getWithdrawal(withdrawalId: string): Promise<Withdrawal> {
    const response = await apiClient.get<ApiResponse<Withdrawal>>(
      `/admin/finance/withdrawals/${withdrawalId}`,
    );
    return response.data.data;
  },

  async approveWithdrawal(withdrawalId: string): Promise<void> {
    await apiClient.post(`/admin/finance/withdrawals/${withdrawalId}/approve`);
  },

  async rejectWithdrawal(withdrawalId: string, reason: string): Promise<void> {
    await apiClient.post(`/admin/finance/withdrawals/${withdrawalId}/reject`, {
      reason,
    });
  },

  async getTransactions(
    params: FinanceSearchParams,
  ): Promise<PaginatedResponse<Transaction>> {
    const response = await apiClient.get<PaginatedResponse<Transaction>>(
      "/admin/finance/transactions",
      { params },
    );
    return response.data;
  },

  async adjustBalance(
    userId: string,
    data: {
      amount: string;
      currency: string;
      reason: string;
      type: "credit" | "debit";
    },
  ): Promise<void> {
    await apiClient.post(`/admin/finance/users/${userId}/adjust-balance`, data);
  },

  async getFinancialSummary(params?: {
    date_from?: string;
    date_to?: string;
  }): Promise<{
    total_deposits: string;
    total_withdrawals: string;
    net_revenue: string;
    ggr: string;
    pending_withdrawals_count: number;
    pending_withdrawals_amount: string;
  }> {
    const response = await apiClient.get("/admin/finance/summary", { params });
    return response.data.data;
  },
};
