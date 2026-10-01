import {
  Card,
  Typography,
  Space,
  Select,
  Button,
  Modal,
  Input,
  Tag,
  message,
} from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import { affiliatesService } from "@/services/affiliates.service";
import { formatDate } from "@/utils/format";
import { getErrorMessage } from "@/utils/errors";
import { hasPermission } from "@/utils/permissions";
import { useAuthStore } from "@/stores/authStore";
import type { ColumnsType } from "antd/es/table";

const { Title } = Typography;

interface Payout {
  id: string;
  affiliate_id: string;
  amount: string;
  currency: string;
  status: string;
  requested_at: string;
  approved_by: string;
  rejection_reason: string;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  requested: "orange",
  reviewing: "blue",
  approved: "cyan",
  processing: "geekblue",
  paid: "green",
  rejected: "red",
  failed: "red",
};

// Mirrors validPayoutTransitions in admin-bff (server enforces the same
// machine and returns 409 on a mismatch — this only hides dead buttons).
const REVIEWABLE_STATUSES = ["requested", "reviewing", "pending"];
const PAYABLE_STATUSES = ["approved"];

export default function AffiliatePayouts() {
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const [rejectModal, setRejectModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [rejectReason, setRejectReason] = useState("");
  const [approveRef, setApproveRef] = useState("");
  const [payModal, setPayModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [payRef, setPayRef] = useState("");
  const queryClient = useQueryClient();
  const { permissions } = useAuthStore();
  const canApprovePayouts = hasPermission(
    permissions,
    "affiliate.payout.approve",
  );

  const { data, isLoading } = useQuery({
    queryKey: ["affiliate-payouts", page, pageSize, status],
    queryFn: () => affiliatesService.getPayouts({ status, page, page_size: pageSize }),
  });

  const approveMutation = useMutation({
    mutationFn: ({ id, ref }: { id: string; ref: string }) =>
      affiliatesService.approvePayout(id, ref),
    onSuccess: () => {
      message.success("Payout approved");
      queryClient.invalidateQueries({ queryKey: ["affiliate-payouts"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const rejectMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      affiliatesService.rejectPayout(id, reason),
    onSuccess: () => {
      message.success("Payout rejected");
      queryClient.invalidateQueries({ queryKey: ["affiliate-payouts"] });
      setRejectModal({ open: false, id: null });
      setRejectReason("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const markPaidMutation = useMutation({
    mutationFn: ({ id, ref }: { id: string; ref: string }) =>
      affiliatesService.markPayoutPaid(id, ref),
    onSuccess: () => {
      message.success("Payout marked as paid");
      queryClient.invalidateQueries({ queryKey: ["affiliate-payouts"] });
      setPayModal({ open: false, id: null });
      setPayRef("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const columns: ColumnsType<Payout> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 100,
      render: (v: string) => v?.slice(0, 8),
    },
    {
      title: "Affiliate",
      dataIndex: "affiliate_id",
      width: 100,
      render: (v: string) => v?.slice(0, 8),
    },
    {
      title: "Amount",
      dataIndex: "amount",
      render: (v: string, r: Payout) => `${v} ${r.currency}`,
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <Tag color={STATUS_COLORS[v] || "default"}>{v}</Tag>
      ),
    },
    {
      title: "Requested",
      dataIndex: "requested_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Actions",
      key: "actions",
      width: 220,
      render: (_, record) => {
        if (!canApprovePayouts) return "—";
        const isReviewable = REVIEWABLE_STATUSES.includes(record.status);
        const isPayable = PAYABLE_STATUSES.includes(record.status);
        if (!isReviewable && !isPayable) return "—";
        return (
          <Space>
            {isReviewable && (
              <>
                <Button
                  size="small"
                  type="primary"
                  onClick={() =>
                    approveMutation.mutate({ id: record.id, ref: approveRef })
                  }
                  loading={approveMutation.isPending}
                >
                  Approve
                </Button>
                <Button
                  size="small"
                  danger
                  onClick={() => setRejectModal({ open: true, id: record.id })}
                >
                  Reject
                </Button>
              </>
            )}
            {isPayable && (
              <Button
                size="small"
                onClick={() => setPayModal({ open: true, id: record.id })}
              >
                Mark Paid
              </Button>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Affiliate Payouts
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Filter by status"
            allowClear
            style={{ width: 180 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={[
              { label: "Requested", value: "requested" },
              { label: "Reviewing", value: "reviewing" },
              { label: "Approved", value: "approved" },
              { label: "Paid", value: "paid" },
              { label: "Rejected", value: "rejected" },
            ]}
          />
        </Space>
        <DataTable
          data={(data?.data || []) as unknown as Payout[]}
          columns={columns}
          loading={isLoading}
          total={data?.pagination?.total || 0}
          page={page}
          pageSize={pageSize}
          onPageChange={(p) => setPage(p)}
        />
      </Card>

      <Modal
        title="Reject Payout"
        open={rejectModal.open}
        onOk={() =>
          rejectModal.id &&
          rejectMutation.mutate({ id: rejectModal.id, reason: rejectReason })
        }
        onCancel={() => {
          setRejectModal({ open: false, id: null });
          setRejectReason("");
        }}
        confirmLoading={rejectMutation.isPending}
        okButtonProps={{ danger: true }}
      >
        <Input.TextArea
          rows={3}
          placeholder="Reason for rejection..."
          value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)}
        />
      </Modal>

      <Modal
        title="Mark Payout as Paid"
        open={payModal.open}
        onOk={() =>
          payModal.id && markPaidMutation.mutate({ id: payModal.id, ref: payRef })
        }
        onCancel={() => {
          setPayModal({ open: false, id: null });
          setPayRef("");
        }}
        confirmLoading={markPaidMutation.isPending}
        okText="Confirm Paid"
      >
        <Input
          placeholder="Provider / PSP transaction reference"
          value={payRef}
          onChange={(e) => setPayRef(e.target.value)}
        />
      </Modal>
    </div>
  );
}
