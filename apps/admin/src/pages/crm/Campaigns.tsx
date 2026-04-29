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
  Modal,
  Form,
  DatePicker,
  message,
} from "antd";
import { PlusOutlined, EditOutlined, ReloadOutlined, SendOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { contentService } from "@/services/content.service";

const { Title } = Typography;

type CampaignStatus = "draft" | "scheduled" | "running" | "paused" | "completed";
interface Campaign {
  id: string;
  name: string;
  status: CampaignStatus;
  type: "email" | "sms" | "push";
  segment?: string;
  sent_count: number;
  open_rate?: number;
  scheduled_at?: string;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  draft: "default",
  scheduled: "blue",
  running: "green",
  paused: "orange",
  completed: "default",
};

export default function Campaigns() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState<{ open: boolean; record: Campaign | null }>({
    open: false,
    record: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["campaigns", search, statusFilter],
    queryFn: () => contentService.getCampaigns({ search: search || undefined, status: statusFilter }) as unknown as Promise<Campaign[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<Campaign>) =>
      editModal.record
        ? contentService.updateCampaign(editModal.record.id, values as any)
        : contentService.createCampaign(values as any),
    onSuccess: () => {
      message.success("Campaign saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["campaigns"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save campaign"),
  });

  const sendMutation = useMutation({
    mutationFn: (id: string) => contentService.launchCampaign(id),
    onSuccess: () => {
      message.success("Campaign launched");
      queryClient.invalidateQueries({ queryKey: ["campaigns"] });
    },
    onError: () => message.error("Failed to launch campaign"),
  });

  const columns = [
    { title: "Name", dataIndex: "name" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={STATUS_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    { title: "Type", dataIndex: "type", render: (v: string) => <Tag>{v.toUpperCase()}</Tag> },
    { title: "Segment", dataIndex: "segment", render: (v?: string) => v || "All Users" },
    { title: "Sent", dataIndex: "sent_count" },
    {
      title: "Open Rate",
      dataIndex: "open_rate",
      render: (v?: number) => (v !== undefined ? `${v}%` : "—"),
    },
    {
      title: "Actions",
      render: (_: unknown, r: Campaign) => (
        <Space>
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => {
              setEditModal({ open: true, record: r });
              form.setFieldsValue({
                ...r,
                scheduled_at: r.scheduled_at ? dayjs(r.scheduled_at) : undefined,
              });
            }}
          >
            Edit
          </Button>
          {r.status === "draft" && (
            <Button
              icon={<SendOutlined />}
              size="small"
              type="primary"
              loading={sendMutation.isPending}
              onClick={() => sendMutation.mutate(r.id)}
            >
              Launch
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Campaigns</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="Search campaigns..."
          allowClear
          onSearch={setSearch}
          style={{ width: 240 }}
        />
        <Select
          allowClear
          placeholder="Status"
          style={{ width: 140 }}
          onChange={setStatusFilter}
          options={[
            { value: "draft", label: "Draft" },
            { value: "scheduled", label: "Scheduled" },
            { value: "running", label: "Running" },
            { value: "paused", label: "Paused" },
            { value: "completed", label: "Completed" },
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
          New Campaign
        </Button>
      </Space>
      <Card>
        <Table
          rowKey="id"
          dataSource={data || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>

      <Modal
        title={editModal.record ? "Edit Campaign" : "New Campaign"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item name="name" label="Campaign Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type" rules={[{ required: true }]} initialValue="email">
            <Select
              options={[
                { value: "email", label: "Email" },
                { value: "sms", label: "SMS" },
                { value: "push", label: "Push Notification" },
              ]}
            />
          </Form.Item>
          <Form.Item name="segment" label="Target Segment">
            <Input placeholder="e.g. vip, high-rollers" />
          </Form.Item>
          <Form.Item name="scheduled_at" label="Schedule At">
            <DatePicker showTime style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
