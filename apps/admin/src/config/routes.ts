import type { Permission } from "@/types/admin";

export interface RouteConfig {
  path: string;
  label: string;
  icon?: string;
  permission?: Permission;
  children?: RouteConfig[];
}

export const routeConfig: RouteConfig[] = [
  { path: "/dashboard", label: "Dashboard", icon: "DashboardOutlined" },
  {
    path: "/users",
    label: "Players",
    icon: "UserOutlined",
    permission: "user.view",
    children: [
      { path: "/users", label: "All Players", permission: "user.view" },
      { path: "/users/:id", label: "Player Detail", permission: "user.view" },
    ],
  },
  {
    path: "/kyc",
    label: "KYC",
    icon: "FileProtectOutlined",
    permission: "kyc.review",
    children: [
      { path: "/kyc/queue", label: "Review Queue", permission: "kyc.review" },
      { path: "/kyc/sof", label: "Source of Funds", permission: "kyc.sof_review" },
    ],
  },
  {
    path: "/finance",
    label: "Finance",
    icon: "DollarOutlined",
    permission: "transaction.view",
    children: [
      { path: "/finance/deposits", label: "Deposits", permission: "transaction.view" },
      { path: "/finance/withdrawals", label: "Withdrawals", permission: "transaction.view" },
      { path: "/finance/transactions", label: "Transactions", permission: "transaction.view" },
      { path: "/finance/chargebacks", label: "Chargebacks", permission: "transaction.view" },
      { path: "/finance/crypto", label: "Crypto Wallets", permission: "finance.crypto_wallet" },
      { path: "/finance/balance-sheet", label: "Balance Sheet", permission: "finance.balance_sheet" },
    ],
  },
  {
    path: "/sports",
    label: "Sportsbook",
    icon: "TrophyOutlined",
    permission: "bet.view",
    children: [
      { path: "/sports/bets", label: "Bets", permission: "bet.view" },
      { path: "/sports/events", label: "Events", permission: "sportsbook.manage" },
      { path: "/sports/trading", label: "Trading Terminal", permission: "sportsbook.trading_terminal" },
    ],
  },
  {
    path: "/casino",
    label: "Casino",
    icon: "PlayCircleOutlined",
    permission: "casino.manage",
    children: [
      { path: "/casino/games", label: "Games", permission: "casino.manage" },
      { path: "/casino/sessions", label: "Sessions", permission: "casino.manage" },
      { path: "/casino/rtp", label: "RTP Config", permission: "casino.rtp_config" },
    ],
  },
  {
    path: "/bonuses",
    label: "Bonuses",
    icon: "GiftOutlined",
    permission: "bonus.create",
  },
  {
    path: "/affiliates",
    label: "Affiliates",
    icon: "TeamOutlined",
    permission: "affiliate.manage",
    children: [
      { path: "/affiliates", label: "All Affiliates", permission: "affiliate.manage" },
      { path: "/affiliates/payouts", label: "Payout Queue", permission: "affiliate.manage" },
      { path: "/affiliates/fraud", label: "Fraud Flags", permission: "affiliate.manage" },
    ],
  },
  {
    path: "/crm",
    label: "CRM",
    icon: "MailOutlined",
    permission: "communication.manage",
    children: [
      { path: "/crm/campaigns", label: "Campaigns", permission: "crm.campaign" },
      { path: "/crm/segments", label: "Segments", permission: "crm.segment" },
      { path: "/crm/templates", label: "Templates", permission: "communication.manage" },
    ],
  },
  {
    path: "/risk",
    label: "Risk & Compliance",
    icon: "SafetyOutlined",
    permission: "fraud.review",
    children: [
      { path: "/risk/alerts", label: "Fraud Alerts", permission: "fraud.review" },
      { path: "/risk/rules", label: "Rule Builder", permission: "fraud.rule_builder" },
      { path: "/risk/screening", label: "PEP/Sanctions", permission: "fraud.screening" },
      { path: "/risk/audit-log", label: "Audit Log", permission: "audit.view" },
    ],
  },
  {
    path: "/reports",
    label: "Reports",
    icon: "BarChartOutlined",
    permission: "reports.view",
    children: [
      { path: "/reports/financial", label: "Financial", permission: "reports.view" },
      { path: "/reports/player", label: "Player Analytics", permission: "reports.view" },
      { path: "/reports/compliance", label: "Compliance", permission: "reports.view" },
    ],
  },
  {
    path: "/support",
    label: "Support",
    icon: "MailOutlined",
    permission: "communication.manage",
    children: [
      { path: "/support/tickets", label: "Tickets", permission: "communication.manage" },
      { path: "/support/dashboard", label: "Team Dashboard", permission: "communication.manage" },
      { path: "/support/sla", label: "SLA Config", permission: "system.config" },
    ],
  },
  {
    path: "/cms",
    label: "CMS",
    icon: "FileTextOutlined",
    permission: "content.manage",
  },
  {
    path: "/settings",
    label: "Settings",
    icon: "SettingOutlined",
    permission: "system.config",
    children: [
      { path: "/settings/general", label: "General", permission: "system.config" },
      { path: "/settings/maintenance", label: "Maintenance", permission: "system.maintenance" },
      { path: "/settings/admin-users", label: "Admin Users", permission: "admin.manage" },
      { path: "/settings/audit", label: "Audit Log", permission: "audit.view" },
    ],
  },
];
