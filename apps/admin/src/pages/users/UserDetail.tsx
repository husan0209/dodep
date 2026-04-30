import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Descriptions,
  Typography,
  Space,
  Button,
  Tabs,
  Table,
  Tag,
  Modal,
  Input,
  Select,
  message,
  Spin,
  Row,
  Col,
  Badge,
  Tooltip,
  Collapse,
  List,
  Empty,
} from "antd";
import {
  ArrowLeftOutlined,
  StopOutlined,
  CheckCircleOutlined,
  EditOutlined,
  GiftOutlined,
  DollarOutlined,
  MessageOutlined,
  TagOutlined,
  TeamOutlined,
  LockOutlined,
} from "@ant-design/icons";
import { useState } from "react";
import { usersService } from "@/services/users.service";
import { financeService } from "@/services/finance.service";
import { sportsService } from "@/services/sports.service";
import { casinoService } from "@/services/casino.service";
import { bonusesService } from "@/services/bonuses.service";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { formatDate } from "@/utils/format";
import { USER_STATUSES, KYC_LEVELS, TRANSACTION_STATUSES } from "@/utils/constants";
import { getErrorMessage } from "@/utils/errors";
import type { ColumnsType } from "antd/es/table";
import type {
  UserProfile,
  PlayerGroup,
  BlockUserPayload,
  KycDocument,
  LinkedAccount,
} from "@/types/user";
import type { CasinoBetSession } from "@/types/casino";
import type { Deposit, Withdrawal } from "@/types/finance";
import type { Bet } from "@/types/bet";
import type { UserBonus } from "@/types/bonus";

const { Title, Text } = Typography;
const { Panel } = Collapse;

const PLAYER_GROUPS: Record<PlayerGroup, { label: string; color: string }> = {
  standard: { label: "Standard", color: "default" },
  vip: { label: "VIP", color: "gold" },
  vvip: { label: "VVIP", color: "purple" },
  whale: { label: "Whale", color: "red" },
};

const AVAILABLE_TAGS = [
  "VIP",
  "High Risk",
  "Bonus Hunter",
  "Multi Account",
  "Self Excluded",
  "PEP",
  "Sanctions",
  "Verified",
];

