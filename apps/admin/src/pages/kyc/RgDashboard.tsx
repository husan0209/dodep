import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Card,
  Table,
  Tag,
  Button,
  Select,
  Typography,
  Space,
  Row,
  Col,
  Statistic,
} from "antd";
import {
  WarningOutlined,
  CheckCircleOutlined,
  UserOutlined,
  ClockCircleOutlined,
} from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";
import type { RgAlert } from "@/types/kyc";

const { Title } = Typography;

const SEVERITY_COLORS: Record<string, string> = {
  low: "blue",
  medium: "orange",
  high: "red",
  critical: "purple",
};

export default function RgDashboard() {
  const [severityFilter, setSeverityFilter] = useState<string | undefined>();

  const { data: alerts, isLoading, refetch } = useQuery({
    queryKey: ["rg-alerts", severityFilter],
    queryFn: () =>
      kycService.getRgAlerts({
        severity: severityFilter,
        acknowledged: false,
        page: 1,
      }),
  });

  const handleAcknowledge = async (id: string) => {
    try {
      await kycService.acknowledgeRgAlert(id);
      refetch();
    } catch {
      // handled by service
    }
  };

  const columns = [
    { title: "Player", render: (_: unknown, r: RgAlert) => r.player_email },
    {
      title: "Type",
      render: (_: unknown, r: RgAlert) => r.alert_type.replace(/_/g, " "),
    },
    {
      title: "Severity",
      render: (_: unknown, r: RgAlert) => (
        <Tag color={SEVERITY_COLORS[r.severity]}>{r.severity.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Created",
      dataIndex: "created_at",
    },
    {
      title: "Actions",
      render: (_: unknown, r: RgAlert) => (
        <Button icon={<CheckCircleOutlined />} onClick={() => handleAcknowledge(r.id)}>
          Acknowledge
        </Button>
      ),
    },
  ];

  const activeSelfExclusions = 0; // Placeholder until endpoint ready
  const activeCoolOffs = 0;

  return (
    <div>
      <Title level={3}>Responsible Gambling Dashboard</Title>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="Unacknowledged Alerts"
              value={alerts?.data?.length || 0}
              prefix={<WarningOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Active Self-Exclusions"
              value={activeSelfExclusions}
              prefix={<UserOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Active Cool-offs"
              value={activeCoolOffs}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Card
        title="RG Alerts"
        extra={
          <Space>
            <Select
              placeholder="Severity"
              allowClear
              onChange={setSeverityFilter}
              style={{ width: 120 }}
              options={[
                { value: "low", label: "Low" },
                { value: "medium", label: "Medium" },
                { value: "high", label: "High" },
                { value: "critical", label: "Critical" },
              ]}
            />
            <Button onClick={() => refetch()}>Refresh</Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={alerts?.data || []}
          rowKey="id"
          loading={isLoading}
          pagination={{ pageSize: 50, total: alerts?.pagination?.total }}
        />
      </Card>
    </div>
  );
}
