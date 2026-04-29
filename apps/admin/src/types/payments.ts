export type ChargebackStatus = "received" | "under_review" | "accepted" | "fighting" | "won" | "lost";
export type P2PStatus = "pending" | "confirmed" | "rejected" | "sent" | "completed";
export type P2PType = "deposit" | "withdrawal";
export type P2PMethod = "papara" | "bank_transfer" | "crypto_p2p";
export type ReconciliationStatus = "pending" | "resolved" | "investigating";

export interface Chargeback {
  id: string;
  player_id: string;
  player_email: string;
  transaction_id: string;
  amount: string;
  currency: string;
  gateway: string;
  gateway_cb_id: string | null;
  reason_code: string | null;
  reason_text: string | null;
  status: ChargebackStatus;
  received_at: string;
  deadline_at: string | null;
  resolved_at: string | null;
  assigned_to: string | null;
  assigned_to_name: string | null;
  fight_evidence: ChargebackEvidence[];
  notes: string | null;
  days_to_deadline: number | null;
}

export interface ChargebackEvidence {
  type: string;
  url: string;
  uploaded_at: string;
}

export interface ChargebackQueueFilters {
  status?: ChargebackStatus;
  gateway?: string;
  assigned_to?: string;
  page?: number;
  page_size?: number;
}

export interface ChargebackActionPayload {
  action: "accept" | "fight" | "assign";
  evidence?: ChargebackEvidence[];
  notes?: string;
}

export interface BalanceSheet {
  as_of: string;
  liabilities: {
    player_balances: string;
    bonus_balances: string;
    pending_withdrawals: string;
    total: string;
  };
  assets: {
    gateways: GatewayBalance[];
    crypto_hot: CryptoBalance[];
    crypto_cold: CryptoBalance[];
    bank_account: string;
    total: string;
  };
  coverage_ratio: number;
}

export interface GatewayBalance {
  name: string;
  balance: string;
  currency: string;
}

export interface CryptoBalance {
  coin: string;
  amount: string;
  usd_equivalent: string;
}

export interface CryptoWallet {
  id: string;
  coin: string;
  wallet_type: "hot" | "cold";
  balance: string;
  address: string;
  daily_withdrawal_avg: string;
  threshold_amount: string;
  is_low: boolean;
  pending_deposits: number;
  pending_withdrawals: number;
  last_updated: string;
}

export interface P2PTransaction {
  id: string;
  player_id: string;
  player_email: string;
  type: P2PType;
  amount: string;
  currency: string;
  method: P2PMethod;
  status: P2PStatus;
  receipt_url: string | null;
  confirmed_by: string | null;
  confirmed_by_name: string | null;
  confirmed_at: string | null;
  notes: string | null;
  created_at: string;
  hours_waiting: number;
}

export interface P2PQueueFilters {
  status?: P2PStatus;
  type?: P2PType;
  method?: P2PMethod;
  page?: number;
  page_size?: number;
}

export interface P2PActionPayload {
  action: "confirm" | "reject" | "mark_sent";
  notes?: string;
}

export interface PaymentMethodConfig {
  id: string;
  country_code: string;
  method: string;
  gateway: string;
  enabled_deposit: boolean;
  enabled_withdrawal: boolean;
  min_deposit: string;
  max_deposit: string | null;
  min_withdrawal: string;
  max_withdrawal: string | null;
  fee_percent: string;
  fee_fixed: string;
  priority: number;
  temporary_disabled_until: string | null;
}

export interface ReconciliationRecord {
  id: string;
  recon_date: string;
  gateway: string;
  expected_balance: string;
  actual_balance: string;
  difference: string;
  pending_tx_count: number;
  failed_callbacks: number;
  chargeback_amount: string;
  status: ReconciliationStatus;
  notes: string | null;
}

export interface ChargebackStats {
  total_this_month: number;
  amount_this_month: string;
  cb_rate_pct: number;
  fight_win_rate_pct: number;
}
