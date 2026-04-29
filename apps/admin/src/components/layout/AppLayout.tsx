import { Layout, Menu, Avatar, Dropdown, Typography, Space, theme } from "antd";
import {
  DashboardOutlined,
  UserOutlined,
  DollarOutlined,
  TrophyOutlined,
  PlayCircleOutlined,
  GiftOutlined,
  TeamOutlined,
  SafetyOutlined,
  SettingOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  FileProtectOutlined,
  MailOutlined,
  BarChartOutlined,
  FileTextOutlined,
} from "@ant-design/icons";
import { useState } from "react";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";
import { useInactivityLogout } from "@/hooks/useInactivityLogout";
import { authService } from "@/services/auth.service";
import { hasPermission } from "@/utils/permissions";
import type { Permission } from "@/types/admin";

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

interface MenuItem {
  key: string;
  icon?: React.ReactNode;
  label: string;
  permission?: Permission;
  children?: MenuItem[];
}

const menuItems: MenuItem[] = [
  { key: "/dashboard", icon: <DashboardOutlined />, label: "Dashboard" },
  {
    key: "/users",
    icon: <UserOutlined />,
    label: "Players",
    permission: "user.view",
  },
  {
    key: "/kyc",
    icon: <FileProtectOutlined />,
    label: "KYC",
    permission: "kyc.review",
    children: [
      { key: "/kyc/queue", label: "Review Queue", permission: "kyc.review" },
      { key: "/kyc/sof", label: "Source of Funds", permission: "kyc.sof_review" },
      { key: "/kyc/expiry", label: "Expiry Monitor", permission: "kyc.review" },
      { key: "/kyc/screening", label: "PEP / Sanctions", permission: "fraud.screening" },
      { key: "/kyc/team", label: "Team Metrics", permission: "kyc.review" },
      { key: "/kyc/rg", label: "RG Dashboard", permission: "kyc.review" },
    ],
  },
  {
    key: "/finance",
    icon: <DollarOutlined />,
    label: "Finance",
    permission: "transaction.view",
    children: [
      {
        key: "/finance/deposits",
        label: "Deposits",
        permission: "transaction.view",
      },
      {
        key: "/finance/withdrawals",
        label: "Withdrawals",
        permission: "transaction.view",
      },
      {
        key: "/finance/transactions",
        label: "Transactions",
        permission: "transaction.view",
      },
      {
        key: "/finance/chargebacks",
        label: "Chargebacks",
        permission: "transaction.view",
      },
      {
        key: "/finance/crypto",
        label: "Crypto Wallets",
        permission: "finance.crypto_wallet",
      },
      {
        key: "/finance/balance-sheet",
        label: "Balance Sheet",
        permission: "finance.balance_sheet",
      },
      {
        key: "/finance/p2p",
        label: "P2P Queue",
        permission: "transaction.view",
      },
      {
        key: "/finance/reconciliation",
        label: "Reconciliation",
        permission: "transaction.view",
      },
    ],
  },
  {
    key: "/sports",
    icon: <TrophyOutlined />,
    label: "Sportsbook",
    permission: "bet.view",
    children: [
      { key: "/sports/bets", label: "Bets", permission: "bet.view" },
      { key: "/sports/events", label: "Events", permission: "sportsbook.manage" },
      { key: "/sports/trading", label: "Trading Terminal", permission: "sportsbook.trading_terminal" },
    ],
  },
  {
    key: "/casino",
    icon: <PlayCircleOutlined />,
    label: "Casino",
    permission: "casino.manage",
    children: [
      { key: "/casino/games", label: "Games", permission: "casino.manage" },
      { key: "/casino/sessions", label: "Sessions", permission: "casino.manage" },
      { key: "/casino/rtp", label: "RTP Config", permission: "casino.rtp_config" },
    ],
  },
  {
    key: "/bonuses",
    icon: <GiftOutlined />,
    label: "Bonuses",
    permission: "bonus.create",
  },
  {
    key: "/affiliates",
    icon: <TeamOutlined />,
    label: "Affiliates",
    permission: "affiliate.manage",
    children: [
      {
        key: "/affiliates",
        label: "All Affiliates",
        permission: "affiliate.manage",
      },
      {
        key: "/affiliates/payouts",
        label: "Payouts",
        permission: "affiliate.manage",
      },
      {
        key: "/affiliates/fraud",
        label: "Fraud Flags",
        permission: "affiliate.manage",
      },
    ],
  },
  {
    key: "/crm",
    icon: <MailOutlined />,
    label: "CRM",
    permission: "communication.manage",
    children: [
      { key: "/crm/campaigns", label: "Campaigns", permission: "crm.campaign" },
      { key: "/crm/segments", label: "Segments", permission: "crm.segment" },
      { key: "/crm/templates", label: "Templates", permission: "communication.manage" },
    ],
  },
  {
    key: "/support",
    icon: <MailOutlined />,
    label: "Support",
    permission: "communication.manage",
    children: [
      { key: "/support/tickets", label: "Tickets", permission: "communication.manage" },
      { key: "/support/dashboard", label: "Team Dashboard", permission: "communication.manage" },
      { key: "/support/sla", label: "SLA Config", permission: "system.config" },
    ],
  },
  {
    key: "/risk",
    icon: <SafetyOutlined />,
    label: "Risk & Compliance",
    permission: "fraud.review",
    children: [
      {
        key: "/risk/alerts",
        label: "Fraud Alerts",
        permission: "fraud.review",
      },
      {
        key: "/risk/rules",
        label: "Rule Builder",
        permission: "fraud.rule_builder",
      },
      {
        key: "/risk/screening",
        label: "PEP/Sanctions",
        permission: "fraud.screening",
      },
      {
        key: "/risk/audit-log",
        label: "Audit Log",
        permission: "audit.view",
      },
    ],
  },
  {
    key: "/reports",
    icon: <BarChartOutlined />,
    label: "Reports",
    permission: "reports.view",
    children: [
      { key: "/reports/financial", label: "Financial", permission: "reports.view" },
      { key: "/reports/player", label: "Player Analytics", permission: "reports.view" },
      { key: "/reports/compliance", label: "Compliance", permission: "reports.view" },
      { key: "/reports/games", label: "Game Analytics", permission: "reports.view" },
    ],
  },
  {
    key: "/regulatory",
    icon: <FileProtectOutlined />,
    label: "Regulatory",
    permission: "reports.view",
    children: [
      { key: "/regulatory", label: "Dashboard", permission: "reports.view" },
      { key: "/regulatory/generator", label: "Report Generator", permission: "reports.view" },
      { key: "/regulatory/sar", label: "SAR Management", permission: "reports.view" },
      { key: "/regulatory/complaints", label: "Complaints Log", permission: "reports.view" },
      { key: "/regulatory/tax", label: "Tax Config", permission: "reports.view" },
      { key: "/regulatory/player-funds", label: "Player Funds", permission: "reports.view" },
    ],
  },
  {
    key: "/cms",
    icon: <FileTextOutlined />,
    label: "CMS",
    permission: "content.manage",
  },
  {
    key: "/settings",
    icon: <SettingOutlined />,
    label: "Settings",
    permission: "system.config",
    children: [
      { key: "/settings/general", label: "General", permission: "system.config" },
      { key: "/settings/maintenance", label: "Maintenance", permission: "system.maintenance" },
      { key: "/settings/admin-users", label: "Admin Users", permission: "admin.manage" },
      { key: "/settings/audit", label: "Audit Log", permission: "audit.view" },
    ],
  },
];

