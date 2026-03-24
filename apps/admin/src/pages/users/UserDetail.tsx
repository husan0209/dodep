import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Descriptions,
  Typography,
  Space,
  Button,
  Tabs,
  Table,
  Tag,
  Modal,
  Input,
  message,
  Spin,
} from "antd";
import {
  ArrowLeftOutlined,
  StopOutlined,
  CheckCircleOutlined,
} from "@ant-design/icons";
import { usersService } from "@/services/users.service";
import { financeService } from "@/services/finance.service";
import { sportsService } from "@/services/sports.service";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { formatDate } from "@/utils/format";
import { USER_STATUSES, KYC_LEVELS } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import { useState } from "react";
import type { ColumnsType } from "antd/es/table";

const { Title, Text } = Typography;

export default function UserDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [blockReason, setBlockReason] = useState("");
  const [showBlockModal, setShowBlockModal] = useState(false);

  const { data: user, isLoading } = useQuery({
    queryKey: ["user", id],
    queryFn: () => usersService.get(id!),
    enabled: !!id,
  });

  const { data: sessions } = useQuery({
    queryKey: ["user-sessions", id],
    queryFn: () => usersService.getSessions(id!),
    enabled: !!id,
  });

  const { data: limits } = useQuery({
    queryKey: ["user-limits", id],
    queryFn: () => usersService.getLimits(id!),
    enabled: !!id,
  });

  const { data: transactions } = useQuery({
    queryKey: ["user-transactions", id],
    queryFn: () =>
      financeService.getTransactions({ user_id: id, page: 1, page_size: 10 }),
    enabled: !!id,
  });

  const { data: bets } = useQuery({
    queryKey: ["user-bets", id],
    queryFn: () =>
      sportsService.getBets({ user_id: id, page: 1, page_size: 10 }),
    enabled: !!id,
  });

  const blockMutation = useMutation({
    mutationFn: () => usersService.block(id!, blockReason),
    onSuccess: () => {
      message.success("User blocked");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
      setShowBlockModal(false);
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const unblockMutation = useMutation({
    mutationFn: () => usersService.unblock(id!),
    onSuccess: () => {
      message.success("User unblocked");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  if (isLoading)
    return (
      <Spin size="large" style={{ display: "block", margin: "100px auto" }} />
    );
  if (!user) return <Text>User not found</Text>;

  const transactionColumns: ColumnsType<Record<string, unknown>> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    { title: "Type", dataIndex: "type" },
    {
      title: "Amount",
      dataIndex: "amount",
      render: (v: string, r: Record<string, unknown>) => (
        <MoneyDisplay amount={v} currency={r.currency_code as string} />
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <StatusTag status={v} />,
    },
    {
      title: "Date",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
  ];

  const betColumns: ColumnsType<Record<string, unknown>> = [
    {
      title: "ID",
      dataIndex: "id",
      width: 80,
      render: (v: string) => v.slice(0, 8),
    },
    { title: "Type", dataIndex: "bet_type" },
    {
      title: "Stake",
      dataIndex: "stake",
      render: (v: string) => <MoneyDisplay amount={v} />,
    },
    { title: "Odds", dataIndex: "odds" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <StatusTag status={v} />,
    },
    {
      title: "Date",
      dataIndex: "placed_at",
      render: (v: string) => formatDate(v),
    },
  ];

  const sessionColumns: ColumnsType<Record<string, unknown>> = [
    {
      title: "Device",
      dataIndex: "device_fingerprint",
      width: 120,
      render: (v: string) => v?.slice(0, 12) || "—",
    },
    { title: "IP", dataIndex: "ip_address" },
    { title: "User Agent", dataIndex: "user_agent", ellipsis: true },
    {
      title: "Created",
      dataIndex: "created_at",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Last Activity",
      dataIndex: "last_activity",
      render: (v: string) => formatDate(v),
    },
    {
      title: "Action",
      key: "action",
      render: (_: unknown, record: Record<string, unknown>) => (
        <Button
          size="small"
          danger
          onClick={() =>
            usersService.revokeSession(id!, record.id as string).then(() =>
              queryClient.invalidateQueries({
                queryKey: ["user-sessions", id],
              }),
            )
          }
        >
          Revoke
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/users")}>
          Back
        </Button>
        <Title level={3} style={{ margin: 0 }}>
          User Detail
        </Title>
      </Space>

      <Card style={{ marginBottom: 16 }}>
        <Descriptions column={{ xs: 1, sm: 2, md: 3 }} bordered>
          <Descriptions.Item label="ID">{user.id}</Descriptions.Item>
          <Descriptions.Item label="Email">{user.email}</Descriptions.Item>
          <Descriptions.Item label="Name">
            {`${user.first_name || ""} ${user.last_name || ""}`.trim() || "—"}
          </Descriptions.Item>
          <Descriptions.Item label="Status">
            <StatusTag status={user.status} config={USER_STATUSES} />
          </Descriptions.Item>
          <Descriptions.Item label="KYC Level">
            <StatusTag status={String(user.kyc_level)} config={KYC_LEVELS} />
          </Descriptions.Item>
          <Descriptions.Item label="Country">
            {user.country_code}
          </Descriptions.Item>
          <Descriptions.Item label="Currency">
            {user.currency_code}
          </Descriptions.Item>
          <Descriptions.Item label="Registered">
            {formatDate(user.created_at)}
          </Descriptions.Item>
          <Descriptions.Item label="Last Login">
            {user.last_login_at ? formatDate(user.last_login_at) : "Never"}
          </Descriptions.Item>
        </Descriptions>

        <Space style={{ marginTop: 16 }}>
          {user.status === "active" ? (
            <Button
              danger
              icon={<StopOutlined />}
              onClick={() => setShowBlockModal(true)}
            >
              Block User
            </Button>
          ) : user.status === "blocked" ? (
            <Button
              type="primary"
              icon={<CheckCircleOutlined />}
              onClick={() => unblockMutation.mutate()}
              loading={unblockMutation.isPending}
            >
              Unblock User
            </Button>
          ) : null}
        </Space>
      </Card>

      {limits && (
        <Card title="Responsible Gambling Limits" style={{ marginBottom: 16 }}>
          <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small">
            <Descriptions.Item label="Daily Deposit Limit">
              {limits.deposit_limit_daily || "No limit"}
            </Descriptions.Item>
            <Descriptions.Item label="Weekly Deposit Limit">
              {limits.deposit_limit_weekly || "No limit"}
            </Descriptions.Item>
            <Descriptions.Item label="Monthly Deposit Limit">
              {limits.deposit_limit_monthly || "No limit"}
            </Descriptions.Item>
            <Descriptions.Item label="Loss Limit">
              {limits.loss_limit || "No limit"}
            </Descriptions.Item>
            <Descriptions.Item label="Session Time Limit">
              {limits.session_time_limit_minutes
                ? `${limits.session_time_limit_minutes} min`
                : "No limit"}
            </Descriptions.Item>
            <Descriptions.Item label="Self Exclusion">
              {limits.self_exclusion_until
                ? formatDate(limits.self_exclusion_until)
                : "None"}
            </Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <Card>
        <Tabs
          items={[
            {
              key: "transactions",
              label: "Recent Transactions",
              children: (
                <Table
                  dataSource={transactions?.data || []}
                  columns={transactionColumns}
                  pagination={false}
                  size="small"
                  rowKey="id"
                />
              ),
            },
            {
              key: "bets",
              label: "Recent Bets",
              children: (
                <Table
                  dataSource={bets?.data || []}
                  columns={betColumns}
                  pagination={false}
                  size="small"
                  rowKey="id"
                />
              ),
            },
            {
              key: "sessions",
              label: "Active Sessions",
              children: (
                <Table
                  dataSource={sessions || []}
                  columns={sessionColumns}
                  pagination={false}
                  size="small"
                  rowKey="id"
                />
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="Block User"
        open={showBlockModal}
        onOk={() => blockMutation.mutate()}
        onCancel={() => setShowBlockModal(false)}
        confirmLoading={blockMutation.isPending}
        okButtonProps={{ danger: true }}
      >
        <Input.TextArea
          rows={3}
          placeholder="Reason for blocking..."
          value={blockReason}
          onChange={(e) => setBlockReason(e.target.value)}
        />
      </Modal>
    </div>
  );
}
