import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Card, Typography, Table, Tag, Button, Space, Modal, Form, Input, Select, message } from "antd";
import { ReloadOutlined, PlusOutlined, LockOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";

const { Title } = Typography;

export default function SARManagement() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["sars", page],
    queryFn: () => regulatoryService.getSARs({ page }),
  });

  const createMutation = useMutation({
    mutationFn: regulatoryService.createSAR,
    onSuccess: () => {
      message.success("SAR created");
      setModalOpen(false);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ["sars"] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => regulatoryService.updateSARStatus(id, status),
    onSuccess: () => {
      message.success("Status updated");
      queryClient.invalidateQueries({ queryKey: ["sars"] });
    },
  });

  const items = data?.data || [];

  const columns = [
    { title: "ID", dataIndex: "id", render: (v: string) => v.slice(0, 8) },
    { title: "Jurisdiction", dataIndex: "jurisdiction", render: (v: string) => v.toUpperCase() },
    { title: "Player", dataIndex: "player_id" },
    { title: "Trigger", dataIndex: "trigger_type" },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => {
        const colors: Record<string, string> = { draft: "default", internal_review: "orange", submitted: "blue", acknowledged: "green" };
        return <Tag color={colors[v]}>{v.toUpperCase()}</Tag>;
      },
    },
    {
      title: "Tipping-off Lock",
      dataIndex: "tipping_off_lock",
      render: (v: boolean) => (v ? <Tag color="red"><LockOutlined /> LOCKED</Tag> : <Tag>UNLOCKED</Tag>),
    },
    {
      title: "Actions",
      render: (_: any, r: any) => (
        <Space>
          {r.status === "draft" && (
            <Button size="small" onClick={() => updateMutation.mutate({ id: r.id, status: "internal_review" })}>Review</Button>
          )}
          {r.status === "internal_review" && (
            <Button size="small" type="primary" onClick={() => updateMutation.mutate({ id: r.id, status: "submitted" })}>Submit</Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>SAR Management</Title>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>Refresh</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>New SAR</Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={items} columns={columns} loading={isLoading} pagination={{ current: page, pageSize: 20, total: data?.pagination?.total, onChange: setPage }} />
      </Card>
      <Modal title="Create SAR" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()} confirmLoading={createMutation.isPending}>
        <Form form={form} layout="vertical" onFinish={(values) => createMutation.mutate(values)}>
          <Form.Item name="jurisdiction" label="Jurisdiction" rules={[{ required: true }]}>
            <Select options={[{ value: "ukgc" }, { value: "mga" }, { value: "curacao" }, { value: "general" }]} />
          </Form.Item>
          <Form.Item name="player_id" label="Player ID" rules={[{ required: true }]}>
            <Input type="number" />
          </Form.Item>
          <Form.Item name="trigger_type" label="Trigger Type" rules={[{ required: true }]}>
            <Select options={[{ value: "manual" }, { value: "auto_aml" }, { value: "auto_threshold" }]} />
          </Form.Item>
          <Form.Item name="description" label="Description" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
