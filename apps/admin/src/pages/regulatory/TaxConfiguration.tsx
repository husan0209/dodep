import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Card, Typography, Table, Button, Space, Modal, Form, Input, Select, message } from "antd";
import { ReloadOutlined, PlusOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";

const { Title } = Typography;

export default function TaxConfiguration() {
  const queryClient = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["tax-configs"],
    queryFn: () => regulatoryService.getTaxConfigs(),
  });

  const mutation = useMutation({
    mutationFn: regulatoryService.saveTaxConfig,
    onSuccess: () => {
      message.success("Tax config saved");
      setModalOpen(false);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ["tax-configs"] });
    },
  });

  const columns = [
    { title: "Jurisdiction", dataIndex: "jurisdiction", render: (v: string) => v.toUpperCase() },
    { title: "Tax Type", dataIndex: "tax_type" },
    { title: "Tax Base", dataIndex: "tax_base" },
    { title: "Rate", dataIndex: "rate", render: (v: string) => `${v}%` },
    { title: "Currency", dataIndex: "currency" },
    { title: "Effective From", dataIndex: "effective_from" },
  ];

  return (
    <div>
      <Title level={3}>Tax Configuration</Title>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>Refresh</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>Add Rate</Button>
      </Space>
      <Card>
        <Table rowKey="id" dataSource={data || []} columns={columns} loading={isLoading} />
      </Card>
      <Modal title="Add Tax Rate" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()} confirmLoading={mutation.isPending}>
        <Form form={form} layout="vertical" onFinish={(values) => mutation.mutate(values)}>
          <Form.Item name="jurisdiction" label="Jurisdiction" rules={[{ required: true }]}>
            <Select options={[{ value: "ukgc" }, { value: "mga" }, { value: "germany" }, { value: "curacao" }]} />
          </Form.Item>
          <Form.Item name="tax_type" label="Tax Type" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="tax_base" label="Tax Base" rules={[{ required: true }]}>
            <Select options={[{ value: "ggr" }, { value: "turnover" }, { value: "poc" }]} />
          </Form.Item>
          <Form.Item name="rate" label="Rate (%)" rules={[{ required: true }]}>
            <Input type="number" step="0.01" />
          </Form.Item>
          <Form.Item name="currency" label="Currency" rules={[{ required: true }]}>
            <Select options={[{ value: "USD" }, { value: "EUR" }, { value: "GBP" }]} />
          </Form.Item>
          <Form.Item name="effective_from" label="Effective From" rules={[{ required: true }]}>
            <Input type="date" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
