import type { AdminRole, Permission } from "@/types/admin";

const ROLE_PERMISSIONS: Record<AdminRole, Permission[]> = {
  support_l1: ["user.view", "bet.view", "transaction.view"],
  support_l2: [
    "user.view",
    "user.edit",
    "bet.view",
    "transaction.view",
    "withdrawal.approve_small",
  ],
  risk_manager: [
    "user.view",
    "user.edit",
    "user.block",
    "bet.view",
    "bet.void",
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_large",
    "fraud.review",
    "reports.view",
  ],
  finance: [
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_small",
    "withdrawal.approve_large",
    "reports.view",
  ],
  marketing: [
    "bonus.create",
    "bonus.edit",
    "bonus.grant",
    "content.manage",
    "affiliate.manage",
    "reports.view",
  ],
  admin: [
    "user.view",
    "user.edit",
    "user.block",
    "bet.view",
    "bet.void",
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_small",
    "withdrawal.approve_large",
    "fraud.review",
    "bonus.create",
    "bonus.edit",
    "bonus.grant",
    "content.manage",
    "affiliate.manage",
    "reports.view",
    "system.config",
  ],
  super_admin: [
    "user.view",
    "user.edit",
    "user.block",
    "user.delete",
    "bet.view",
    "bet.void",
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_small",
    "withdrawal.approve_large",
    "fraud.review",
    "bonus.create",
    "bonus.edit",
    "bonus.grant",
    "content.manage",
    "affiliate.manage",
    "reports.view",
    "system.config",
  ],
};

export function getPermissionsForRole(role: AdminRole): Permission[] {
  return ROLE_PERMISSIONS[role] || [];
}

export function hasPermission(
  permissions: Permission[],
  required: Permission,
): boolean {
  return permissions.includes(required);
}

export function hasAnyPermission(
  permissions: Permission[],
  required: Permission[],
): boolean {
  return required.some((p) => permissions.includes(p));
}

export function hasAllPermissions(
  permissions: Permission[],
  required: Permission[],
): boolean {
  return required.every((p) => permissions.includes(p));
}
