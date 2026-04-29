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
  Switch,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  EyeOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { contentService, type ContentPage as CmsPage } from "@/services/content.service";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  draft: "default",
  published: "green",
  archived: "orange",
};

export default function CmsPages() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState<{
    open: boolean;
    record: CmsPage | null;
  }>({ open: false, record: null });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["cms-pages", search, statusFilter],
    queryFn: () =>
      contentService.getPages({
        status: statusFilter as any,
        search: search || undefined,
      }),
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<CmsPage> & { id?: string }) =>
      values.id
        ? (contentService.updatePage(values.id, values as any) as Promise<CmsPage>)
        : (contentService.createPage(values as any) as Promise<CmsPage>),
    onSuccess: () => {
      message.success("Page saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["cms-pages"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save page"),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, published }: { id: string; published: boolean }) =>
      contentService.updatePage(id, { status: published ? "published" : "draft" }),
    onSuccess: () => {
      message.success("Status updated");
      queryClient.invalidateQueries({ queryKey: ["cms-pages"] });
    },
  });

  const columns = [
    { title: "Slug", dataIndex: "slug", render: (v: string) => <code>{v}</code> },
    { title: "Title", dataIndex: "title" },
    { title: "Locale", dataIndex: "locale", width: 80 },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={STATUS_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    {
      title: "Published",
      render: (_: unknown, r: CmsPage) => (
        <Switch
          checked={r.status === "published"}
          onChange={(checked) => toggleMutation.mutate({ id: r.id, published: checked })}
        />
      ),
    },
    {
      title: "Updated",
      dataIndex: "updated_at",
      render: (v: string) => new Date(v).toLocaleDateString(),
    },
    {
      title: "Actions",
      render: (_: unknown, r: CmsPage) => (
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
          <Button icon={<EyeOutlined />} size="small" href={`/page/${r.slug}`} target="_blank">
            View
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Content Management</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="Search pages..."
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
            { value: "published", label: "Published" },
            { value: "archived", label: "Archived" },
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
          New Page
        </Button>
      </Space>
      <Card>
        <Table
          rowKey="id"
          dataSource={(data?.data || []) as CmsPage[]}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>

      <Modal
        title={editModal.record ? "Edit Page" : "New Page"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={720}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) =>
            saveMutation.mutate({
              ...values,
              id: editModal.record?.id,
            })
          }
        >
          <Form.Item
            name="slug"
            label="Slug"
            rules={[{ required: true }]}
          >
            <Input placeholder="terms-and-conditions" />
          </Form.Item>
          <Form.Item
            name="title"
            label="Title"
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="locale" label="Locale" initialValue="en">
            <Select options={[{ value: "en" }, { value: "ru" }, { value: "de" }]} />
          </Form.Item>
          <Form.Item name="status" label="Status" initialValue="draft">
            <Select
              options={[
                { value: "draft", label: "Draft" },
                { value: "published", label: "Published" },
                { value: "archived", label: "Archived" },
              ]}
            />
          </Form.Item>
          <Form.Item name="content" label="Content (HTML/Markdown)">
            <Input.TextArea rows={8} placeholder="Page content..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
