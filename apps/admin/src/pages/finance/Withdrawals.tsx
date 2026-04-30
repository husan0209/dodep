import {
  Card,
  Typography,
  Space,
  Select,
  Button,
  Modal,
  Input,
  message,
} from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { financeService } from "@/services/finance.service";
import { formatDate } from "@/utils/format";
import { WITHDRAWAL_STATUSES } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import { hasPermission } from "@/utils/permissions";
import { useAuthStore } from "@/stores/authStore";
import type { ColumnsType } from "antd/es/table";
import type { Withdrawal } from "@/types/finance";

const { Title } = Typography;

export default function Withdrawals() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const [rejectModal, setRejectModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [rejectReason, setRejectReason] = useState("");
  const queryClient = useQueryClient();
  const { permissions } = useAuthStore();

  const { data, isLoading } = useQuery({
    queryKey: ["withdrawals", page, pageSize, status],
    queryFn: () =>
      financeService.getWithdrawals({ page, page_size: pageSize, status }),
  });

  const approveMutation = useMutation({
    mutationFn: (id: string) => financeService.approveWithdrawal(id),
    onSuccess: () => {
      message.success("Withdrawal approved");
      queryClient.invalidateQueries({ queryKey: ["withdrawals"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const rejectMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      financeService.rejectWithdrawal(id, reason),
    onSuccess: () => {
      message.success("Withdrawal rejected");
      queryClient.invalidateQueries({ queryKey: ["withdrawals"] });
      setRejectModal({ open: false, id: null });
      setRejectReason("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const canApprove = hasPermission(permissions, "withdrawal.approve_small");

  const columns: ColumnsType<Withdrawal> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    {
      title: "User ID",
      dataIndex: "user_id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    {
      title: "Amount",
      dataIndex: "amount",
      render: (v: string, r: any) => (
        <MoneyDisplay amount={v} currency={r.currency_code as string} />
      ),
    },
    { title: "Method", dataIndex: "method" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <StatusTag status={v} config={WITHDRAWAL_STATUSES} />
      ),
    },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Reviewed By",
      dataIndex: "reviewed_by",
      render: (v: string) => v || "—",
    },
    {
      title: "Actions",
      key: "actions",
      width: 200,
      render: (_, record) => {
        if (record.status !== "pending" || !canApprove) return "—";
        return (
          <Space>
            <Button
              size="small"
              type="primary"
              onClick={() => approveMutation.mutate(record.id)}
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
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Withdrawals
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Status"
            allowClear
            style={{ width: 160 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={Object.entries(WITHDRAWAL_STATUSES).map(([key, val]) => ({
              label: val.label,
              value: key,
            }))}
          />
        </Space>
        <DataTable
          data={data?.data || []}
          columns={columns}
          loading={isLoading}
          total={data?.pagination.total || 0}
          page={page}
          pageSize={pageSize}
          onPageChange={(p, ps) => {
            setPage(p);
            setPageSize(ps);
          }}
        />
      </Card>

      <Modal
        title="Reject Withdrawal"
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
    </div>
  );
}
