import { Card, Typography, Space, Select, Input } from "antd";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { financeService } from "@/services/finance.service";
import { formatDate } from "@/utils/format";
import { TRANSACTION_STATUSES } from "@/utils/constants";
import type { ColumnsType } from "antd/es/table";
import type { Transaction } from "@/types/finance";

const { Title } = Typography;

export default function Transactions() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const [userId, setUserId] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["transactions", page, pageSize, status, userId],
    queryFn: () =>
      financeService.getTransactions({
        page,
        page_size: pageSize,
        status,
        user_id: userId || undefined,
      }),
  });

  const columns: ColumnsType<Transaction> = [
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
    { title: "Type", dataIndex: "type" },
    {
      title: "Amount",
      dataIndex: "amount",
      render: (v: string, r: any) => (
        <MoneyDisplay amount={v} currency={r.currency_code as string} />
      ),
    },
    {
      title: "Balance Before",
      dataIndex: "balance_before",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    {
      title: "Balance After",
      dataIndex: "balance_after",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <StatusTag status={v} config={TRANSACTION_STATUSES} />
      ),
    },
    {
      title: "Reference",
      dataIndex: "reference_type",
      render: (v: string, r: any) =>
        v ? `${v} #${(r.reference_id as string)?.slice(0, 8)}` : "—",
    },
    {
      title: "Date",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Transactions
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
            style={{ width: 160 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={Object.entries(TRANSACTION_STATUSES).map(([k, v]) => ({
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
