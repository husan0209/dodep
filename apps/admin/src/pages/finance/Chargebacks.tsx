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
  Statistic,
  Row,
  Col,
  message,
} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { paymentsService } from "@/services/payments.service";
import type { Chargeback, ChargebackActionPayload } from "@/types/payments";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  received: "blue",
  under_review: "orange",
  accepted: "red",
  fighting: "purple",
  won: "green",
  lost: "default",
};

export default function Chargebacks() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [selected, setSelected] = useState<Chargeback | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["chargebacks", statusFilter],
    queryFn: () =>
      paymentsService.getChargebacks({
        status: statusFilter as any,
        page: 1,
        page_size: 50,
      }),
  });

  const { data: stats } = useQuery({
    queryKey: ["chargeback-stats"],
    queryFn: () => paymentsService.getChargebackStats(),
  });

  const handleAction = async (action: ChargebackActionPayload["action"]) => {
    if (!selected) return;
    try {
      await paymentsService.actionChargeback(selected.id, { action });
      message.success(`Chargeback ${action}d`);
      setDrawerOpen(false);
      refetch();
    } catch {
      message.error("Action failed");
    }
  };

  const columns = [
    { title: "Player", render: (_: unknown, r: Chargeback) => r.player_email },
    {
      title: "Amount",
      render: (_: unknown, r: Chargeback) => `${r.amount} ${r.currency}`,
    },
    { title: "Gateway", dataIndex: "gateway" },
    {
      title: "Status",
      render: (_: unknown, r: Chargeback) => (
        <Tag color={STATUS_COLORS[r.status]}>{r.status.replace(/_/g, " ").toUpperCase()}</Tag>
      ),
    },
    {
      title: "Deadline",
      render: (_: unknown, r: Chargeback) =>
        r.days_to_deadline !== null ? (
          <span style={{ color: r.days_to_deadline <= 3 ? "red" : undefined }}>
            {r.days_to_deadline}d
          </span>
        ) : (
          "—"
        ),
    },
    {
      title: "Actions",
      render: (_: unknown, r: Chargeback) => (
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
      <Title level={3}>Chargeback Management</Title>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="This Month" value={stats?.total_this_month || 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Amount" value={stats?.amount_this_month || "0"} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="CB Rate %"
              value={stats?.cb_rate_pct || 0}
              suffix="%"
              valueStyle={{ color: (stats?.cb_rate_pct || 0) > 0.5 ? "#cf1322" : "#3f8600" }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Fight Win Rate" value={stats?.fight_win_rate_pct || 0} suffix="%" />
          </Card>
        </Col>
      </Row>

      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Select
            placeholder="Status"
            allowClear
            onChange={setStatusFilter}
            style={{ width: 160 }}
            options={[
              { value: "received", label: "Received" },
              { value: "under_review", label: "Under Review" },
              { value: "accepted", label: "Accepted" },
              { value: "fighting", label: "Fighting" },
              { value: "won", label: "Won" },
              { value: "lost", label: "Lost" },
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
        title="Chargeback Detail"
        width={700}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        {selected && (
          <>
            <Descriptions column={1} bordered>
              <Descriptions.Item label="Player">{selected.player_email}</Descriptions.Item>
              <Descriptions.Item label="Amount">
                {selected.amount} {selected.currency}
              </Descriptions.Item>
              <Descriptions.Item label="Gateway">{selected.gateway}</Descriptions.Item>
              <Descriptions.Item label="Gateway CB ID">{selected.gateway_cb_id || "—"}</Descriptions.Item>
              <Descriptions.Item label="Reason Code">{selected.reason_code || "—"}</Descriptions.Item>
              <Descriptions.Item label="Reason">{selected.reason_text || "—"}</Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={STATUS_COLORS[selected.status]}>
                  {selected.status.replace(/_/g, " ").toUpperCase()}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Received">{selected.received_at}</Descriptions.Item>
              <Descriptions.Item label="Deadline">
                {selected.deadline_at || "—"} ({selected.days_to_deadline ?? "—"}d)
              </Descriptions.Item>
              <Descriptions.Item label="Assigned">
                {selected.assigned_to_name || "Unassigned"}
              </Descriptions.Item>
              <Descriptions.Item label="Notes">{selected.notes || "—"}</Descriptions.Item>
            </Descriptions>

            <Space style={{ marginTop: 24 }}>
              <Button
                type="primary"
                danger
                icon={<CloseCircleOutlined />}
                onClick={() => handleAction("accept")}
              >
                Accept CB
              </Button>
              <Button icon={<SafetyCertificateOutlined />} onClick={() => handleAction("fight")}>
                Fight CB
              </Button>
              <Button onClick={() => handleAction("assign")}>Assign to Me</Button>
            </Space>
          </>
        )}
      </Drawer>
    </div>
  );
}
