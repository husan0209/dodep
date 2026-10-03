import { describe, expect, it } from "vitest";

import type { AdminRole, Permission } from "../types/admin";
import {
  getPermissionsForRole,
  hasAllPermissions,
  hasAnyPermission,
  hasPermission,
} from "./permissions";

describe("getPermissionsForRole", () => {
  it("grants a read-only role only read permissions", () => {
    const viewer = getPermissionsForRole("VIEWER");
    expect(viewer).toContain("user.view");
    expect(viewer).not.toContain("withdrawal.approve_small");
    expect(viewer).not.toContain("system.config");
  });

  it("gives the finance manager withdrawal approval but not user deletion", () => {
    const finance = getPermissionsForRole("FINANCE_MANAGER");
    expect(finance).toContain("withdrawal.approve_small");
    expect(finance).toContain("withdrawal.approve_large");
    expect(finance).not.toContain("user.delete");
  });

  it("keeps every SUPER_ADMIN capability the other roles rely on", () => {
    const admin = getPermissionsForRole("SUPER_ADMIN");
    for (const permission of [
      "user.view",
      "withdrawal.approve_large",
      "bet.void",
      "kyc.review",
      "affiliate.manage",
      "system.maintenance",
    ] as Permission[]) {
      expect(admin).toContain(permission);
    }
  });

  it("returns an empty list for an unknown role", () => {
    const unknown = "NOT_A_ROLE" as AdminRole;
    expect(getPermissionsForRole(unknown)).toEqual([]);
  });
});

describe("hasPermission", () => {
  const permissions: Permission[] = ["user.view", "bet.view"];

  it("is true only for a granted permission", () => {
    expect(hasPermission(permissions, "user.view")).toBe(true);
    expect(hasPermission(permissions, "user.edit")).toBe(false);
  });

  it("is false for an empty permission list", () => {
    expect(hasPermission([], "user.view")).toBe(false);
  });
});

describe("hasAnyPermission", () => {
  const permissions: Permission[] = ["user.view", "bet.view"];

  it("is true when at least one requirement is granted", () => {
    expect(hasAnyPermission(permissions, ["bet.view", "user.edit"])).toBe(true);
  });

  it("is false when none are granted", () => {
    expect(hasAnyPermission(permissions, ["user.edit", "bet.void"])).toBe(false);
  });

  it("is false for an empty requirement list", () => {
    expect(hasAnyPermission(permissions, [])).toBe(false);
  });
});

describe("hasAllPermissions", () => {
  const permissions: Permission[] = ["user.view", "bet.view"];

  it("is true only when every requirement is granted", () => {
    expect(hasAllPermissions(permissions, ["user.view", "bet.view"])).toBe(true);
    expect(hasAllPermissions(permissions, ["user.view", "bet.void"])).toBe(false);
  });

  it("is true for an empty requirement list", () => {
    expect(hasAllPermissions(permissions, [])).toBe(true);
  });
});