export default function UserDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState("overview");
  const [showBlockModal, setShowBlockModal] = useState(false);
  const [blockForm, setBlockForm] = useState<{
    type: BlockUserPayload["type"];
    duration_hours?: number;
    reason: string;
  }>({ type: "full", reason: "" });

  // User data
  const { data: user, isLoading } = useQuery({
    queryKey: ["user", id],
    queryFn: () => usersService.get(id!),
    enabled: !!id,
  });

  // Lazy loaded tabs data
  const { data: deposits, isLoading: depositsLoading } = useQuery({
    queryKey: ["user-deposits", id],
    queryFn: () => financeService.getDeposits({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "deposits",
  });

  const { data: withdrawals, isLoading: withdrawalsLoading } = useQuery({
    queryKey: ["user-withdrawals", id],
    queryFn: () => financeService.getWithdrawals({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "withdrawals",
  });

  const { data: transactions, isLoading: transactionsLoading } = useQuery({
    queryKey: ["user-transactions", id],
    queryFn: () => financeService.getTransactions({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "transactions",
  });

  const { data: casinoBets, isLoading: casinoBetsLoading } = useQuery({
    queryKey: ["user-casino-bets", id],
    queryFn: () => casinoService.getCasinoBets({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "casino_bets",
  });

  const { data: sportsBets, isLoading: sportsBetsLoading } = useQuery({
    queryKey: ["user-sports-bets", id],
    queryFn: () => sportsService.getBets({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "sports_bets",
  });

  const { data: bonuses, isLoading: bonusesLoading } = useQuery({
    queryKey: ["user-bonuses", id],
    queryFn: () => bonusesService.getUserBonuses({ user_id: id, page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "bonuses",
  });

  const { data: kycDocs, isLoading: kycLoading } = useQuery({
    queryKey: ["user-kyc", id],
    queryFn: () => usersService.getKycDocuments(id!),
    enabled: !!id && activeTab === "kyc",
  });

  const { data: limits, isLoading: limitsLoading } = useQuery({
    queryKey: ["user-limits", id],
    queryFn: () => usersService.getLimits(id!),
    enabled: !!id && activeTab === "rg",
  });

  const { data: supportChats, isLoading: supportLoading } = useQuery({
    queryKey: ["user-support", id],
    queryFn: () => usersService.getSupportChats(id!, { page: 1, page_size: 20 }),
    enabled: !!id && activeTab === "support",
  });

  const { data: linkedAccounts, isLoading: linkedLoading } = useQuery({
    queryKey: ["user-linked", id],
    queryFn: () => usersService.getLinkedAccounts(id!),
    enabled: !!id && activeTab === "linked",
  });

  // Quick Action state & mutations
  const [showBalanceModal, setShowBalanceModal] = useState(false);
  const [balanceForm, setBalanceForm] = useState({ amount: "", type: "credit" as "credit" | "debit", reason: "" });

  const [showBonusModal, setShowBonusModal] = useState(false);
  const [bonusForm, setBonusForm] = useState({ bonus_id: "", reason: "" });

  const [showLimitsModal, setShowLimitsModal] = useState(false);
  const [limitsForm, setLimitsForm] = useState({
    max_deposit_daily: "",
    max_deposit_weekly: "",
    max_withdrawal_daily: "",
    max_bet: "",
    max_loss_daily: "",
    reason: "",
  });

  const [showMessageModal, setShowMessageModal] = useState(false);
  const [messageForm, setMessageForm] = useState({ channel: "email" as "email" | "sms" | "push", subject: "", body: "" });

  const { data: notes, isLoading: notesLoading } = useQuery({
    queryKey: ["user-notes", id],
    queryFn: () => usersService.getNotes(id!),
    enabled: !!id && activeTab === "notes",
  });

  const { data: sessions, isLoading: sessionsLoading } = useQuery({
    queryKey: ["user-sessions", id],
    queryFn: () => usersService.getSessions(id!),
    enabled: !!id && activeTab === "sessions",
  });

  // Mutations
  const blockMutation = useMutation({
    mutationFn: () =>
      usersService.block(id!, {
        type: blockForm.type,
        duration_hours: blockForm.duration_hours,
        reason: blockForm.reason,
      }),
    onSuccess: () => {
      message.success("User blocked");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
      setShowBlockModal(false);
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const unblockMutation = useMutation({
    mutationFn: () => usersService.unblock(id!, "Manual unblock by admin"),
    onSuccess: () => {
      message.success("User unblocked");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const updateTagsMutation = useMutation({
    mutationFn: (payload: { add?: string[]; remove?: string[] }) =>
      usersService.updateTags(id!, { ...payload, reason: "Admin update" }),
    onSuccess: () => {
      message.success("Tags updated");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const updateGroupMutation = useMutation({
    mutationFn: (group: PlayerGroup) =>
      usersService.updateGroup(id!, { group, reason: "Admin update" }),
    onSuccess: () => {
      message.success("Group updated");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const adjustBalanceMutation = useMutation({
    mutationFn: () => usersService.adjustBalance(id!, {
      amount: balanceForm.amount,
      currency: user?.currency_code || "USD",
      type: balanceForm.type,
      reason: balanceForm.reason,
    }),
    onSuccess: () => {
      message.success("Balance adjusted");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
      setShowBalanceModal(false);
      setBalanceForm({ amount: "", type: "credit", reason: "" });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const giveBonusMutation = useMutation({
    mutationFn: () => usersService.giveBonus(id!, {
      bonus_id: bonusForm.bonus_id,
      reason: bonusForm.reason,
    }),
    onSuccess: () => {
      message.success("Bonus granted");
      queryClient.invalidateQueries({ queryKey: ["user", id] });
      queryClient.invalidateQueries({ queryKey: ["user-bonuses", id] });
      setShowBonusModal(false);
      setBonusForm({ bonus_id: "", reason: "" });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const adjustLimitsMutation = useMutation({
    mutationFn: () => usersService.updateLimits(id!, {
      max_deposit_daily: limitsForm.max_deposit_daily || undefined,
      max_deposit_weekly: limitsForm.max_deposit_weekly || undefined,
      max_withdrawal_daily: limitsForm.max_withdrawal_daily || undefined,
      max_bet: limitsForm.max_bet || undefined,
      max_loss_daily: limitsForm.max_loss_daily || undefined,
      reason: limitsForm.reason,
    }),
    onSuccess: () => {
      message.success("Limits updated");
      queryClient.invalidateQueries({ queryKey: ["user-limits", id] });
      setShowLimitsModal(false);
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const sendMessageMutation = useMutation({
    mutationFn: () => usersService.sendMessage(id!, {
      channel: messageForm.channel,
      subject: messageForm.channel === "email" ? messageForm.subject : undefined,
      body: messageForm.body,
    }),
    onSuccess: () => {
      message.success("Message sent");
      setShowMessageModal(false);
      setMessageForm({ channel: "email", subject: "", body: "" });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  const addNoteMutation = useMutation({
    mutationFn: (text: string) => usersService.addNote(id!, { text }),
    onSuccess: () => {
      message.success("Note added");
      queryClient.invalidateQueries({ queryKey: ["user-notes", id] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  // Quick Actions Handlers
  const handleBlock = () => setShowBlockModal(true);
  const handleUnblock = () => {
    Modal.confirm({
      title: "Unblock User",
      content: "Are you sure you want to unblock this user?",
      onOk: () => unblockMutation.mutate(),
    });
  };

  const handleAdjustBalance = () => setShowBalanceModal(true);
  const handleGiveBonus = () => setShowBonusModal(true);
  const handleAdjustLimits = () => setShowLimitsModal(true);
  const handleSendMessage = () => setShowMessageModal(true);

  const handleAddNote = () => {
    let noteText = "";
    Modal.confirm({
      title: "Add Admin Note",
      content: (
        <Input.TextArea
          rows={4}
          placeholder="Enter note..."
          onChange={(e) => (noteText = e.target.value)}
        />
      ),
      onOk: () => addNoteMutation.mutate(noteText),
    });
  };

  const handleUpdateTags = () => {
    const currentTags = user?.tags || [];
    Modal.confirm({
      title: "Update Tags",
      content: (
        <Select
          mode="multiple"
          defaultValue={currentTags}
          placeholder="Select tags"
          style={{ width: "100%", marginTop: 16 }}
          onChange={(values) => updateTagsMutation.mutate({ add: values as string[] })}
        >
          {AVAILABLE_TAGS.map((tag) => (
            <Select.Option key={tag} value={tag}>
              {tag}
            </Select.Option>
          ))}
        </Select>
      ),
    });
  };

  const handleUpdateGroup = () => {
    Modal.confirm({
      title: "Change Group",
      content: (
        <Select
          defaultValue={user?.group}
          placeholder="Select group"
          style={{ width: "100%", marginTop: 16 }}
          onChange={(value) => updateGroupMutation.mutate(value as PlayerGroup)}
        >
          {Object.entries(PLAYER_GROUPS).map(([key, { label }]) => (
            <Select.Option key={key} value={key}>
              {label}
            </Select.Option>
          ))}
        </Select>
      ),
    });
  };

  if (isLoading) return <Spin size="large" style={{ display: "block", margin: "100px auto" }} />;
  if (!user) return <Text>User not found</Text>;

  const getRiskScoreColor = (score: number) => {
    if (score >= 80) return "red";
    if (score >= 50) return "orange";
    if (score >= 30) return "gold";
    return "green";
  };

  // Tab 0: Overview
  const OverviewTab = () => (
    <div>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8}>
          <Card title="Personal Information">
            <Descriptions column={1} size="small">
              <Descriptions.Item label="ID">
                <Text copyable code>{user.id.slice(0, 16)}...</Text>
              </Descriptions.Item>
              <Descriptions.Item label="Username">{user.username || "—"}</Descriptions.Item>
              <Descriptions.Item label="Email">{user.email}</Descriptions.Item>
              <Descriptions.Item label="Phone">{user.phone || "—"}</Descriptions.Item>
              <Descriptions.Item label="Full Name">
                {`${user.first_name || ""} ${user.last_name || ""}`.trim() || "—"}
              </Descriptions.Item>
              <Descriptions.Item label="Date of Birth">
                {user.date_of_birth ? formatDate(user.date_of_birth) : "—"}
              </Descriptions.Item>
              <Descriptions.Item label="Address">{user.address || "—"}</Descriptions.Item>
              <Descriptions.Item label="City">{user.city || "—"}</Descriptions.Item>
              <Descriptions.Item label="Country">{user.country_code}</Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title="Financial Summary">
            <Descriptions column={1} size="small">
              <Descriptions.Item label="Balance">
                <MoneyDisplay amount={user.balance} currency={user.currency_code} />
              </Descriptions.Item>
              <Descriptions.Item label="Bonus Balance">
                <MoneyDisplay amount={user.bonus_balance} currency={user.currency_code} />
              </Descriptions.Item>
              <Descriptions.Item label="Total Deposits">
                <MoneyDisplay amount={user.deposit_total} currency={user.currency_code} />
              </Descriptions.Item>
              <Descriptions.Item label="Total Withdrawals">
                <MoneyDisplay amount={user.withdrawal_total} currency={user.currency_code} />
              </Descriptions.Item>
              <Descriptions.Item label="Net Deposits">
                <MoneyDisplay
                  amount={String(Number(user.deposit_total) - Number(user.withdrawal_total))}
                  currency={user.currency_code}
                />
              </Descriptions.Item>
              <Descriptions.Item label="GGR">
                <MoneyDisplay amount={user.ggr} currency={user.currency_code} />
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title="Gaming Stats">
            <Descriptions column={1} size="small">
              <Descriptions.Item label="Status">
                <StatusTag status={user.status} config={USER_STATUSES} />
              </Descriptions.Item>
              <Descriptions.Item label="KYC Level">
                <StatusTag status={String(user.kyc_level)} config={KYC_LEVELS} />
              </Descriptions.Item>
              <Descriptions.Item label="Group">
                <Tag color={PLAYER_GROUPS[user.group]?.color}>{PLAYER_GROUPS[user.group]?.label}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Risk Score">
                <Badge
                  count={user.risk_score}
                  style={{ backgroundColor: getRiskScoreColor(user.risk_score) }}
                />
              </Descriptions.Item>
              <Descriptions.Item label="Tags">
                <Space wrap>
                  {user.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="Affiliate ID">{user.affiliate_id || "—"}</Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
      </Row>

      <Card title="Quick Actions" style={{ marginTop: 16 }}>
        <Space wrap>
          {user.status === "active" ? (
            <Button danger icon={<StopOutlined />} onClick={handleBlock}>
              Block User
            </Button>
          ) : (
            <Button type="primary" icon={<CheckCircleOutlined />} onClick={handleUnblock}>
              Unblock User
            </Button>
          )}
          <Button icon={<LockOutlined />} onClick={handleAdjustLimits}>
            Adjust Limits
          </Button>
          <Button icon={<GiftOutlined />} onClick={handleGiveBonus}>
            Give Bonus
          </Button>
          <Button icon={<DollarOutlined />} onClick={handleAdjustBalance}>
            Adjust Balance
          </Button>
          <Button icon={<EditOutlined />} onClick={handleAddNote}>
            Add Note
          </Button>
          <Button icon={<MessageOutlined />} onClick={handleSendMessage}>
            Send Message
          </Button>
          <Button icon={<TagOutlined />} onClick={handleUpdateTags}>
            Update Tags
          </Button>
          <Button icon={<TeamOutlined />} onClick={handleUpdateGroup}>
            Change Group
          </Button>
        </Space>
      </Card>
    </div>
  );

  // Tab 1: Deposits
  const DepositsTab = () => {
    const columns: ColumnsType<Deposit> = [
      { title: "ID", dataIndex: "id", render: (v: string) => v.slice(0, 8), width: 80 },
      { title: "Method", dataIndex: "method" },
      {
        title: "Amount",
        dataIndex: "amount",
        render: (v: string, r) => <MoneyDisplay amount={v} currency={r.currency_code} />,
      },
      { title: "Provider", dataIndex: "provider" },
      { title: "Status", dataIndex: "status", render: (v) => <StatusTag status={v} config={TRANSACTION_STATUSES} /> },
      { title: "PSP Ref", dataIndex: "psp_reference", render: (v) => v?.slice(0, 12) || "—" },
      { title: "Created", dataIndex: "created_at", render: (v) => formatDate(v) },
      { title: "Completed", dataIndex: "completed_at", render: (v) => (v ? formatDate(v) : "—") },
    ];
    return (
      <Table
        dataSource={deposits?.data || []}
        columns={columns}
        loading={depositsLoading}
        rowKey="id"
        pagination={{ total: deposits?.pagination.total, pageSize: 20 }}
      />
    );
  };

  // Tab 2: Withdrawals
  const WithdrawalsTab = () => {
    const columns: ColumnsType<Withdrawal> = [
      { title: "ID", dataIndex: "id", render: (v: string) => v.slice(0, 8), width: 80 },
      { title: "Method", dataIndex: "method" },
      {
        title: "Amount",
        dataIndex: "amount",
        render: (v: string, r) => <MoneyDisplay amount={v} currency={r.currency_code} />,
      },
      { title: "Destination", dataIndex: "destination" },
      { title: "Status", dataIndex: "status", render: (v) => <StatusTag status={v} /> },
      { title: "Reviewed By", dataIndex: "reviewed_by", render: (v) => v?.slice(0, 8) || "—" },
      { title: "Created", dataIndex: "created_at", render: (v) => formatDate(v) },
    ];
    return (
      <Table
        dataSource={withdrawals?.data || []}
        columns={columns}
        loading={withdrawalsLoading}
        rowKey="id"
        pagination={{ total: withdrawals?.pagination.total, pageSize: 20 }}
      />
    );
  };

  // Tab 3: Casino Bets
  const CasinoBetsTab = () => {
    return (
      <Collapse>
        {casinoBets?.data?.map((session: CasinoBetSession) => (
          <Panel
            header={
              <Space>
                <Text strong>{session.game_name}</Text>
                <Text type="secondary">({session.provider})</Text>
                <Tag>Total Bet: {session.total_bet}</Tag>
                <Tag>Total Win: {session.total_win}</Tag>
                <Tag>{session.rounds} rounds</Tag>
              </Space>
            }
            key={session.session_id}
          >
            <Table
              dataSource={session.bets}
              rowKey="id"
              size="small"
              pagination={false}
              columns={[
                { title: "Bet ID", dataIndex: "id", render: (v: string) => v.slice(0, 8) },
                { title: "Amount", dataIndex: "bet_amount" },
                { title: "Win", dataIndex: "win_amount" },
                { title: "Balance After", dataIndex: "balance_after" },
                { title: "Time", dataIndex: "created_at", render: (v) => formatDate(v) },
              ]}
            />
          </Panel>
        ))}
        {!casinoBets?.data?.length && <Empty description="No casino bets" />}
      </Collapse>
    );
  };

  // Tab 4: Sports Bets
  const SportsBetsTab = () => {
    const columns: ColumnsType<Bet> = [
      { title: "ID", dataIndex: "id", render: (v: string) => v.slice(0, 8), width: 80 },
      { title: "Type", dataIndex: "bet_type" },
      {
        title: "Stake",
        dataIndex: "stake",
        render: (v: string, r) => <MoneyDisplay amount={v} currency={r.currency_code} />,
      },
      { title: "Odds", dataIndex: "odds" },
      {
        title: "Potential Win",
        dataIndex: "potential_win",
        render: (v: string, r) => <MoneyDisplay amount={v} currency={r.currency_code} />,
      },
      { title: "Status", dataIndex: "status", render: (v) => <StatusTag status={v} /> },
      { title: "Placed At", dataIndex: "placed_at", render: (v) => formatDate(v) },
    ];
    return (
      <Table
        dataSource={sportsBets?.data || []}
        columns={columns}
        loading={sportsBetsLoading}
        rowKey="id"
        pagination={{ total: sportsBets?.pagination.total, pageSize: 20 }}
      />
    );
  };

  // Tab 5: Bonuses
  const BonusesTab = () => {
    const columns: ColumnsType<UserBonus> = [
      { title: "Campaign", dataIndex: "campaign_name" },
      {
        title: "Amount",
        dataIndex: "bonus_amount",
        render: (v: string) => <MoneyDisplay amount={v} />,
      },
      {
        title: "Wagering Req",
        dataIndex: "wagering_requirement",
        render: (v: string) => <MoneyDisplay amount={v} />,
      },
      {
        title: "Progress",
        dataIndex: "wagering_progress",
        render: (v: string, r) => (
          <Tooltip title={`${v} / ${r.wagering_requirement}`}>
            {Math.round((Number(v) / Number(r.wagering_requirement)) * 100)}%
          </Tooltip>
        ),
      },
      { title: "Status", dataIndex: "status", render: (v) => <Tag color={v === "active" ? "green" : "default"}>{v}</Tag> },
      { title: "Claimed", dataIndex: "claimed_at", render: (v) => formatDate(v) },
      { title: "Expires", dataIndex: "expires_at", render: (v) => formatDate(v) },
    ];
    return (
      <Table
        dataSource={bonuses?.data || []}
        columns={columns}
        loading={bonusesLoading}
        rowKey="id"
        pagination={{ total: bonuses?.pagination.total, pageSize: 20 }}
      />
    );
  };

  // Tab 6: KYC
  const KycTab = () => {
    const columns: ColumnsType<KycDocument> = [
      { title: "Type", dataIndex: "type" },
      { title: "Status", dataIndex: "status", render: (v) => <StatusTag status={v} /> },
      { title: "Uploaded", dataIndex: "uploaded_at", render: (v) => formatDate(v) },
      { title: "Reviewed By", dataIndex: "reviewed_by_name", render: (v) => v || "—" },
      { title: "Reviewed At", dataIndex: "reviewed_at", render: (v) => (v ? formatDate(v) : "—") },
      { title: "Notes", dataIndex: "notes", render: (v) => v || "—" },
      { title: "Expires", dataIndex: "expires_at", render: (v) => (v ? formatDate(v) : "—") },
    ];
    return (
      <Table
        dataSource={kycDocs || []}
        columns={columns}
        loading={kycLoading}
        rowKey="id"
        pagination={false}
      />
    );
  };

  // Tab 7: RG
  const RgTab = () => (
    <Card title="Responsible Gambling Limits">
      {limits ? (
        <Descriptions column={2} bordered>
          <Descriptions.Item label="Daily Deposit">{limits.deposit_limit_daily || "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Weekly Deposit">{limits.deposit_limit_weekly || "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Monthly Deposit">{limits.deposit_limit_monthly || "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Wager Limit">{limits.wager_limit_daily || "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Loss Limit">{limits.loss_limit || "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Session Time">{limits.session_time_limit_minutes ? `${limits.session_time_limit_minutes} min` : "No limit"}</Descriptions.Item>
          <Descriptions.Item label="Self Exclusion">{limits.self_exclusion_until ? formatDate(limits.self_exclusion_until) : "None"}</Descriptions.Item>
        </Descriptions>
      ) : (
        <Empty description="No limits set" />
      )}
    </Card>
  );

  // Tab 8: Support
  const SupportTab = () => (
    <List
      loading={supportLoading}
      dataSource={supportChats?.data || []}
      renderItem={(chat) => (
        <List.Item>
          <List.Item.Meta
            title={chat.subject}
            description={
              <Space direction="vertical">
                <Text type="secondary">Agent: {chat.agent_name || "—"}</Text>
                <Text>{chat.last_message}</Text>
                <Text type="secondary">{formatDate(chat.last_message_at)}</Text>
              </Space>
            }
          />
          <Tag color={chat.status === "open" ? "red" : "default"}>{chat.status}</Tag>
        </List.Item>
      )}
    />
  );

  // Tab 9: Linked Accounts
  const LinkedTab = () => (
    <Card title="Linked Accounts Analysis">
      <Table
        dataSource={linkedAccounts || []}
        loading={linkedLoading}
        rowKey={(r: LinkedAccount) => `${r.player_id}-${r.link_type}`}
        columns={[
          { title: "Player ID", dataIndex: "player_id", render: (v: string) => v.slice(0, 8) },
          { title: "Username", dataIndex: "username" },
          { title: "Link Type", dataIndex: "link_type" },
          { title: "Link Value", dataIndex: "link_value", render: (v: string) => v.slice(0, 16) },
          {
            title: "Confidence",
            dataIndex: "confidence",
            render: (v: number) => <Badge count={`${v}%`} style={{ backgroundColor: v > 80 ? "red" : v > 50 ? "orange" : "blue" }} />,
          },
        ]}
      />
    </Card>
  );

  // Tab 10: Notes & Audit
  const NotesTab = () => (
    <Card title="Admin Notes">
      <Button icon={<EditOutlined />} onClick={handleAddNote} style={{ marginBottom: 16 }}>
        Add Note
      </Button>
      <List
        loading={notesLoading}
        dataSource={notes || []}
        renderItem={(note) => (
          <List.Item>
            <List.Item.Meta
              title={
                <Space>
                  <Text strong>{note.author_name}</Text>
                  <Text type="secondary">{formatDate(note.created_at)}</Text>
                </Space>
              }
              description={note.text}
            />
          </List.Item>
        )}
      />
    </Card>
  );

  // Tab 11: Sessions
  const SessionsTab = () => (
    <Table
      dataSource={sessions || []}
      loading={sessionsLoading}
      rowKey="id"
      columns={[
        { title: "Device", dataIndex: "device_fingerprint", render: (v: string) => v?.slice(0, 12) },
        { title: "IP", dataIndex: "ip_address" },
        { title: "User Agent", dataIndex: "user_agent", ellipsis: true },
        { title: "Created", dataIndex: "created_at", render: (v) => formatDate(v) },
        { title: "Last Activity", dataIndex: "last_activity", render: (v) => formatDate(v) },
        {
          title: "Action",
          render: (_, record: { id: string }) => (
            <Button
              size="small"
              danger
              onClick={() =>
                usersService.revokeSession(id!, record.id).then(() =>
                  queryClient.invalidateQueries({ queryKey: ["user-sessions", id] })
                )
              }
            >
              Revoke
            </Button>
          ),
        },
      ]}
    />
  );

  const tabItems = [
    { key: "overview", label: "Overview", children: <OverviewTab /> },
    { key: "deposits", label: "Deposits", children: <DepositsTab /> },
    { key: "withdrawals", label: "Withdrawals", children: <WithdrawalsTab /> },
    { key: "casino_bets", label: "Casino Bets", children: <CasinoBetsTab /> },
    { key: "sports_bets", label: "Sports Bets", children: <SportsBetsTab /> },
    { key: "bonuses", label: "Bonuses", children: <BonusesTab /> },
    { key: "kyc", label: "KYC Documents", children: <KycTab /> },
    { key: "rg", label: "RG & Limits", children: <RgTab /> },
    { key: "support", label: "Support History", children: <SupportTab /> },
    { key: "linked", label: "Linked Accounts", children: <LinkedTab /> },
    { key: "notes", label: "Notes & Audit", children: <NotesTab /> },
    { key: "sessions", label: "Sessions", children: <SessionsTab /> },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/users")}>
          Back
        </Button>
        <Title level={3} style={{ margin: 0 }}>
          {user.username || user.email} ({user.id.slice(0, 8)})
        </Title>
        <Badge
          count={user.risk_score}
          style={{ backgroundColor: getRiskScoreColor(user.risk_score) }}
        />
        <StatusTag status={user.status} config={USER_STATUSES} />
      </Space>

      <Card>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
      </Card>

      {/* Balance Modal */}
      <Modal
        title="Adjust Balance"
        open={showBalanceModal}
        onOk={() => adjustBalanceMutation.mutate()}
        onCancel={() => setShowBalanceModal(false)}
        confirmLoading={adjustBalanceMutation.isPending}
        okText="Confirm"
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Select
            style={{ width: "100%" }}
            value={balanceForm.type}
            onChange={(val) => setBalanceForm({ ...balanceForm, type: val })}
            options={[
              { label: "Credit (Add Funds)", value: "credit" },
              { label: "Debit (Deduct Funds)", value: "debit" },
            ]}
          />
          <Input
            type="number"
            placeholder="Amount"
            value={balanceForm.amount}
            onChange={(e) => setBalanceForm({ ...balanceForm, amount: e.target.value })}
            prefix={user?.currency_code || "USD"}
          />
          <Input.TextArea
            rows={3}
            placeholder="Reason for adjustment..."
            value={balanceForm.reason}
            onChange={(e) => setBalanceForm({ ...balanceForm, reason: e.target.value })}
          />
        </Space>
      </Modal>

      {/* Bonus Modal */}
      <Modal
        title="Give Bonus"
        open={showBonusModal}
        onOk={() => giveBonusMutation.mutate()}
        onCancel={() => setShowBonusModal(false)}
        confirmLoading={giveBonusMutation.isPending}
        okText="Grant Bonus"
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Input
            placeholder="Bonus Campaign ID"
            value={bonusForm.bonus_id}
            onChange={(e) => setBonusForm({ ...bonusForm, bonus_id: e.target.value })}
          />
          <Input.TextArea
            rows={3}
            placeholder="Reason..."
            value={bonusForm.reason}
            onChange={(e) => setBonusForm({ ...bonusForm, reason: e.target.value })}
          />
        </Space>
      </Modal>

      {/* Limits Modal */}
      <Modal
        title="Adjust RG & Limits"
        open={showLimitsModal}
        onOk={() => adjustLimitsMutation.mutate()}
        onCancel={() => setShowLimitsModal(false)}
        confirmLoading={adjustLimitsMutation.isPending}
        okText="Update Limits"
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Input
            placeholder="Daily Deposit Limit (leave empty to remove)"
            value={limitsForm.max_deposit_daily}
            onChange={(e) => setLimitsForm({ ...limitsForm, max_deposit_daily: e.target.value })}
          />
          <Input
            placeholder="Weekly Deposit Limit"
            value={limitsForm.max_deposit_weekly}
            onChange={(e) => setLimitsForm({ ...limitsForm, max_deposit_weekly: e.target.value })}
          />
          <Input
            placeholder="Daily Withdrawal Limit"
            value={limitsForm.max_withdrawal_daily}
            onChange={(e) => setLimitsForm({ ...limitsForm, max_withdrawal_daily: e.target.value })}
          />
          <Input
            placeholder="Max Bet Limit"
            value={limitsForm.max_bet}
            onChange={(e) => setLimitsForm({ ...limitsForm, max_bet: e.target.value })}
          />
          <Input
            placeholder="Daily Loss Limit"
            value={limitsForm.max_loss_daily}
            onChange={(e) => setLimitsForm({ ...limitsForm, max_loss_daily: e.target.value })}
          />
          <Input.TextArea
            rows={2}
            placeholder="Reason..."
            value={limitsForm.reason}
            onChange={(e) => setLimitsForm({ ...limitsForm, reason: e.target.value })}
          />
        </Space>
      </Modal>

      {/* Message Modal */}
      <Modal
        title="Send Message"
        open={showMessageModal}
        onOk={() => sendMessageMutation.mutate()}
        onCancel={() => setShowMessageModal(false)}
        confirmLoading={sendMessageMutation.isPending}
        okText="Send"
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Select
            style={{ width: "100%" }}
            value={messageForm.channel}
            onChange={(val) => setMessageForm({ ...messageForm, channel: val })}
            options={[
              { label: "Email", value: "email" },
              { label: "SMS", value: "sms" },
              { label: "Push Notification", value: "push" },
            ]}
          />
          {messageForm.channel === "email" && (
            <Input
              placeholder="Subject"
              value={messageForm.subject}
              onChange={(e) => setMessageForm({ ...messageForm, subject: e.target.value })}
            />
          )}
          <Input.TextArea
            rows={4}
            placeholder="Message body..."
            value={messageForm.body}
            onChange={(e) => setMessageForm({ ...messageForm, body: e.target.value })}
          />
        </Space>
      </Modal>

      {/* Block Modal */}
      <Modal
        title="Block User"
        open={showBlockModal}
        onOk={() => blockMutation.mutate()}
        onCancel={() => setShowBlockModal(false)}
        confirmLoading={blockMutation.isPending}
        okButtonProps={{ danger: true }}
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          <Select
            style={{ width: "100%" }}
            value={blockForm.type}
            onChange={(val) => setBlockForm({ ...blockForm, type: val })}
            options={[
              { label: "Full Block", value: "full" },
              { label: "Casino Only", value: "casino" },
              { label: "Sports Only", value: "sports" },
              { label: "Temporary", value: "temporary" },
            ]}
          />
          {blockForm.type === "temporary" && (
            <Input
              type="number"
              placeholder="Duration (hours)"
              value={blockForm.duration_hours}
              onChange={(e) => setBlockForm({ ...blockForm, duration_hours: Number(e.target.value) })}
            />
          )}
          <Input.TextArea
            rows={3}
            placeholder="Reason for blocking..."
            value={blockForm.reason}
            onChange={(e) => setBlockForm({ ...blockForm, reason: e.target.value })}
          />
        </Space>
      </Modal>
    </div>
  );
}
