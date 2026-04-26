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
    label: "User Management",
    icon: "UserOutlined",
    permission: "user.view",
    children: [
      { path: "/users", label: "All Users", permission: "user.view" },
      { path: "/users/:id", label: "User Detail", permission: "user.view" },
    ],
  },
  {
    path: "/finance",
    label: "Finance",
    icon: "DollarOutlined",
    permission: "transaction.view",
    children: [
      {
        path: "/finance/deposits",
        label: "Deposits",
        permission: "transaction.view",
      },
      {
        path: "/finance/withdrawals",
        label: "Withdrawals",
        permission: "transaction.view",
      },
      {
        path: "/finance/transactions",
        label: "Transactions",
        permission: "transaction.view",
      },
    ],
  },
  {
    path: "/sports",
    label: "Sports",
    icon: "TrophyOutlined",
    permission: "bet.view",
    children: [
      { path: "/sports/bets", label: "Bets", permission: "bet.view" },
      { path: "/sports/events", label: "Events", permission: "bet.view" },
    ],
  },
  {
    path: "/casino",
    label: "Casino",
    icon: "PlayCircleOutlined",
    permission: "reports.view",
    children: [
      { path: "/casino/games", label: "Games", permission: "reports.view" },
      {
        path: "/casino/sessions",
        label: "Sessions",
        permission: "reports.view",
      },
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
    path: "/risk",
    label: "Risk & Compliance",
    icon: "SafetyOutlined",
    permission: "fraud.review",
    children: [
      {
        path: "/risk/alerts",
        label: "Fraud Alerts",
        permission: "fraud.review",
      },
      {
        path: "/risk/audit-log",
        label: "Audit Log",
        permission: "reports.view",
      },
    ],
  },
  {
    path: "/system",
    label: "System",
    icon: "SettingOutlined",
    permission: "system.config",
    children: [
      { path: "/system/health", label: "Health", permission: "system.config" },
      {
        path: "/system/config",
        label: "Configuration",
        permission: "system.config",
      },
    ],
  },
];
