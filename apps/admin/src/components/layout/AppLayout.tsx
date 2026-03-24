import { Layout, Menu, Avatar, Dropdown, Typography, Space, theme } from "antd";
import {
  DashboardOutlined,
  UserOutlined,
  DollarOutlined,
  TrophyOutlined,
  PlayCircleOutlined,
  GiftOutlined,
  SafetyOutlined,
  SettingOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from "@ant-design/icons";
import { useState } from "react";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";
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
    label: "Users",
    permission: "user.view",
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
    ],
  },
  {
    key: "/sports",
    icon: <TrophyOutlined />,
    label: "Sports",
    permission: "bet.view",
    children: [{ key: "/sports/bets", label: "Bets", permission: "bet.view" }],
  },
  {
    key: "/casino",
    icon: <PlayCircleOutlined />,
    label: "Casino",
    permission: "reports.view",
    children: [
      { key: "/casino/games", label: "Games", permission: "reports.view" },
      {
        key: "/casino/sessions",
        label: "Sessions",
        permission: "reports.view",
      },
    ],
  },
  {
    key: "/bonuses",
    icon: <GiftOutlined />,
    label: "Bonuses",
    permission: "bonus.create",
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
        key: "/risk/audit-log",
        label: "Audit Log",
        permission: "reports.view",
      },
    ],
  },
  {
    key: "/system",
    icon: <SettingOutlined />,
    label: "System",
    permission: "system.config",
    children: [
      { key: "/system/health", label: "Health", permission: "system.config" },
      {
        key: "/system/config",
        label: "Configuration",
        permission: "system.config",
      },
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

  const filteredMenu = filterMenuByPermissions(menuItems, permissions);

  const handleLogout = () => {
    clearAuth();
    navigate("/login");
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
            {collapsed ? "OC" : "Opus Casino"}
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
