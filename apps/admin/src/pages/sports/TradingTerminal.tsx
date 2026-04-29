import { useState, useEffect } from "react";
import { Card, Typography, Select, Tag, Statistic, Row, Col, List, Button, message } from "antd";
import { ArrowUpOutlined, ArrowDownOutlined, ReloadOutlined } from "@ant-design/icons";
import { sportsService } from "@/services/sports.service";

const { Title, Text } = Typography;

interface OddsUpdate {
  event_id: string;
  home_odds: number;
  draw_odds?: number;
  away_odds: number;
  timestamp: string;
}

export default function TradingTerminal() {
  const [sport, setSport] = useState<string>("soccer");
  const [updates, setUpdates] = useState<OddsUpdate[]>([]);
  const [loading, setLoading] = useState(false);

  const refreshOdds = async () => {
    setLoading(true);
    try {
      const res = await sportsService.getOddsSnapshot(sport);
      setUpdates(res.data || []);
      message.success("Odds refreshed");
    } catch {
      message.error("Failed to refresh odds");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refreshOdds();
    const interval = setInterval(refreshOdds, 30000);
    return () => clearInterval(interval);
  }, [sport]);

  return (
    <div>
      <Title level={3}>Trading Terminal</Title>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col>
          <Select
            value={sport}
            onChange={setSport}
            style={{ width: 160 }}
            options={[
              { value: "soccer", label: "Soccer" },
              { value: "basketball", label: "Basketball" },
              { value: "tennis", label: "Tennis" },
              { value: "esports", label: "Esports" },
            ]}
          />
        </Col>
        <Col>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={refreshOdds}>
            Refresh Now
          </Button>
        </Col>
      </Row>
      <Row gutter={16}>
        {updates.map((u) => (
          <Col span={8} key={u.event_id} style={{ marginBottom: 16 }}>
            <Card
              title={u.event_id.slice(0, 8)}
              extra={<Tag color="blue">LIVE</Tag>}
            >
              <Row gutter={16}>
                <Col span={8}>
                  <Statistic
                    title="Home"
                    value={u.home_odds}
                    precision={2}
                    valueStyle={{ color: "#3f8600" }}
                    prefix={<ArrowUpOutlined />}
                  />
                </Col>
                <Col span={8}>
                  {u.draw_odds && (
                    <Statistic title="Draw" value={u.draw_odds} precision={2} valueStyle={{ color: "#faad14" }} />
                  )}
                </Col>
                <Col span={8}>
                  <Statistic
                    title="Away"
                    value={u.away_odds}
                    precision={2}
                    valueStyle={{ color: "#cf1322" }}
                    prefix={<ArrowDownOutlined />}
                  />
                </Col>
              </Row>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Updated: {new Date(u.timestamp).toLocaleTimeString()}
              </Text>
            </Card>
          </Col>
        ))}
      </Row>
      {updates.length === 0 && (
        <Card>
          <Text type="secondary">No live odds available for {sport}. Select another sport or wait for market open.</Text>
        </Card>
      )}
    </div>
  );
}
