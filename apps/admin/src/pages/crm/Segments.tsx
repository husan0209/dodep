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
  Modal,
  Form,
  Select,
  InputNumber,
  message,
} from "antd";
import { PlusOutlined, EditOutlined, ReloadOutlined } from "@ant-design/icons";
import { usersService } from "@/services/users.service";

const { Title } = Typography;

interface PlayerSegment {
  id: string;
  name: string;
  description?: string;
  criteria: Record<string, unknown>;
  user_count: number;
  tags: string[];
  created_at: string;
}

export default function Segments() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [editModal, setEditModal] = useState<{ open: boolean; record: PlayerSegment | null }>({
    open: false,
    record: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["segments", search],
    queryFn: () => usersService.getSegments({ search: search || undefined }) as Promise<PlayerSegment[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<PlayerSegment>) =>
      editModal.record
        ? usersService.updateSegment(editModal.record.id, values)
        : usersService.createSegment(values as Omit<PlayerSegment, "id" | "created_at" | "user_count">),
    onSuccess: () => {
      message.success("Segment saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["segments"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save segment"),
  });

  const columns = [
    { title: "Name", dataIndex: "name" },
    { title: "Description", dataIndex: "description", ellipsis: true },
    {
      title: "Users",
      dataIndex: "user_count",
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: "Tags",
      dataIndex: "tags",
      render: (tags: string[]) => (
        <Space>
          {tags.map((t) => (
            <Tag key={t}>{t}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: "Actions",
      render: (_: unknown, r: PlayerSegment) => (
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
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Player Segments</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="Search segments..."
          allowClear
          onSearch={setSearch}
          style={{ width: 240 }}
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
          New Segment
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
        title={editModal.record ? "Edit Segment" : "New Segment"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item name="name" label="Segment Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="tags" label="Tags">
            <Select mode="tags" placeholder="Add tags..." />
          </Form.Item>
          <Form.Item name="criteria" label="Criteria (JSON)">
            <Input.TextArea rows={4} placeholder='{"min_deposits": 5, "vip": true}' />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
