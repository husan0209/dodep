import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { AdminRole, Permission } from "@/types/admin";

interface AuthState {
  // Runtime only (NOT persisted)
  // Phase 0.5: refreshToken stays in memory as a one-release fallback while
  // the body field is being deprecated. The HttpOnly cookie is the source of
  // truth for refresh — JS never reads it. Do NOT add refreshToken to
  // partialize() or onRehydrateStorage.
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;

  // Persisted (UX only: remember admin profile across reloads).
  // The refresh cookie restores the actual session via the API interceptor.
  adminId: string | null;
  adminEmail: string | null;
  adminName: string | null;
  adminRole: AdminRole | null;
  permissions: Permission[];

  setTokens: (accessToken: string, refreshToken?: string | null) => void;
  setAdmin: (
    id: string,
    email: string,
    name: string,
    role: AdminRole,
    permissions: Permission[],
  ) => void;
  clearAuth: () => void;
  getAccessToken: () => string | null;
  restoreFromStorage: () => void;
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
      
      restoreFromStorage: () => {
        // Phase 0.5: session is restored via the HttpOnly refresh cookie.
        // If we have a remembered adminId, optimistically mark the user as
        // authenticated. The first API call will trigger a refresh through
        // the cookie; if it fails, the interceptor clears auth and redirects.
        const state = get();
        if (state.adminId) {
          set({ isAuthenticated: true });
        }
      },
    }),
    {
      name: "admin-auth-storage",
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        // Phase 0.5: refreshToken intentionally NOT persisted — it lives in
        // an HttpOnly cookie. accessToken & isAuthenticated stay in memory.
        adminId: state.adminId,
        adminEmail: state.adminEmail,
        adminName: state.adminName,
        adminRole: state.adminRole,
        permissions: state.permissions,
      }),
      onRehydrateStorage: () => (state) => {
        if (!state) return;
        state.accessToken = null;
        state.refreshToken = null;
        // If profile fields survived from localStorage, treat the user as
        // optimistically authenticated; the refresh interceptor will resolve
        // the real status on the first request.
        state.isAuthenticated = Boolean(state.adminId);
      },
    },
  ),
);
