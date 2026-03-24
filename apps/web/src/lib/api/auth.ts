import { api } from "./client";

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  username: string;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

export interface User {
  id: number;
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
    api.post<AuthTokens & { user: User }>("/api/v1/auth/login", data).then((r) => r.data),

  register: (data: RegisterRequest) =>
    api.post<AuthTokens & { user: User }>("/api/v1/auth/register", data).then((r) => r.data),

  logout: () => api.post("/api/v1/auth/logout"),

  me: () => api.get<User>("/api/v1/auth/me").then((r) => r.data),

  refresh: (refreshToken: string) =>
    api.post<AuthTokens>("/api/v1/auth/refresh", { refresh_token: refreshToken }).then((r) => r.data),
};
