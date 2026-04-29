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
  message,
} from "antd";
import { PlusOutlined, EditOutlined, ReloadOutlined, CopyOutlined } from "@ant-design/icons";
import { contentService } from "@/services/content.service";

const { Title } = Typography;

interface CommTemplate {
  id: string;
  name: string;
  channel: "email" | "sms" | "push";
  subject?: string;
  locale: string;
  status: "active" | "draft" | "archived";
  last_used?: string;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  active: "green",
  draft: "default",
  archived: "orange",
};

export default function Templates() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [channelFilter, setChannelFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState<{ open: boolean; record: CommTemplate | null }>({
    open: false,
    record: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["comm-templates", search, channelFilter],
    queryFn: () => contentService.getTemplates({ search: search || undefined, channel: channelFilter }) as Promise<CommTemplate[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<CommTemplate>) =>
      editModal.record
        ? contentService.updateTemplate(editModal.record.id, values)
        : contentService.createTemplate(values as Omit<CommTemplate, "id" | "created_at">),
    onSuccess: () => {
      message.success("Template saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["comm-templates"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save template"),
  });

  const columns = [
    { title: "Name", dataIndex: "name" },
    { title: "Subject", dataIndex: "subject", ellipsis: true },
    {
      title: "Channel",
      dataIndex: "channel",
      render: (v: string) => <Tag>{v.toUpperCase()}</Tag>,
    },
    { title: "Locale", dataIndex: "locale", width: 80 },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={STATUS_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    {
      title: "Actions",
      render: (_: unknown, r: CommTemplate) => (
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
          <Button icon={<CopyOutlined />} size="small" onClick={() => message.info("Copied to clipboard")}>
            Copy
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Communication Templates</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search placeholder="Search templates..." allowClear onSearch={setSearch} style={{ width: 240 }} />
        <Select
          allowClear
          placeholder="Channel"
          style={{ width: 140 }}
          onChange={setChannelFilter}
          options={[
            { value: "email", label: "Email" },
            { value: "sms", label: "SMS" },
            { value: "push", label: "Push" },
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
          New Template
        </Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={data || []} columns={columns} loading={isLoading} pagination={{ pageSize: 20 }} />
      </Card>

      <Modal
        title={editModal.record ? "Edit Template" : "New Template"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={720}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item name="name" label="Template Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="channel" label="Channel" rules={[{ required: true }]} initialValue="email">
            <Select options={[{ value: "email" }, { value: "sms" }, { value: "push" }]} />
          </Form.Item>
          <Form.Item name="subject" label="Subject">
            <Input />
          </Form.Item>
          <Form.Item name="locale" label="Locale" initialValue="en">
            <Select options={[{ value: "en" }, { value: "ru" }, { value: "de" }]} />
          </Form.Item>
          <Form.Item name="status" label="Status" initialValue="draft">
            <Select options={[{ value: "draft" }, { value: "active" }, { value: "archived" }]} />
          </Form.Item>
          <Form.Item name="body" label="Body (HTML/Text)">
            <Input.TextArea rows={8} placeholder="Template body..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
