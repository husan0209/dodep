import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, Typography, DatePicker, Button, Table, Tag, Space, Select, Row, Col, Statistic } from "antd";
import { ReloadOutlined, DownloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { usersService } from "@/services/users.service";

const { Title } = Typography;

export default function PlayerAnalytics() {
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([dayjs().subtract(30, "day"), dayjs()]);
  const [metric, setMetric] = useState("ltv");

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["player-analytics", dateRange[0]?.format("YYYY-MM-DD"), dateRange[1]?.format("YYYY-MM-DD"), metric],
    queryFn: () =>
      usersService.getPlayerAnalytics({
        from: dateRange[0]?.format("YYYY-MM-DD"),
        to: dateRange[1]?.format("YYYY-MM-DD"),
        metric,
      }) as Promise<any>,
  });

  const columns = [
    { title: "Player ID", dataIndex: "user_id", render: (v: string) => v.slice(0, 8) },
    { title: "Metric", dataIndex: "metric_value", render: (v: number) => v.toLocaleString() },
    { title: "Segment", dataIndex: "segment", render: (v?: string) => (v ? <Tag>{v}</Tag> : "—") },
    { title: "Last Active", dataIndex: "last_active", render: (v: string) => new Date(v).toLocaleDateString() },
  ];

  return (
    <div>
      <Title level={3}>Player Analytics</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <DatePicker.RangePicker value={dateRange as any} onChange={(dates) => setDateRange(dates as any)} />
        <Select value={metric} onChange={setMetric} style={{ width: 160 }} options={[
          { value: "ltv", label: "Lifetime Value" },
          { value: "activity", label: "Activity Score" },
          { value: "churn_risk", label: "Churn Risk" },
          { value: "arpu", label: "ARPU" },
        ]} />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>Generate</Button>
        <Button icon={<DownloadOutlined />} disabled={!data}>Export CSV</Button>
      </Space>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}><Card><Statistic title="Players Analyzed" value={data?.count || 0} /></Card></Col>
        <Col span={8}><Card><Statistic title="Avg Metric" value={data?.avg || 0} precision={2} /></Card></Col>
        <Col span={8}><Card><Statistic title="Top Segment" value={data?.top_segment || "—"} /></Card></Col>
      </Row>
      <Card>
        <Table rowKey="user_id" dataSource={data?.rows || []} columns={columns} loading={isLoading} pagination={{ pageSize: 20 }} />
      </Card>
    </div>
  );
}
