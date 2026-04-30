import { useState, useCallback } from "react";
import {
  Card,
  Col,
  Row,
  Statistic,
  Typography,
  Spin,
  Badge,
  Tag,
  List,
  Table,
  Progress,
  Tooltip,
} from "antd";
import {
  UserOutlined,
  DollarOutlined,
  WarningOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  GlobalOutlined,
  TrophyOutlined,
  PlayCircleOutlined,
  WifiOutlined,
  DisconnectOutlined,
  MailOutlined,
  FileProtectOutlined,
} from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import {
  LineChart,
  Line,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as ReTooltip,
  Legend,
  ResponsiveContainer,
  AreaChart,
  Area,
} from "recharts";
import { systemService } from "@/services/system.service";
import { financeService } from "@/services/finance.service";
import { useWebSocket } from "@/hooks/useWebSocket";
import { API_BASE_URL } from "@/utils/constants";
import { formatMoney } from "@/utils/format";
import type { LiveMetrics, ProviderHealth, GatewayHealth, TopItem } from "@/types/admin";

const { Title, Text } = Typography;

const WS_BASE = API_BASE_URL.replace(/^http/, "ws");

function useLiveMetrics() {
  const [metrics, setMetrics] = useState<LiveMetrics | null>(null);

  const handleMessage = useCallback((msg: { topic: string; payload: unknown }) => {
    if (msg.topic === "admin.metrics.live") {
      setMetrics(msg.payload as LiveMetrics);
    }
  }, []);

  const { isConnected } = useWebSocket({
    url: `${WS_BASE}/admin/ws`,
    onMessage: handleMessage,
  });

  return { metrics, isConnected };
}

