import { useQuery } from "@tanstack/react-query";
import { Card, Statistic, Row, Col, Table, Typography, Tag } from "antd";
import { ArrowUpOutlined, ArrowDownOutlined, WarningOutlined } from "@ant-design/icons";
import { paymentsService } from "@/services/payments.service";

const { Title } = Typography;

export default function BalanceSheet() {
  const { data, isLoading } = useQuery({
    queryKey: ["balance-sheet"],
    queryFn: () => paymentsService.getBalanceSheet(),
  });

  const ratio = data?.coverage_ratio || 0;
  const ratioColor = ratio < 1.0 ? "red" : ratio < 1.2 ? "orange" : "green";

  return (
    <div>
      <Title level={3}>Platform Balance Sheet</Title>
      <Typography.Text type="secondary">As of: {data?.as_of || "—"}</Typography.Text>

      <Row gutter={16} style={{ marginTop: 16, marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="Coverage Ratio"
              value={ratio}
              precision={2}
              valueStyle={{ color: ratioColor === "red" ? "#cf1322" : ratioColor === "orange" ? "#faad14" : "#3f8600" }}
              prefix={ratio >= 1.2 ? <ArrowUpOutlined /> : <ArrowDownOutlined />}
              suffix={ratio < 1.0 ? <WarningOutlined style={{ color: "#cf1322" }} /> : undefined}
            />
            {ratio < 1.0 && (
              <Tag color="red" style={{ marginTop: 8 }}>
                CRITICAL: Assets &lt; Liabilities
              </Tag>
            )}
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="Total Assets" value={data?.assets?.total || "0"} prefix="$" />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="Total Liabilities" value={data?.liabilities?.total || "0"} prefix="$" />
          </Card>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="Liabilities Breakdown" loading={isLoading}>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Statistic title="Player Balances" value={data?.liabilities?.player_balances || "0"} prefix="$" />
              </Col>
              <Col span={12}>
                <Statistic title="Bonus Balances" value={data?.liabilities?.bonus_balances || "0"} prefix="$" />
              </Col>
              <Col span={12}>
                <Statistic title="Pending Withdrawals" value={data?.liabilities?.pending_withdrawals || "0"} prefix="$" />
              </Col>
            </Row>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="Assets Breakdown" loading={isLoading}>
            <Table
              size="small"
              pagination={false}
              columns={[
                { title: "Source", dataIndex: "name" },
                { title: "Balance", dataIndex: "balance" },
                { title: "Currency", dataIndex: "currency" },
              ]}
              dataSource={data?.assets?.gateways || []}
              rowKey="name"
              title={() => "Payment Gateways"}
            />
            <Table
              size="small"
              pagination={false}
              columns={[
                { title: "Coin", dataIndex: "coin" },
                { title: "Amount", dataIndex: "amount" },
                { title: "USD", dataIndex: "usd_equivalent" },
              ]}
              dataSource={(data?.assets?.crypto_hot || []).map((c) => ({
                coin: c.coin,
                amount: c.amount,
                usd_equivalent: c.usd_equivalent,
                key: `hot-${c.coin}`,
              }))}
              rowKey="key"
              title={() => "Crypto Hot"}
              style={{ marginTop: 16 }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
