import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useMutation } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Space,
  Button,
  Input,
  Steps,
  Alert,
  Descriptions,
  Tag,
  Divider,
  Modal,
  Spin,
  message,
} from "antd";
import { ArrowLeftOutlined, SafetyOutlined, MergeCellsOutlined } from "@ant-design/icons";
import { usersService } from "@/services/users.service";
import { formatDate } from "@/utils/format";
import type { MergePreviewResponse } from "@/types/user";

const { Title, Text } = Typography;
const { Step } = Steps;

export default function PlayerMerge() {
  const navigate = useNavigate();
  const [currentStep, setCurrentStep] = useState(0);
  const [primaryId, setPrimaryId] = useState("");
  const [secondaryId, setSecondaryId] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [confirmedBy, setConfirmedBy] = useState("");
  const [reason, setReason] = useState("");

  const { data: preview, isLoading: previewLoading, error: previewError } = useQuery({
    queryKey: ["merge-preview", primaryId, secondaryId],
    queryFn: () => usersService.getMergePreview(primaryId, secondaryId),
    enabled: primaryId.length >= 8 && secondaryId.length >= 8 && currentStep === 1,
  });

  const mergeMutation = useMutation({
    mutationFn: () =>
      usersService.mergePlayers({
        primary_id: primaryId,
        secondary_id: secondaryId,
        reason,
        totp_code: totpCode,
        confirmed_by: confirmedBy,
      }),
    onSuccess: () => {
      message.success("Players merged successfully");
      navigate("/users");
    },
    onError: (error: unknown) => {
      message.error(`Merge failed: ${error instanceof Error ? error.message : "Unknown error"}`);
    },
  });

  const handleNext = () => {
    if (currentStep === 0 && (!primaryId || !secondaryId)) {
      message.error("Please enter both player IDs");
      return;
    }
    if (currentStep === 2 && (!totpCode || !confirmedBy)) {
      message.error("TOTP code and second admin confirmation required");
      return;
    }
    if (currentStep === 3 && !reason) {
      message.error("Please provide a reason for the merge");
      return;
    }
    setCurrentStep(currentStep + 1);
  };

  const handlePrev = () => setCurrentStep(currentStep - 1);

  const handleMerge = () => {
    Modal.confirm({
      title: "CRITICAL: Confirm Player Merge",
      content: (
        <Alert
          message="This action cannot be undone!"
          description="The secondary account will be permanently merged into the primary account and then blocked."
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
        />
      ),
      okText: "Yes, Merge Players",
      okType: "danger",
      cancelText: "Cancel",
      onOk: () => mergeMutation.mutate(),
    });
  };

  const renderStep0 = () => (
    <Space direction="vertical" style={{ width: "100%" }} size="large">
      <Alert
        message="SUPER_ADMIN Only"
        description="This is a critical operation that requires TOTP verification and second admin confirmation."
        type="warning"
        showIcon
      />
      <Card title="Enter Player IDs">
        <Space direction="vertical" style={{ width: "100%" }}>
          <div>
            <Text strong>Primary Account (remains active):</Text>
            <Input
              placeholder="Player ID that will remain active"
              value={primaryId}
              onChange={(e) => setPrimaryId(e.target.value)}
              style={{ marginTop: 8 }}
            />
            <Text type="secondary">This account will receive the balance and history from the secondary account.</Text>
          </div>
          <Divider />
          <div>
            <Text strong>Secondary Account (will be blocked after merge):</Text>
            <Input
              placeholder="Player ID that will be merged and blocked"
              value={secondaryId}
              onChange={(e) => setSecondaryId(e.target.value)}
              style={{ marginTop: 8 }}
            />
            <Text type="secondary">This account will be blocked after the merge operation.</Text>
          </div>
        </Space>
      </Card>
    </Space>
  );

  const renderStep1 = () => {
    if (previewLoading) return <Spin size="large" style={{ display: "block", margin: "40px auto" }} />;
    if (previewError) return <Alert message="Failed to load preview" type="error" />;
    if (!preview) return null;

    return (
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        <Alert
          message="Merge Preview"
          description="Review the merge details before proceeding."
          type="info"
          showIcon
        />

        {preview.conflicts.length > 0 && (
          <Alert
            message="Conflicts Detected"
            description={
              <ul>
                {preview.conflicts.map((conflict, i) => (
                  <li key={i}>{conflict}</li>
                ))}
              </ul>
            }
            type="warning"
            showIcon
          />
        )}

        <Card title="Primary Account (Active)">
          <Descriptions column={2} size="small">
            <Descriptions.Item label="ID">{preview.primary.id}</Descriptions.Item>
            <Descriptions.Item label="Username">{preview.primary.username}</Descriptions.Item>
            <Descriptions.Item label="Email">{preview.primary.email}</Descriptions.Item>
            <Descriptions.Item label="Current Balance">
              {preview.primary.balance} {preview.primary.currency_code}
            </Descriptions.Item>
            <Descriptions.Item label="Registered">{formatDate(preview.primary.created_at)}</Descriptions.Item>
            <Descriptions.Item label="Status">{preview.primary.status}</Descriptions.Item>
          </Descriptions>
        </Card>

        <Card title="Secondary Account (Will be blocked)">
          <Descriptions column={2} size="small">
            <Descriptions.Item label="ID">{preview.secondary.id}</Descriptions.Item>
            <Descriptions.Item label="Username">{preview.secondary.username}</Descriptions.Item>
            <Descriptions.Item label="Email">{preview.secondary.email}</Descriptions.Item>
            <Descriptions.Item label="Balance to transfer">
              {preview.secondary.balance} {preview.secondary.currency_code}
            </Descriptions.Item>
            <Descriptions.Item label="Registered">{formatDate(preview.secondary.created_at)}</Descriptions.Item>
            <Descriptions.Item label="Status">{preview.secondary.status}</Descriptions.Item>
          </Descriptions>
        </Card>

        <Card title="Final State After Merge">
          <Descriptions column={2} size="small">
            <Descriptions.Item label="Primary ID">{preview.final_state.id}</Descriptions.Item>
            <Descriptions.Item label="New Balance">
              <Tag color="green">{preview.final_state.balance} {preview.primary.currency_code}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Tags">
              <Space wrap>
                {preview.final_state.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
              </Space>
            </Descriptions.Item>
          </Descriptions>
        </Card>
      </Space>
    );
  };

  const renderStep2 = () => (
    <Space direction="vertical" style={{ width: "100%" }} size="large">
      <Alert
        message="TOTP Verification Required"
        description="Enter your TOTP code to authorize this operation."
        type="warning"
        showIcon
      />
      <Card title="Authentication">
        <Space direction="vertical" style={{ width: "100%" }}>
          <div>
            <Text strong>Your TOTP Code:</Text>
            <Input
              placeholder="6-digit code"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              maxLength={6}
              style={{ width: 200, marginTop: 8 }}
            />
          </div>
          <Divider />
          <div>
            <Text strong>Second SUPER_ADMIN ID:</Text>
            <Input
              placeholder="Another SUPER_ADMIN must confirm this merge"
              value={confirmedBy}
              onChange={(e) => setConfirmedBy(e.target.value)}
              style={{ marginTop: 8 }}
            />
            <Text type="secondary">Enter the ID of another SUPER_ADMIN who has reviewed and approved this merge.</Text>
          </div>
        </Space>
      </Card>
    </Space>
  );

  const renderStep3 = () => (
    <Space direction="vertical" style={{ width: "100%" }} size="large">
      <Alert
        message="Final Confirmation"
        description="Provide a detailed reason for this merge operation. This will be logged in the audit trail."
        type="error"
        showIcon
      />
      <Card title="Merge Reason">
        <Input.TextArea
          rows={4}
          placeholder="Detailed reason for merging these accounts..."
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
        <Text type="secondary">This reason will be recorded in the audit log.</Text>
      </Card>

      <Card title="Summary">
        <Descriptions column={1}>
          <Descriptions.Item label="Primary Account">{preview?.primary.username} ({primaryId.slice(0, 8)})</Descriptions.Item>
          <Descriptions.Item label="Secondary Account">{preview?.secondary.username} ({secondaryId.slice(0, 8)})</Descriptions.Item>
          <Descriptions.Item label="Balance Transfer">{preview?.secondary.balance} {preview?.primary.currency_code}</Descriptions.Item>
          <Descriptions.Item label="TOTP Verified">{totpCode ? "Yes" : "No"}</Descriptions.Item>
          <Descriptions.Item label="Second Admin">{confirmedBy.slice(0, 8)}...</Descriptions.Item>
        </Descriptions>
      </Card>
    </Space>
  );

  const steps = [
    { title: "Enter IDs", icon: <MergeCellsOutlined /> },
    { title: "Preview", icon: previewLoading ? <Spin size="small" /> : null },
    { title: "TOTP Auth", icon: <SafetyOutlined /> },
    { title: "Confirm", icon: null },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/users")}>
          Back
        </Button>
        <Title level={3} style={{ margin: 0 }}>
          Player Merge
        </Title>
      </Space>

      <Card>
        <Steps current={currentStep} items={steps} style={{ marginBottom: 24 }} />

        {currentStep === 0 && renderStep0()}
        {currentStep === 1 && renderStep1()}
        {currentStep === 2 && renderStep2()}
        {currentStep === 3 && renderStep3()}

        <Divider />

        <Space>
          {currentStep > 0 && <Button onClick={handlePrev}>Previous</Button>}
          {currentStep < 3 && (
            <Button type="primary" onClick={handleNext}>
              Next
            </Button>
          )}
          {currentStep === 3 && (
            <Button
              type="primary"
              danger
              onClick={handleMerge}
              loading={mergeMutation.isPending}
            >
              Execute Merge
            </Button>
          )}
        </Space>
      </Card>
    </div>
  );
}
