import type { AdminRole, Permission } from "@/types/admin";

const ROLE_PERMISSIONS: Record<AdminRole, Permission[]> = {
  VIEWER: ["user.view", "bet.view", "reports.view"],
  SUPPORT_AGENT: [
    "user.view",
    "user.edit",
    "bet.view",
    "transaction.view",
    "communication.manage",
  ],
  KYC_OFFICER: [
    "user.view",
    "kyc.review",
    "kyc.sof_review",
    "fraud.screening",
  ],
  FINANCE_MANAGER: [
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_small",
    "withdrawal.approve_large",
    "finance.balance_sheet",
    "finance.crypto_wallet",
    "reports.view",
    "reports.export",
  ],
  RISK_MANAGER: [
    "user.view",
    "user.block",
    "bet.view",
    "bet.void",
    "transaction.view",
    "fraud.review",
    "fraud.rule_builder",
    "fraud.screening",
    "reports.view",
    "audit.view",
  ],
  AFFILIATE_MANAGER: [
    "affiliate.manage",
    "reports.view",
    "reports.export",
  ],
  CONTENT_MANAGER: ["content.manage", "crm.campaign", "communication.manage"],
  CRM_MANAGER: [
    "user.view",
    "bonus.create",
    "bonus.edit",
    "bonus.grant",
    "crm.campaign",
    "crm.segment",
    "communication.manage",
    "reports.view",
    "reports.export",
  ],
  SPORTS_TRADER: [
    "bet.view",
    "bet.void",
    "sportsbook.manage",
    "sportsbook.trading_terminal",
    "reports.view",
  ],
  COMPLIANCE_OFFICER: [
    "user.view",
    "kyc.review",
    "fraud.screening",
    "audit.view",
    "reports.view",
    "reports.export",
  ],
  SUPER_ADMIN: [
    "user.view",
    "user.edit",
    "user.block",
    "user.delete",
    "user.merge",
    "bet.view",
    "bet.void",
    "transaction.view",
    "transaction.adjust",
    "withdrawal.approve_small",
    "withdrawal.approve_large",
    "finance.balance_sheet",
    "finance.crypto_wallet",
    "fraud.review",
    "fraud.rule_builder",
    "fraud.screening",
    "bonus.create",
    "bonus.edit",
    "bonus.grant",
    "casino.manage",
    "casino.rtp_config",
    "sportsbook.manage",
    "sportsbook.trading_terminal",
    "affiliate.manage",
    "content.manage",
    "crm.campaign",
    "crm.segment",
    "communication.manage",
    "kyc.review",
    "kyc.sof_review",
    "reports.view",
    "reports.export",
    "system.config",
    "system.maintenance",
    "admin.manage",
    "audit.view",
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
