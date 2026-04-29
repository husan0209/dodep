import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, Table, Statistic, Row, Col, Select, Typography, Tag } from "antd";
import { TeamOutlined, FieldTimeOutlined, WarningOutlined } from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";

const { Title } = Typography;

export default function TeamDashboard() {
  const [period, setPeriod] = useState<"today" | "week" | "month">("today");

  const { data, isLoading } = useQuery({
    queryKey: ["kyc-team-stats", period],
    queryFn: () => kycService.getTeamStats(period),
  });

  const columns = [
    { title: "Officer", dataIndex: "officer_name" },
    { title: "Date", dataIndex: "metric_date" },
    { title: "Reviewed", dataIndex: "reviewed_count" },
    {
      title: "Avg Time",
      render: (_: unknown, r: any) => `${r.avg_review_time_minutes} min`,
    },
    { title: "Approved", dataIndex: "approve_count" },
    { title: "Rejected", dataIndex: "reject_count" },
    {
      title: "SLA Breaches",
      render: (_: unknown, r: any) =>
        r.sla_breach_count > 0 ? <Tag color="red">{r.sla_breach_count}</Tag> : <Tag color="green">0</Tag>,
    },
  ];

  return (
    <div>
      <Title level={3}>KYC Team Metrics</Title>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="Queue Depth"
              value={data?.today.queue_depth || 0}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Avg Review Time"
              value={data?.today.avg_review_minutes || 0}
              suffix="min"
              prefix={<FieldTimeOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="SLA Breaches"
              value={data?.today.sla_breaches || 0}
              prefix={<WarningOutlined />}
              valueStyle={{ color: (data?.today.sla_breaches || 0) > 0 ? "#cf1322" : "#3f8600" }}
            />
          </Card>
        </Col>
      </Row>

      <Card
        title="By Officer"
        extra={
          <Select value={period} onChange={setPeriod} style={{ width: 120 }}>
            <Select.Option value="today">Today</Select.Option>
            <Select.Option value="week">This Week</Select.Option>
            <Select.Option value="month">This Month</Select.Option>
          </Select>
        }
      >
        <Table
          columns={columns}
          dataSource={data?.officers || []}
          rowKey="officer_id"
          loading={isLoading}
          pagination={false}
        />
      </Card>
    </div>
  );
}
