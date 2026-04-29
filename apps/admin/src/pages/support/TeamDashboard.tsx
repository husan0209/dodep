import { useQuery } from "@tanstack/react-query";
import {
  Card,
  Statistic,
  Row,
  Col,
  Table,
  Typography,
  Tag,
  Badge,
} from "antd";
import {
  TeamOutlined,
  ClockCircleOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  InboxOutlined,
  MessageOutlined,
  PauseCircleOutlined,
} from "@ant-design/icons";
import { supportService } from "@/services/support.service";
import type { AgentWorkload } from "@/types/support";

const { Title } = Typography;

export default function SupportTeamDashboard() {
  const { data, isLoading } = useQuery({
    queryKey: ["support-team-dashboard"],
    queryFn: () => supportService.getTeamDashboard(),
  });

  const stats = data?.stats;

  const agentColumns = [
    { title: "Agent", dataIndex: "agent_name" },
    {
      title: "Open Tickets",
      render: (_: unknown, r: AgentWorkload) => (
        <Tag color={r.open_tickets > 5 ? "red" : r.open_tickets > 0 ? "orange" : "green"}>
          {r.open_tickets}
        </Tag>
      ),
    },
    { title: "Resolved Today", dataIndex: "resolved_today" },
    {
      title: "Avg Resolution",
      render: (_: unknown, r: AgentWorkload) =>
        r.avg_resolution_minutes ? `${Math.round(r.avg_resolution_minutes)}m` : "—",
    },
  ];

  return (
    <div>
      <Title level={3}>Support Team Dashboard</Title>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card>
            <Statistic
              title="Open"
              value={stats?.total_open || 0}
              prefix={<InboxOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="Pending Player"
              value={stats?.total_pending_player || 0}
              prefix={<PauseCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="Pending Internal"
              value={stats?.total_pending_internal || 0}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="Resolved Today"
              value={stats?.total_resolved_today || 0}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="SLA Breaches"
              value={stats?.sla_breach_count || 0}
              prefix={<WarningOutlined />}
              valueStyle={{ color: (stats?.sla_breach_count || 0) > 0 ? "#cf1322" : "#3f8600" }}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="Avg Resolution"
              value={stats?.avg_resolution_minutes ? Math.round(stats.avg_resolution_minutes) : 0}
              suffix="min"
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="Agent Workload" loading={isLoading}>
            <Table
              columns={agentColumns}
              dataSource={data?.agent_workloads || []}
              rowKey="agent_id"
              pagination={false}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="Current SLA Breaches" loading={isLoading}>
            {(data?.sla_breaches || []).length === 0 ? (
              <Badge color="green" text="No active SLA breaches" />
            ) : (
              <ul>
                {(data?.sla_breaches || []).map((t) => (
                  <li key={t.id} style={{ marginBottom: 8 }}>
                    <Tag color="red">{t.priority.toUpperCase()}</Tag>
                    <span>{t.subject}</span>
                    <span style={{ color: "#888", marginLeft: 8 }}>— {t.player_email}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Col>
      </Row>

      <Card title="Tickets by Category" style={{ marginTop: 16 }} loading={isLoading}>
        <Row gutter={16}>
          {stats?.by_category &&
            Object.entries(stats.by_category).map(([cat, count]) => (
              <Col span={4} key={cat}>
                <Card size="small">
                  <Statistic title={cat.toUpperCase()} value={count} />
                </Card>
              </Col>
            ))}
        </Row>
      </Card>
    </div>
  );
}