function filterMenuByPermissions(
  items: MenuItem[],
  permissions: Permission[],
): MenuItem[] {
  return items
    .filter(
      (item) => !item.permission || hasPermission(permissions, item.permission),
    )
    .map((item) => ({
      ...item,
      children: item.children
        ? filterMenuByPermissions(item.children, permissions)
        : undefined,
    }))
    .filter((item) => !item.children || item.children.length > 0);
}

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken();
  const { adminName, adminEmail, adminRole, permissions, clearAuth } =
    useAuthStore();

  useInactivityLogout();

  const filteredMenu = filterMenuByPermissions(menuItems, permissions);

  const handleLogout = async () => {
    try {
      await authService.logout();
    } catch {
      // ignore network errors on logout
    } finally {
      clearAuth();
      navigate("/login", { replace: true });
    }
  };

  const dropdownItems = {
    items: [
      {
        key: "role",
        label: <Text type="secondary">Role: {adminRole}</Text>,
        disabled: true,
      },
      { type: "divider" as const },
      {
        key: "logout",
        icon: <LogoutOutlined />,
        label: "Logout",
        onClick: handleLogout,
      },
    ],
  };

  const selectedKey = location.pathname;
  const openKeys = menuItems
    .filter((item) =>
      item.children?.some((child) => selectedKey.startsWith(child.key)),
    )
    .map((item) => item.key);

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider trigger={null} collapsible collapsed={collapsed} theme="dark">
        <div
          style={{
            height: 64,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            padding: "16px",
          }}
        >
          <Text
            strong
            style={{
              color: "#fff",
              fontSize: collapsed ? 14 : 18,
              whiteSpace: "nowrap",
            }}
          >
            {collapsed ? "DOD" : "DOD"}
          </Text>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          defaultOpenKeys={openKeys}
          items={filteredMenu.map((item) => ({
            key: item.key,
            icon: item.icon,
            label: item.label,
            children: item.children?.map((child) => ({
              key: child.key,
              label: child.label,
            })),
          }))}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            padding: "0 24px",
            background: colorBgContainer,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            {collapsed ? (
              <MenuUnfoldOutlined
                onClick={() => setCollapsed(false)}
                style={{ fontSize: 18, cursor: "pointer" }}
              />
            ) : (
              <MenuFoldOutlined
                onClick={() => setCollapsed(true)}
                style={{ fontSize: 18, cursor: "pointer" }}
              />
            )}
          </div>
          <Dropdown menu={dropdownItems} placement="bottomRight">
            <Space style={{ cursor: "pointer" }}>
              <Avatar icon={<UserOutlined />} />
              <Text>{adminName || adminEmail}</Text>
            </Space>
          </Dropdown>
        </Header>
        <Content
          style={{
            margin: 24,
            padding: 24,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            minHeight: 280,
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
