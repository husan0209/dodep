import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AdminRole, Permission } from "@/types/admin";

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  adminId: string | null;
  adminEmail: string | null;
  adminName: string | null;
  adminRole: AdminRole | null;
  permissions: Permission[];
  isAuthenticated: boolean;

  setTokens: (accessToken: string, refreshToken: string) => void;
  setAdmin: (
    id: string,
    email: string,
    name: string,
    role: AdminRole,
    permissions: Permission[],
  ) => void;
  clearAuth: () => void;
  getAccessToken: () => string | null;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      adminId: null,
      adminEmail: null,
      adminName: null,
      adminRole: null,
      permissions: [],
      isAuthenticated: false,

      setTokens: (accessToken, refreshToken) =>
        set({ accessToken, refreshToken, isAuthenticated: true }),

      setAdmin: (id, email, name, role, permissions) =>
        set({
          adminId: id,
          adminEmail: email,
          adminName: name,
          adminRole: role,
          permissions,
        }),

      clearAuth: () =>
        set({
          accessToken: null,
          refreshToken: null,
          adminId: null,
          adminEmail: null,
          adminName: null,
          adminRole: null,
          permissions: [],
          isAuthenticated: false,
        }),

      getAccessToken: () => get().accessToken,
    }),
    {
      name: "admin-auth-storage",
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        adminId: state.adminId,
        adminEmail: state.adminEmail,
        adminName: state.adminName,
        adminRole: state.adminRole,
        permissions: state.permissions,
        isAuthenticated: state.isAuthenticated,
      }),
    },
  ),
);
