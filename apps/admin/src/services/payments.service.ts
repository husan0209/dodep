import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  Chargeback,
  ChargebackQueueFilters,
  ChargebackActionPayload,
  ChargebackStats,
  BalanceSheet,
  CryptoWallet,
  P2PTransaction,
  P2PQueueFilters,
  P2PActionPayload,
  PaymentMethodConfig,
  ReconciliationRecord,
} from "@/types/payments";

export const paymentsService = {
  // Chargebacks
  async getChargebacks(
    filters: ChargebackQueueFilters,
  ): Promise<PaginatedResponse<Chargeback>> {
    const response = await apiClient.get<PaginatedResponse<Chargeback>>(
      "/admin/payments/chargebacks",
      { params: filters },
    );
    return response.data;
  },

  async getChargeback(id: string): Promise<Chargeback> {
    const response = await apiClient.get<ApiResponse<Chargeback>>(
      `/admin/payments/chargebacks/${id}`,
    );
    return response.data.data;
  },

  async actionChargeback(
    id: string,
    payload: ChargebackActionPayload,
  ): Promise<void> {
    if (payload.action === "assign") {
      await apiClient.put(`/admin/payments/chargebacks/${id}/assign`, {
        assigned_to: "admin-id",
        ...payload,
      });
    } else if (payload.action === "fight") {
      await apiClient.post(`/admin/payments/chargebacks/${id}/fight`, payload);
    } else if (payload.action === "accept") {
      await apiClient.post(`/admin/payments/chargebacks/${id}/accept`, payload);
    }
  },

  async getChargebackStats(): Promise<ChargebackStats> {
    const response = await apiClient.get<ApiResponse<ChargebackStats>>(
      "/admin/payments/chargebacks/stats",
    );
    return response.data.data;
  },

  // Balance Sheet
  async getBalanceSheet(): Promise<BalanceSheet> {
    const response = await apiClient.get<ApiResponse<BalanceSheet>>(
      "/admin/payments/balance-sheet",
    );
    return response.data.data;
  },

  // Crypto Wallets
  async getCryptoWallets(): Promise<CryptoWallet[]> {
    const response = await apiClient.get<ApiResponse<CryptoWallet[]>>(
      "/admin/payments/crypto-wallets",
    );
    return response.data.data;
  },

  async refreshCryptoWallet(id: string): Promise<void> {
    await apiClient.post(`/admin/payments/crypto-wallets/${id}/refresh`);
  },

  // P2P Transactions
  async getP2PTransactions(
    filters: P2PQueueFilters,
  ): Promise<PaginatedResponse<P2PTransaction>> {
    const response = await apiClient.get<PaginatedResponse<P2PTransaction>>(
      "/admin/payments/p2p",
      { params: filters },
    );
    return response.data;
  },

  async actionP2P(id: string, payload: P2PActionPayload): Promise<void> {
    if (payload.action === "confirm") {
      await apiClient.post(`/admin/payments/p2p/${id}/confirm`, payload);
    } else if (payload.action === "reject") {
      await apiClient.post(`/admin/payments/p2p/${id}/reject`, payload);
    } else if (payload.action === "mark_sent") {
      await apiClient.post(`/admin/payments/p2p/${id}/mark-sent`, payload);
    }
  },

  // Payment Method Config
  async getPaymentConfigs(
    params?: { country?: string },
  ): Promise<PaymentMethodConfig[]> {
    const response = await apiClient.get<ApiResponse<PaymentMethodConfig[]>>(
      "/admin/payments/method-configs",
      { params },
    );
    return response.data.data;
  },

  async updatePaymentConfig(
    id: string,
    payload: Partial<PaymentMethodConfig>,
  ): Promise<PaymentMethodConfig> {
    const response = await apiClient.put<ApiResponse<PaymentMethodConfig>>(
      `/admin/payments/method-configs/${id}`,
      payload,
    );
    return response.data.data;
  },

  // Reconciliation
  async getReconciliation(
    params?: { date?: string; page?: number },
  ): Promise<PaginatedResponse<ReconciliationRecord>> {
    const response = await apiClient.get<PaginatedResponse<ReconciliationRecord>>(
      "/admin/payments/reconciliation",
      { params },
    );
    return response.data;
  },

  async runReconciliation(date: string): Promise<void> {
    await apiClient.post("/admin/payments/reconciliation/run", { date });
  },

  async getFinancialReport(params: { from?: string; to?: string; type?: string }): Promise<unknown> {
    const response = await apiClient.get("/admin/payments/financial-report", { params });
    return response.data.data;
  },

  async getComplianceReport(params: { from?: string; to?: string; type?: string }): Promise<unknown> {
    const response = await apiClient.get("/admin/payments/compliance-report", { params });
    return response.data.data;
  },
};
