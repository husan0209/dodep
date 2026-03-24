import {
  Card,
  Typography,
  Space,
  Button,
  Tag,
  Input,
  Select,
  DatePicker,
} from "antd";
import { PlusOutlined, SearchOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useState } from "react";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import { usersService } from "@/services/users.service";
import { formatDate } from "@/utils/format";
import { USER_STATUSES, KYC_LEVELS } from "@/utils/constants";
import type { ColumnsType } from "antd/es/table";
import type { UserProfile } from "@/types/user";

const { Title } = Typography;

export default function UserList() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<string>();
  const [kycLevel, setKycLevel] = useState<number>();

  const { data, isLoading } = useQuery({
    queryKey: ["users", page, pageSize, search, status, kycLevel],
    queryFn: () =>
      usersService.list({
        page,
        page_size: pageSize,
        search,
        status,
        kyc_level: kycLevel,
      }),
  });

  const columns: ColumnsType<UserProfile> = [
    {
      title: "ID",
      dataIndex: "id",
      key: "id",
      width: 80,
      render: (id: string) => (
        <Typography.Text copyable={{ text: id }}>
          {id.slice(0, 8)}...
        </Typography.Text>
      ),
    },
    {
      title: "Email",
      dataIndex: "email",
      key: "email",
      sorter: true,
    },
    {
      title: "Name",
      key: "name",
      render: (_, record) =>
        `${record.first_name || ""} ${record.last_name || ""}`.trim() || "—",
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: string) => (
        <StatusTag status={status} config={USER_STATUSES} />
      ),
      filters: Object.entries(USER_STATUSES).map(([key, val]) => ({
        text: val.label,
        value: key,
      })),
    },
    {
      title: "KYC Level",
      dataIndex: "kyc_level",
      key: "kyc_level",
      render: (level: number) => (
        <StatusTag status={String(level)} config={KYC_LEVELS} />
      ),
    },
    {
      title: "Country",
      dataIndex: "country_code",
      key: "country_code",
      width: 80,
    },
    {
      title: "Currency",
      dataIndex: "currency_code",
      key: "currency_code",
      width: 70,
    },
    {
      title: "Registered",
      dataIndex: "created_at",
      key: "created_at",
      render: (date: string) => formatDate(date, "YYYY-MM-DD"),
      sorter: true,
    },
    {
      title: "Last Login",
      dataIndex: "last_login_at",
      key: "last_login_at",
      render: (date: string | null) =>
        date ? formatDate(date, "YYYY-MM-DD HH:mm") : "Never",
    },
    {
      title: "Actions",
      key: "actions",
      width: 100,
      render: (_, record) => (
        <Button type="link" onClick={() => navigate(`/users/${record.id}`)}>
          View
        </Button>
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
          User Management
        </Title>
      </div>

      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            placeholder="Search by email, ID, name..."
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            allowClear
            style={{ width: 280 }}
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
            options={Object.entries(USER_STATUSES).map(([key, val]) => ({
              label: val.label,
              value: key,
            }))}
          />
          <Select
            placeholder="KYC Level"
            allowClear
            style={{ width: 160 }}
            value={kycLevel}
            onChange={(val) => {
              setKycLevel(val);
              setPage(1);
            }}
            options={Object.entries(KYC_LEVELS).map(([key, val]) => ({
              label: val.label,
              value: Number(key),
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
