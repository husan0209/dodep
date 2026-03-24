import { Card, Typography, Space, Input, Select } from "antd";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import { riskService } from "@/services/risk.service";
import { formatDate } from "@/utils/format";
import type { ColumnsType } from "antd/es/table";
import type { AuditLogEntry } from "@/types/risk";

const { Title, Text } = Typography;

export default function AuditLog() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const { data, isLoading } = useQuery({
    queryKey: ["audit-log", page, pageSize],
    queryFn: () => riskService.getAuditLog({ page, page_size: pageSize }),
  });

  const columns: ColumnsType<AuditLogEntry> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    { title: "Admin", dataIndex: "admin_email" },
    {
      title: "Action",
      dataIndex: "action",
      render: (v: string) => <Text code>{v}</Text>,
    },
    { title: "Resource", dataIndex: "resource_type" },
    {
      title: "Resource ID",
      dataIndex: "resource_id",
      width: 100,
      render: (v: string) => v?.slice(0, 8) || "—",
    },
    { title: "IP", dataIndex: "ip_address" },
    {
      title: "Date",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Audit Log
      </Title>
      <Card>
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
