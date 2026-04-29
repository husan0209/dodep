import { useQuery } from "@tanstack/react-query";
import { Card, Statistic, Row, Col, Table, Tag, Typography } from "antd";
import { ClockCircleOutlined, ExclamationCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";

const { Title } = Typography;

export default function ExpiryMonitor() {
  const { data: stats } = useQuery({
    queryKey: ["kyc-expiry-stats"],
    queryFn: () => kycService.getExpiryStats(),
  });

  const { data: expiring30d, isLoading: loading30 } = useQuery({
    queryKey: ["kyc-expiring", 30],
    queryFn: () => kycService.getExpiringDocuments(30, 1),
  });

  const columns = [
    { title: "Player", render: (_: unknown, r: any) => r.player_email },
    { title: "Type", dataIndex: "type" },
    { title: "Status", render: (_: unknown, r: any) => <Tag>{r.status}</Tag> },
    {
      title: "Expires At",
      render: (_: unknown, r: any) => {
        const days = r.days_until_expiry;
        const color = days <= 7 ? "red" : days <= 30 ? "orange" : "green";
        return <Tag color={color}>{r.expires_at} ({days}d)</Tag>;
      },
    },
  ];

  return (
    <div>
      <Title level={3}>KYC Expiry Monitor</Title>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="Expiring in 30d"
              value={stats?.expiring_30d || 0}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ color: "#faad14" }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Expiring in 7d"
              value={stats?.expiring_7d || 0}
              prefix={<ExclamationCircleOutlined />}
              valueStyle={{ color: "#ff4d4f" }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Expired (active players)"
              value={stats?.expired || 0}
              prefix={<CloseCircleOutlined />}
              valueStyle={{ color: "#cf1322" }}
            />
          </Card>
        </Col>
      </Row>

      <Card title="Documents Expiring in 30 Days">
        <Table
          columns={columns}
          dataSource={expiring30d?.data || []}
          rowKey="id"
          loading={loading30}
          pagination={{ pageSize: 50 }}
        />
      </Card>
    </div>
  );
}
