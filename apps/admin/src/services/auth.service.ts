import apiClient from "./api";
import type { LoginRequest, LoginResponse, ApiResponse } from "@/types/api";

export const authService = {
  async login(data: LoginRequest): Promise<LoginResponse> {
    const response = await apiClient.post<ApiResponse<LoginResponse>>(
      "/admin/auth/login",
      data,
    );
    return response.data.data;
  },

  async logout(): Promise<void> {
    await apiClient.post("/admin/auth/logout");
  },

  async me(): Promise<LoginResponse["admin"]> {
    const response =
      await apiClient.get<ApiResponse<LoginResponse["admin"]>>(
        "/admin/auth/me",
      );
    return response.data.data;
  },

  // NOTE: refresh is handled automatically by the API interceptor in api.ts
  // Do NOT call refresh manually — the interceptor handles token rotation
};
