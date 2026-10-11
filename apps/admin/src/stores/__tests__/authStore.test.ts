import { describe, it, expect, beforeEach, vi } from "vitest";
import type { useAuthStore } from "@/stores/authStore";

type AuthStore = typeof useAuthStore;

const STORAGE_KEY = "admin-auth-storage";

/**
 * The store is created at module scope and its persist middleware rehydrates
 * during module evaluation, so every test needs a pristine module instance
 * plus an empty localStorage.
 */
async function loadStore(): Promise<AuthStore> {
  vi.resetModules();
  const mod = await import("@/stores/authStore");
  return mod.useAuthStore;
}

function readPersisted(): { state?: Record<string, unknown>; version?: number } {
  const raw = localStorage.getItem(STORAGE_KEY);
  expect(raw).not.toBeNull();
  return JSON.parse(raw as string) as {
    state?: Record<string, unknown>;
    version?: number;
  };
}

beforeEach(() => {
  localStorage.clear();
  vi.resetModules();
});

describe("authStore initial state", () => {
  it("starts logged out with an empty permission list", async () => {
    const store = await loadStore();
    const state = store.getState();

    expect(state.accessToken).toBeNull();
    expect(state.refreshToken).toBeNull();
    expect(state.isAuthenticated).toBe(false);
    expect(state.adminId).toBeNull();
    expect(state.adminEmail).toBeNull();
    expect(state.adminRole).toBeNull();
    expect(state.permissions).toEqual([]);
  });
});

describe("authStore transitions", () => {
  it("setTokens stores the access token and flips isAuthenticated", async () => {
    const store = await loadStore();

    store.getState().setTokens("access-123", "refresh-456");

    const state = store.getState();
    expect(state.accessToken).toBe("access-123");
    expect(state.refreshToken).toBe("refresh-456");
    expect(state.isAuthenticated).toBe(true);
    expect(state.getAccessToken()).toBe("access-123");
  });

  it("setTokens without a refresh token still authenticates", async () => {
    const store = await loadStore();

    store.getState().setTokens("access-123");

    const state = store.getState();
    expect(state.accessToken).toBe("access-123");
    expect(state.isAuthenticated).toBe(true);
    // No refresh token was supplied, so none may be remembered.
    expect(state.refreshToken).toBeFalsy();
  });

  it("KNOWN WART: setTokens writes `undefined`, not `null`, into refreshToken", async () => {
    // AuthState declares `refreshToken: string | null`, but `set({ refreshToken })`
    // with an omitted second argument stores `undefined`, so the runtime value
    // escapes the declared type. Pinned so the eventual fix is a visible change.
    const store = await loadStore();

    store.getState().setTokens("access-123");

    expect(store.getState().refreshToken).toBeUndefined();
    expect(store.getState().refreshToken).not.toBeNull();
  });

  it("setAdmin records the profile without authenticating on its own", async () => {
    const store = await loadStore();

    store
      .getState()
      .setAdmin(
        "admin-1",
        "boss@example.com",
        "The Boss",
        "FINANCE_MANAGER",
        ["transaction.view", "reports.view"],
      );

    const state = store.getState();
    expect(state.adminId).toBe("admin-1");
    expect(state.adminEmail).toBe("boss@example.com");
    expect(state.adminName).toBe("The Boss");
    expect(state.adminRole).toBe("FINANCE_MANAGER");
    expect(state.permissions).toEqual(["transaction.view", "reports.view"]);
    // A remembered profile alone must not unlock the UI.
    expect(state.isAuthenticated).toBe(false);
    expect(state.accessToken).toBeNull();
  });

  it("clearAuth wipes tokens, profile and permissions", async () => {
    const store = await loadStore();
    const { setTokens, setAdmin, clearAuth } = store.getState();
    setTokens("access-123", "refresh-456");
    setAdmin("admin-1", "boss@example.com", "Boss", "SUPER_ADMIN", ["user.view"]);

    clearAuth();

    const state = store.getState();
    expect(state.accessToken).toBeNull();
    expect(state.refreshToken).toBeNull();
    expect(state.adminId).toBeNull();
    expect(state.adminEmail).toBeNull();
    expect(state.adminName).toBeNull();
    expect(state.adminRole).toBeNull();
    expect(state.permissions).toEqual([]);
    expect(state.isAuthenticated).toBe(false);
    expect(state.getAccessToken()).toBeNull();
  });
});

