import { describe, it, expect } from "vitest";
import { routeConfig, type RouteConfig } from "@/config/routes";
import { getPermissionsForRole } from "@/utils/permissions";
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

/** Every permission the role matrix is capable of granting. */
const GRANTABLE_PERMISSIONS = new Set<Permission>(
  ALL_ROLES.flatMap((role) => getPermissionsForRole(role)),
);

interface FlatRoute extends RouteConfig {
  /** Absolute path of the owning group, or "/" for a top-level entry. */
  parentPath: string;
}

function flatten(routes: RouteConfig[], parentPath = "/"): FlatRoute[] {
  return routes.flatMap((route) => [
    { ...route, parentPath },
    ...flatten(route.children ?? [], route.path),
  ]);
}

const allRoutes = flatten(routeConfig);
/** A leaf is a menu entry that navigates somewhere (no `children`). */
const leafRoutes = allRoutes.filter((route) => !route.children);

describe("routeConfig integrity", () => {
  it("is not empty", () => {
    expect(routeConfig.length).toBeGreaterThan(0);
    expect(allRoutes.length).toBeGreaterThan(10);
  });

  it("gives every entry a non-empty label", () => {
    for (const route of allRoutes) {
      expect(route.label.trim().length).toBeGreaterThan(0);
    }
  });

  it("uses absolute, lowercase paths", () => {
    for (const route of allRoutes) {
      expect(route.path.startsWith("/")).toBe(true);
      expect(route.path).not.toMatch(/\s/);
      expect(route.path).not.toMatch(/[A-Z]/);
      expect(route.path.endsWith("/")).toBe(false);
    }
  });

  it("has no duplicate navigable (leaf) paths", () => {
    const paths = leafRoutes.map((route) => route.path);
    const duplicates = paths.filter((p, i) => paths.indexOf(p) !== i);
    expect(duplicates).toEqual([]);
  });

  it("has no duplicate top-level group paths", () => {
    const paths = routeConfig.map((route) => route.path);
    expect(new Set(paths).size).toBe(paths.length);
  });

  it("keeps every child either at its parent path or beneath it", () => {
    for (const route of leafRoutes) {
      if (route.parentPath === "/") continue;
      const isIndexChild = route.path === route.parentPath;
      const isNested = route.path.startsWith(`${route.parentPath}/`);
      expect(isIndexChild || isNested).toBe(true);
    }
  });

  it("only references permissions that the role matrix can actually grant", () => {
    // A typo'd permission string would silently hide the menu entry from
    // every role, so it must never survive here.
    for (const route of allRoutes) {
      if (!route.permission) continue;
      expect(GRANTABLE_PERMISSIONS.has(route.permission)).toBe(true);
    }
  });

  it("gives every grouped (non-dashboard) section a permission gate", () => {
    for (const route of routeConfig) {
      if (route.path === "/dashboard") continue;
      expect(route.permission).toBeDefined();
    }
  });

  it("keeps the dashboard reachable without any permission", () => {
    const dashboard = routeConfig.find((r) => r.path === "/dashboard");
    expect(dashboard).toBeDefined();
    expect(dashboard?.permission).toBeUndefined();
    expect(dashboard?.children).toBeUndefined();
  });

  it("requires a permission on every leaf that is not the dashboard", () => {
    for (const route of leafRoutes) {
      if (route.path === "/dashboard") continue;
      expect(route.permission).toBeDefined();
    }
  });

  it("declares an icon only on groups / top-level entries", () => {
    for (const route of leafRoutes) {
      if (route.parentPath === "/") continue;
      expect(route.icon).toBeUndefined();
    }
  });

  it("exposes only admins holding a permission to every gated leaf", () => {
    const leafPaths = leafRoutes.map((route) => route.path);
    expect(leafPaths).toContain("/finance/withdrawals");
    expect(leafPaths).toContain("/settings/admin-users");
    expect(leafPaths).toContain("/kyc/sof");

    // A super admin sees the whole table; a viewer only sees read-only views.
    const superAdmin = getPermissionsForRole("SUPER_ADMIN");
    const viewer = getPermissionsForRole("VIEWER");
    const visibleTo = (perms: Permission[]) =>
      leafRoutes
        .filter((r) => !r.permission || perms.includes(r.permission))
        .map((r) => r.path);

    expect(visibleTo(superAdmin)).toEqual(leafPaths);
    expect(visibleTo(viewer)).toEqual([
      "/dashboard",
      "/users",
      "/users/:id",
      "/sports/bets",
      "/reports/financial",
      "/reports/player",
      "/reports/compliance",
    ]);
  });
});
