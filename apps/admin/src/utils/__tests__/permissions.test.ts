import { describe, it, expect } from "vitest";
import {
  getPermissionsForRole,
  hasPermission,
  hasAnyPermission,
  hasAllPermissions,
} from "@/utils/permissions";
import type { AdminRole, Permission } from "@/types/admin";

const ALL_ROLES: AdminRole[] = [
  "SUPER_ADMIN",
  "FINANCE_MANAGER",
  "RISK_MANAGER",
  "CRM_MANAGER",
  "SPORTS_TRADER",
  "SUPPORT_AGENT",
  "KYC_OFFICER",
  "AFFILIATE_MANAGER",
  "CONTENT_MANAGER",
  "VIEWER",
  "COMPLIANCE_OFFICER",
];

describe("ROLE_PERMISSIONS matrix", () => {
  it("covers every AdminRole", () => {
    for (const role of ALL_ROLES) {
      expect(getPermissionsForRole(role).length).toBeGreaterThan(0);
    }
  });

  it("returns an empty list for an unknown role instead of throwing", () => {
    expect(getPermissionsForRole("NOT_A_ROLE" as AdminRole)).toEqual([]);
  });

  it("contains no duplicate entries within a role", () => {
    for (const role of ALL_ROLES) {
      const perms = getPermissionsForRole(role);
      expect(new Set(perms).size).toBe(perms.length);
    }
  });

  it("SUPER_ADMIN is a strict superset of every other role (least privilege holds downwards)", () => {
    const superAdmin = new Set(getPermissionsForRole("SUPER_ADMIN"));
    for (const role of ALL_ROLES) {
      if (role === "SUPER_ADMIN") continue;
      for (const permission of getPermissionsForRole(role)) {
        expect(superAdmin.has(permission)).toBe(true);
      }
    }
  });

  it("VIEWER is read-only: no write, approve, or admin permissions", () => {
    const viewer = getPermissionsForRole("VIEWER");
    expect(viewer).toEqual(["user.view", "bet.view", "reports.view"]);
    for (const permission of viewer) {
      expect(permission.endsWith(".view")).toBe(true);
    }
  });

  it("VIEWER cannot reach anything the other roles can do", () => {
    const viewer = getPermissionsForRole("VIEWER");
    for (const denied of [
      "user.edit",
      "user.block",
      "user.delete",
      "transaction.adjust",
      "withdrawal.approve_small",
      "withdrawal.approve_large",
      "bonus.grant",
      "system.config",
      "system.maintenance",
      "admin.manage",
      "audit.view",
    ] as Permission[]) {
      expect(hasPermission(viewer, denied)).toBe(false);
    }
  });

  it("FINANCE_MANAGER can approve withdrawals but cannot touch system or KYC", () => {
    const finance = getPermissionsForRole("FINANCE_MANAGER");
    expect(hasPermission(finance, "withdrawal.approve_small")).toBe(true);
    expect(hasPermission(finance, "withdrawal.approve_large")).toBe(true);
    expect(hasPermission(finance, "transaction.adjust")).toBe(true);
    expect(hasPermission(finance, "system.config")).toBe(false);
    expect(hasPermission(finance, "admin.manage")).toBe(false);
    expect(hasPermission(finance, "kyc.review")).toBe(false);
  });

  it("SUPPORT_AGENT can edit players but never block them or move money", () => {
    const support = getPermissionsForRole("SUPPORT_AGENT");
    expect(hasPermission(support, "user.edit")).toBe(true);
    expect(hasPermission(support, "user.block")).toBe(false);
    expect(hasPermission(support, "transaction.adjust")).toBe(false);
    expect(hasPermission(support, "withdrawal.approve_large")).toBe(false);
  });

  it("only SUPER_ADMIN holds admin.manage", () => {
    const holders = ALL_ROLES.filter((role) =>
      hasPermission(getPermissionsForRole(role), "admin.manage"),
    );
    expect(holders).toEqual(["SUPER_ADMIN"]);
  });

  it("only SUPER_ADMIN holds user.delete", () => {
    const holders = ALL_ROLES.filter((role) =>
      hasPermission(getPermissionsForRole(role), "user.delete"),
    );
    expect(holders).toEqual(["SUPER_ADMIN"]);
  });
});

describe("hasPermission", () => {
  it("is an exact membership check", () => {
    expect(hasPermission(["user.view"], "user.view")).toBe(true);
    expect(hasPermission(["user.view"], "user.edit")).toBe(false);
    expect(hasPermission([], "user.view")).toBe(false);
  });

  it("does not do prefix or fuzzy matching", () => {
    // "user.view" must not satisfy a hypothetical "user" check.
    expect(hasPermission(["user.view"], "user" as Permission)).toBe(false);
    expect(hasPermission(["kyc.review"], "kyc.sof_review")).toBe(false);
  });
});

describe("hasAnyPermission / hasAllPermissions", () => {
  const perms: Permission[] = ["user.view", "bet.view", "reports.view"];

  it("hasAnyPermission is true when at least one is granted", () => {
    expect(hasAnyPermission(perms, ["user.edit", "bet.view"])).toBe(true);
    expect(hasAnyPermission(perms, ["user.edit", "bet.void"])).toBe(false);
  });

  it("hasAnyPermission is false for an empty requirement list", () => {
    expect(hasAnyPermission(perms, [])).toBe(false);
  });

  it("hasAllPermissions requires every entry", () => {
    expect(hasAllPermissions(perms, ["user.view", "reports.view"])).toBe(true);
    expect(hasAllPermissions(perms, ["user.view", "user.edit"])).toBe(false);
  });

  it("hasAllPermissions is vacuously true for an empty requirement list", () => {
    expect(hasAllPermissions(perms, [])).toBe(true);
    expect(hasAllPermissions([], [])).toBe(true);
  });
});
