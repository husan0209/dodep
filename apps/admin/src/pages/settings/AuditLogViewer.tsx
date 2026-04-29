import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Table,
  Tag,
  Space,
  Input,
  DatePicker,
  Select,
  Button,
} from "antd";
import { ReloadOutlined, DownloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { systemService } from "@/services/system.service";

const { Title } = Typography;

interface AuditLog {
  id: string;
  admin_id: string;
  admin_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details?: Record<string, unknown>;
  ip_address: string;
  created_at: string;
}

const ACTION_COLORS: Record<string, string> = {
  create: "green",
  update: "blue",
  delete: "red",
  login: "purple",
  logout: "default",
  export: "orange",
};

export default function AuditLogViewer() {
  const [search, setSearch] = useState("");
  const [actionFilter, setActionFilter] = useState<string | undefined>();
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([
    dayjs().subtract(7, "day"),
    dayjs(),
  ]);

  const { data, isLoading, refetch } = useQuery({
    queryKey: [
      "audit-logs",
      search,
      actionFilter,
      dateRange[0]?.format("YYYY-MM-DD"),
      dateRange[1]?.format("YYYY-MM-DD"),
    ],
    queryFn: () =>
      systemService.getAuditLogs({
        search: search || undefined,
        action: actionFilter,
        from: dateRange[0]?.format("YYYY-MM-DD"),
        to: dateRange[1]?.format("YYYY-MM-DD"),
      }) as Promise<AuditLog[]>,
  });

  const columns = [
    { title: "ID", dataIndex: "id", width: 80, render: (v: string) => v.slice(0, 8) },
    { title: "Admin", dataIndex: "admin_name" },
    {
      title: "Action",
      dataIndex: "action",
      render: (v: string) => <Tag color={ACTION_COLORS[v] || "default"}>{v.toUpperCase()}</Tag>,
    },
    {
      title: "Entity",
      dataIndex: "entity_type",
      render: (v: string, r: AuditLog) => `${v} #${r.entity_id.slice(0, 8)}`,
    },
    {
      title: "IP",
      dataIndex: "ip_address",
      render: (v: string) => <code>{v}</code>,
    },
    {
      title: "Time",
      dataIndex: "created_at",
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ];

  return (
    <div>
      <Title level={3}>Audit Log Viewer</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <DatePicker.RangePicker
          value={dateRange as any}
          onChange={(dates) => setDateRange(dates as [dayjs.Dayjs | null, dayjs.Dayjs | null])}
        />
        <Select
          allowClear
          placeholder="Action"
          style={{ width: 140 }}
          onChange={setActionFilter}
          options={[
            { value: "create", label: "Create" },
            { value: "update", label: "Update" },
            { value: "delete", label: "Delete" },
            { value: "login", label: "Login" },
            { value: "logout", label: "Logout" },
            { value: "export", label: "Export" },
          ]}
        />
        <Input.Search
          placeholder="Admin or entity..."
          allowClear
          onSearch={setSearch}
          style={{ width: 240 }}
        />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>
          Refresh
        </Button>
        <Button icon={<DownloadOutlined />} disabled={!data}>
          Export
        </Button>
      </Space>
      <Card>
        <Table
          rowKey="id"
          dataSource={data || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 50 }}
        />
      </Card>
    </div>
  );
}
