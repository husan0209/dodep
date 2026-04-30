import { Card, Typography, Space, Select, DatePicker, Tag } from "antd";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { financeService } from "@/services/finance.service";
import { formatDate } from "@/utils/format";
import { TRANSACTION_STATUSES } from "@/utils/constants";
import type { ColumnsType } from "antd/es/table";
import type { Deposit } from "@/types/finance";

const { Title } = Typography;

export default function Deposits() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();

  const { data, isLoading } = useQuery({
    queryKey: ["deposits", page, pageSize, status],
    queryFn: () =>
      financeService.getDeposits({ page, page_size: pageSize, status }),
  });

  const columns: ColumnsType<Deposit> = [
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
    {
      title: "Method",
      dataIndex: "method",
      render: (v: string) => <Tag>{v}</Tag>,
    },
    { title: "Provider", dataIndex: "provider" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <StatusTag status={v} config={TRANSACTION_STATUSES} />
      ),
    },
    {
      title: "PSP Ref",
      dataIndex: "psp_reference",
      render: (v: string) => v || "—",
    },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Completed",
      dataIndex: "completed_at",
      render: (v: string | null) => (v ? formatDate(v) : "—"),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Deposits
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
            options={Object.entries(TRANSACTION_STATUSES).map(([key, val]) => ({
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
    </div>
  );
}
