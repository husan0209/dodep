import {
  Card,
  Typography,
  Space,
  Select,
  Button,
  Tag,
  Modal,
  Input,
  message,
} from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import { riskService } from "@/services/risk.service";
import { formatDate } from "@/utils/format";
import { ALERT_SEVERITIES, ALERT_STATUSES } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import type { ColumnsType } from "antd/es/table";
import type { FraudAlert } from "@/types/risk";

const { Title } = Typography;

const CATEGORY_LABELS: Record<string, string> = {
  velocity: "Velocity",
  amount_anomaly: "Amount Anomaly",
  pattern: "Suspicious Pattern",
  multi_account: "Multi Account",
  bonus_abuse: "Bonus Abuse",
  payment_fraud: "Payment Fraud",
  collusion: "Collusion",
};

export default function FraudAlerts() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [severity, setSeverity] = useState<string>();
  const [status, setStatus] = useState<string>();
  const [resolveModal, setResolveModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [resolution, setResolution] = useState("");
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["fraud-alerts", page, pageSize, severity, status],
    queryFn: () =>
      riskService.getAlerts({
        page,
        page_size: pageSize,
        severity: severity,
        status: status,
      }),
  });

  const resolveMutation = useMutation({
    mutationFn: ({ id, resolution }: { id: string; resolution: string }) =>
      riskService.updateAlertStatus(id, "resolved", resolution),
    onSuccess: () => {
      message.success("Alert resolved");
      queryClient.invalidateQueries({ queryKey: ["fraud-alerts"] });
      setResolveModal({ open: false, id: null });
      setResolution("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const columns: ColumnsType<FraudAlert> = [
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
      title: "Category",
      dataIndex: "category",
      render: (v: string) => <Tag>{CATEGORY_LABELS[v] || v}</Tag>,
    },
    {
      title: "Severity",
      dataIndex: "severity",
      render: (v: string) => <StatusTag status={v} config={ALERT_SEVERITIES} />,
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <StatusTag status={v} config={ALERT_STATUSES} />,
    },
    {
      title: "Score",
      dataIndex: "risk_score",
      width: 70,
      render: (v: number) => (
        <Tag
          color={
            v >= 80 ? "red" : v >= 60 ? "orange" : v >= 30 ? "gold" : "green"
          }
        >
          {v}
        </Tag>
      ),
    },
    { title: "Title", dataIndex: "title", ellipsis: true },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Actions",
      key: "actions",
      width: 150,
      render: (_, record) => {
        if (record.status === "resolved" || record.status === "false_positive")
          return "—";
        return (
          <Space>
            <Button
              size="small"
              type="primary"
              onClick={() => setResolveModal({ open: true, id: record.id })}
            >
              Resolve
            </Button>
            <Button
              size="small"
              onClick={() =>
                riskService
                  .updateAlertStatus(record.id, "false_positive")
                  .then(() =>
                    queryClient.invalidateQueries({
                      queryKey: ["fraud-alerts"],
                    }),
                  )
              }
            >
              FP
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Fraud Alerts
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Severity"
            allowClear
            style={{ width: 140 }}
            value={severity}
            onChange={setSeverity}
            options={Object.entries(ALERT_SEVERITIES).map(([k, v]) => ({
              label: v.label,
              value: k,
            }))}
          />
          <Select
            placeholder="Status"
            allowClear
            style={{ width: 140 }}
            value={status}
            onChange={setStatus}
            options={Object.entries(ALERT_STATUSES).map(([k, v]) => ({
              label: v.label,
              value: k,
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
        title="Resolve Alert"
        open={resolveModal.open}
        onOk={() =>
          resolveModal.id &&
          resolveMutation.mutate({ id: resolveModal.id, resolution })
        }
        onCancel={() => {
          setResolveModal({ open: false, id: null });
          setResolution("");
        }}
        confirmLoading={resolveMutation.isPending}
      >
        <Input.TextArea
          rows={3}
          placeholder="Resolution notes..."
          value={resolution}
          onChange={(e) => setResolution(e.target.value)}
        />
      </Modal>
    </div>
  );
}
