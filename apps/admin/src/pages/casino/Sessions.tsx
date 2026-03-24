import { Card, Typography, Space, Input, Select, Tag } from "antd";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { casinoService } from "@/services/casino.service";
import { formatDate } from "@/utils/format";
import type { ColumnsType } from "antd/es/table";
import type { GameSession } from "@/types/casino";

const { Title } = Typography;

export default function Sessions() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [userId, setUserId] = useState("");
  const [status, setStatus] = useState<string>();

  const { data, isLoading } = useQuery({
    queryKey: ["casino-sessions", page, pageSize, userId, status],
    queryFn: () =>
      casinoService.getGameSessions({
        page,
        page_size: pageSize,
        user_id: userId || undefined,
        status,
      }),
  });

  const columns: ColumnsType<GameSession> = [
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
      title: "Game ID",
      dataIndex: "game_id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <Tag
          color={
            v === "active"
              ? "processing"
              : v === "completed"
                ? "green"
                : "default"
          }
        >
          {v}
        </Tag>
      ),
    },
    {
      title: "Total Bet",
      dataIndex: "total_bet",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    {
      title: "Total Win",
      dataIndex: "total_win",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    { title: "Rounds", dataIndex: "rounds_played" },
    {
      title: "Started",
      dataIndex: "started_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Ended",
      dataIndex: "ended_at",
      render: (v: string | null) => (v ? formatDate(v) : "Active"),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Game Sessions
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
            onChange={setStatus}
            options={[
              { label: "Active", value: "active" },
              { label: "Completed", value: "completed" },
              { label: "Abandoned", value: "abandoned" },
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
