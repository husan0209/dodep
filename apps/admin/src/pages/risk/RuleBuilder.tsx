import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Button,
  Table,
  Tag,
  Space,
  Input,
  Select,
  Switch,
  Modal,
  Form,
  InputNumber,
  message,
} from "antd";
import { PlusOutlined, EditOutlined, ReloadOutlined, DeleteOutlined } from "@ant-design/icons";
import { riskService } from "@/services/risk.service";

const { Title } = Typography;

interface RiskRule {
  id: string;
  name: string;
  description?: string;
  rule_type: "velocity" | "threshold" | "pattern" | "mule" | "geo";
  priority: number;
  enabled: boolean;
  condition: Record<string, unknown>;
  action: "flag" | "block" | "review" | "notify";
  hit_count: number;
  last_hit?: string;
  created_at: string;
}

const TYPE_COLORS: Record<string, string> = {
  velocity: "blue",
  threshold: "orange",
  pattern: "purple",
  mule: "red",
  geo: "cyan",
};

const ACTION_COLORS: Record<string, string> = {
  flag: "orange",
  block: "red",
  review: "blue",
  notify: "green",
};

export default function RuleBuilder() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState<{ open: boolean; record: RiskRule | null }>({
    open: false,
    record: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["risk-rules", search, typeFilter],
    queryFn: () => riskService.getRules({ search: search || undefined, rule_type: typeFilter }) as Promise<RiskRule[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<RiskRule>) =>
      editModal.record
        ? riskService.updateRule(editModal.record.id, values)
        : riskService.createRule(values as any),
    onSuccess: () => {
      message.success("Rule saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["risk-rules"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save rule"),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => riskService.updateRule(id, { enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["risk-rules"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => riskService.deleteRule(id),
    onSuccess: () => {
      message.success("Rule deleted");
      queryClient.invalidateQueries({ queryKey: ["risk-rules"] });
    },
  });

  const columns = [
    { title: "Name", dataIndex: "name" },
    { title: "Description", dataIndex: "description", ellipsis: true },
    {
      title: "Type",
      dataIndex: "rule_type",
      render: (v: string) => <Tag color={TYPE_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    { title: "Priority", dataIndex: "priority" },
    {
      title: "Enabled",
      render: (_: unknown, r: RiskRule) => (
        <Switch checked={r.enabled} onChange={(checked) => toggleMutation.mutate({ id: r.id, enabled: checked })} />
      ),
    },
    {
      title: "Action",
      dataIndex: "action",
      render: (v: string) => <Tag color={ACTION_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    { title: "Hits", dataIndex: "hit_count" },
    {
      title: "Actions",
      render: (_: unknown, r: RiskRule) => (
        <Space>
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => {
              setEditModal({ open: true, record: r });
              form.setFieldsValue(r);
            }}
          >
            Edit
          </Button>
          <Button icon={<DeleteOutlined />} size="small" danger onClick={() => deleteMutation.mutate(r.id)} loading={deleteMutation.isPending}>
            Delete
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Risk Rule Builder</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search placeholder="Search rules..." allowClear onSearch={setSearch} style={{ width: 240 }} />
        <Select
          allowClear
          placeholder="Type"
          style={{ width: 140 }}
          onChange={setTypeFilter}
          options={[
            { value: "velocity", label: "Velocity" },
            { value: "threshold", label: "Threshold" },
            { value: "pattern", label: "Pattern" },
            { value: "mule", label: "Mule" },
            { value: "geo", label: "Geo" },
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
          Refresh
        </Button>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setEditModal({ open: true, record: null });
            form.resetFields();
          }}
        >
          New Rule
        </Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={data || []} columns={columns} loading={isLoading} pagination={{ pageSize: 20 }} />
      </Card>

      <Modal
        title={editModal.record ? "Edit Rule" : "New Rule"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={720}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item name="name" label="Rule Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="rule_type" label="Type" rules={[{ required: true }]} initialValue="velocity">
            <Select options={[
              { value: "velocity", label: "Velocity" },
              { value: "threshold", label: "Threshold" },
              { value: "pattern", label: "Pattern" },
              { value: "mule", label: "Mule" },
              { value: "geo", label: "Geo" },
            ]} />
          </Form.Item>
          <Form.Item name="action" label="Action" rules={[{ required: true }]} initialValue="flag">
            <Select options={[
              { value: "flag", label: "Flag" },
              { value: "block", label: "Block" },
              { value: "review", label: "Review" },
              { value: "notify", label: "Notify" },
            ]} />
          </Form.Item>
          <Form.Item name="priority" label="Priority" initialValue={100}>
            <InputNumber min={1} max={1000} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          <Form.Item name="condition" label="Condition (JSON)">
            <Input.TextArea rows={4} placeholder='{"field": "amount", "operator": ">", "value": 1000}' />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
