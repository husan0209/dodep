import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Card, Typography, Form, Input, Select, DatePicker, Button, message } from "antd";
import { FileAddOutlined } from "@ant-design/icons";
import { regulatoryService } from "@/services/regulatory.service";
import dayjs from "dayjs";

const { Title } = Typography;

export default function ReportGenerator() {
  const [form] = Form.useForm();

  const mutation = useMutation({
    mutationFn: regulatoryService.createReport,
    onSuccess: () => {
      message.success("Report draft created");
      form.resetFields();
    },
    onError: () => message.error("Failed to create report"),
  });

  return (
    <div>
      <Title level={3}>Regulatory Report Generator</Title>
      <Card style={{ maxWidth: 600 }}>
        <Form form={form} layout="vertical" onFinish={(values) => mutation.mutate(values)}>
          <Form.Item name="jurisdiction" label="Jurisdiction" rules={[{ required: true }]}>
            <Select options={[{ value: "ukgc", label: "UKGC" }, { value: "mga", label: "MGA" }, { value: "curacao", label: "Curacao" }, { value: "general", label: "General" }]} />
          </Form.Item>
          <Form.Item name="report_type" label="Report Type" rules={[{ required: true }]}>
            <Select options={[
              { value: "quarterly_return", label: "Quarterly Return" },
              { value: "sar", label: "SAR" },
              { value: "social_responsibility", label: "Social Responsibility" },
              { value: "monthly_financial", label: "Monthly Financial" },
              { value: "player_funds", label: "Player Funds" },
            ]} />
          </Form.Item>
          <Form.Item name="period" label="Period" rules={[{ required: true }]}>
            <DatePicker.RangePicker />
          </Form.Item>
          <Form.Item name="notes" label="Notes">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Button type="primary" htmlType="submit" icon={<FileAddOutlined />} loading={mutation.isPending}>
            Create Draft
          </Button>
        </Form>
      </Card>
    </div>
  );
}
