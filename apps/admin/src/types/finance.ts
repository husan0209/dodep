export type TransactionType =
  | "deposit"
  | "withdrawal"
  | "bet_place"
  | "bet_win"
  | "bet_refund"
  | "bonus_credit"
  | "bonus_wager"
  | "adjustment";
export type TransactionStatus =
  | "pending"
  | "processing"
  | "completed"
  | "failed"
  | "cancelled"
  | "requires_review";
export type DepositMethod =
  | "card"
  | "bank_transfer"
  | "e_wallet"
  | "crypto"
  | "local";
export type WithdrawalStatus =
  | "pending"
  | "approved"
  | "processing"
  | "completed"
  | "rejected"
  | "cancelled";

export interface Transaction {
  id: string;
  user_id: string;
  wallet_id: string;
  type: TransactionType;
  amount: string;
  currency_code: string;
  balance_before: string;
  balance_after: string;
  reference_type: string | null;
  reference_id: string | null;
  idempotency_key: string;
  status: TransactionStatus;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface Deposit {
  id: string;
  user_id: string;
  amount: string;
  currency_code: string;
  method: DepositMethod;
  provider: string;
  status: TransactionStatus;
  psp_reference: string | null;
  created_at: string;
  completed_at: string | null;
}

export interface Withdrawal {
  id: string;
  user_id: string;
  amount: string;
  currency_code: string;
  method: DepositMethod;
  destination: string;
  status: WithdrawalStatus;
  psp_reference: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  rejection_reason: string | null;
  created_at: string;
  completed_at: string | null;
}

export interface WalletBalance {
  user_id: string;
  currency_code: string;
  available: string;
  locked: string;
  bonus: string;
  total: string;
}

export interface FinanceSearchParams {
  user_id?: string;
  status?: string;
  type?: string;
  method?: string;
  amount_min?: string;
  amount_max?: string;
  created_from?: string;
  created_to?: string;
  page?: number;
  page_size?: number;
}
