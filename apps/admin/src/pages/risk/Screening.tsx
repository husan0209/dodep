import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Button,
  Table,
  Tag,
  Space,
  Select,
  Modal,
  message,
} from "antd";
import { CheckCircleOutlined, ReloadOutlined, EyeOutlined } from "@ant-design/icons";
import { riskService } from "@/services/risk.service";

const { Title } = Typography;

interface ScreeningHit {
  id: string;
  user_id: string;
  list_type: "pep" | "sanctions" | "adverse_media";
  match_name: string;
  match_score: number;
  status: "pending" | "confirmed" | "false_positive" | "resolved";
  source: string;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  pending: "orange",
  confirmed: "red",
  false_positive: "green",
  resolved: "default",
};

export default function Screening() {
  const queryClient = useQueryClient();
  const [listFilter, setListFilter] = useState<string | undefined>();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["screening-hits", listFilter],
    queryFn: () =>
      riskService.getScreeningHits({ list_type: listFilter } as any) as Promise<ScreeningHit[]>,
  });

  const resolveMutation = useMutation({
    mutationFn: ({ id, status, notes }: { id: string; status: string; notes?: string }) =>
      riskService.resolveScreeningHit(id, { status, notes }),
    onSuccess: () => {
      message.success("Hit resolved");
      queryClient.invalidateQueries({ queryKey: ["screening-hits"] });
    },
    onError: () => message.error("Failed to resolve"),
  });

  const columns = [
    { title: "User ID", dataIndex: "user_id", render: (v: string) => v.slice(0, 8) },
    {
      title: "List",
      dataIndex: "list_type",
      render: (v: string) => <Tag color={v === "pep" ? "blue" : v === "sanctions" ? "red" : "purple"}>{v.toUpperCase()}</Tag>,
    },
    { title: "Match", dataIndex: "match_name" },
    {
      title: "Score",
      dataIndex: "match_score",
      render: (v: number) => (
        <Tag color={v >= 90 ? "red" : v >= 70 ? "orange" : "green"}>{v}%</Tag>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={STATUS_COLORS[v]}>{v.toUpperCase().replace("_", " ")}</Tag>,
    },
    { title: "Source", dataIndex: "source", ellipsis: true },
    {
      title: "Actions",
      render: (_: unknown, r: ScreeningHit) => (
        <Space>
          <Button
            icon={<CheckCircleOutlined />}
            size="small"
            disabled={r.status !== "pending"}
            onClick={() => {
              Modal.confirm({
                title: "Resolve Hit",
                content: "Mark as confirmed match or false positive?",
                okText: "Confirmed",
                cancelText: "False Positive",
                onOk: () => resolveMutation.mutate({ id: r.id, status: "confirmed" }),
                onCancel: () => resolveMutation.mutate({ id: r.id, status: "false_positive" }),
              });
            }}
          >
            Resolve
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>PEP / Sanctions Screening</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          allowClear
          placeholder="List Type"
          style={{ width: 160 }}
          onChange={setListFilter}
          options={[
            { value: "pep", label: "PEP" },
            { value: "sanctions", label: "Sanctions" },
            { value: "adverse_media", label: "Adverse Media" },
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
          Refresh
        </Button>
      </Space>
      <Card>
        <Table
          rowKey="id"
          dataSource={data || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>
    </div>
  );
}
