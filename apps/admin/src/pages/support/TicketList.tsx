import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Space,
  Button,
  Select,
  Input,
  Card,
  Typography,
  Badge,
} from "antd";
import { EyeOutlined, ReloadOutlined, PlusOutlined } from "@ant-design/icons";
import { supportService } from "@/services/support.service";
import type { SupportTicket, TicketFilters } from "@/types/support";
import { useNavigate } from "react-router-dom";

const { Title } = Typography;
const { Search } = Input;

const STATUS_COLORS: Record<string, string> = {
  open: "blue",
  pending_player: "orange",
  pending_internal: "cyan",
  resolved: "green",
  closed: "default",
};

const PRIORITY_COLORS: Record<string, string> = {
  low: "blue",
  normal: "green",
  high: "orange",
  urgent: "red",
};

export default function TicketList() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<TicketFilters>({ page: 1, page_size: 50 });

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["tickets", filters],
    queryFn: () => supportService.getTickets(filters),
  });

  const columns = [
    { title: "ID", dataIndex: "id", width: 80, ellipsis: true },
    {
      title: "Subject",
      render: (_: unknown, r: SupportTicket) => (
        <Space direction="vertical" size={0}>
          <span>{r.subject}</span>
          <span style={{ fontSize: 12, color: "#888" }}>
            {r.player_email} · {r.category}
          </span>
        </Space>
      ),
    },
    {
      title: "Priority",
      render: (_: unknown, r: SupportTicket) => (
        <Tag color={PRIORITY_COLORS[r.priority]}>{r.priority.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Status",
      render: (_: unknown, r: SupportTicket) => (
        <Badge status={STATUS_COLORS[r.status] as any} text={r.status.replace(/_/g, " ").toUpperCase()} />
      ),
    },
    {
      title: "Assigned",
      render: (_: unknown, r: SupportTicket) => r.assigned_to_name || <Tag>Unassigned</Tag>,
    },
    {
      title: "SLA",
      render: (_: unknown, r: SupportTicket) =>
        r.is_sla_breach ? <Tag color="red">BREACH</Tag> : <Tag color="green">OK</Tag>,
    },
    {
      title: "Last Update",
      render: (_: unknown, r: SupportTicket) => r.last_message_at || r.updated_at,
    },
    {
      title: "Actions",
      render: (_: unknown, r: SupportTicket) => (
        <Button icon={<EyeOutlined />} onClick={() => navigate(`/support/tickets/${r.id}`)}>
          Open
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Support Tickets</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Search
            placeholder="Search subject or player..."
            allowClear
            onSearch={(v) => setFilters((f) => ({ ...f, search: v, page: 1 }))}
            style={{ width: 240 }}
          />
          <Select
            placeholder="Status"
            allowClear
            onChange={(v) => setFilters((f) => ({ ...f, status: v, page: 1 }))}
            style={{ width: 140 }}
            options={[
              { value: "open", label: "Open" },
              { value: "pending_player", label: "Pending Player" },
              { value: "pending_internal", label: "Pending Internal" },
              { value: "resolved", label: "Resolved" },
              { value: "closed", label: "Closed" },
            ]}
          />
          <Select
            placeholder="Category"
            allowClear
            onChange={(v) => setFilters((f) => ({ ...f, category: v, page: 1 }))}
            style={{ width: 140 }}
            options={[
              { value: "payment", label: "Payment" },
              { value: "bonus", label: "Bonus" },
              { value: "technical", label: "Technical" },
              { value: "account", label: "Account" },
              { value: "kyc", label: "KYC" },
              { value: "general", label: "General" },
            ]}
          />
          <Select
            placeholder="Priority"
            allowClear
            onChange={(v) => setFilters((f) => ({ ...f, priority: v, page: 1 }))}
            style={{ width: 120 }}
            options={[
              { value: "low", label: "Low" },
              { value: "normal", label: "Normal" },
              { value: "high", label: "High" },
              { value: "urgent", label: "Urgent" },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            Refresh
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate("/support/tickets/new")}>
            New Ticket
          </Button>
        </Space>
      </Card>

      <Table
        columns={columns}
        dataSource={data?.data || []}
        rowKey="id"
        loading={isLoading}
        pagination={{
          pageSize: filters.page_size,
          current: filters.page,
          total: data?.pagination?.total,
          onChange: (page) => setFilters((f) => ({ ...f, page })),
        }}
      />
    </div>
  );
}
