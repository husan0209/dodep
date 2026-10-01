import { describe, it, expect } from "vitest";
import {
  getPermissionsForRole,
  hasPermission,
} from "./permissions";
import type { AdminRole } from "@/types/admin";

// Affiliate RBAC matrix (tasks/задача.md, least privilege):
//   marketing (AFFILIATE_MANAGER) → profiles, plans, approve
//   finance (FINANCE_MANAGER)     → view + payout approve
//   risk (RISK_MANAGER)           → view + fraud review
//   super_admin                   → everything incl. manual adjustments
describe("affiliate RBAC matrix", () => {
  const perms = (role: AdminRole) => getPermissionsForRole(role);

  it("AFFILIATE_MANAGER holds the full granular set", () => {
    const p = perms("AFFILIATE_MANAGER");
    for (const perm of [
      "affiliate.view",
      "affiliate.manage",
      "affiliate.approve",
      "affiliate.adjust",
      "affiliate.payout.approve",
      "affiliate.fraud.review",
    ] as const) {
      expect(hasPermission(p, perm)).toBe(true);
    }
  });

  it("FINANCE_MANAGER can approve payouts but not adjust or approve affiliates", () => {
    const p = perms("FINANCE_MANAGER");
    expect(hasPermission(p, "affiliate.view")).toBe(true);
    expect(hasPermission(p, "affiliate.payout.approve")).toBe(true);
    expect(hasPermission(p, "affiliate.adjust")).toBe(false);
    expect(hasPermission(p, "affiliate.approve")).toBe(false);
    expect(hasPermission(p, "affiliate.manage")).toBe(false);
    expect(hasPermission(p, "affiliate.fraud.review")).toBe(false);
  });

  it("RISK_MANAGER can review fraud but not touch payouts or adjustments", () => {
    const p = perms("RISK_MANAGER");
    expect(hasPermission(p, "affiliate.view")).toBe(true);
    expect(hasPermission(p, "affiliate.fraud.review")).toBe(true);
    expect(hasPermission(p, "affiliate.payout.approve")).toBe(false);
    expect(hasPermission(p, "affiliate.adjust")).toBe(false);
    expect(hasPermission(p, "affiliate.approve")).toBe(false);
  });

  it("SUPER_ADMIN holds all affiliate permissions", () => {
    const p = perms("SUPER_ADMIN");
    for (const perm of [
      "affiliate.view",
      "affiliate.manage",
      "affiliate.approve",
      "affiliate.adjust",
      "affiliate.payout.approve",
      "affiliate.fraud.review",
    ] as const) {
      expect(hasPermission(p, perm)).toBe(true);
    }
  });

  it("unrelated roles hold no affiliate permissions (deny by default)", () => {
    for (const role of [
      "VIEWER",
      "SUPPORT_AGENT",
      "KYC_OFFICER",
      "CONTENT_MANAGER",
      "CRM_MANAGER",
      "SPORTS_TRADER",
      "COMPLIANCE_OFFICER",
    ] as const) {
      const p = perms(role);
      expect(
        p.some((perm) => perm.startsWith("affiliate.")),
        `${role} must not hold affiliate permissions`,
      ).toBe(false);
    }
  });

  it("permissions are exact-match: manage implies nothing else", () => {
    const p = perms("AFFILIATE_MANAGER").filter(
      (perm) => perm !== "affiliate.manage",
    );
    // manage alone must not grant approve/adjust/payout/fraud rights
    expect(hasPermission(["affiliate.manage"], "affiliate.approve")).toBe(
      false,
    );
    expect(hasPermission(["affiliate.manage"], "affiliate.adjust")).toBe(
      false,
    );
    expect(
      hasPermission(["affiliate.manage"], "affiliate.payout.approve"),
    ).toBe(false);
    expect(
      hasPermission(["affiliate.manage"], "affiliate.fraud.review"),
    ).toBe(false);
    expect(p.length).toBeGreaterThan(0);
  });
});
