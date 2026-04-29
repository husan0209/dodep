import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Card, Form, Input, Select, Button, message, Space, Typography } from "antd";
import { ArrowLeftOutlined, SaveOutlined } from "@ant-design/icons";
import { supportService } from "@/services/support.service";
import type { TicketCategory, TicketPriority } from "@/types/support";

const { Title } = Typography;

export default function NewTicket() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const createMutation = useMutation({
    mutationFn: (values: {
      player_id: string;
      subject: string;
      category: TicketCategory;
      priority: TicketPriority;
      body: string;
    }) => supportService.createTicket({ ...values, created_via: "manual" }),
    onSuccess: (ticket) => {
      message.success("Ticket created");
      navigate(`/support/tickets/${ticket.id}`);
    },
    onError: () => message.error("Failed to create ticket"),
  });

  const onFinish = (values: {
    player_id: string;
    subject: string;
    category: TicketCategory;
    priority: TicketPriority;
    body: string;
  }) => {
    createMutation.mutate(values);
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/support/tickets")}>
          Back to Tickets
        </Button>
      </Space>
      <Title level={3}>Create Support Ticket</Title>
      <Card style={{ maxWidth: 720 }}>
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item
            name="player_id"
            label="Player ID"
            rules={[{ required: true, message: "Enter player ID" }]}
          >
            <Input placeholder="Player ID or email" />
          </Form.Item>
          <Form.Item
            name="subject"
            label="Subject"
            rules={[{ required: true, message: "Enter subject" }]}
          >
            <Input placeholder="Brief subject" />
          </Form.Item>
          <Form.Item
            name="category"
            label="Category"
            rules={[{ required: true }]}
            initialValue="general"
          >
            <Select
              options={[
                { value: "payment", label: "Payment" },
                { value: "bonus", label: "Bonus" },
                { value: "technical", label: "Technical" },
                { value: "account", label: "Account" },
                { value: "kyc", label: "KYC" },
                { value: "general", label: "General" },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="priority"
            label="Priority"
            initialValue="normal"
          >
            <Select
              options={[
                { value: "low", label: "Low" },
                { value: "normal", label: "Normal" },
                { value: "high", label: "High" },
                { value: "urgent", label: "Urgent" },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="body"
            label="Message"
            rules={[{ required: true, message: "Enter message" }]}
          >
            <Input.TextArea rows={6} placeholder="Describe the issue..." />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              icon={<SaveOutlined />}
              loading={createMutation.isPending}
            >
              Create Ticket
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
