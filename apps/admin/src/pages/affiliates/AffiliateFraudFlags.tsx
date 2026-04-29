import {
  Card,
  Typography,
  Select,
  Space,
  Tag,
} from "antd";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DataTable from "@/components/common/DataTable";
import { affiliatesService } from "@/services/affiliates.service";
import { formatDate } from "@/utils/format";
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

  const { data, isLoading } = useQuery({
    queryKey: ["affiliate-fraud-flags", page, pageSize, status],
    queryFn: () =>
      affiliatesService.getFraudFlags({ status, page, page_size: pageSize }),
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
    </div>
  );
}
