import { Card, Typography, Empty, Tag } from "antd";

const { Title, Text } = Typography;

interface PlaceholderPageProps {
  title: string;
  description?: string;
  status?: "planned" | "in_progress" | "coming_soon";
}

export default function PlaceholderPage({
  title,
  description = "This module is planned for an upcoming iteration.",
  status = "planned",
}: PlaceholderPageProps) {
  const statusMap = {
    planned: { color: "blue", text: "Planned" },
    in_progress: { color: "orange", text: "In Progress" },
    coming_soon: { color: "green", text: "Coming Soon" },
  };

  const { color, text } = statusMap[status];

  return (
    <Card>
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <div style={{ textAlign: "center" }}>
            <Title level={4} style={{ marginTop: 16 }}>
              {title}
            </Title>
            <Tag color={color}>{text}</Tag>
            <Text type="secondary" style={{ display: "block", marginTop: 8 }}>
              {description}
            </Text>
          </div>
        }
      />
    </Card>
  );
}