describe("authStore persistence", () => {
  it("never writes tokens or isAuthenticated to localStorage", async () => {
    const store = await loadStore();
    const { setTokens, setAdmin } = store.getState();
    setTokens("super-secret-access", "super-secret-refresh");
    setAdmin("admin-1", "boss@example.com", "Boss", "SUPER_ADMIN", ["user.view"]);

    const raw = localStorage.getItem(STORAGE_KEY);
    expect(raw).not.toBeNull();
    expect(raw).not.toContain("super-secret");

    expect(readPersisted()).toEqual({
      state: {
        adminId: "admin-1",
        adminEmail: "boss@example.com",
        adminName: "Boss",
        adminRole: "SUPER_ADMIN",
        permissions: ["user.view"],
      },
      version: 0,
    });
  });

  it("clearAuth also purges the persisted profile", async () => {
    const store = await loadStore();
    const { setAdmin, clearAuth } = store.getState();
    setAdmin("admin-1", "boss@example.com", "Boss", "VIEWER", []);
    expect(readPersisted().state).toMatchObject({ adminId: "admin-1" });

    clearAuth();

    expect(readPersisted().state).toMatchObject({
      adminId: null,
      adminEmail: null,
      adminRole: null,
      permissions: [],
    });
  });

  it("rehydrates the remembered profile on import and authenticates optimistically", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          adminId: "admin-9",
          adminEmail: "kyc@example.com",
          adminName: "KYC Officer",
          adminRole: "KYC_OFFICER",
          permissions: ["kyc.review", "kyc.sof_review"],
        },
        version: 0,
      }),
    );

    const state = (await loadStore()).getState();

    expect(state.adminId).toBe("admin-9");
    expect(state.adminEmail).toBe("kyc@example.com");
    expect(state.adminRole).toBe("KYC_OFFICER");
    expect(state.permissions).toEqual(["kyc.review", "kyc.sof_review"]);
    expect(state.isAuthenticated).toBe(true);
    // Tokens are memory-only and must never come back from disk.
    expect(state.accessToken).toBeNull();
    expect(state.refreshToken).toBeNull();
  });

  it("stays logged out when localStorage holds no remembered admin", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          adminId: null,
          adminEmail: null,
          adminName: null,
          adminRole: null,
          permissions: [],
        },
        version: 0,
      }),
    );

    const state = (await loadStore()).getState();

    expect(state.isAuthenticated).toBe(false);
    expect(state.permissions).toEqual([]);
    expect(state.adminId).toBeNull();
  });

  it("ignores a refreshToken smuggled into localStorage", async () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          adminId: "admin-9",
          adminEmail: "attacker@example.com",
          adminName: "Mallory",
          adminRole: "SUPER_ADMIN",
          permissions: ["admin.manage"],
          accessToken: "stolen-access-token",
          refreshToken: "stolen-refresh-token",
        },
        version: 0,
      }),
    );

    const state = (await loadStore()).getState();

    expect(state.accessToken).toBeNull();
    expect(state.refreshToken).toBeNull();
    expect(state.getAccessToken()).toBeNull();
  });
});

describe("authStore.restoreFromStorage", () => {
  it("authenticates when a remembered adminId exists, without inventing a token", async () => {
    const store = await loadStore();
    store.getState().setAdmin("admin-1", "boss@example.com", "Boss", "VIEWER", []);

    store.getState().restoreFromStorage();

    expect(store.getState().isAuthenticated).toBe(true);
    expect(store.getState().accessToken).toBeNull();
  });

  it("does nothing when there is no remembered admin", async () => {
    const store = await loadStore();

    store.getState().restoreFromStorage();

    const state = store.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.adminId).toBeNull();
  });
});
