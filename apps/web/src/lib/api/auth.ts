import { api } from "./client";

export interface LoginRequest {
  email: string;
  password: string;
  device_id?: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  username: string;
  country_code: string;
  currency_code: string;
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
  country: string;
  currency: string;
  kyc_level: number;
  status: "pending" | "active" | "blocked" | "suspended";
  created_at: string;
  updated_at: string;
  last_login_at?: string;
}

export const authApi = {
  login: (data: LoginRequest) =>
    api.post<AuthResult>("/api/v1/auth/login", data),

  register: (data: RegisterRequest) =>
    api.post<AuthResult>("/api/v1/auth/register", data),

  logout: () => api.post("/api/v1/auth/logout"),

  me: () => api.get<User>("/api/v1/auth/me"),

  refresh: (refreshToken: string) =>
    api.post<TokenPair>("/api/v1/auth/refresh", { refresh_token: refreshToken }),
};
