import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Card, Table, InputNumber, Switch, Button, message, Typography } from "antd";
import { SaveOutlined, ReloadOutlined } from "@ant-design/icons";
import { supportService } from "@/services/support.service";
import type { SlaConfig } from "@/types/support";
import { useState } from "react";

const { Title } = Typography;

export default function SlaConfigPage() {
  const queryClient = useQueryClient();
  const [edits, setEdits] = useState<Record<string, Partial<SlaConfig>>>({});

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["sla-config"],
    queryFn: () => supportService.getSlaConfig(),
  });

  const updateMutation = useMutation({
    mutationFn: async ({ category, payload }: { category: string; payload: Partial<SlaConfig> }) => {
      await supportService.updateSlaConfig(category, payload);
    },
    onSuccess: () => {
      message.success("SLA config updated");
      queryClient.invalidateQueries({ queryKey: ["sla-config"] });
    },
  });

  const handleSave = (item: SlaConfig) => {
    const payload = edits[item.category];
    if (!payload) return;
    updateMutation.mutate({ category: item.category, payload });
  };

  const handleChange = (category: string, field: keyof SlaConfig, value: unknown) => {
    setEdits((prev) => ({
      ...prev,
      [category]: { ...prev[category], [field]: value },
    }));
  };

  const columns = [
    { title: "Category", dataIndex: "category", render: (v: string) => v.toUpperCase() },
    {
      title: "First Response (min)",
      render: (_: unknown, record: SlaConfig) => (
        <InputNumber
          value={edits[record.category]?.first_response_minutes ?? record.first_response_minutes}
          onChange={(v) => handleChange(record.category, "first_response_minutes", v)}
          min={1}
          style={{ width: 100 }}
        />
      ),
    },
    {
      title: "Resolution (min)",
      render: (_: unknown, record: SlaConfig) => (
        <InputNumber
          value={edits[record.category]?.resolution_minutes ?? record.resolution_minutes}
          onChange={(v) => handleChange(record.category, "resolution_minutes", v)}
          min={1}
          style={{ width: 100 }}
        />
      ),
    },
    {
      title: "Active",
      render: (_: unknown, record: SlaConfig) => (
        <Switch
          checked={edits[record.category]?.active ?? record.active}
          onChange={(v) => handleChange(record.category, "active", v)}
        />
      ),
    },
    {
      title: "Actions",
      render: (_: unknown, record: SlaConfig) => (
        <Button
          type="primary"
          icon={<SaveOutlined />}
          size="small"
          onClick={() => handleSave(record)}
          disabled={!edits[record.category]}
        >
          Save
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>SLA Configuration</Title>
      <Card
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            Refresh
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={data || []}
          rowKey="category"
          loading={isLoading}
          pagination={false}
        />
      </Card>
    </div>
  );
}
