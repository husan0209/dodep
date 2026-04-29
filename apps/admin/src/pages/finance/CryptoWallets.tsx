import { useQuery } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Button,
  Card,
  Typography,
  Statistic,
  Row,
  Col,
  Space,
} from "antd";
import { ReloadOutlined, WarningOutlined } from "@ant-design/icons";
import { paymentsService } from "@/services/payments.service";
import type { CryptoWallet } from "@/types/payments";

const { Title } = Typography;

export default function CryptoWallets() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["crypto-wallets"],
    queryFn: () => paymentsService.getCryptoWallets(),
  });

  const lowWallets = data?.filter((w) => w.is_low) || [];

  const columns = [
    { title: "Coin", dataIndex: "coin" },
    { title: "Type", render: (_: unknown, r: CryptoWallet) => <Tag>{r.wallet_type.toUpperCase()}</Tag> },
    { title: "Balance", dataIndex: "balance" },
    { title: "Address", dataIndex: "address", ellipsis: true },
    {
      title: "Daily Avg Withdrawal",
      render: (_: unknown, r: CryptoWallet) => r.daily_withdrawal_avg,
    },
    {
      title: "Threshold",
      render: (_: unknown, r: CryptoWallet) => r.threshold_amount,
    },
    {
      title: "Status",
      render: (_: unknown, r: CryptoWallet) =>
        r.is_low ? (
          <Tag icon={<WarningOutlined />} color="red">
            LOW
          </Tag>
        ) : (
          <Tag color="green">OK</Tag>
        ),
    },
    {
      title: "Pending",
      render: (_: unknown, r: CryptoWallet) => `D:${r.pending_deposits} / W:${r.pending_withdrawals}`,
    },
    {
      title: "Actions",
      render: (_: unknown, r: CryptoWallet) => (
        <Button
          size="small"
          icon={<ReloadOutlined />}
          onClick={() => paymentsService.refreshCryptoWallet(r.id)}
        >
          Refresh
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Crypto Wallet Management</Title>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic title="Total Wallets" value={data?.length || 0} />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="Low Balance Alerts"
              value={lowWallets.length}
              valueStyle={{ color: lowWallets.length > 0 ? "#cf1322" : "#3f8600" }}
            />
          </Card>
        </Col>
      </Row>

      <Card
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            Refresh All
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={data || []}
          rowKey="id"
          loading={isLoading}
          pagination={false}
        />
      </Card>
    </div>
  );
}
