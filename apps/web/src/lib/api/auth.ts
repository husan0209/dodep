import { authApiClient, resolveAuthApiBaseUrl } from "./client";

export interface LoginRequest {
  identifier?: string;
  username?: string;
  email?: string;
  password: string;
  device_id?: string;
  deviceId?: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  username: string;
  country_code: string;
  countryCode?: string;
  currency_code: string;
  currencyCode?: string;
  device_id?: string;
  deviceId?: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  refresh_expires_in: number;
  token_type: string;
}

export interface Session {
  id: string;
  user_id: string;
  device_id: string;
  ip_address: string;
  country: string;
  created_at: string;
  expires_at: string;
  is_active: boolean;
}

export interface AuthResult {
  user_id: string;
  tokens: TokenPair;
  session: Session;
  requires_2fa: boolean;
  temp_token?: string;
}

export interface User {
  id: string;
  uuid: string;
  email: string;
  username: string;
  phone?: string;
  country_code: string;
  currency_code: string;
  kyc_level: number;
  status: "pending" | "active" | "blocked" | "suspended" | "self_excluded" | "closed";
  created_at: string;
  updated_at: string;
  last_login_at?: string;
}

export const authApi = {
  login: (data: LoginRequest) =>
    authApiClient.post<AuthResult>("/api/v1/auth/login", data),

  register: (data: RegisterRequest) =>
    authApiClient.post<AuthResult>("/api/v1/auth/register", data),

  logout: () => authApiClient.post("/api/v1/auth/logout"),

  me: () => authApiClient.get<User>("/api/v1/auth/me"),

  refresh: (refreshToken: string) =>
    authApiClient.post<TokenPair>("/api/v1/auth/refresh", { refresh_token: refreshToken }),

  getGoogleStartUrl: () =>
    `${resolveAuthApiBaseUrl()}/api/v1/auth/google/start`,
};
