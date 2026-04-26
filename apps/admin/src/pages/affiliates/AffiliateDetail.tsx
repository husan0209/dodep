import {
  Card,
  Typography,
  Descriptions,
  Tag,
  Space,
  Button,
  InputNumber,
  Input,
  Select,
  Modal,
  Statistic,
  Row,
  Col,
  message,
  Divider,
} from "antd";
import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { affiliatesService } from "@/services/affiliates.service";
import { getErrorMessage } from "@/utils/errors";
import { formatDate } from "@/utils/format";

const { Title, Text } = Typography;

const STATUS_COLORS: Record<string, string> = {
  active: "green",
  pending_review: "orange",
  suspended: "red",
  rejected: "default",
  closed: "default",
};

export default function AffiliateDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [adjustModal, setAdjustModal] = useState(false);
  const [adjustType, setAdjustType] = useState<"credit" | "debit">("credit");
  const [adjustAmount, setAdjustAmount] = useState<number>(0);
  const [adjustReason, setAdjustReason] = useState("");
  const [rateModal, setRateModal] = useState(false);
  const [newRate, setNewRate] = useState<number>(20);

  const { data, isLoading } = useQuery({
    queryKey: ["affiliate", id],
    queryFn: () => affiliatesService.getAffiliate(id!),
    enabled: !!id,
  });

  const suspendMutation = useMutation({
    mutationFn: () => affiliatesService.suspendAffiliate(id!),
    onSuccess: () => {
      message.success("Affiliate suspended");
      queryClient.invalidateQueries({ queryKey: ["affiliate", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const updateRateMutation = useMutation({
    mutationFn: (rate: number) =>
      affiliatesService.updateCommissionRate(id!, rate / 100),
    onSuccess: () => {
      message.success("Commission rate updated");
      queryClient.invalidateQueries({ queryKey: ["affiliate", id] });
      setRateModal(false);
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const adjustMutation = useMutation({
    mutationFn: () =>
      affiliatesService.createAdjustment(id!, {
        adjustment_type: adjustType,
        amount: String(adjustAmount),
        reason: adjustReason,
      }),
    onSuccess: () => {
      message.success("Adjustment created");
      queryClient.invalidateQueries({ queryKey: ["affiliate", id] });
      setAdjustModal(false);
      setAdjustAmount(0);
      setAdjustReason("");
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  if (isLoading || !data) {
    return <Card loading={true} />;
  }

  const profile = data.profile || data;
  const dashboard = data.dashboard;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button onClick={() => navigate("/affiliates")}>← Back</Button>
        <Title level={3} style={{ margin: 0 }}>
          Affiliate: {profile.affiliate_code}
        </Title>
        <Tag color={STATUS_COLORS[profile.status]}>{profile.status}</Tag>
      </Space>

      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Card title="Profile">
            <Descriptions column={3} bordered size="small">
              <Descriptions.Item label="ID">
                {profile.id}
              </Descriptions.Item>
              <Descriptions.Item label="User ID">
                {profile.user_id}
              </Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={STATUS_COLORS[profile.status]}>
                  {profile.status}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Affiliate Code">
                <Text copyable>{profile.affiliate_code}</Text>
              </Descriptions.Item>
              <Descriptions.Item label="Commission Rate">
                {(parseFloat(profile.commission_rate) * 100).toFixed(0)}%
              </Descriptions.Item>
              <Descriptions.Item label="Currency">
                {profile.currency}
              </Descriptions.Item>
              <Descriptions.Item label="Hold Period">
                {profile.hold_period_days} days
              </Descriptions.Item>
              <Descriptions.Item label="Min Payout">
                {profile.min_payout_amount} {profile.currency}
              </Descriptions.Item>
              <Descriptions.Item label="KYC Required">
                {profile.kyc_required ? "Yes" : "No"}
              </Descriptions.Item>
              <Descriptions.Item label="Created">
                {formatDate(profile.created_at)}
              </Descriptions.Item>
              <Descriptions.Item label="Approved By">
                {profile.approved_by || "—"}
              </Descriptions.Item>
              <Descriptions.Item label="Approved At">
                {profile.approved_at ? formatDate(profile.approved_at) : "—"}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>

        {dashboard && (
          <Col span={24}>
            <Card title="Dashboard">
              <Row gutter={16}>
                <Col span={4}>
                  <Statistic title="Clicks" value={dashboard.clicks} />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="Registrations"
                    value={dashboard.registrations}
                  />
                </Col>
                <Col span={4}>
                  <Statistic title="FTD" value={dashboard.ftd_count} />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="GGR"
                    value={dashboard.ggr_amount}
                    prefix="$"
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="NGR"
                    value={dashboard.ngr_amount}
                    prefix="$"
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="Available"
                    value={dashboard.available_amount}
                    prefix="$"
                    valueStyle={{ color: "#3f8600" }}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        )}

        <Col span={24}>
          <Card title="Admin Actions">
            <Space>
              <Button onClick={() => setRateModal(true)}>
                Change Commission Rate
              </Button>
              <Button onClick={() => setAdjustModal(true)}>
                Manual Adjustment
              </Button>
              {profile.status === "active" && (
                <Button
                  danger
                  onClick={() => suspendMutation.mutate()}
                  loading={suspendMutation.isPending}
                >
                  Suspend Affiliate
                </Button>
              )}
            </Space>
          </Card>
        </Col>
      </Row>

      <Modal
        title="Update Commission Rate"
        open={rateModal}
        onOk={() => updateRateMutation.mutate(newRate)}
        onCancel={() => setRateModal(false)}
        confirmLoading={updateRateMutation.isPending}
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Text>New commission rate (%):</Text>
          <InputNumber
            min={0}
            max={100}
            value={newRate}
            onChange={(v) => setNewRate(v || 0)}
            addonAfter="%"
            style={{ width: "100%" }}
          />
        </Space>
      </Modal>

      <Modal
        title="Manual Adjustment"
        open={adjustModal}
        onOk={() => adjustMutation.mutate()}
        onCancel={() => {
          setAdjustModal(false);
          setAdjustAmount(0);
          setAdjustReason("");
        }}
        confirmLoading={adjustMutation.isPending}
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Select
            value={adjustType}
            onChange={setAdjustType}
            style={{ width: "100%" }}
            options={[
              { label: "Credit (add balance)", value: "credit" },
              { label: "Debit (remove balance)", value: "debit" },
            ]}
          />
          <InputNumber
            min={0}
            value={adjustAmount}
            onChange={(v) => setAdjustAmount(v || 0)}
            addonBefore="$"
            style={{ width: "100%" }}
            placeholder="Amount"
          />
          <Divider style={{ margin: "8px 0" }} />
          <Input.TextArea
            rows={2}
            placeholder="Reason for adjustment..."
            value={adjustReason}
            onChange={(e) => setAdjustReason(e.target.value)}
          />
        </Space>
      </Modal>
    </div>
  );
}
