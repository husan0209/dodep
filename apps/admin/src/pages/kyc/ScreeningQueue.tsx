import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Table,
  Tag,
  Space,
  Button,
  Select,
  Card,
  Typography,
  Drawer,
  Descriptions,
  message,
} from "antd";
import { CheckCircleOutlined, CloseCircleOutlined, EyeOutlined, ReloadOutlined } from "@ant-design/icons";
import { kycService } from "@/services/kyc.service";
import type { ScreeningResult, ScreeningStatus } from "@/types/kyc";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  clear: "green",
  pep_match: "orange",
  sanctions_hit: "red",
  review_required: "blue",
};

export default function ScreeningQueue() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [selected, setSelected] = useState<ScreeningResult | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["kyc-screenings", statusFilter],
    queryFn: () =>
      kycService.getScreenings({
        status: statusFilter,
        page: 1,
        page_size: 50,
      }),
  });

  const handleReview = async (decision: ScreeningStatus) => {
    if (!selected) return;
    try {
      await kycService.reviewScreening(selected.id, { decision, notes: `Reviewed as ${decision}` });
      message.success("Screening reviewed");
      setDrawerOpen(false);
      refetch();
    } catch {
      message.error("Failed to review screening");
    }
  };

  const columns = [
    { title: "Player", render: (_: unknown, r: ScreeningResult) => r.player_email },
    {
      title: "Status",
      render: (_: unknown, r: ScreeningResult) => (
        <Tag color={STATUS_COLORS[r.status]}>{r.status.replace(/_/g, " ").toUpperCase()}</Tag>
      ),
    },
    {
      title: "Match Score",
      render: (_: unknown, r: ScreeningResult) => (r.match_score ? `${r.match_score}%` : "—"),
    },
    {
      title: "Matched Lists",
      render: (_: unknown, r: ScreeningResult) =>
        r.matched_lists.length > 0 ? r.matched_lists.join(", ") : "—",
    },
    { title: "Screened At", dataIndex: "screened_at" },
    {
      title: "Actions",
      render: (_: unknown, r: ScreeningResult) => (
        <Button
          icon={<EyeOutlined />}
          onClick={() => {
            setSelected(r);
            setDrawerOpen(true);
          }}
        >
          Review
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>PEP / Sanctions Screening</Title>
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Select
            placeholder="Status"
            allowClear
            onChange={setStatusFilter}
            style={{ width: 160 }}
            options={[
              { value: "pep_match", label: "PEP Match" },
              { value: "sanctions_hit", label: "Sanctions Hit" },
              { value: "review_required", label: "Review Required" },
              { value: "clear", label: "Clear" },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            Refresh
          </Button>
        </Space>
      </Card>

      <Table
        columns={columns}
        dataSource={data?.data || []}
        rowKey="id"
        loading={isLoading}
        pagination={{ pageSize: 50, total: data?.pagination?.total }}
      />

      <Drawer
        title="Screening Review"
        width={640}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        {selected && (
          <>
            <Descriptions column={1} bordered>
              <Descriptions.Item label="Player">{selected.player_email}</Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={STATUS_COLORS[selected.status]}>
                  {selected.status.replace(/_/g, " ").toUpperCase()}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Match Score">{selected.match_score}%</Descriptions.Item>
              <Descriptions.Item label="Matched Lists">
                {selected.matched_lists.join(", ") || "None"}
              </Descriptions.Item>
              <Descriptions.Item label="Screened At">{selected.screened_at}</Descriptions.Item>
              <Descriptions.Item label="Screened By">{selected.screened_by}</Descriptions.Item>
              <Descriptions.Item label="Next Screen">
                {selected.next_screen_at || "—"}
              </Descriptions.Item>
            </Descriptions>

            <Space style={{ marginTop: 24 }}>
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={() => handleReview("clear")}
              >
                Mark Clear
              </Button>
              <Button danger icon={<CloseCircleOutlined />} onClick={() => handleReview("sanctions_hit")}>
                Confirm Sanctions
              </Button>
              <Button onClick={() => handleReview("review_required")}>Keep Review</Button>
            </Space>
          </>
        )}
      </Drawer>
    </div>
  );
}
