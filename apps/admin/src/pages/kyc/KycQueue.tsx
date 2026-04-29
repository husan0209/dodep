import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Space,
  Button,
  Select,
  Card,
  Badge,
  Typography,
  Drawer,
  Descriptions,
  message,
  Input,
  Modal,
} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
  EyeOutlined,
  UserAddOutlined,
} from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";
import { useAuthStore } from "@/stores/authStore";
import type { KycReviewItem, KycReviewPayload } from "@/types/kyc";

const { Title, Text } = Typography;
const { TextArea } = Input;

const PRIORITY_COLORS: Record<string, string> = {
  high: "red",
  medium: "orange",
  low: "blue",
};

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  pending: { label: "Pending", color: "default" },
  in_review: { label: "In Review", color: "processing" },
  approved: { label: "Approved", color: "success" },
  rejected: { label: "Rejected", color: "error" },
  resubmission_requested: { label: "Resubmission", color: "warning" },
};

export default function KycQueue() {
  const queryClient = useQueryClient();
  const adminId = useAuthStore((s) => s.adminId);
  const [statusFilter, setStatusFilter] = useState<string>("pending");
  const [priorityFilter, setPriorityFilter] = useState<string | undefined>();
  const [selectedReview, setSelectedReview] = useState<KycReviewItem | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [decisionModal, setDecisionModal] = useState<{ open: boolean; type: KycReviewPayload["decision"] | null }>({ open: false, type: null });
  const [decisionNotes, setDecisionNotes] = useState("");

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["kyc-queue", statusFilter, priorityFilter],
    queryFn: () =>
      kycService.getQueue({
        status: statusFilter as any,
        priority: priorityFilter as any,
        page: 1,
        page_size: 50,
      }),
  });

  const assignMutation = useMutation({
    mutationFn: ({ reviewId, assigneeId }: { reviewId: string; assigneeId: string }) =>
      kycService.assignReview(reviewId, assigneeId),
    onSuccess: () => {
      message.success("Assigned");
      queryClient.invalidateQueries({ queryKey: ["kyc-queue"] });
      refetch();
    },
    onError: () => message.error("Failed to assign"),
  });

  const reviewMutation = useMutation({
    mutationFn: ({ reviewId, payload }: { reviewId: string; payload: KycReviewPayload }) =>
      kycService.submitReview(reviewId, payload),
    onSuccess: () => {
      message.success("Review submitted");
      setDecisionModal({ open: false, type: null });
      setDecisionNotes("");
      setDrawerOpen(false);
      queryClient.invalidateQueries({ queryKey: ["kyc-queue"] });
      refetch();
    },
    onError: () => message.error("Failed to submit review"),
  });

  const handleReview = (item: KycReviewItem) => {
    setSelectedReview(item);
    setDrawerOpen(true);
  };

  const handleAssign = (record: KycReviewItem) => {
    if (!adminId) return;
    Modal.confirm({
      title: "Assign to yourself?",
      onOk: () => assignMutation.mutate({ reviewId: record.id, assigneeId: adminId }),
    });
  };

  const openDecisionModal = (type: KycReviewPayload["decision"]) => {
    if (type === "approve") {
      // Approve immediately without notes
      if (!selectedReview) return;
      reviewMutation.mutate({ reviewId: selectedReview.id, payload: { decision: "approve", notes: "Approved" } });
    } else {
      setDecisionModal({ open: true, type });
    }
  };

  const submitDecision = () => {
    if (!selectedReview || !decisionModal.type) return;
    reviewMutation.mutate({
      reviewId: selectedReview.id,
      payload: { decision: decisionModal.type, notes: decisionNotes || `${decisionModal.type}d` },
    });
  };

  const columns = [
    {
      title: "Player",
      render: (_: unknown, record: KycReviewItem) => (
        <Space direction="vertical" size={0}>
          <span>{record.player_email}</span>
          <span style={{ fontSize: 12, color: "#888" }}>
            @{record.player_username} · {record.player_group}
          </span>
        </Space>
      ),
    },
    { title: "Type", dataIndex: "document_type" },
    {
      title: "Priority",
      render: (_: unknown, record: KycReviewItem) => (
        <Tag color={PRIORITY_COLORS[record.priority]}>{record.priority.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Status",
      render: (_: unknown, record: KycReviewItem) => {
        const s = STATUS_LABELS[record.status];
        return <Badge status={s.color as any} text={s.label} />;
      },
    },
    {
      title: "Wait Time",
      render: (_: unknown, record: KycReviewItem) => `${record.wait_time_minutes}m`,
    },
    {
      title: "Assigned",
      render: (_: unknown, record: KycReviewItem) =>
        record.assigned_to_name || <Tag color="default">Unassigned</Tag>,
    },
    {
      title: "Actions",
      render: (_: unknown, record: KycReviewItem) => (
        <Space>
          <Button icon={<EyeOutlined />} onClick={() => handleReview(record)}>
            Review
          </Button>
          {!record.assigned_to_name && (
            <Button icon={<UserAddOutlined />} onClick={() => handleAssign(record)} loading={assignMutation.isPending}>
              Assign
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>KYC Review Queue</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select
            value={statusFilter}
            onChange={setStatusFilter}
            style={{ width: 160 }}
            options={[
              { value: "pending", label: "Pending" },
              { value: "in_review", label: "In Review" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Rejected" },
            ]}
          />
          <Select
            placeholder="Priority"
            allowClear
            onChange={setPriorityFilter}
            style={{ width: 120 }}
            options={[
              { value: "high", label: "High" },
              { value: "medium", label: "Medium" },
              { value: "low", label: "Low" },
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
        title="KYC Document Review"
        width={720}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        {selectedReview && (
          <>
            <Descriptions column={1} bordered>
              <Descriptions.Item label="Player">{selectedReview.player_email}</Descriptions.Item>
              <Descriptions.Item label="Username">@{selectedReview.player_username}</Descriptions.Item>
              <Descriptions.Item label="Document Type">{selectedReview.document_type}</Descriptions.Item>
              <Descriptions.Item label="Priority">
                <Tag color={PRIORITY_COLORS[selectedReview.priority]}>
                  {selectedReview.priority.toUpperCase()}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Wait Time">
                {selectedReview.wait_time_minutes} minutes
              </Descriptions.Item>
            </Descriptions>

            <div style={{ marginTop: 24 }}>
              <Title level={5}>Document Preview</Title>
              <div
                style={{
                  width: "100%",
                  height: 300,
                  background: "#f0f0f0",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  borderRadius: 8,
                }}
              >
                <Typography.Text type="secondary">Document viewer placeholder</Typography.Text>
              </div>
            </div>

            <div style={{ marginTop: 24, display: "flex", gap: 12 }}>
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={() => openDecisionModal("approve")}
              >
                Approve
              </Button>
              <Button danger icon={<CloseCircleOutlined />} onClick={() => openDecisionModal("reject")}>
                Reject
              </Button>
              <Button onClick={() => openDecisionModal("resubmission")}>Request Resubmission</Button>
            </div>
          </>
        )}
      </Drawer>

      {/* Decision Reason Modal */}
      <Modal
        title={`${decisionModal.type === "reject" ? "Reject" : "Request Resubmission"} — Reason Required`}
        open={decisionModal.open}
        onOk={submitDecision}
        onCancel={() => setDecisionModal({ open: false, type: null })}
        confirmLoading={reviewMutation.isPending}
        okText="Confirm"
        okButtonProps={{ danger: decisionModal.type === "reject" }}
      >
        <TextArea
          rows={4}
          placeholder="Enter reason / rejection notes..."
          value={decisionNotes}
          onChange={(e) => setDecisionNotes(e.target.value)}
          style={{ marginTop: 12 }}
        />
      </Modal>
    </div>
  );
}
