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
import { PlusOutlined, EditOutlined, ReloadOutlined, LockOutlined, UnlockOutlined } from "@ant-design/icons";
import { usersService } from "@/services/users.service";
import type { AdminUser } from "@/types/admin";

const { Title } = Typography;

const ROLE_COLORS: Record<string, string> = {
  super_admin: "red",
  admin: "orange",
  manager: "blue",
  support: "green",
  viewer: "default",
};

export default function AdminUsers() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState<{ open: boolean; record: AdminUser | null }>({
    open: false,
    record: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["admin-users", search, roleFilter],
    queryFn: () =>
      usersService.getAdminUsers({
        search: search || undefined,
        role: roleFilter,
      }) as Promise<AdminUser[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Partial<AdminUser>) =>
      editModal.record
        ? usersService.updateAdminUser(editModal.record.id, values)
        : usersService.createAdminUser(values as Omit<AdminUser, "id" | "created_at">),
    onSuccess: () => {
      message.success("Admin user saved");
      setEditModal({ open: false, record: null });
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to save admin user"),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, locked }: { id: string; locked: boolean }) =>
      usersService.updateAdminUser(id, { locked }),
    onSuccess: () => {
      message.success("User status updated");
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    },
  });

  const columns = [
    { title: "Name", dataIndex: "name" },
    { title: "Email", dataIndex: "email" },
    {
      title: "Role",
      dataIndex: "role",
      render: (v: string) => <Tag color={ROLE_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    {
      title: "Status",
      dataIndex: "locked",
      render: (v: boolean) => (v ? <Tag color="red">LOCKED</Tag> : <Tag color="green">ACTIVE</Tag>),
    },
    {
      title: "2FA",
      dataIndex: "totp_enabled",
      render: (v: boolean) => (v ? <Tag color="green">ON</Tag> : <Tag>OFF</Tag>),
    },
    { title: "Last Login", dataIndex: "last_login_at", render: (v?: string) => (v ? new Date(v).toLocaleString() : "Never") },
    {
      title: "Actions",
      render: (_: unknown, r: AdminUser) => (
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
          <Button
            icon={r.locked ? <UnlockOutlined /> : <LockOutlined />}
            size="small"
            danger={!r.locked}
            onClick={() => toggleMutation.mutate({ id: r.id, locked: !r.locked })}
            loading={toggleMutation.isPending}
          >
            {r.locked ? "Unlock" : "Lock"}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Admin Users Management</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search placeholder="Search..." allowClear onSearch={setSearch} style={{ width: 240 }} />
        <Select
          allowClear
          placeholder="Role"
          style={{ width: 140 }}
          onChange={setRoleFilter}
          options={[
            { value: "super_admin", label: "Super Admin" },
            { value: "admin", label: "Admin" },
            { value: "manager", label: "Manager" },
            { value: "support", label: "Support" },
            { value: "viewer", label: "Viewer" },
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
          New Admin
        </Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={data || []} columns={columns} loading={isLoading} pagination={{ pageSize: 20 }} />
      </Card>

      <Modal
        title={editModal.record ? "Edit Admin User" : "New Admin User"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, record: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        width={500}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="email" label="Email" rules={[{ required: true, type: "email" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="role" label="Role" rules={[{ required: true }]} initialValue="viewer">
            <Select
              options={[
                { value: "super_admin", label: "Super Admin" },
                { value: "admin", label: "Admin" },
                { value: "manager", label: "Manager" },
                { value: "support", label: "Support" },
                { value: "viewer", label: "Viewer" },
              ]}
            />
          </Form.Item>
          {!editModal.record && (
            <Form.Item name="password" label="Initial Password" rules={[{ required: true }]}>
              <Input.Password />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  );
}