export default function Dashboard() {
  const { metrics, isConnected } = useLiveMetrics();
  const { data: stats, isLoading } = useQuery({
    queryKey: ["dashboard-stats"],
    queryFn: systemService.getDashboardStats,
    refetchInterval: 30000,
  });

  const { data: financialSummary } = useQuery({
    queryKey: ["financial-summary"],
    queryFn: () => financeService.getFinancialSummary(),
  });

  const { data: providerHealth } = useQuery({
    queryKey: ["provider-health"],
    queryFn: systemService.getProviderHealth,
    refetchInterval: 60000,
  });

  const { data: gatewayHealth } = useQuery({
    queryKey: ["gateway-health"],
    queryFn: systemService.getGatewayHealth,
    refetchInterval: 60000,
  });

  const { data: ggrChart } = useQuery({
    queryKey: ["ggr-chart", "30d"],
    queryFn: () => systemService.getGGRChart("30d"),
  });

  const { data: dwChart } = useQuery({
    queryKey: ["dw-chart"],
    queryFn: systemService.getDepositsVsWithdrawalsChart,
  });

  const { data: funnel } = useQuery({
    queryKey: ["conversion-funnel"],
    queryFn: systemService.getConversionFunnel,
  });

  const { data: topGames } = useQuery({
    queryKey: ["top-games"],
    queryFn: () => systemService.getTopGames(5),
  });

  const { data: topEvents } = useQuery({
    queryKey: ["top-events"],
    queryFn: () => systemService.getTopEvents(5),
  });

  const { data: topCountries } = useQuery({
    queryKey: ["top-countries"],
    queryFn: () => systemService.getTopCountries(5),
  });

  const live = metrics || {
    online: {
      casino: stats?.online_casino || 0,
      sports: stats?.online_sports || 0,
      total: (stats?.online_casino || 0) + (stats?.online_sports || 0),
    },
    ggr_today: {
      casino: parseFloat(stats?.ggr_today || "0"),
      sports: 0,
      live_casino: 0,
    },
    deposits_today: parseFloat(stats?.total_deposits_today || "0"),
    withdrawals_today: parseFloat(stats?.total_withdrawals_today || "0"),
    ftd_today: { count: stats?.ftd_count_today || 0, amount: 0 },
    pending_withdrawals: {
      count: stats?.pending_withdrawals || 0,
      amount: 0,
    },
    pending_kyc: stats?.pending_kyc_reviews || 0,
    open_tickets: stats?.open_support_tickets || 0,
  };

  if (isLoading)
    return (
      <Spin size="large" style={{ display: "block", margin: "100px auto" }} />
    );

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 24 }}>
        <Title level={3} style={{ margin: 0 }}>
          Dashboard
        </Title>
        <Tooltip title={isConnected ? "Live connection active" : "Offline — using cached data"}>
          <Badge
            status={isConnected ? "processing" : "default"}
            text={
              isConnected ? (
                <Text type="secondary"><WifiOutlined /> Live</Text>
              ) : (
                <Text type="secondary"><DisconnectOutlined /> Cached</Text>
              )
            }
          />
        </Tooltip>
      </div>

      {/* Live Overview Metrics */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title={
                <span>
                  <GlobalOutlined /> Online
                </span>
              }
              value={live.online.total}
              suffix={`(${live.online.casino}C / ${live.online.sports}S)`}
              valueStyle={{ fontSize: 20 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title="GGR Today"
              value={formatMoney(String(live.ggr_today.casino))}
              prefix={<DollarOutlined style={{ color: "#1677ff" }} />}
              valueStyle={{ fontSize: 20, color: "#1677ff" }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title="Deposits"
              value={formatMoney(String(live.deposits_today))}
              prefix={<ArrowDownOutlined style={{ color: "#52c41a" }} />}
              valueStyle={{ fontSize: 20, color: "#52c41a" }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title="Withdrawals"
              value={formatMoney(String(live.withdrawals_today))}
              prefix={<ArrowUpOutlined style={{ color: "#ff4d4f" }} />}
              valueStyle={{ fontSize: 20 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title="FTD Today"
              value={live.ftd_today.count}
              suffix={`(${formatMoney(String(live.ftd_today.amount))})`}
              valueStyle={{ fontSize: 20 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={4}>
          <Card size="small">
            <Statistic
              title="Pending"
              value={live.pending_withdrawals.count}
              suffix={`W / ${live.pending_kyc}KYC`}
              valueStyle={{
                fontSize: 20,
                color: live.pending_withdrawals.count > 0 ? "#faad14" : undefined,
              }}
            />
          </Card>
        </Col>
      </Row>

      {/* Charts Row */}
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col xs={24} lg={12}>
          <Card title="GGR / NGR — Last 30 Days">
            <ResponsiveContainer width="100%" height={280}>
              <AreaChart data={ggrChart || []}>
                <defs>
                  <linearGradient id="ggrGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#1677ff" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#1677ff" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="ngrGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#52c41a" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#52c41a" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                <YAxis tick={{ fontSize: 12 }} />
                <ReTooltip formatter={(v: number) => formatMoney(String(v))} />
                <Legend />
                <Area
                  type="monotone"
                  dataKey="ggr"
                  stroke="#1677ff"
                  fill="url(#ggrGrad)"
                  name="GGR"
                />
                <Area
                  type="monotone"
                  dataKey="ngr"
                  stroke="#52c41a"
                  fill="url(#ngrGrad)"
                  name="NGR"
                />
              </AreaChart>
            </ResponsiveContainer>
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="Deposits vs Withdrawals (Today by Hour)">
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={dwChart || []}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                <YAxis tick={{ fontSize: 12 }} />
                <ReTooltip formatter={(v: number) => formatMoney(String(v))} />
                <Legend />
                <Bar dataKey="deposits" fill="#52c41a" name="Deposits" radius={[4, 4, 0, 0]} />
                <Bar dataKey="withdrawals" fill="#ff4d4f" name="Withdrawals" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
      </Row>

      {/* Conversion Funnel + Provider Health */}
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col xs={24} lg={8}>
          <Card title="Conversion Funnel">
            {funnel && (
              <div style={{ padding: "0 16px" }}>
                {[
                  { label: "Visits", value: funnel.visits, color: "#1677ff" },
                  { label: "Registrations", value: funnel.registrations, color: "#52c41a" },
                  { label: "FTD", value: funnel.ftd, color: "#faad14" },
                  { label: "2nd Deposit", value: funnel.second_deposit, color: "#eb2f96" },
                ].map((step, idx, arr) => {
                  const prev = idx > 0 ? arr[idx - 1].value : step.value;
                  const rate = prev > 0 ? Math.round((step.value / prev) * 100) : 0;
                  return (
                    <div key={step.label} style={{ marginBottom: 16 }}>
                      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                        <Text strong>{step.label}</Text>
                        <Text>{step.value.toLocaleString()}</Text>
                      </div>
                      <Progress
                        percent={rate}
                        strokeColor={step.color}
                        showInfo={false}
                        size="small"
                      />
                      {idx > 0 && (
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          {rate}% conversion from previous
                        </Text>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card
            title={
              <span>
                <PlayCircleOutlined /> Provider Health
              </span>
            }
          >
            <Table
              dataSource={providerHealth || []}
              rowKey="name"
              size="small"
              pagination={false}
              columns={[
                {
                  title: "Provider",
                  dataIndex: "name",
                  ellipsis: true,
                },
                {
                  title: "Status",
                  dataIndex: "status",
                  render: (s: string) => (
                    <Tag
                      color={
                        s === "online" ? "success" : s === "degraded" ? "warning" : "error"
                      }
                    >
                      {s}
                    </Tag>
                  ),
                },
                {
                  title: "Latency",
                  dataIndex: "latency_p99_ms",
                  render: (v: number) => `${v}ms`,
                },
                {
                  title: "Error %",
                  dataIndex: "error_rate_pct",
                  render: (v: number) => `${v.toFixed(2)}%`,
                },
              ]}
            />
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card
            title={
              <span>
                <DollarOutlined /> Gateway Health
              </span>
            }
          >
            <Table
              dataSource={gatewayHealth || []}
              rowKey="name"
              size="small"
              pagination={false}
              columns={[
                {
                  title: "Gateway",
                  dataIndex: "name",
                  ellipsis: true,
                },
                {
                  title: "Success %",
                  dataIndex: "success_rate_pct",
                  render: (v: number) => (
                    <Tag color={v >= 80 ? "success" : v >= 60 ? "warning" : "error"}>
                      {v.toFixed(1)}%
                    </Tag>
                  ),
                },
                {
                  title: "Latency",
                  dataIndex: "avg_latency_ms",
                  render: (v: number) => `${v}ms`,
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      {/* Top Lists */}
      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        <Col xs={24} lg={8}>
          <Card
            title={
              <span>
                <TrophyOutlined /> Top 5 Games (by GGR)
              </span>
            }
          >
            <List
              dataSource={topGames || []}
              renderItem={(item: TopItem) => (
                <List.Item>
                  <Text ellipsis style={{ maxWidth: "70%" }}>
                    {item.name}
                  </Text>
                  <Text strong>{formatMoney(String(item.value))}</Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card
            title={
              <span>
                <TrophyOutlined /> Top 5 Events (by Stakes)
              </span>
            }
          >
            <List
              dataSource={topEvents || []}
              renderItem={(item: TopItem) => (
                <List.Item>
                  <Text ellipsis style={{ maxWidth: "70%" }}>
                    {item.name}
                  </Text>
                  <Text strong>{formatMoney(String(item.value))}</Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>

        <Col xs={24} lg={8}>
          <Card
            title={
              <span>
                <GlobalOutlined /> Top 5 Countries
              </span>
            }
          >
            <List
              dataSource={topCountries || []}
              renderItem={(item: TopItem) => (
                <List.Item>
                  <Text ellipsis style={{ maxWidth: "70%" }}>
                    {item.name}
                  </Text>
                  <Text strong>{item.value.toLocaleString()} players</Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>

      {/* Pending Actions + Alerts */}
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
                <Col span={8}>
                  <Statistic
                    title="Withdrawals"
                    value={financialSummary.pending_withdrawals_count}
                    suffix={`(${formatMoney(financialSummary.pending_withdrawals_amount)})`}
                    valueStyle={{ color: "#faad14" }}
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title={
                      <span>
                        <FileProtectOutlined /> KYC
                      </span>
                    }
                    value={stats?.pending_kyc_reviews || 0}
                  />
                </Col>
                <Col span={8}>
                  <Statistic
                    title={
                      <span>
                        <MailOutlined /> Tickets
                      </span>
                    }
                    value={stats?.open_support_tickets || 0}
                    valueStyle={{ color: "#1677ff" }}
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
