import apiClient from "./api";
import { useAuthStore } from "@/stores/authStore";
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
    try {
      await apiClient.post("/admin/auth/logout");
    } finally {
      useAuthStore.getState().clearAuth();
    }
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
