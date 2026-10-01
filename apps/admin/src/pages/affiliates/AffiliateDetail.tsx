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
  Table,
  Form,
} from "antd";
import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  affiliatesService,
  type AffiliatePlayerSummary,
  type PostbackConfig,
  type PostbackEvent,
  type PostbackMethod,
} from "@/services/affiliates.service";
import { getErrorMessage } from "@/utils/errors";
import { formatDate } from "@/utils/format";
import { hasPermission } from "@/utils/permissions";
import { useAuthStore } from "@/stores/authStore";
import type { ColumnsType } from "antd/es/table";

const { Title, Text } = Typography;

const STATUS_COLORS: Record<string, string> = {
  active: "green",
  pending_review: "orange",
  suspended: "red",
  rejected: "default",
  closed: "default",
};

const POSTBACK_EVENTS: PostbackEvent[] = [
  "registration",
  "ftd",
  "deposit",
  "redeposit",
];

const POSTBACK_METHODS: PostbackMethod[] = ["GET", "POST"];

const POSTBACK_BACKOFFS = ["exponential", "fixed", "none"];

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
  const [postbacks, setPostbacks] = useState<PostbackConfig[]>([]);
  const [postbacksSynced, setPostbacksSynced] = useState(false);
  const [postbackModal, setPostbackModal] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [postbackForm] = Form.useForm();
  const [statsWindow, setStatsWindow] = useState<number | undefined>(undefined);
  const [playersPage, setPlayersPage] = useState(1);
  const { permissions } = useAuthStore();
  const canManageAffiliates = hasPermission(permissions, "affiliate.manage");
  const canApproveAffiliates = hasPermission(permissions, "affiliate.approve");
  const canAdjustAffiliates = hasPermission(permissions, "affiliate.adjust");

  const { data, isLoading } = useQuery({
    queryKey: ["affiliate", id],
    queryFn: () => affiliatesService.getAffiliate(id!),
    enabled: !!id,
  });

  const { data: postbackData, isLoading: postbacksLoading } = useQuery({
    queryKey: ["affiliate", id, "postbacks"],
    queryFn: () => affiliatesService.getPostbackConfigs(id!),
    enabled: !!id,
  });

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ["affiliate", id, "stats", statsWindow ?? "all"],
    queryFn: () => affiliatesService.getAffiliateStats(id!, statsWindow),
    enabled: !!id,
  });

  const { data: players, isLoading: playersLoading } = useQuery({
    queryKey: ["affiliate", id, "players", playersPage],
    queryFn: () =>
      affiliatesService.getAffiliatePlayers(id!, {
        page: playersPage,
        page_size: 10,
      }),
    enabled: !!id,
  });

  useEffect(() => {
    if (postbackData && !postbacksSynced) {
      setPostbacks(postbackData);
      setPostbacksSynced(true);
    }
  }, [postbackData, postbacksSynced]);

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

  const savePostbacksMutation = useMutation({
    mutationFn: (configs: PostbackConfig[]) =>
      affiliatesService.updatePostbackConfigs(id!, configs),
    onSuccess: () => {
      message.success("Postback configurations saved");
      queryClient.invalidateQueries({
        queryKey: ["affiliate", id, "postbacks"],
      });
      setPostbacksSynced(false);
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const openPostbackModal = (index: number | null) => {
    setEditingIndex(index);
    const cfg = index !== null ? postbacks[index] : undefined;
    postbackForm.setFieldsValue({
      event: cfg?.event ?? "registration",
      method: cfg?.method ?? "GET",
      url: cfg?.url ?? "",
      retry_count: cfg?.retry_count ?? 3,
      retry_backoff: cfg?.retry_backoff ?? "exponential",
      variables: Object.entries(cfg?.variables ?? {}).map(([key, value]) => ({
        key,
        value,
      })),
    });
    setPostbackModal(true);
  };

  const submitPostbackModal = async () => {
    const values = await postbackForm.validateFields();
    const url = String(values.url || "").trim();
    try {
      const parsed = new URL(url);
      if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
        throw new Error("unsupported protocol");
      }
    } catch {
      message.error("URL must be a valid http(s) address");
      return;
    }
    const variables: Record<string, string> = {};
    for (const row of values.variables ?? []) {
      if (row?.key) {
        variables[String(row.key)] = String(row?.value ?? "");
      }
    }
    const cfg: PostbackConfig = {
      event: values.event as PostbackEvent,
      method: values.method as PostbackMethod,
      url,
      variables,
      retry_count: values.retry_count ?? 0,
      retry_backoff: values.retry_backoff ?? "",
    };
    setPostbacks((prev) => {
      const next = [...prev];
      if (editingIndex !== null) {
        next[editingIndex] = cfg;
      } else {
        next.push(cfg);
      }
      return next;
    });
    setPostbackModal(false);
    postbackForm.resetFields();
  };

  const deletePostback = (index: number) => {
    setPostbacks((prev) => prev.filter((_, i) => i !== index));
  };

  const postbacksDirty =
    JSON.stringify(postbacks) !== JSON.stringify(postbackData ?? []);

  const playerColumns: ColumnsType<AffiliatePlayerSummary> = [
    {
      title: "Player",
      dataIndex: "player_ref",
      width: 180,
      // Pseudonymous id — the admin UI never sees raw player identifiers.
      render: (v: string) => <Text code>{v}</Text>,
    },
    {
      title: "Attributed",
      dataIndex: "attributed_at",
      width: 150,
      render: (v: string) => formatDate(v),
    },
    {
      title: "FTD",
      dataIndex: "ftd_qualified",
      width: 100,
      render: (v: boolean, r) =>
        v ? (
          <Tag color="green">FTD {r.ftd_at ? formatDate(r.ftd_at) : ""}</Tag>
        ) : (
          <Tag>no</Tag>
        ),
    },
    {
      title: "NGR",
      dataIndex: "ngr_amount",
      align: "right",
      render: (v: string) => `${v} ${profile.currency}`,
    },
    {
      title: "Commission",
      dataIndex: "commission_amount",
      align: "right",
      render: (v: string) => `${v} ${profile.currency}`,
    },
  ];

  const postbackColumns: ColumnsType<PostbackConfig & { key: number }> = [
    {
      title: "Event",
      dataIndex: "event",
      width: 130,
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: "Method",
      dataIndex: "method",
      width: 90,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: "URL",
      dataIndex: "url",
      render: (v: string) => (
        <Text copyable={{ text: v }} ellipsis style={{ maxWidth: 320 }}>
          {v}
        </Text>
      ),
    },
    {
      title: "Variables",
      dataIndex: "variables",
      width: 120,
      render: (v: Record<string, string> | undefined) =>
        v ? Object.keys(v).length : 0,
    },
    {
      title: "Retries",
      dataIndex: "retry_count",
      width: 90,
      render: (v: number, record) =>
        `${v ?? 0} (${record.retry_backoff || "—"})`,
    },
    {
      title: "Actions",
      key: "actions",
      width: 160,
      render: (_, record) => {
        if (!canManageAffiliates) return "—";
        return (
          <Space>
            <Button size="small" onClick={() => openPostbackModal(record.key)}>
              Edit
            </Button>
            <Button size="small" danger onClick={() => deletePostback(record.key)}>
              Delete
            </Button>
          </Space>
        );
      },
    },
  ];

  if (isLoading || !data) {
    return <Card loading={true} />;
  }

  const profile = data.profile || data;

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

        <Col span={24}>
          <Card
            title="Performance"
            loading={statsLoading}
            extra={
              <Select
                size="small"
                style={{ width: 160 }}
                value={statsWindow ?? "all"}
                onChange={(v) =>
                  setStatsWindow(v === "all" ? undefined : Number(v))
                }
                options={[
                  { label: "All time", value: "all" },
                  { label: "Last 7 days", value: 7 },
                  { label: "Last 30 days", value: 30 },
                  { label: "Last 90 days", value: 90 },
                ]}
              />
            }
          >
            {stats && (
              <>
                <Row gutter={16}>
                  <Col span={4}>
                    <Statistic title="Clicks" value={stats.clicks} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="Registrations" value={stats.registrations} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="FTD" value={stats.ftd_count} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="Referred players" value={stats.players} />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="GGR"
                      value={stats.ggr}
                      prefix={`${profile.currency} `}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="NGR"
                      value={stats.ngr}
                      prefix={`${profile.currency} `}
                      valueStyle={{ color: "#3f8600" }}
                    />
                  </Col>
                </Row>
                <Divider style={{ margin: "12px 0" }} />
                <Row gutter={16}>
                  <Col span={4}>
                    <Statistic
                      title="Commission accrued"
                      value={stats.commission_accrued}
                      prefix={`${profile.currency} `}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="Pending (hold)"
                      value={stats.commission_pending}
                      prefix={`${profile.currency} `}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="Available / owed"
                      value={stats.owed}
                      prefix={`${profile.currency} `}
                      valueStyle={{ color: "#3f8600" }}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="Paid"
                      value={stats.commission_paid}
                      prefix={`${profile.currency} `}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="Reversed"
                      value={stats.commission_reversed}
                      prefix={`${profile.currency} `}
                      valueStyle={{ color: "#cf1322" }}
                    />
                  </Col>
                  <Col span={4}>
                    <Statistic
                      title="Open fraud flags"
                      value={stats.open_fraud_flags}
                      valueStyle={
                        stats.open_fraud_flags > 0
                          ? { color: "#cf1322" }
                          : undefined
                      }
                    />
                  </Col>
                </Row>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  Conversion: click→reg{" "}
                  {(stats.click_to_reg_rate * 100).toFixed(2)}% · reg→FTD{" "}
                  {(stats.reg_to_ftd_rate * 100).toFixed(2)}%. Commission is
                  accrued from finalized NGR only (never from RTP).
                </Text>
              </>
            )}
          </Card>
        </Col>

        <Col span={24}>
          <Card title="Referred Players (anonymised)" loading={playersLoading}>
            <Table
              dataSource={players?.data || []}
              columns={playerColumns}
              size="small"
              rowKey="player_ref"
              pagination={{
                current: playersPage,
                pageSize: 10,
                total: players?.pagination?.total || 0,
                showSizeChanger: false,
                onChange: (p) => setPlayersPage(p),
              }}
              locale={{
                emptyText:
                  "No referred players yet — attribution appears after the first referral click converts to a registration.",
              }}
            />
          </Card>
        </Col>

        <Col span={24}>
          <Card title="Admin Actions">
            <Space>
              {canManageAffiliates && (
                <Button onClick={() => setRateModal(true)}>
                  Change Commission Rate
                </Button>
              )}
              {canAdjustAffiliates && (
                <Button onClick={() => setAdjustModal(true)}>
                  Manual Adjustment
                </Button>
              )}
              {canApproveAffiliates && profile.status === "active" && (
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

        <Col span={24}>
          <Card
            title="Postback Configurations"
            extra={
              <Space>
                {canManageAffiliates && (
                  <Button size="small" onClick={() => openPostbackModal(null)}>
                    Add Postback
                  </Button>
                )}
                <Button
                  size="small"
                  type="primary"
                  disabled={!postbacksDirty || !canManageAffiliates}
                  loading={savePostbacksMutation.isPending}
                  onClick={() => savePostbacksMutation.mutate(postbacks)}
                >
                  Save Changes
                </Button>
              </Space>
            }
          >
            <Table
              dataSource={postbacks.map((cfg, index) => ({ ...cfg, key: index }))}
              columns={postbackColumns}
              loading={postbacksLoading}
              pagination={false}
              size="small"
              locale={{
                emptyText:
                  "No postbacks configured. Partner callbacks for registration, FTD and deposits are disabled.",
              }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              Variables support {"{click_id}"}, {"{player_id}"}, {"{amount}"} —
              substituted by the backend on each event. Failed deliveries retry
              with exponential backoff (1m, 5m, 30m) and are audit-logged.
            </Text>
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

      <Modal
        title={editingIndex !== null ? "Edit Postback" : "Add Postback"}
        open={postbackModal}
        onOk={submitPostbackModal}
        onCancel={() => {
          setPostbackModal(false);
          postbackForm.resetFields();
        }}
        width={560}
      >
        <Form form={postbackForm} layout="vertical" preserve={false}>
          <Form.Item
            name="event"
            label="Event"
            rules={[{ required: true, message: "Event is required" }]}
          >
            <Select
              options={POSTBACK_EVENTS.map((e) => ({ label: e, value: e }))}
            />
          </Form.Item>
          <Form.Item
            name="method"
            label="HTTP Method"
            rules={[{ required: true, message: "Method is required" }]}
          >
            <Select
              options={POSTBACK_METHODS.map((m) => ({ label: m, value: m }))}
            />
          </Form.Item>
          <Form.Item
            name="url"
            label="Postback URL"
            rules={[{ required: true, message: "URL is required" }]}
          >
            <Input placeholder="https://partner.example/postback?click_id={click_id}" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="retry_count" label="Retry Count">
                <InputNumber min={0} max={10} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="retry_backoff" label="Retry Backoff">
                <Select
                  options={POSTBACK_BACKOFFS.map((b) => ({
                    label: b,
                    value: b,
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Divider style={{ margin: "8px 0 12px" }} />
          <Text strong style={{ fontSize: 12 }}>
            URL Variables
          </Text>
          <Form.List name="variables">
            {(fields, { add, remove }) => (
              <div style={{ marginTop: 8 }}>
                {fields.map(({ key, name, ...restField }) => (
                  <Space
                    key={key}
                    style={{ display: "flex", marginBottom: 8 }}
                    align="baseline"
                  >
                    <Form.Item
                      {...restField}
                      name={[name, "key"]}
                      rules={[{ required: true, message: "Key required" }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="{click_id}" />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, "value"]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="value or {player_id}" />
                    </Form.Item>
                    <Button danger size="small" onClick={() => remove(name)}>
                      ✕
                    </Button>
                  </Space>
                ))}
                <Button size="small" onClick={() => add()}>
                  + Add variable
                </Button>
              </div>
            )}
          </Form.List>
        </Form>
      </Modal>
    </div>
  );
}
