import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  Deposit,
  Withdrawal,
  CursorPage,
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

  /**
   * Withdrawal review queue. Payment Service paginates with a keyset cursor,
   * so callers pass page_token/prev_token instead of a page number.
   */
  async getWithdrawals(params: {
    status?: string;
    player_id?: string;
    page_size?: number;
    page_token?: string;
    prev_token?: string;
  }): Promise<CursorPage<Withdrawal>> {
    const response = await apiClient.get<CursorPage<Withdrawal>>(
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

  /** Approves a withdrawal awaiting manual review (executes the payout). */
  async approveWithdrawal(withdrawalId: string): Promise<Withdrawal> {
    const response = await apiClient.post<ApiResponse<Withdrawal>>(
      `/admin/finance/withdrawals/${withdrawalId}/approve`,
    );
    return response.data.data;
  },

  /** Rejects a withdrawal awaiting manual review (releases the funds). */
  async rejectWithdrawal(
    withdrawalId: string,
    reason: string,
  ): Promise<Withdrawal> {
    const response = await apiClient.post<ApiResponse<Withdrawal>>(
      `/admin/finance/withdrawals/${withdrawalId}/reject`,
      { reason },
    );
    return response.data.data;
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
