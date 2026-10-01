import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Select,
  Space,
  Typography,
  message,
} from "antd";
import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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

const PAGE_SIZE = 20;

/**
 * Withdrawal review queue.
 *
 * The queue is read from Payment Service with a keyset cursor, so paging is
 * "next / previous" instead of page numbers: approving a request cannot make
 * rows shift between pages. Only `pending_review` rows can be decided — the
 * funds are reserved in the wallet and no provider payout exists yet.
 */
export default function Withdrawals() {
  const [status, setStatus] = useState<string>();
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [rejectModal, setRejectModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [rejectReason, setRejectReason] = useState("");
  const queryClient = useQueryClient();
  const { permissions } = useAuthStore();

  const cursor = cursorStack[cursorStack.length - 1] ?? "";
  const pageNumber = cursorStack.length + 1;

  const { data, isLoading, isError, error, isFetching } = useQuery({
    queryKey: ["withdrawals", status, cursor],
    queryFn: () =>
      financeService.getWithdrawals({
        status,
        page_size: PAGE_SIZE,
        page_token: cursor || undefined,
      }),
  });

  const refresh = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ["withdrawals"] });
  }, [queryClient]);

  const approveMutation = useMutation({
    mutationFn: (id: string) => financeService.approveWithdrawal(id),
    onSuccess: (w) => {
      message.success(`Withdrawal approved, payout ${w.status.toLowerCase()}`);
      refresh();
    },
    onError: (err: unknown) => message.error(getErrorMessage(err)),
  });

  const rejectMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      financeService.rejectWithdrawal(id, reason),
    onSuccess: () => {
      message.success("Withdrawal rejected, funds released");
      refresh();
      setRejectModal({ open: false, id: null });
      setRejectReason("");
    },
    onError: (err: unknown) => message.error(getErrorMessage(err)),
  });

  const canApprove = hasPermission(permissions, "withdrawal.approve_small");
  const isDecidable = (s: Withdrawal["status"]) => s === "pending_review";

  const columns: ColumnsType<Withdrawal> = useMemo(
    () => [
      {
        title: "ID",
        dataIndex: "id",
        width: 90,
        render: (v: string) => v.slice(0, 8),
      },
      {
        title: "User",
        dataIndex: "user_id",
        width: 90,
        render: (v: string) => v.slice(0, 8),
      },
      {
        title: "Amount",
        dataIndex: "amount",
        render: (v: string, r: Withdrawal) => (
          <MoneyDisplay amount={v} currency={r.currency_code} />
        ),
      },
      {
        title: "Status",
        dataIndex: "status",
        width: 160,
        render: (v: string) => (
          <StatusTag status={v} config={WITHDRAWAL_STATUSES} />
        ),
      },
      {
        title: "Destination",
        dataIndex: "destination",
        ellipsis: true,
        render: (v: string) =>
          v ? (
            <Typography.Text copyable={{ text: v }}>
              {v.length > 18 ? `${v.slice(0, 8)}…${v.slice(-6)}` : v}
            </Typography.Text>
          ) : (
            "—"
          ),
      },
      {
        title: "Payout Ref",
        dataIndex: "psp_reference",
        width: 130,
        render: (v: string) => (v ? <Typography.Text copyable>{v}</Typography.Text> : "—"),
      },
      {
        title: "Created",
        dataIndex: "created_at",
        width: 170,
        render: (v: string) => formatDate(v),
      },
      {
        title: "Decided By",
        dataIndex: "reviewed_by",
        width: 140,
        render: (v: string) => v || "—",
      },
      {
        title: "Actions",
        key: "actions",
        width: 190,
        fixed: "right",
        render: (_, record) => {
          if (!isDecidable(record.status) || !canApprove) return "—";
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
    ],
    [approveMutation, canApprove, rejectMutation],
  );

  const hasMore = data?.pagination.has_more ?? false;
  const rows = data?.data ?? [];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Withdrawals
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            placeholder="Status"
            allowClear
            style={{ width: 200 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setCursorStack([]);
            }}
            options={Object.entries(WITHDRAWAL_STATUSES).map(([key, val]) => ({
              label: val.label,
              value: key,
            }))}
          />
          <Typography.Text type="secondary">
            Page {pageNumber}
          </Typography.Text>
          <Button
            disabled={cursorStack.length === 0}
            onClick={() => setCursorStack((s) => s.slice(0, -1))}
          >
            Previous
          </Button>
          <Button
            disabled={!hasMore}
            loading={isFetching}
            onClick={() =>
              setCursorStack((s) => [...s, data?.pagination.next_cursor ?? ""])
            }
          >
            Next
          </Button>
        </Space>

        {isError ? (
          <Alert
            type="error"
            showIcon
            message="Failed to load withdrawals"
            description={getErrorMessage(error)}
          />
        ) : rows.length === 0 && !isLoading ? (
          <Empty description="No withdrawals match the filter" />
        ) : (
          <DataTable
            data={rows}
            columns={columns}
            loading={isLoading}
            pagination={false}
          />
        )}
      </Card>

      <Modal
        title="Reject Withdrawal"
        open={rejectModal.open}
        onOk={() =>
          rejectModal.id &&
          rejectModal.id.length > 0 &&
          rejectMutation.mutate({
            id: rejectModal.id,
            reason: rejectReason,
          })
        }
        onCancel={() => {
          setRejectModal({ open: false, id: null });
          setRejectReason("");
        }}
        confirmLoading={rejectMutation.isPending}
        okButtonProps={{ danger: true, disabled: rejectReason.trim() === "" }}
        okText="Reject and refund"
      >
        <Typography.Paragraph type="secondary">
          The reserved funds are returned to the player&apos;s wallet and the
          reason is recorded on the withdrawal.
        </Typography.Paragraph>
        <Input.TextArea
          rows={3}
          placeholder="Reason for rejection (required, shown to the player)..."
          value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)}
        />
      </Modal>
    </div>
  );
}