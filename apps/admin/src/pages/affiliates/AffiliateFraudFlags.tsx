import {
  Card,
  Typography,
  Select,
  Space,
  Tag,
  Button,
  Modal,
  Input,
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

interface FraudFlag {
  id: string;
  affiliate_id: string;
  referred_user_id: number;
  flag_type: string;
  severity: string;
  status: string;
  details: Record<string, string>;
  created_at: string;
  resolved_at: string | null;
  resolved_by: string;
}

const SEVERITY_COLORS: Record<string, string> = {
  low: "blue",
  medium: "orange",
  high: "red",
  critical: "magenta",
};

const FLAG_STATUS_COLORS: Record<string, string> = {
  open: "red",
  in_review: "orange",
  resolved: "green",
  dismissed: "default",
};

export default function AffiliateFraudFlags() {
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [status, setStatus] = useState<string>("open");
  const [actionModal, setActionModal] = useState<{
    action: "resolve" | "dismiss";
    id: string | null;
  }>({ action: "resolve", id: null });
  const [notes, setNotes] = useState("");
  const queryClient = useQueryClient();
  const { permissions } = useAuthStore();
  const canReview = hasPermission(permissions, "affiliate.fraud.review");

  const { data, isLoading } = useQuery({
    queryKey: ["affiliate-fraud-flags", page, pageSize, status],
    queryFn: () =>
      affiliatesService.getFraudFlags({ status, page, page_size: pageSize }),
  });

  const resolveMutation = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) =>
      affiliatesService.resolveFraudFlag(id, note),
    onSuccess: () => {
      message.success("Fraud flag marked as confirmed fraud");
      queryClient.invalidateQueries({ queryKey: ["affiliate-fraud-flags"] });
      setActionModal({ action: "resolve", id: null });
      setNotes("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const dismissMutation = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) =>
      affiliatesService.dismissFraudFlag(id, note),
    onSuccess: () => {
      message.success("Fraud flag dismissed as false positive");
      queryClient.invalidateQueries({ queryKey: ["affiliate-fraud-flags"] });
      setActionModal({ action: "dismiss", id: null });
      setNotes("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const columns: ColumnsType<FraudFlag> = [
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
      title: "Referred User",
      dataIndex: "referred_user_id",
      width: 100,
    },
    {
      title: "Type",
      dataIndex: "flag_type",
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: "Severity",
      dataIndex: "severity",
      width: 100,
      render: (v: string) => (
        <Tag color={SEVERITY_COLORS[v] || "default"}>{v}</Tag>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      width: 100,
      render: (v: string) => (
        <Tag color={FLAG_STATUS_COLORS[v] || "default"}>{v}</Tag>
      ),
    },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Details",
      dataIndex: "details",
      render: (v: Record<string, string>) =>
        v ? Object.entries(v).map(([k, val]) => (
          <Tag key={k} style={{ marginBottom: 2 }}>
            {k}: {val}
          </Tag>
        )) : "—",
    },
    {
      title: "Reviewed",
      dataIndex: "resolved_at",
      width: 150,
      render: (v: string | null, r: FraudFlag) =>
        v ? (
          <span>
            {formatDate(v)}
            {r.resolved_by ? ` by ${r.resolved_by}` : ""}
          </span>
        ) : (
          "—"
        ),
    },
    {
      title: "Actions",
      key: "actions",
      width: 200,
      render: (_, record) => {
        // open / in_review are the only actionable states (backend enforces
        // the same state machine and returns 409 for anything else).
        if (!canReview) return "—";
        if (!["open", "in_review"].includes(record.status)) return "—";
        return (
          <Space>
            <Button
              size="small"
              type="primary"
              danger
              onClick={() => setActionModal({ action: "resolve", id: record.id })}
            >
              Confirm Fraud
            </Button>
            <Button
              size="small"
              onClick={() => setActionModal({ action: "dismiss", id: record.id })}
            >
              False Positive
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Affiliate Fraud Flags
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            value={status}
            style={{ width: 180 }}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={[
              { label: "Open", value: "open" },
              { label: "In Review", value: "in_review" },
              { label: "Resolved", value: "resolved" },
              { label: "Dismissed", value: "dismissed" },
            ]}
          />
        </Space>
        <DataTable
          data={(data?.data || []) as unknown as FraudFlag[]}
          columns={columns}
          loading={isLoading}
          total={(data?.pagination?.total || 0) as number}
          page={page}
          pageSize={pageSize}
          onPageChange={(p) => setPage(p)}
        />
      </Card>

      <Modal
        title={
          actionModal.action === "resolve"
            ? "Confirm Fraud"
            : "Dismiss as False Positive"
        }
        open={actionModal.id !== null}
        onOk={() => {
          if (!actionModal.id) return;
          const payload = { id: actionModal.id, note: notes };
          if (actionModal.action === "resolve") {
            resolveMutation.mutate(payload);
          } else {
            dismissMutation.mutate(payload);
          }
        }}
        onCancel={() => {
          setActionModal({ action: "resolve", id: null });
          setNotes("");
        }}
        confirmLoading={
          resolveMutation.isPending || dismissMutation.isPending
        }
        okButtonProps={{ danger: actionModal.action === "resolve" }}
        okText={actionModal.action === "resolve" ? "Confirm" : "Dismiss"}
      >
        <Input.TextArea
          rows={3}
          placeholder={
            actionModal.action === "resolve"
              ? "Investigation notes (required)..."
              : "Reason for false positive (optional)..."
          }
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
        />
      </Modal>
    </div>
  );
}
