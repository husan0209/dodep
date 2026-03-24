import { Card, Col, Row, Statistic, Typography, Spin } from "antd";
import {
  UserOutlined,
  DollarOutlined,
  ShoppingCartOutlined,
  WarningOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { systemService } from "@/services/system.service";
import { financeService } from "@/services/finance.service";
import { formatMoney } from "@/utils/format";

const { Title } = Typography;

export default function Dashboard() {
  const { data: stats, isLoading } = useQuery({
    queryKey: ["dashboard-stats"],
    queryFn: systemService.getDashboardStats,
    refetchInterval: 30000,
  });

  const { data: financialSummary } = useQuery({
    queryKey: ["financial-summary"],
    queryFn: () => financeService.getFinancialSummary(),
  });

  if (isLoading)
    return (
      <Spin size="large" style={{ display: "block", margin: "100px auto" }} />
    );

  return (
    <div>
      <Title level={3} style={{ marginBottom: 24 }}>
        Dashboard
      </Title>

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Total Users"
              value={stats?.total_users || 0}
              prefix={<UserOutlined />}
            />
            <div style={{ marginTop: 8 }}>
              <Statistic
                title="Active Today"
                value={stats?.active_users_today || 0}
                valueStyle={{ fontSize: 14, color: "#52c41a" }}
              />
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Deposits Today"
              value={
                stats?.total_deposits_today
                  ? formatMoney(stats.total_deposits_today)
                  : "$0.00"
              }
              prefix={<ArrowDownOutlined style={{ color: "#52c41a" }} />}
            />
            <div style={{ marginTop: 8 }}>
              <Statistic
                title="Withdrawals Today"
                value={
                  stats?.total_withdrawals_today
                    ? formatMoney(stats.total_withdrawals_today)
                    : "$0.00"
                }
                valueStyle={{ fontSize: 14 }}
              />
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="GGR Today"
              value={stats?.ggr_today ? formatMoney(stats.ggr_today) : "$0.00"}
              prefix={<DollarOutlined style={{ color: "#1677ff" }} />}
            />
            <div style={{ marginTop: 8 }}>
              <Statistic
                title="Bets Placed"
                value={stats?.bets_placed_today || 0}
                valueStyle={{ fontSize: 14 }}
              />
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Open Fraud Alerts"
              value={stats?.open_fraud_alerts || 0}
              prefix={<WarningOutlined style={{ color: "#ff4d4f" }} />}
              valueStyle={{
                color: stats?.open_fraud_alerts ? "#ff4d4f" : undefined,
              }}
            />
            <div style={{ marginTop: 8 }}>
              <Statistic
                title="Pending Withdrawals"
                value={stats?.pending_withdrawals || 0}
                valueStyle={{
                  fontSize: 14,
                  color: stats?.pending_withdrawals ? "#faad14" : undefined,
                }}
              />
            </div>
          </Card>
        </Col>
      </Row>

      {financialSummary && (
        <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
          <Col xs={24} lg={12}>
            <Card title="Financial Summary">
              <Row gutter={16}>
                <Col span={8}>
                  <Statistic
                    title="Total Deposits"
                    value={formatMoney(financialSummary.total_deposits)}
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title="Total Withdrawals"
                    value={formatMoney(financialSummary.total_withdrawals)}
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title="Net Revenue"
                    value={formatMoney(financialSummary.net_revenue)}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="Pending Actions">
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic
                    title="Pending Withdrawals"
                    value={financialSummary.pending_withdrawals_count}
                    suffix={`(${formatMoney(financialSummary.pending_withdrawals_amount)})`}
                  />
                </Col>
                <Col span={12}>
                  <Statistic
                    title="Pending KYC Reviews"
                    value={stats?.pending_kyc_reviews || 0}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        </Row>
      )}
    </div>
  );
}
