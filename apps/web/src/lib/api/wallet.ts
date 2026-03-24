import { api } from "./client";

export interface WalletBalance {
  user_id: number;
  currency: string;
  available: string;
  locked: string;
  total: string;
}

export interface Transaction {
  id: number;
  user_id: number;
  wallet_id: number;
  type: "deposit" | "withdrawal" | "bet_place" | "bet_win" | "bet_refund" | "bonus" | "adjustment";
  amount: string;
  balance_before: string;
  balance_after: string;
  reference_type?: string;
  reference_id?: number;
  status: "pending" | "completed" | "failed";
  created_at: string;
  metadata?: Record<string, unknown>;
}

export interface DepositRequest {
  amount: number;
  method: string;
  currency?: string;
}

export interface WithdrawRequest {
  amount: number;
  method: string;
  currency?: string;
}

export interface TransactionFilters {
  type?: string;
  status?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export const walletApi = {
  getBalances: () =>
    api.get<WalletBalance[]>("/api/v1/wallet/balances").then((r) => r.data),

  getBalance: (currency: string) =>
    api.get<WalletBalance>(`/api/v1/wallet/balances/${currency}`).then((r) => r.data),

  getTransactions: (filters?: TransactionFilters) =>
    api.get<Transaction[]>("/api/v1/wallet/transactions", filters as Record<string, string>).then((r) => r.data),

  deposit: (data: DepositRequest) =>
    api.post<{ deposit_id: string; url?: string }>("/api/v1/wallet/deposit", data).then((r) => r.data),

  withdraw: (data: WithdrawRequest) =>
    api.post<{ withdrawal_id: string }>("/api/v1/wallet/withdraw", data).then((r) => r.data),
};
