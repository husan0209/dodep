import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Space,
  Button,
  Select,
  Card,
  Typography,
  Drawer,
  Descriptions,
  message,
} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { paymentsService } from "@/services/payments.service";
import type { P2PTransaction, P2PActionPayload } from "@/types/payments";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  pending: "orange",
  confirmed: "green",
  rejected: "red",
  sent: "blue",
  completed: "cyan",
};

export default function P2PQueue() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>("pending");
  const [typeFilter, setTypeFilter] = useState<string | undefined>();
  const [selected, setSelected] = useState<P2PTransaction | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["p2p-transactions", statusFilter, typeFilter],
    queryFn: () =>
      paymentsService.getP2PTransactions({
        status: statusFilter as any,
        type: typeFilter as any,
        page: 1,
        page_size: 50,
      }),
  });

  const handleAction = async (action: P2PActionPayload["action"]) => {
    if (!selected) return;
    try {
      await paymentsService.actionP2P(selected.id, { action });
      message.success(`Transaction ${action}d`);
      setDrawerOpen(false);
      refetch();
    } catch {
      message.error("Action failed");
    }
  };

  const columns = [
    { title: "Player", render: (_: unknown, r: P2PTransaction) => r.player_email },
    {
      title: "Type",
      render: (_: unknown, r: P2PTransaction) => (
        <Tag color={r.type === "deposit" ? "blue" : "purple"}>{r.type.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Amount",
      render: (_: unknown, r: P2PTransaction) => `${r.amount} ${r.currency}`,
    },
    { title: "Method", dataIndex: "method" },
    {
      title: "Status",
      render: (_: unknown, r: P2PTransaction) => (
        <Tag color={STATUS_COLORS[r.status]}>{r.status.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Wait Time",
      render: (_: unknown, r: P2PTransaction) => `${r.hours_waiting}h`,
    },
    {
      title: "Actions",
      render: (_: unknown, r: P2PTransaction) => (
        <Button
          icon={<EyeOutlined />}
          onClick={() => {
            setSelected(r);
            setDrawerOpen(true);
          }}
        >
          Review
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>P2P Payment Queue</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Select
            value={statusFilter}
            allowClear
            onChange={setStatusFilter}
            style={{ width: 140 }}
            options={[
              { value: "pending", label: "Pending" },
              { value: "confirmed", label: "Confirmed" },
              { value: "rejected", label: "Rejected" },
              { value: "sent", label: "Sent" },
              { value: "completed", label: "Completed" },
            ]}
          />
          <Select
            placeholder="Type"
            allowClear
            onChange={setTypeFilter}
            style={{ width: 120 }}
            options={[
              { value: "deposit", label: "Deposit" },
              { value: "withdrawal", label: "Withdrawal" },
            ]}
          />
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

      <Drawer
        title="P2P Transaction Review"
        width={640}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        {selected && (
          <>
            <Descriptions column={1} bordered>
              <Descriptions.Item label="Player">{selected.player_email}</Descriptions.Item>
              <Descriptions.Item label="Type">{selected.type.toUpperCase()}</Descriptions.Item>
              <Descriptions.Item label="Amount">
                {selected.amount} {selected.currency}
              </Descriptions.Item>
              <Descriptions.Item label="Method">{selected.method}</Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={STATUS_COLORS[selected.status]}>{selected.status.toUpperCase()}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Receipt">
                {selected.receipt_url || "—"}
              </Descriptions.Item>
              <Descriptions.Item label="Wait Time">{selected.hours_waiting} hours</Descriptions.Item>
              <Descriptions.Item label="Notes">{selected.notes || "—"}</Descriptions.Item>
            </Descriptions>

            <Space style={{ marginTop: 24 }}>
              {selected.type === "deposit" && selected.status === "pending" && (
                <>
                  <Button
                    type="primary"
                    icon={<CheckCircleOutlined />}
                    onClick={() => handleAction("confirm")}
                  >
                    Confirm
                  </Button>
                  <Button danger icon={<CloseCircleOutlined />} onClick={() => handleAction("reject")}>
                    Reject
                  </Button>
                </>
              )}
              {selected.type === "withdrawal" && selected.status === "pending" && (
                <Button icon={<SendOutlined />} onClick={() => handleAction("mark_sent")}>
                  Mark as Sent
                </Button>
              )}
            </Space>
          </>
        )}
      </Drawer>
    </div>
  );
}
