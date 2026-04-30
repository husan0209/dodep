import { useState } from "react";
import { Form, Input, Button, Card, Typography, App as AntApp, Space, Steps } from "antd";
import { UserOutlined, LockOutlined, SafetyOutlined } from "@ant-design/icons";
import { useNavigate, useLocation } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { z } from "zod";
import { useAuthStore } from "@/stores/authStore";
import { authService } from "@/services/auth.service";
import { getPermissionsForRole } from "@/utils/permissions";
import type { AdminRole, Permission } from "@/types/admin";

const { Title, Text } = Typography;

type LoginStep = "credentials" | "totp";

interface CredentialsForm {
  email: string;
  password: string;
}

interface TOTPForm {
  totp_code: string;
}

const credentialsSchema = z.object({
  email: z.string().email("Please enter a valid email").min(1, "Email is required"),
  password: z.string().min(1, "Password is required"),
});

const totpSchema = z.object({
  totp_code: z.string().length(6, "TOTP code must be 6 digits").regex(/^\d+$/, "Only digits allowed"),
});

const loginResponseSchema = z.object({
  access_token: z.string(),
  refresh_token: z.string().optional(),
  admin: z.object({
    id: z.union([z.string(), z.number()]).transform((v) => String(v)),
    email: z.string(),
    name: z.string().optional(),
    first_name: z.string().optional(),
    last_name: z.string().optional(),
    role: z.string(),
    permissions: z.array(z.string()),
  }),
  expires_in: z.number().optional(),
  totp_required: z.boolean().optional(),
});

export default function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const { setTokens, setAdmin } = useAuthStore();
  const { message } = AntApp.useApp();
  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname ||
    "/dashboard";

  const [step, setStep] = useState<LoginStep>("credentials");
  const [credentials, setCredentials] = useState<CredentialsForm | null>(null);
  const [totpError, setTOTPError] = useState<string | null>(null);

  const loginMutation = useMutation({
    mutationFn: (data: { email: string; password: string; totp_code?: string }) =>
      authService.login(data),
    onSuccess: (rawData) => {
      const data = loginResponseSchema.parse(rawData);

      if (data.totp_required) {
        setStep("totp");
        return;
      }

      setTokens(data.access_token, data.refresh_token);
      const role = data.admin.role as AdminRole;
      const permissions: Permission[] =
        data.admin.permissions.length > 0
          ? (data.admin.permissions as Permission[])
          : getPermissionsForRole(role);
      const displayName =
        data.admin.name ||
        [data.admin.first_name, data.admin.last_name].filter(Boolean).join(" ") ||
        data.admin.email;
      setAdmin(
        data.admin.id,
        data.admin.email,
        displayName,
        role,
        permissions,
      );
      message.success("Login successful", 3);
      navigate(from, { replace: true });
    },
    onError: (error: Error) => {
      const errorMsg = error.message || "Login failed";
      if (step === "totp") {
        setTOTPError(errorMsg);
      } else {
        message.error(errorMsg, 3);
      }
    },
  });

  const handleCredentialsSubmit = (values: CredentialsForm) => {
    setCredentials(values);
    loginMutation.mutate({ email: values.email, password: values.password });
  };

  const handleTOTPSubmit = (values: TOTPForm) => {
    if (!credentials) return;
    setTOTPError(null);
    loginMutation.mutate({
      email: credentials.email,
      password: credentials.password,
      totp_code: values.totp_code,
    });
  };

  const handleBack = () => {
    setStep("credentials");
    setCredentials(null);
    setTOTPError(null);
  };

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
      <Card style={{ width: 420, boxShadow: "0 4px 20px rgba(0,0,0,0.3)" }}>
        <Space
          direction="vertical"
          size="large"
          style={{ width: "100%", textAlign: "center" }}
        >
          <div>
            <Title level={2} style={{ marginBottom: 4 }}>
              DOD
            </Title>
            <Text type="secondary">Admin Panel</Text>
          </div>

          <Steps
            size="small"
            current={step === "credentials" ? 0 : 1}
            items={[
              { title: "Credentials" },
              { title: "2FA" },
            ]}
          />

          {step === "credentials" ? (
            <Form
              name="login-credentials"
              onFinish={handleCredentialsSubmit}
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
                <Input prefix={<UserOutlined />} placeholder="Email" autoComplete="email" />
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
                  autoComplete="current-password"
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
          ) : (
            <Form
              name="login-totp"
              onFinish={handleTOTPSubmit}
              layout="vertical"
              size="large"
            >
              <Text type="secondary" style={{ display: "block", marginBottom: 16 }}>
                Two-factor authentication required. Enter the 6-digit code from your authenticator app.
              </Text>

              <Form.Item
                name="totp_code"
                validateStatus={totpError ? "error" : undefined}
                help={totpError}
                rules={[
                  { required: true, message: "Please enter TOTP code" },
                  { len: 6, message: "Code must be 6 digits" },
                  { pattern: /^\d+$/, message: "Only digits allowed" },
                ]}
              >
                <Input.OTP
                  length={6}
                  autoFocus
                />
              </Form.Item>

              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={loginMutation.isPending}
                  block
                >
                  Verify & Sign In
                </Button>
              </Form.Item>

              <Button type="link" onClick={handleBack} block>
                Back to credentials
              </Button>
            </Form>
          )}
        </Space>
      </Card>
    </div>
  );
}
