import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
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
  message,
  Modal,
  Form,
} from "antd";
import { ReloadOutlined, PlusOutlined, EditOutlined } from "@ant-design/icons";
import { sportsService } from "@/services/sports.service";
import type { SportEvent } from "@/types/bet";

const { Title } = Typography;

const STATUS_COLORS: Record<string, string> = {
  upcoming: "blue",
  live: "green",
  completed: "default",
  cancelled: "red",
};

export default function SportsEvents() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [editModal, setEditModal] = useState(false);
  const [selected, setSelected] = useState<SportEvent | null>(null);
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["sport-events", search, statusFilter],
    queryFn: () =>
      sportsService.getEvents({
        search: search || undefined,
        status: statusFilter as any,
      }),
  });

  const toggleLive = (id: string, live: boolean) => {
    sportsService
      .updateEvent(id, { live })
      .then(() => {
        message.success(live ? "Event is now live" : "Event paused");
        refetch();
      })
      .catch(() => message.error("Failed to update"));
  };

  const columns = [
    { title: "ID", dataIndex: "id", width: 80, render: (v: string) => v.slice(0, 8) },
    { title: "Sport", dataIndex: "sport" },
    { title: "League", dataIndex: "league" },
    { title: "Home", dataIndex: "home_team" },
    { title: "Away", dataIndex: "away_team" },
    {
      title: "Starts At",
      dataIndex: "starts_at",
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => <Tag color={STATUS_COLORS[v]}>{v.toUpperCase()}</Tag>,
    },
    {
      title: "Live",
      render: (_: unknown, r: SportEvent) => (
        <Switch checked={r.live} onChange={(checked) => toggleLive(r.id, checked)} />
      ),
    },
    {
      title: "Actions",
      render: (_: unknown, r: SportEvent) => (
        <Button
          icon={<EditOutlined />}
          size="small"
          onClick={() => {
            setSelected(r);
            form.setFieldsValue(r);
            setEditModal(true);
          }}
        >
          Edit
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Sports Events & Markets</Title>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="Search events..."
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
            { value: "upcoming", label: "Upcoming" },
            { value: "live", label: "Live" },
            { value: "completed", label: "Completed" },
            { value: "cancelled", label: "Cancelled" },
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
          Refresh
        </Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setSelected(null); form.resetFields(); setEditModal(true); }}>
          Add Event
        </Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={data?.data || []} columns={columns} loading={isLoading} pagination={{ pageSize: 20 }} />
      </Card>
      <Modal
        title={selected ? "Edit Event" : "New Event"}
        open={editModal}
        onCancel={() => setEditModal(false)}
        onOk={() => form.submit()}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="sport" label="Sport" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="league" label="League">
            <Input />
          </Form.Item>
          <Form.Item name="home_team" label="Home Team" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="away_team" label="Away Team" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="starts_at" label="Starts At" rules={[{ required: true }]}>
            <Input type="datetime-local" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
