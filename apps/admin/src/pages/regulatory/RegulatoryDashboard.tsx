import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, Typography, Table, Tag, Statistic, Row, Col, Button, Space } from "antd";
import { ReloadOutlined, FileTextOutlined, WarningOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";

const { Title } = Typography;

export default function RegulatoryDashboard() {
  const [page, setPage] = useState(1);

  const { data: reportsData, isLoading: reportsLoading, refetch: refetchReports } = useQuery({
    queryKey: ["regulatory-reports", page],
    queryFn: () => regulatoryService.getReports({ page }),
  });

  const reports = reportsData?.data || [];

  const columns = [
    { title: "Jurisdiction", dataIndex: "jurisdiction", render: (v: string) => v.toUpperCase() },
    { title: "Type", dataIndex: "report_type" },
    { title: "Period", render: (_: any, r: any) => `${r.period_start} — ${r.period_end}` },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => {
        const colors: Record<string, string> = {
          draft: "default",
          generated: "processing",
          submitted: "success",
          accepted: "green",
          rejected: "red",
        };
        return <Tag color={colors[v] || "default"}>{v.toUpperCase()}</Tag>;
      },
    },
    { title: "Submitted", dataIndex: "submitted_at", render: (v?: string) => (v ? new Date(v).toLocaleDateString() : "—") },
    { title: "Ref", dataIndex: "regulator_ref", render: (v?: string) => v || "—" },
  ];

  return (
    <div>
      <Title level={3}>Regulatory Dashboard</Title>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic title="Draft Reports" value={reports.filter((r: any) => r.status === "draft").length} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Submitted" value={reports.filter((r: any) => r.status === "submitted").length} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Active SARs" value={0} prefix={<WarningOutlined style={{ color: "#cf1322" }} />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Open Complaints" value={0} prefix={<FileTextOutlined />} />
          </Card>
        </Col>
      </Row>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => refetchReports()} loading={reportsLoading}>
          Refresh
        </Button>
      </Space>
      <Card title="Regulatory Reports">
        <Table
          rowKey="id"
          dataSource={reports}
          columns={columns}
          loading={reportsLoading}
          pagination={{ current: page, pageSize: 20, total: reportsData?.pagination?.total, onChange: setPage }}
        />
      </Card>
    </div>
  );
}
