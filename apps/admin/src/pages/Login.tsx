import { Form, Input, Button, Card, Typography, message, Space } from "antd";
import { UserOutlined, LockOutlined } from "@ant-design/icons";
import { useNavigate, useLocation } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { z } from "zod";
import { useAuthStore } from "@/stores/authStore";
import { authService } from "@/services/auth.service";
import { getPermissionsForRole } from "@/utils/permissions";
import type { AdminRole, Permission } from "@/types/admin";

const { Title, Text } = Typography;

const loginSchema = z.object({
  email: z
    .string()
    .email("Please enter a valid email")
    .min(1, "Email is required"),
  password: z.string().min(1, "Password is required"),
});

type LoginForm = z.infer<typeof loginSchema>;

const loginResponseSchema = z.object({
  access_token: z.string(),
  refresh_token: z.string(),
  admin: z.object({
    id: z.string(),
    email: z.string(),
    name: z.string(),
    role: z.string(),
    permissions: z.array(z.string()),
  }),
  expires_in: z.number(),
});

export default function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const { setTokens, setAdmin } = useAuthStore();
  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname ||
    "/dashboard";

  const loginMutation = useMutation({
    mutationFn: (data: LoginForm) => authService.login(data),
    onSuccess: (rawData) => {
      const data = loginResponseSchema.parse(rawData);
      setTokens(data.access_token, data.refresh_token);
      const role = data.admin.role as AdminRole;
      const permissions: Permission[] =
        data.admin.permissions.length > 0
          ? (data.admin.permissions as Permission[])
          : getPermissionsForRole(role);
      setAdmin(
        data.admin.id,
        data.admin.email,
        data.admin.name,
        role,
        permissions,
      );
      message.success("Login successful");
      navigate(from, { replace: true });
    },
    onError: (error: Error) => {
      const errorMsg = error.message || "Login failed";
      message.error(errorMsg);
    },
  });

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "linear-gradient(135deg, #001529 0%, #003a70 100%)",
      }}
    >
      <Card style={{ width: 400, boxShadow: "0 4px 20px rgba(0,0,0,0.3)" }}>
        <Space
          direction="vertical"
          size="large"
          style={{ width: "100%", textAlign: "center" }}
        >
          <div>
            <Title level={2} style={{ marginBottom: 4 }}>
              Opus Casino
            </Title>
            <Text type="secondary">Admin Panel</Text>
          </div>

          <Form
            name="login"
            onFinish={(values) => loginMutation.mutate(values)}
            layout="vertical"
            size="large"
          >
            <Form.Item
              name="email"
              rules={[
                { required: true, message: "Please enter your email" },
                { type: "email", message: "Please enter a valid email" },
              ]}
            >
              <Input prefix={<UserOutlined />} placeholder="Email" />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[
                { required: true, message: "Please enter your password" },
              ]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="Password"
              />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={loginMutation.isPending}
                block
              >
                Sign In
              </Button>
            </Form.Item>
          </Form>
        </Space>
      </Card>
    </div>
  );
}
