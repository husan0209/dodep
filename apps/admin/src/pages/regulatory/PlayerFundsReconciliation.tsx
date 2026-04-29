import { useQuery } from "@tanstack/react-query";
import { Card, Typography, Statistic, Row, Col, Button, Alert } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";

const { Title } = Typography;

export default function PlayerFundsReconciliation() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["player-funds"],
    queryFn: () => regulatoryService.getPlayerFunds(),
  });

  const funds = data || { total_player_balances: "0.00", funds_in_segregated: "0.00", segregation_ratio: 1.0, liabilities_total: "0.00" };

  return (
    <div>
      <Title level={3}>Player Funds Reconciliation</Title>
      <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} style={{ marginBottom: 16 }}>
        Refresh
      </Button>
      {funds.segregation_ratio < 1.0 && (
        <Alert message="Segregation ratio below 1.0 — immediate action required" type="error" showIcon style={{ marginBottom: 16 }} />
      )}
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic title="Total Player Balances" value={funds.total_player_balances} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Segregated Funds" value={funds.funds_in_segregated} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Liabilities" value={funds.liabilities_total} prefix="$" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="Segregation Ratio"
              value={funds.segregation_ratio}
              precision={2}
              valueStyle={{ color: funds.segregation_ratio < 1.0 ? "#cf1322" : funds.segregation_ratio < 1.2 ? "#faad14" : "#3f8600" }}
              suffix="x"
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
