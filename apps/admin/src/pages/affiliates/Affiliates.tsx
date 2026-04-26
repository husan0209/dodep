import {
  Card,
  Typography,
  Space,
  Select,
  Button,
  Tag,
  message,
} from "antd";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import DataTable from "@/components/common/DataTable";
import { affiliatesService, type Affiliate } from "@/services/affiliates.service";
import { formatDate } from "@/utils/format";
import { hasPermission } from "@/utils/permissions";
import { useAuthStore } from "@/stores/authStore";
import { getErrorMessage } from "@/utils/errors";
import type { ColumnsType } from "antd/es/table";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  active: "green",
  pending_review: "orange",
  suspended: "red",
  rejected: "default",
  closed: "default",
};

export default function Affiliates() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { permissions } = useAuthStore();

  const { data, isLoading } = useQuery({
    queryKey: ["affiliates", page, pageSize, status],
    queryFn: () =>
      affiliatesService.getAffiliates({ status, page, page_size: pageSize }),
  });

  const approveMutation = useMutation({
    mutationFn: (userId: string) => affiliatesService.approveAffiliate(userId),
    onSuccess: () => {
      message.success("Affiliate approved");
      queryClient.invalidateQueries({ queryKey: ["affiliates"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const suspendMutation = useMutation({
    mutationFn: (id: string) => affiliatesService.suspendAffiliate(id),
    onSuccess: () => {
      message.success("Affiliate suspended");
      queryClient.invalidateQueries({ queryKey: ["affiliates"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const canManage = hasPermission(permissions, "affiliate.manage");

  const columns: ColumnsType<Affiliate> = [
    {
      title: "Affiliate Code",
      dataIndex: "affiliate_code",
      width: 120,
      render: (v: string) => <Tag>{v || "—"}</Tag>,
    },
    {
      title: "User ID",
      dataIndex: "user_id",
      width: 80,
    },
    {
      title: "Status",
      dataIndex: "status",
      width: 120,
      render: (v: string) => (
        <Tag color={STATUS_COLORS[v] || "default"}>{v}</Tag>
      ),
    },
    {
      title: "Rate",
      dataIndex: "commission_rate",
      width: 80,
      render: (v: string) => `${(parseFloat(v) * 100).toFixed(0)}%`,
    },
    {
      title: "Total Earned",
      dataIndex: "total_commission",
      render: (v: string) => v || "$0.00",
    },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Actions",
      key: "actions",
      width: 240,
      render: (_, record) => {
        if (!canManage) return "—";
        return (
          <Space>
            <Button
              size="small"
              onClick={() => navigate(`/affiliates/${record.id}`)}
            >
              View
            </Button>
            {record.status === "pending_review" && (
              <Button
                size="small"
                type="primary"
                onClick={() => approveMutation.mutate(String(record.user_id))}
                loading={approveMutation.isPending}
              >
                Approve
              </Button>
            )}
            {record.status === "active" && (
              <Button
                size="small"
                danger
                onClick={() => suspendMutation.mutate(record.id)}
                loading={suspendMutation.isPending}
              >
                Suspend
              </Button>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Affiliate Management
      </Title>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            placeholder="Filter by status"
            allowClear
            style={{ width: 180 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={[
              { label: "Active", value: "active" },
              { label: "Pending Review", value: "pending_review" },
              { label: "Suspended", value: "suspended" },
              { label: "Rejected", value: "rejected" },
              { label: "Closed", value: "closed" },
            ]}
          />
        </Space>
        <DataTable
          data={data?.data || []}
          columns={columns}
          loading={isLoading}
          total={data?.pagination?.total || 0}
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
