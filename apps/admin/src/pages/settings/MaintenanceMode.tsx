import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Button,
  Switch,
  Alert,
  Form,
  Input,
  DatePicker,
  message,
  Space,
  Descriptions,
  Tag,
} from "antd";
import { ReloadOutlined, SaveOutlined, PoweroffOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { systemService } from "@/services/system.service";

const { Title } = Typography;

interface MaintenanceStatus {
  enabled: boolean;
  message: string;
  scheduled_start?: string;
  scheduled_end?: string;
  last_changed_by?: string;
  last_changed_at?: string;
}

export default function MaintenanceMode() {
  const queryClient = useQueryClient();
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["maintenance-status"],
    queryFn: () => systemService.getMaintenanceStatus() as unknown as Promise<MaintenanceStatus>,
  });

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => systemService.setMaintenanceMode(enabled),
    onSuccess: () => {
      message.success(`Maintenance mode ${data?.enabled ? "disabled" : "enabled"}`);
      queryClient.invalidateQueries({ queryKey: ["maintenance-status"] });
    },
    onError: () => message.error("Failed to toggle maintenance mode"),
  });

  const scheduleMutation = useMutation({
    mutationFn: (values: { message: string; start?: dayjs.Dayjs; end?: dayjs.Dayjs }) =>
      systemService.scheduleMaintenance({
        message: values.message,
        start: values.start?.toISOString(),
        end: values.end?.toISOString(),
      }),
    onSuccess: () => {
      message.success("Maintenance scheduled");
      queryClient.invalidateQueries({ queryKey: ["maintenance-status"] });
      form.resetFields();
    },
    onError: () => message.error("Failed to schedule maintenance"),
  });

  return (
    <div>
      <Title level={3}>Maintenance Mode</Title>
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        {data?.enabled && (
          <Alert
            message="Maintenance mode is ACTIVE"
            description={data?.message || "Platform is under maintenance. Users cannot access the site."}
            type="warning"
            showIcon
            closable={false}
          />
        )}
        <Card title="Current Status" loading={isLoading}>
          <Descriptions bordered column={1}>
            <Descriptions.Item label="Enabled">
              <Tag color={data?.enabled ? "red" : "green"}>{data?.enabled ? "ACTIVE" : "INACTIVE"}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Message">{data?.message || "—"}</Descriptions.Item>
            <Descriptions.Item label="Last Changed">
              {data?.last_changed_at ? `${new Date(data.last_changed_at).toLocaleString()} by ${data.last_changed_by || "unknown"}` : "—"}
            </Descriptions.Item>
          </Descriptions>
          <Button
            type={data?.enabled ? "primary" : "default"}
            danger={!data?.enabled}
            icon={<PoweroffOutlined />}
            loading={toggleMutation.isPending}
            onClick={() => toggleMutation.mutate(!data?.enabled)}
            style={{ marginTop: 16 }}
          >
            {data?.enabled ? "Disable Maintenance Mode" : "Enable Maintenance Mode"}
          </Button>
        </Card>

        <Card title="Schedule Maintenance">
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) => scheduleMutation.mutate(values)}
          >
            <Form.Item name="message" label="Maintenance Message" rules={[{ required: true }]}>
              <Input.TextArea rows={3} placeholder="We are performing scheduled maintenance..." />
            </Form.Item>
            <Form.Item name="start" label="Start Time">
              <DatePicker showTime style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item name="end" label="End Time">
              <DatePicker showTime style={{ width: "100%" }} />
            </Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={scheduleMutation.isPending}>
              Schedule
            </Button>
          </Form>
        </Card>
      </Space>
    </div>
  );
}
