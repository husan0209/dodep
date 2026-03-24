import {
  Card,
  Typography,
  Space,
  Select,
  Input,
  Button,
  Modal,
  message,
} from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { sportsService } from "@/services/sports.service";
import { formatDate } from "@/utils/format";
import { BET_STATUSES } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import type { ColumnsType } from "antd/es/table";
import type { Bet } from "@/types/bet";

const { Title } = Typography;

export default function Bets() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const [userId, setUserId] = useState("");
  const [voidModal, setVoidModal] = useState<{
    open: boolean;
    id: string | null;
  }>({ open: false, id: null });
  const [voidReason, setVoidReason] = useState("");
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["bets", page, pageSize, status, userId],
    queryFn: () =>
      sportsService.getBets({
        page,
        page_size: pageSize,
        status: status,
        user_id: userId || undefined,
      }),
  });

  const voidMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      sportsService.voidBet(id, reason),
    onSuccess: () => {
      message.success("Bet voided");
      queryClient.invalidateQueries({ queryKey: ["bets"] });
      setVoidModal({ open: false, id: null });
      setVoidReason("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const columns: ColumnsType<Bet> = [
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
    { title: "Type", dataIndex: "bet_type" },
    {
      title: "Stake",
      dataIndex: "stake",
      render: (v: string, r: Record<string, unknown>) => (
        <MoneyDisplay amount={v} currency={r.currency_code as string} />
      ),
    },
    { title: "Odds", dataIndex: "odds" },
    {
      title: "Potential Win",
      dataIndex: "potential_win",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <StatusTag status={v} config={BET_STATUSES} />,
    },
    {
      title: "Placed",
      dataIndex: "placed_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Settled",
      dataIndex: "settled_at",
      render: (v: string | null) => (v ? formatDate(v) : "—"),
    },
    {
      title: "Actions",
      key: "actions",
      width: 100,
      render: (_, record) => {
        if (record.status !== "active") return "—";
        return (
          <Button
            size="small"
            danger
            onClick={() => setVoidModal({ open: true, id: record.id })}
          >
            Void
          </Button>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Bets
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Input
            placeholder="User ID"
            value={userId}
            onChange={(e) => {
              setUserId(e.target.value);
              setPage(1);
            }}
            allowClear
            style={{ width: 200 }}
          />
          <Select
            placeholder="Status"
            allowClear
            style={{ width: 140 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={Object.entries(BET_STATUSES).map(([k, v]) => ({
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
        title="Void Bet"
        open={voidModal.open}
        onOk={() =>
          voidModal.id &&
          voidMutation.mutate({ id: voidModal.id, reason: voidReason })
        }
        onCancel={() => {
          setVoidModal({ open: false, id: null });
          setVoidReason("");
        }}
        confirmLoading={voidMutation.isPending}
        okButtonProps={{ danger: true }}
      >
        <Input.TextArea
          rows={3}
          placeholder="Reason for voiding..."
          value={voidReason}
          onChange={(e) => setVoidReason(e.target.value)}
        />
      </Modal>
    </div>
  );
}
