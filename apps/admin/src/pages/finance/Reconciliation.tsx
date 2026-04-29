import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Button,
  Card,
  Typography,
  DatePicker,
  Space,
  message,
} from "antd";
import { ReloadOutlined, PlayCircleOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { paymentsService } from "@/services/payments.service";
import type { ReconciliationRecord } from "@/types/payments";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  pending: "orange",
  resolved: "green",
  investigating: "red",
};

export default function Reconciliation() {
  const [date, setDate] = useState<dayjs.Dayjs | null>(dayjs().subtract(1, "day"));

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["reconciliation", date?.format("YYYY-MM-DD")],
    queryFn: () =>
      paymentsService.getReconciliation({
        date: date?.format("YYYY-MM-DD"),
        page: 1,
      }),
  });

  const handleRun = async () => {
    if (!date) return;
    try {
      await paymentsService.runReconciliation(date.format("YYYY-MM-DD"));
      message.success("Reconciliation started");
      refetch();
    } catch {
      message.error("Failed to start reconciliation");
    }
  };

  const columns = [
    { title: "Date", dataIndex: "recon_date" },
    { title: "Gateway", dataIndex: "gateway" },
    {
      title: "Expected",
      render: (_: unknown, r: ReconciliationRecord) => r.expected_balance,
    },
    {
      title: "Actual",
      render: (_: unknown, r: ReconciliationRecord) => r.actual_balance,
    },
    {
      title: "Difference",
      render: (_: unknown, r: ReconciliationRecord) => (
        <span style={{ color: Number(r.difference) !== 0 ? "#cf1322" : "#3f8600" }}>
          {r.difference}
        </span>
      ),
    },
    {
      title: "Pending TX",
      dataIndex: "pending_tx_count",
    },
    {
      title: "Failed Callbacks",
      dataIndex: "failed_callbacks",
    },
    {
      title: "Status",
      render: (_: unknown, r: ReconciliationRecord) => (
        <Tag color={STATUS_COLORS[r.status]}>{r.status.toUpperCase()}</Tag>
      ),
    },
    { title: "Notes", dataIndex: "notes", ellipsis: true },
  ];

  return (
    <div>
      <Title level={3}>Financial Reconciliation</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <DatePicker value={date} onChange={setDate} />
          <Button icon={<PlayCircleOutlined />} onClick={handleRun}>
            Run Reconciliation
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            Refresh
          </Button>
        </Space>
      </Card>

      <Table
        columns={columns}
        dataSource={data?.data || []}
        rowKey="id"
        loading={isLoading}
        pagination={{ pageSize: 50, total: data?.pagination?.total }}
      />
    </div>
  );
}
