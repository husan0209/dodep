import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Card, Typography, Table, Tag, Button, Space, Modal, Form, Input, Select, message } from "antd";
import { ReloadOutlined, PlusOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";

const { Title } = Typography;

export default function ComplaintsLog() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["complaints", page],
    queryFn: () => regulatoryService.getComplaints({ page }),
  });

  const createMutation = useMutation({
    mutationFn: regulatoryService.createComplaint,
    onSuccess: () => {
      message.success("Complaint created");
      setModalOpen(false);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ["complaints"] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, status, resolution }: { id: string; status: string; resolution?: string }) =>
      regulatoryService.updateComplaintStatus(id, status, resolution),
    onSuccess: () => {
      message.success("Status updated");
      queryClient.invalidateQueries({ queryKey: ["complaints"] });
    },
  });

  const items = data?.data || [];

  const columns = [
    { title: "ID", dataIndex: "id", render: (v: string) => v.slice(0, 8) },
    { title: "Player", dataIndex: "player_id" },
    { title: "Category", dataIndex: "category" },
    { title: "Description", dataIndex: "description", ellipsis: true },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => {
        const colors: Record<string, string> = { open: "red", investigating: "orange", resolved: "green", escalated_to_adr: "blue" };
        return <Tag color={colors[v]}>{v.toUpperCase()}</Tag>;
      },
    },
    { title: "ADR Ref", dataIndex: "adr_ref", render: (v?: string) => v || "—" },
    {
      title: "Actions",
      render: (_: any, r: any) => (
        <Space>
          {r.status === "open" && (
            <Button size="small" onClick={() => updateMutation.mutate({ id: r.id, status: "investigating" })}>Investigate</Button>
          )}
          {r.status === "investigating" && (
            <>
              <Button size="small" onClick={() => updateMutation.mutate({ id: r.id, status: "resolved", resolution: "Resolved by operator" })}>Resolve</Button>
              <Button size="small" type="primary" danger onClick={() => updateMutation.mutate({ id: r.id, status: "escalated_to_adr" })}>Escalate</Button>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>Player Complaints Log</Title>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>Refresh</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>Log Complaint</Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={items} columns={columns} loading={isLoading} pagination={{ current: page, pageSize: 20, total: data?.pagination?.total, onChange: setPage }} />
      </Card>
      <Modal title="Log Complaint" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()} confirmLoading={createMutation.isPending}>
        <Form form={form} layout="vertical" onFinish={(values) => createMutation.mutate(values)}>
          <Form.Item name="player_id" label="Player ID" rules={[{ required: true }]}>
            <Input type="number" />
          </Form.Item>
          <Form.Item name="category" label="Category" rules={[{ required: true }]}>
            <Select options={[{ value: "payment" }, { value: "bonus" }, { value: "technical" }, { value: "fairness" }, { value: "rg" }]} />
          </Form.Item>
          <Form.Item name="description" label="Description" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
