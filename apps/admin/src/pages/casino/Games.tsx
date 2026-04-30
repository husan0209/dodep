import { Card, Typography, Space, Select, Switch, Tag } from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import { casinoService } from "@/services/casino.service";
import type { ColumnsType } from "antd/es/table";
import type { Game } from "@/types/casino";

const { Title } = Typography;

export default function Games() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [category, setCategory] = useState<string>();
  const [enabled, setEnabled] = useState<boolean>();
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["casino-games", page, pageSize, category, enabled],
    queryFn: () =>
      casinoService.getGames({ page, page_size: pageSize, category, enabled }),
  });

  const columns: ColumnsType<Game> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    { title: "Name", dataIndex: "name", ellipsis: true },
    { title: "Provider", dataIndex: "provider" },
    {
      title: "Category",
      dataIndex: "category",
      render: (v: string) => <Tag>{v}</Tag>,
    },
    { title: "RTP", dataIndex: "rtp", render: (v: number) => `${v}%` },
    {
      title: "Volatility",
      dataIndex: "volatility",
      render: (v: string) => (
        <Tag color={v === "low" ? "green" : v === "high" ? "red" : "orange"}>
          {v}
        </Tag>
      ),
    },
    { title: "Min Bet", dataIndex: "min_bet" },
    { title: "Max Bet", dataIndex: "max_bet" },
    {
      title: "Enabled",
      dataIndex: "enabled",
      render: (v: boolean) => <Switch checked={v} size="small" disabled />,
    },
    {
      title: "Featured",
      dataIndex: "featured",
      render: (v: boolean) => (v ? <Tag color="gold">Featured</Tag> : "—"),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Casino Games
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Category"
            allowClear
            style={{ width: 140 }}
            value={category}
            onChange={setCategory}
            options={[
              { label: "Slots", value: "slots" },
              { label: "Table", value: "table" },
              { label: "Live", value: "live" },
              { label: "Crash", value: "crash" },
            ]}
          />
          <Select
            placeholder="Enabled"
            allowClear
            style={{ width: 120 }}
            value={enabled}
            onChange={setEnabled}
            options={[
              { label: "Enabled", value: true },
              { label: "Disabled", value: false },
            ]}
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
