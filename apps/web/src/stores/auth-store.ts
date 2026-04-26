import { create } from "zustand";
import { persist } from "zustand/middleware";
import { authApi, type AuthResult } from "@/lib/api/auth";
import { getErrorMessage } from "@/lib/api/errors";

interface User {
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

interface AuthState {
  // State
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;

  // Actions
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, username: string, countryCode: string, currencyCode: string) => Promise<void>;
  logout: () => Promise<void>;
  fetchUser: () => Promise<void>;
  clearError: () => void;
  setTokens: (accessToken: string, refreshToken: string) => void;
  refreshTokens: () => Promise<void>;
}

const handleAuthResult = (result: AuthResult) => ({
  accessToken: result.tokens.access_token,
  refreshToken: result.tokens.refresh_token,
  isAuthenticated: true,
  isLoading: false,
  error: null,
});

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      // Initial state
      user: null,
  accessToken: null,
  refreshToken: null,
  isAuthenticated: false,
  isLoading: false,
  error: null,

  setTokens: (accessToken: string, refreshToken: string) => {
    set({ accessToken, refreshToken, isAuthenticated: true });
  },

  login: async (email: string, password: string) => {
    set({ isLoading: true, error: null });
    try {
      const result = await authApi.login({ email, password });
      set(handleAuthResult(result));
    } catch (error) {
      set({
        isLoading: false,
        error: getErrorMessage(error),
      });
      throw error;
    }
  },

  register: async (email: string, password: string, username: string, countryCode: string, currencyCode: string) => {
    set({ isLoading: true, error: null });
    try {
      const result = await authApi.register({ email, password, username, country_code: countryCode, currency_code: currencyCode });
      set(handleAuthResult(result));
    } catch (error) {
      set({
        isLoading: false,
        error: getErrorMessage(error),
      });
      throw error;
    }
  },

  logout: async () => {
    try {
      await authApi.logout();
    } catch (error) {
      console.error("Logout error:", error);
    } finally {
      set({
        user: null,
        accessToken: null,
        refreshToken: null,
        isAuthenticated: false,
        error: null,
      });
    }
  },

  fetchUser: async () => {
    const { accessToken } = get();
    if (!accessToken) {
      set({ isAuthenticated: false });
      return;
    }

    set({ isLoading: true });
    try {
      const user = await authApi.me();
      set({
        user,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (error) {
      set({
        isAuthenticated: false,
        user: null,
        isLoading: false,
      });
    }
  },

  refreshTokens: async () => {
    const { refreshToken } = get();
    if (!refreshToken) {
      throw new Error("No refresh token");
    }

    try {
      const tokens = await authApi.refresh(refreshToken);
      set({
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
      });
    } catch (error) {
      set({
        accessToken: null,
        refreshToken: null,
        isAuthenticated: false,
      });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
    }),
    {
      name: "auth-storage",
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);
