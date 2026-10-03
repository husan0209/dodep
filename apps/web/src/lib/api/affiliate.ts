import { api } from "./client";

// Backend (Go/Fiber + encoding/json without json tags) currently serializes
// domain structs with PascalCase keys (e.g. `EarningsToday`), while the
// documented contract and older payloads use snake_case (e.g.
// `earnings_today`). Normalize both shapes so the UI does not silently fall
// back to mock data when the backend returns PascalCase.

type UnknownRecord = Record<string, unknown>;

function toSnakeKey(key: string): string {
  // PascalCase / camelCase -> snake_case; leave existing snake_case untouched.
  return key
    .replace(/([A-Z])/g, (m) => `_${m.toLowerCase()}`)
    .replace(/^_/, "")
    .toLowerCase();
}

export function normalizeKeys<T>(input: T): T {
  if (Array.isArray(input)) {
    return input.map((v) => normalizeKeys(v)) as unknown as T;
  }
  if (input && typeof input === "object") {
    const out: UnknownRecord = {};
    for (const [k, v] of Object.entries(input as UnknownRecord)) {
      out[toSnakeKey(k)] = normalizeKeys(v);
    }
    return out as unknown as T;
  }
  return input;
}

function pick<T>(obj: UnknownRecord, ...keys: string[]): T | undefined {
  for (const k of keys) {
    if (obj[k] !== undefined && obj[k] !== null) return obj[k] as T;
  }
  // Fall back to snake-normalized lookup.
  const norm = normalizeKeys(obj);
  for (const k of keys) {
    const sk = toSnakeKey(k);
    if (norm[sk] !== undefined && norm[sk] !== null) return norm[sk] as T;
  }
  return undefined;
}

export interface AffiliateProfile {
  id?: string;
  user_id?: number;
  status?: string;
  affiliate_code?: string;
  commission_rate?: string;
  hold_period_days?: number;
  min_payout_amount?: string;
  currency?: string;
  payout_schedule?: string;
  kyc_required?: boolean;
}

export interface AffiliateDashboard {
  earnings_today?: string;
  earnings_this_month?: string;
  pending_amount?: string;
  available_amount?: string;
  paid_amount?: string;
  next_payout_date?: string | null;
  clicks?: number;
  registrations?: number;
  ftd_count?: number;
  active_players?: number;
  ggr_amount?: string;
  ngr_amount?: string;
  commission_amount?: string;
  currency?: string;
}

export interface AffiliateLink {
  id?: string;
  campaign_name?: string;
  landing_page?: string;
  referral_code?: string;
  referral_url?: string;
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
  is_active?: boolean;
  created_at?: string;
}

export interface AffiliateEarning {
  id?: string;
  period_start?: string;
  period_end?: string;
  created_at?: string;
  ggr_amount?: string;
  ngr_amount?: string;
  commission_amount?: string;
  status?: string;
}

export interface AffiliatePayout {
  id?: string;
  amount?: string;
  currency?: string;
  status?: string;
  requested_at?: string;
  created_at?: string;
  method_id?: string;
}

export interface PayoutMethod {
  id?: string;
  method_type?: string;
  display_name?: string;
  details_masked?: string;
  is_default?: boolean;
  is_verified?: boolean;
}

export function normalizeDashboard(raw: unknown): AffiliateDashboard {
  return normalizeKeys<AffiliateDashboard>((raw as UnknownRecord) ?? {});
}

export function normalizeProfile(raw: unknown): AffiliateProfile {
  return normalizeKeys<AffiliateProfile>((raw as UnknownRecord) ?? {});
}

export function normalizeLink(raw: unknown): AffiliateLink {
  const obj = normalizeKeys<UnknownRecord>((raw as UnknownRecord) ?? {});
  const code = pick<string>(obj, "referral_code", "code") ?? "";
  // Backend AffiliateLink has ReferralURL; older payloads used referral_url.
  const url =
    pick<string>(obj, "referral_url", "url") ?? (code ? `/r/${code}` : "");
  return { ...(obj as AffiliateLink), referral_code: code, referral_url: url };
}

export function normalizeEarning(raw: unknown): AffiliateEarning {
  return normalizeKeys<AffiliateEarning>((raw as UnknownRecord) ?? {});
}

export function normalizePayout(raw: unknown): AffiliatePayout {
  return normalizeKeys<AffiliatePayout>((raw as UnknownRecord) ?? {});
}

export function normalizePayoutMethod(raw: unknown): PayoutMethod {
  return normalizeKeys<PayoutMethod>((raw as UnknownRecord) ?? {});
}

function asArray<T>(payload: unknown, ...keys: string[]): T[] {
  if (Array.isArray(payload)) return payload as T[];
  if (payload && typeof payload === "object") {
    const obj = payload as UnknownRecord;
    for (const k of keys) {
      const v = obj[k] ?? obj[toSnakeKey(k)];
      if (Array.isArray(v)) return v as T[];
    }
    // Paginated shape: { data: [...] }
    if (Array.isArray(obj["data"])) return obj["data"] as T[];
  }
  return [];
}

export const affiliateApi = {
  getProfile: async (): Promise<AffiliateProfile> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/profile");
    const data = (raw as UnknownRecord)?.["data"] ?? raw;
    return normalizeProfile(data);
  },

  enroll: (reason?: string) =>
    api.post<unknown>("/api/v1/affiliate/enroll", reason ? { reason } : {}),

  getDashboard: async (): Promise<AffiliateDashboard> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/dashboard");
    const data = (raw as UnknownRecord)?.["data"] ?? raw;
    return normalizeDashboard(data);
  },

  listLinks: async (): Promise<AffiliateLink[]> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/links");
    return asArray<unknown>(raw, "links").map(normalizeLink);
  },

  createLink: (input: {
    campaign_name: string;
    landing_page?: string;
    utm_source?: string;
    utm_medium?: string;
    utm_campaign?: string;
  }) => api.post<unknown>("/api/v1/affiliate/links", input),

  listEarnings: async (params?: {
    status?: string;
    limit?: number;
  }): Promise<AffiliateEarning[]> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/earnings", {
      ...(params?.status ? { status: params.status } : {}),
      ...(params?.limit ? { limit: String(params.limit) } : {}),
    });
    return asArray<unknown>(raw, "earnings").map(normalizeEarning);
  },

  listPayouts: async (): Promise<AffiliatePayout[]> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/payouts");
    return asArray<unknown>(raw, "payouts").map(normalizePayout);
  },

  listPayoutMethods: async (): Promise<PayoutMethod[]> => {
    const raw = await api.get<unknown>("/api/v1/affiliate/payout-methods");
    return asArray<unknown>(raw, "methods").map(normalizePayoutMethod);
  },

  requestPayout: (input: {
    method_id: string;
    amount: string;
    idempotency_key?: string;
  }) =>
    api.post<unknown>("/api/v1/affiliate/payouts/request", {
      ...input,
      idempotency_key: input.idempotency_key ?? crypto.randomUUID(),
    }),
};
