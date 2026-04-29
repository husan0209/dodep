import {
  Card,
  Typography,
  Space,
  Select,
  Button,
  Tag,
  Switch,
  message,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { bonusesService } from "@/services/bonuses.service";
import { formatDate } from "@/utils/format";
import { BONUS_TYPES } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import type { ColumnsType } from "antd/es/table";
import type { BonusCampaign } from "@/types/bonus";

const { Title } = Typography;

const BONUS_STATUSES: Record<string, { label: string; color: string }> = {
  active: { label: "Active", color: "green" },
  paused: { label: "Paused", color: "orange" },
  expired: { label: "Expired", color: "default" },
  draft: { label: "Draft", color: "default" },
};

export default function CampaignList() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [type, setType] = useState<string>();
  const [status, setStatus] = useState<string>();
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["bonus-campaigns", page, pageSize, type, status],
    queryFn: () =>
      bonusesService.getCampaigns({
        page,
        page_size: pageSize,
        type: type as any,
        status: status as any,
      }),
  });

  const toggleMutation = useMutation({
    mutationFn: ({
      id,
      newStatus,
    }: {
      id: string;
      newStatus: "active" | "paused";
    }) => bonusesService.toggleCampaign(id, newStatus),
    onSuccess: () => {
      message.success("Campaign status updated");
      queryClient.invalidateQueries({ queryKey: ["bonus-campaigns"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const columns: ColumnsType<BonusCampaign> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    { title: "Name", dataIndex: "name", ellipsis: true },
    {
      title: "Type",
      dataIndex: "type",
      render: (v: string) => <StatusTag status={v} config={BONUS_TYPES} />,
    },
    {
      title: "Match %",
      dataIndex: "match_percent",
      render: (v: number) => `${v}%`,
    },
    {
      title: "Max Amount",
      dataIndex: "max_amount",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    {
      title: "Wagering",
      dataIndex: "wagering_multiplier",
      render: (v: number) => `${v}x`,
    },
    { title: "Claims", dataIndex: "total_claims" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <StatusTag status={v} config={BONUS_STATUSES} />,
    },
    {
      title: "Start",
      dataIndex: "start_date",
      render: (v: string) => formatDate(v, "YYYY-MM-DD"),
    },
    {
      title: "End",
      dataIndex: "end_date",
      render: (v: string | null) => (v ? formatDate(v, "YYYY-MM-DD") : "—"),
    },
    {
      title: "Active",
      key: "active",
      width: 60,
      render: (_, record) => (
        <Switch
          size="small"
          checked={record.status === "active"}
          onChange={(checked) =>
            toggleMutation.mutate({
              id: record.id,
              newStatus: checked ? "active" : "paused",
            })
          }
          loading={toggleMutation.isPending}
        />
      ),
    },
  ];

  return (
    <div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          marginBottom: 16,
        }}
      >
        <Title level={3} style={{ margin: 0 }}>
          Bonus Campaigns
        </Title>
      </div>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Type"
            allowClear
            style={{ width: 150 }}
            value={type}
            onChange={setType}
            options={Object.entries(BONUS_TYPES).map(([k, v]) => ({
              label: v.label,
              value: k,
            }))}
          />
          <Select
            placeholder="Status"
            allowClear
            style={{ width: 130 }}
            value={status}
            onChange={setStatus}
            options={Object.entries(BONUS_STATUSES).map(([k, v]) => ({
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
    </div>
  );
}
