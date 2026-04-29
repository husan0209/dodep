import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Card,
  Typography,
  DatePicker,
  Button,
  Table,
  Tag,
  Space,
  Select,
  Row,
  Col,
  Statistic,
} from "antd";
import { ReloadOutlined, DownloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { paymentsService } from "@/services/payments.service";

const { Title } = Typography;

export default function FinancialReports() {
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([
    dayjs().subtract(30, "day"),
    dayjs(),
  ]);
  const [reportType, setReportType] = useState("summary");

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["financial-report", dateRange[0]?.format("YYYY-MM-DD"), dateRange[1]?.format("YYYY-MM-DD"), reportType],
    queryFn: () =>
      paymentsService.getFinancialReport({
        from: dateRange[0]?.format("YYYY-MM-DD"),
        to: dateRange[1]?.format("YYYY-MM-DD"),
        type: reportType,
      }),
  });

  const report: any = data || {};

  const columns = [
    { title: "Period", dataIndex: "period" },
    { title: "Deposits", dataIndex: "deposits" },
    { title: "Withdrawals", dataIndex: "withdrawals" },
    { title: "Net Revenue", dataIndex: "net_revenue" },
    { title: "Chargebacks", dataIndex: "chargebacks" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={v === "finalized" ? "green" : "orange"}>{v}</Tag>,
    },
  ];

  return (
    <div>
      <Title level={3}>Financial Reports</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <DatePicker.RangePicker
          value={dateRange as any}
          onChange={(dates) => setDateRange(dates as [dayjs.Dayjs | null, dayjs.Dayjs | null])}
        />
        <Select
          value={reportType}
          onChange={setReportType}
          style={{ width: 160 }}
          options={[
            { value: "summary", label: "Summary" },
            { value: "detailed", label: "Detailed" },
            { value: "gateway", label: "By Gateway" },
            { value: "currency", label: "By Currency" },
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
            <Statistic title="Total Deposits" value={report.total_deposits || 0} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Total Withdrawals" value={report.total_withdrawals || 0} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Net Revenue" value={report.net_revenue || 0} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Chargebacks" value={report.total_chargebacks || 0} prefix="$" valueStyle={{ color: "#cf1322" }} />
          </Card>
        </Col>
      </Row>
      <Card>
        <Table
          rowKey="period"
          dataSource={report.rows || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>
    </div>
  );
}
