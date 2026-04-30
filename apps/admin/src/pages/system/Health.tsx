import { Card, Typography, Row, Col, Tag, Spin, Button, Space } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { systemService } from "@/services/system.service";

const { Title, Text } = Typography;

const STATUS_COLORS: Record<string, string> = {
  healthy: "green",
  degraded: "orange",
  unhealthy: "red",
};

export default function Health() {
  const { data, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ["system-health"],
    queryFn: systemService.getHealthStatus,
    refetchInterval: 30000,
  });

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
          System Health
        </Title>
        <Button
          icon={<ReloadOutlined />}
          onClick={() => refetch()}
          loading={isRefetching}
        >
          Refresh
        </Button>
      </div>

      {isLoading ? (
        <Spin size="large" style={{ display: "block", margin: "100px auto" }} />
      ) : (
        <Row gutter={[16, 16]}>
          {data &&
            Object.entries(data).map(([service, info]) => (
              <Col xs={24} sm={12} md={8} lg={6} key={service}>
                <Card size="small">
                  <Space
                    direction="vertical"
                    size="small"
                    style={{ width: "100%" }}
                  >
                    <Text strong>{service}</Text>
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "center",
                      }}
                    >
                      <Tag color={STATUS_COLORS[info.status] || "default"}>
                        {info.status}
                      </Tag>
                      <Text type="secondary">{info.latency_ms}ms</Text>
                    </div>
                  </Space>
                </Card>
              </Col>
            ))}
        </Row>
      )}
    </div>
  );
}
