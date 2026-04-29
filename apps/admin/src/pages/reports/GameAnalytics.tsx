import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, Typography, DatePicker, Button, Table, Tag, Space, Select, Row, Col, Statistic } from "antd";
import { ReloadOutlined, DownloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { reportsService } from "@/services/reports.service";

const { Title } = Typography;

export default function GameAnalytics() {
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([
    dayjs().subtract(30, "day"),
    dayjs(),
  ]);
  const [provider, setProvider] = useState<string | undefined>();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["game-analytics", dateRange[0]?.format("YYYY-MM-DD"), dateRange[1]?.format("YYYY-MM-DD"), provider],
    queryFn: () =>
      reportsService.getGameAnalytics({
        from: dateRange[0]?.format("YYYY-MM-DD"),
        to: dateRange[1]?.format("YYYY-MM-DD"),
        provider,
      }),
  });

  const report: any = data || {};

  const columns = [
    { title: "Game ID", dataIndex: "game_id", render: (v: string) => v.slice(0, 8) },
    { title: "Name", dataIndex: "game_name" },
    { title: "Provider", dataIndex: "provider" },
    { title: "GGR", dataIndex: "ggr", render: (v: string) => `$${v}` },
    { title: "Rounds", dataIndex: "rounds" },
    { title: "Players", dataIndex: "unique_players" },
    {
      title: "Actual RTP",
      dataIndex: "actual_rtp",
      render: (v: number) => <Tag color={v < 90 ? "red" : v < 95 ? "orange" : "green"}>{v?.toFixed(2)}%</Tag>,
    },
    { title: "Theoretical RTP", dataIndex: "theoretical_rtp", render: (v: number) => `${v?.toFixed(2)}%` },
  ];

  return (
    <div>
      <Title level={3}>Game Analytics</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <DatePicker.RangePicker
          value={dateRange as any}
          onChange={(dates) => setDateRange(dates as [dayjs.Dayjs | null, dayjs.Dayjs | null])}
        />
        <Select
          allowClear
          placeholder="Provider"
          style={{ width: 160 }}
          value={provider}
          onChange={setProvider}
          options={[
            { value: "netent", label: "NetEnt" },
            { value: "pragmatic", label: "Pragmatic Play" },
            { value: "evolution", label: "Evolution" },
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>
          Generate
        </Button>
        <Button icon={<DownloadOutlined />} disabled={!data}>
          Export CSV
        </Button>
      </Space>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic title="Total Games" value={report.rows?.length || 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Total Rounds" value={report.rows?.reduce((sum: number, r: any) => sum + (r.rounds || 0), 0) || 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Avg Actual RTP" value={report.avg_rtp || 0} suffix="%" precision={2} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="RTP Variance > 5%"
              value={report.variance_count || 0}
              valueStyle={{ color: report.variance_count > 0 ? "#cf1322" : "#3f8600" }}
            />
          </Card>
        </Col>
      </Row>
      <Card>
        <Table
          rowKey="game_id"
          dataSource={report.rows || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>
    </div>
  );
}
