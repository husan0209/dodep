import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Button,
  Form,
  Input,
  Switch,
  Select,
  Slider,
  InputNumber,
  message,
  Space,
  Tabs,
} from "antd";
import { SaveOutlined, ReloadOutlined } from "@ant-design/icons";
import { systemService } from "@/services/system.service";

const { Title } = Typography;
const { TabPane } = Tabs;

export default function GeneralSettings() {
  const queryClient = useQueryClient();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["general-settings"],
    queryFn: () => systemService.getSettings() as Promise<Record<string, unknown>>,
  });

  const saveMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => systemService.updateSettings(values),
    onSuccess: () => {
      message.success("Settings saved");
      queryClient.invalidateQueries({ queryKey: ["general-settings"] });
    },
    onError: () => message.error("Failed to save settings"),
  });

  const onFinish = (values: Record<string, unknown>) => {
    saveMutation.mutate(values);
  };

  return (
    <div>
      <Title level={3}>General Settings</Title>
      <Form
        form={form}
        layout="vertical"
        initialValues={data || {}}
        onFinish={onFinish}
      >
        <Tabs defaultActiveKey="platform">
          <TabPane tab="Platform" key="platform">
            <Card>
              <Form.Item name="platform_name" label="Platform Name" rules={[{ required: true }]}>
                <Input placeholder="Opus Casino" />
              </Form.Item>
              <Form.Item name="default_locale" label="Default Locale" initialValue="en">
                <Select options={[{ value: "en" }, { value: "ru" }, { value: "de" }, { value: "fr" }]} />
              </Form.Item>
              <Form.Item name="maintenance_mode" label="Maintenance Mode" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name="registration_enabled" label="Registration Enabled" valuePropName="checked" initialValue={true}>
                <Switch />
              </Form.Item>
            </Card>
          </TabPane>
          <TabPane tab="Limits" key="limits">
            <Card>
              <Form.Item name="max_daily_withdrawal" label="Max Daily Withdrawal" initialValue={10000}>
                <InputNumber prefix="$" style={{ width: 200 }} min={0} />
              </Form.Item>
              <Form.Item name="max_single_bet" label="Max Single Bet" initialValue={5000}>
                <InputNumber prefix="$" style={{ width: 200 }} min={0} />
              </Form.Item>
              <Form.Item name="min_deposit" label="Min Deposit" initialValue={10}>
                <InputNumber prefix="$" style={{ width: 200 }} min={0} />
              </Form.Item>
              <Form.Item name="session_timeout_minutes" label="Session Timeout (min)" initialValue={30}>
                <Slider min={5} max={120} marks={{ 5: "5m", 30: "30m", 60: "1h", 120: "2h" }} />
              </Form.Item>
            </Card>
          </TabPane>
          <TabPane tab="KYC" key="kyc">
            <Card>
              <Form.Item name="kyc_threshold_deposit" label="KYC Trigger Deposit" initialValue={2000}>
                <InputNumber prefix="$" style={{ width: 200 }} min={0} />
              </Form.Item>
              <Form.Item name="kyc_threshold_withdrawal" label="KYC Trigger Withdrawal" initialValue={1000}>
                <InputNumber prefix="$" style={{ width: 200 }} min={0} />
              </Form.Item>
              <Form.Item name="kyc_document_expiry_days" label="Doc Expiry (days)" initialValue={90}>
                <InputNumber style={{ width: 120 }} min={1} />
              </Form.Item>
            </Card>
          </TabPane>
        </Tabs>
        <Space style={{ marginTop: 16 }}>
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saveMutation.isPending}>
            Save Settings
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => queryClient.invalidateQueries({ queryKey: ["general-settings"] })}>
            Reset
          </Button>
        </Space>
      </Form>
    </div>
  );
}
