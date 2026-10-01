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
/**
 * Withdrawal lifecycle as owned by Payment Service.
 * `pending_review` means the funds are reserved in the wallet and a finance
 * operator must approve or reject before the payout is sent to the provider.
 */
export type WithdrawalStatus =
  | "pending_review"
  | "approved"
  | "processing"
  | "sending"
  | "sent"
  | "finished"
  | "rejected"
  | "failed"
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
  /** Payment Service withdrawal UUID — the id used by approve/reject. */
  id: string;
  user_id: string;
  amount: string;
  currency_code: string;
  status: WithdrawalStatus;
  /** Provider (NOWPayments) payout id; empty until the payout is created. */
  psp_reference: string;
  destination: string;
  reviewed_by: string;
  rejection_reason: string;
  idempotency_key: string;
  created_at: string;
  updated_at: string;
  completed_at: string;
  approved_at: string;
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

/**
 * Cursor pagination as returned by Payment Service. Page numbers are not
 * available: the queue is read with a keyset cursor so concurrent approvals
 * cannot make rows shift between pages.
 */
export interface CursorPage<T> {
  data: T[];
  pagination: {
    next_cursor: string;
    prev_cursor: string;
    has_more: boolean;
  };
}
