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
import { CheckCircleOutlined, CloseCircleOutlined, EyeOutlined } from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";
import type { SofRequest } from "@/types/kyc";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  open: "blue",
  submitted: "cyan",
  under_review: "orange",
  approved: "green",
  rejected: "red",
  expired: "default",
};

export default function SofRequests() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [selected, setSelected] = useState<SofRequest | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["sof-requests", statusFilter],
    queryFn: () =>
      kycService.getSofRequests({
        status: statusFilter,
        page: 1,
        page_size: 50,
      }),
  });

  const handleReview = async (decision: "approve" | "reject") => {
    if (!selected) return;
    try {
      await kycService.reviewSof(selected.id, { decision });
      message.success(`SOF request ${decision}d`);
      setDrawerOpen(false);
      refetch();
    } catch {
      message.error("Failed to review SOF request");
    }
  };

  const columns = [
    { title: "Player", render: (_: unknown, r: SofRequest) => r.player_email },
    { title: "Trigger", dataIndex: "trigger_type" },
    {
      title: "Status",
      render: (_: unknown, r: SofRequest) => (
        <Tag color={STATUS_COLORS[r.status]}>{r.status.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Deadline",
      render: (_: unknown, r: SofRequest) => {
        const isOverdue = new Date(r.deadline_at) < new Date();
        return <span style={{ color: isOverdue ? "red" : undefined }}>{r.deadline_at}</span>;
      },
    },
    {
      title: "Actions",
      render: (_: unknown, r: SofRequest) => (
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
      <Title level={3}>Source of Funds Requests</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Select
            placeholder="Status"
            allowClear
            onChange={setStatusFilter}
            style={{ width: 140 }}
            options={[
              { value: "open", label: "Open" },
              { value: "submitted", label: "Submitted" },
              { value: "under_review", label: "Under Review" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Rejected" },
              { value: "expired", label: "Expired" },
            ]}
          />
          <Button onClick={() => refetch()}>Refresh</Button>
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
        title="SOF Request Review"
        width={640}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        {selected && (
          <>
            <Descriptions column={1} bordered>
              <Descriptions.Item label="Player">{selected.player_email}</Descriptions.Item>
              <Descriptions.Item label="Trigger">{selected.trigger_type}</Descriptions.Item>
              <Descriptions.Item label="Threshold">{selected.threshold_amount}</Descriptions.Item>
              <Descriptions.Item label="Period (days)">{selected.period_days}</Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={STATUS_COLORS[selected.status]}>{selected.status.toUpperCase()}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Deadline">{selected.deadline_at}</Descriptions.Item>
              <Descriptions.Item label="Notes">{selected.notes || "—"}</Descriptions.Item>
            </Descriptions>

            <Title level={5} style={{ marginTop: 24 }}>
              Documents ({selected.documents.length})
            </Title>
            {selected.documents.map((doc, idx) => (
              <Card key={idx} size="small" style={{ marginBottom: 8 }}>
                <div>
                  <strong>Type:</strong> {doc.type}
                </div>
                <div>
                  <strong>URL:</strong> {doc.file_url}
                </div>
              </Card>
            ))}

            <Space style={{ marginTop: 24 }}>
              <Button type="primary" icon={<CheckCircleOutlined />} onClick={() => handleReview("approve")}>
                Approve
              </Button>
              <Button danger icon={<CloseCircleOutlined />} onClick={() => handleReview("reject")}>
                Reject
              </Button>
            </Space>
          </>
        )}
      </Drawer>
    </div>
  );
}
