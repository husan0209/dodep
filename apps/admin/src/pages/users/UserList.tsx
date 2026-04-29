import {
  Card,
  Typography,
  Space,
  Button,
  Tag,
  Input,
  Select,
  DatePicker,
  Dropdown,
  Badge,
  Tooltip,
  message,
  Modal,
} from "antd";
import {
  SearchOutlined,
  MoreOutlined,
  TagOutlined,
  TeamOutlined,
  ExportOutlined,
  MergeCellsOutlined,
} from "@ant-design/icons";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useState, useCallback } from "react";
import type { Dayjs } from "dayjs";
import DataTable from "@/components/common/DataTable";
import StatusTag from "@/components/common/StatusTag";
import MoneyDisplay from "@/components/common/MoneyDisplay";
import { usersService } from "@/services/users.service";
import { formatDate } from "@/utils/format";
import { USER_STATUSES, KYC_LEVELS } from "@/utils/constants";
import type { ColumnsType } from "antd/es/table";
import type { SorterResult } from "antd/es/table/interface";
import type { UserProfile, UserSearchParams, PlayerGroup } from "@/types/user";

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;

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

export default function UserList() {
  const navigate = useNavigate();
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [sorter, setSorter] = useState<SorterResult<UserProfile>>();

  // Filters state
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<string>();
  const [kycLevel, setKycLevel] = useState<number>();
  const [countryCode, setCountryCode] = useState<string>();
  const [playerGroup, setPlayerGroup] = useState<PlayerGroup>();
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [regDateRange, setRegDateRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);
  const [lastLoginRange, setLastLoginRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);
  const [depositRange, setDepositRange] = useState<{ min?: string; max?: string }>({});
  const [ggrRange, setGgrRange] = useState<{ min?: string; max?: string }>({});
  const [balanceRange, setBalanceRange] = useState<{ min?: string; max?: string }>({});
  const [riskScoreRange, setRiskScoreRange] = useState<{ min?: number; max?: number }>({});

  // Pagination
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const buildSearchParams = useCallback((): UserSearchParams => {
    return {
      page,
      page_size: pageSize,
      search: search || undefined,
      status: status as UserProfile["status"],
      kyc_level: kycLevel as UserProfile["kyc_level"],
      country_code: countryCode,
      player_group: playerGroup,
      tags: selectedTags.length > 0 ? selectedTags : undefined,
      created_from: regDateRange[0]?.toISOString(),
      created_to: regDateRange[1]?.toISOString(),
      last_login_from: lastLoginRange[0]?.toISOString(),
      last_login_to: lastLoginRange[1]?.toISOString(),
      deposit_min: depositRange.min,
      deposit_max: depositRange.max,
      ggr_min: ggrRange.min,
      ggr_max: ggrRange.max,
      balance_min: balanceRange.min,
      balance_max: balanceRange.max,
      risk_score_min: riskScoreRange.min,
      risk_score_max: riskScoreRange.max,
      sort_by: sorter?.field as string,
      sort_order: sorter?.order === "ascend" ? "asc" : "desc",
    };
  }, [
    page, pageSize, search, status, kycLevel, countryCode, playerGroup, selectedTags,
    regDateRange, lastLoginRange, depositRange, ggrRange, balanceRange, riskScoreRange,
    sorter,
  ]);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["users", buildSearchParams()],
    queryFn: () => usersService.list(buildSearchParams()),
  });

  const exportMutation = useMutation({
    mutationFn: () => usersService.export(buildSearchParams()),
    onSuccess: (result) => {
      message.success(`Export started. Download link: ${result.download_url}`);
    },
    onError: () => message.error("Export failed"),
  });

  const bulkAddTagsMutation = useMutation({
    mutationFn: (tags: string[]) =>
      usersService.bulkAddTags(selectedRowKeys as string[], { tags, reason: "Bulk tag update" }),
    onSuccess: () => {
      message.success("Tags added to selected users");
      setSelectedRowKeys([]);
      refetch();
    },
    onError: () => message.error("Failed to add tags"),
  });

  const bulkUpdateGroupMutation = useMutation({
    mutationFn: (group: PlayerGroup) =>
      usersService.bulkUpdateGroup(selectedRowKeys as string[], { group, reason: "Bulk group update" }),
    onSuccess: () => {
      message.success("Group updated for selected users");
      setSelectedRowKeys([]);
      refetch();
    },
    onError: () => message.error("Failed to update group"),
  });

  const handleBulkAddTags = () => {
    Modal.confirm({
      title: "Add Tags to Selected Users",
      content: (
        <Select
          mode="multiple"
          placeholder="Select tags"
          style={{ width: "100%", marginTop: 16 }}
          onChange={(values) => bulkAddTagsMutation.mutate(values as string[])}
        >
          {AVAILABLE_TAGS.map((tag) => (
            <Select.Option key={tag} value={tag}>
              {tag}
            </Select.Option>
          ))}
        </Select>
      ),
      onOk: () => {},
    });
  };

  const handleBulkUpdateGroup = () => {
    Modal.confirm({
      title: "Change Group for Selected Users",
      content: (
        <Select
          placeholder="Select group"
          style={{ width: "100%", marginTop: 16 }}
          onChange={(value) => bulkUpdateGroupMutation.mutate(value as PlayerGroup)}
        >
          {Object.entries(PLAYER_GROUPS).map(([key, { label }]) => (
            <Select.Option key={key} value={key}>
              {label}
            </Select.Option>
          ))}
        </Select>
      ),
      onOk: () => {},
    });
  };

  const getRiskScoreColor = (score: number) => {
    if (score >= 80) return "red";
    if (score >= 50) return "orange";
    if (score >= 30) return "gold";
    return "green";
  };

  const columns: ColumnsType<UserProfile> = [
    {
      title: "ID",
      dataIndex: "id",
      key: "id",
      width: 100,
      render: (id: string) => (
        <Text copyable={{ text: id }} code style={{ fontSize: 12 }}>
          {id.slice(0, 8)}
        </Text>
      ),
    },
    {
      title: "Username",
      dataIndex: "username",
      key: "username",
      render: (username: string | null, record) => (
        <div>
          <div>{username || "—"}</div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {record.email}
          </Text>
        </div>
      ),
    },
    {
      title: "Country",
      dataIndex: "country_code",
      key: "country_code",
      width: 80,
      render: (code: string) => (
        <Tooltip title={code}>
          <span className="fi fi-{code.toLowerCase()}">{code}</span>
        </Tooltip>
      ),
    },
    {
      title: "Group",
      dataIndex: "group",
      key: "group",
      width: 100,
      render: (group: PlayerGroup) => {
        const config = PLAYER_GROUPS[group];
        return <Tag color={config?.color}>{config?.label || group}</Tag>;
      },
      filters: Object.entries(PLAYER_GROUPS).map(([key, { label }]) => ({
        text: label,
        value: key,
      })),
    },
    {
      title: "Tags",
      dataIndex: "tags",
      key: "tags",
      width: 150,
      render: (tags: string[]) => (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
          {tags?.slice(0, 3).map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
          {tags?.length > 3 && (
            <Tooltip title={tags.slice(3).join(", ")}>
              <Tag>+{tags.length - 3}</Tag>
            </Tooltip>
          )}
        </div>
      ),
    },
    {
      title: "Deposits",
      dataIndex: "deposit_total",
      key: "deposit_total",
      width: 120,
      sorter: true,
      render: (amount: string, record) => (
        <MoneyDisplay amount={amount} currency={record.currency_code} />
      ),
    },
    {
      title: "GGR",
      dataIndex: "ggr",
      key: "ggr",
      width: 100,
      sorter: true,
      render: (amount: string, record) => (
        <MoneyDisplay amount={amount} currency={record.currency_code} />
      ),
    },
    {
      title: "Balance",
      dataIndex: "balance",
      key: "balance",
      width: 100,
      sorter: true,
      render: (amount: string, record) => (
        <MoneyDisplay amount={amount} currency={record.currency_code} />
      ),
    },
    {
      title: "Risk",
      dataIndex: "risk_score",
      key: "risk_score",
      width: 80,
      sorter: true,
      render: (score: number) => (
        <Badge
          count={score}
          style={{
            backgroundColor: getRiskScoreColor(score),
            fontSize: 12,
            minWidth: 28,
          }}
        />
      ),
    },
    {
      title: "KYC",
      dataIndex: "kyc_level",
      key: "kyc_level",
      width: 80,
      render: (level: number) => (
        <StatusTag status={String(level)} config={KYC_LEVELS} />
      ),
      filters: Object.entries(KYC_LEVELS).map(([key, val]) => ({
        text: val.label,
        value: Number(key),
      })),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status: string) => <StatusTag status={status} config={USER_STATUSES} />,
      filters: Object.entries(USER_STATUSES).map(([key, val]) => ({
        text: val.label,
        value: key,
      })),
    },
    {
      title: "Registered",
      dataIndex: "created_at",
      key: "created_at",
      width: 120,
      sorter: true,
      render: (date: string) => formatDate(date, "YYYY-MM-DD"),
    },
    {
      title: "Last Login",
      dataIndex: "last_login_at",
      key: "last_login_at",
      width: 120,
      sorter: true,
      render: (date: string | null) =>
        date ? formatDate(date, "YYYY-MM-DD HH:mm") : "Never",
    },
    {
      title: "Actions",
      key: "actions",
      fixed: "right",
      width: 80,
      render: (_, record) => (
        <Dropdown
          menu={{
            items: [
              { key: "view", label: "View Profile", onClick: () => navigate(`/users/${record.id}`) },
              { key: "edit", label: "Edit", onClick: () => navigate(`/users/${record.id}`) },
              { type: "divider" },
              { key: "block", label: "Block", danger: true },
            ],
          }}
          trigger={["click"]}
        >
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ),
    },
  ];

  const rowSelection = {
    selectedRowKeys,
    onChange: (newSelectedRowKeys: React.Key[]) => setSelectedRowKeys(newSelectedRowKeys),
  };

  return (
    <div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: 16,
        }}
      >
        <Title level={3} style={{ margin: 0 }}>
          Player Management
        </Title>
        <Space>
          {selectedRowKeys.length > 0 && (
            <>
              <Text type="secondary">{selectedRowKeys.length} selected</Text>
              <Button icon={<TagOutlined />} onClick={handleBulkAddTags}>
                Add Tags
              </Button>
              <Button icon={<TeamOutlined />} onClick={handleBulkUpdateGroup}>
                Change Group
              </Button>
            </>
          )}
          <Button
            icon={<ExportOutlined />}
            loading={exportMutation.isPending}
            onClick={() => exportMutation.mutate()}
          >
            Export
          </Button>
          <Button icon={<MergeCellsOutlined />} onClick={() => navigate("/users/merge")}>
            Merge
          </Button>
        </Space>
      </div>

      <Card style={{ marginBottom: 16 }}>
        <Space style={{ marginBottom: 16 }} wrap size="small">
          <Input.Search
            placeholder="Search by email, ID, username..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onSearch={() => setPage(1)}
            allowClear
            style={{ width: 280 }}
          />

          <Select
            placeholder="Status"
            allowClear
            style={{ width: 130 }}
            value={status}
            onChange={(val) => {
              setStatus(val);
              setPage(1);
            }}
            options={Object.entries(USER_STATUSES).map(([key, val]) => ({
              label: val.label,
              value: key,
            }))}
          />

          <Select
            placeholder="KYC Level"
            allowClear
            style={{ width: 130 }}
            value={kycLevel}
            onChange={(val) => {
              setKycLevel(val);
              setPage(1);
            }}
            options={Object.entries(KYC_LEVELS).map(([key, val]) => ({
              label: val.label,
              value: Number(key),
            }))}
          />

          <Select
            placeholder="Group"
            allowClear
            style={{ width: 120 }}
            value={playerGroup}
            onChange={(val) => {
              setPlayerGroup(val);
              setPage(1);
            }}
            options={Object.entries(PLAYER_GROUPS).map(([key, { label }]) => ({
              label,
              value: key,
            }))}
          />

          <Select
            placeholder="Tags"
            allowClear
            mode="multiple"
            maxTagCount={1}
            style={{ width: 180 }}
            value={selectedTags}
            onChange={(val) => {
              setSelectedTags(val as string[]);
              setPage(1);
            }}
            options={AVAILABLE_TAGS.map((tag) => ({ label: tag, value: tag }))}
          />

          <Select
            placeholder="Country"
            allowClear
            style={{ width: 100 }}
            value={countryCode}
            onChange={(val) => {
              setCountryCode(val);
              setPage(1);
            }}
            showSearch
            options={[
              { label: "DE", value: "DE" },
              { label: "BR", value: "BR" },
              { label: "TR", value: "TR" },
              { label: "IN", value: "IN" },
              { label: "CA", value: "CA" },
              { label: "UK", value: "UK" },
              { label: "AU", value: "AU" },
            ]}
          />
        </Space>

        <Space wrap size="small">
          <RangePicker
            placeholder={["Reg From", "Reg To"]}
            value={regDateRange}
            onChange={(dates) => {
              setRegDateRange(dates || [null, null]);
              setPage(1);
            }}
          />

          <RangePicker
            placeholder={["Login From", "Login To"]}
            value={lastLoginRange}
            onChange={(dates) => {
              setLastLoginRange(dates || [null, null]);
              setPage(1);
            }}
          />

          <Space.Compact>
            <Input
              placeholder="Min Deposit"
              value={depositRange.min}
              onChange={(e) => setDepositRange({ ...depositRange, min: e.target.value })}
              style={{ width: 100 }}
            />
            <Input
              placeholder="Max"
              value={depositRange.max}
              onChange={(e) => setDepositRange({ ...depositRange, max: e.target.value })}
              style={{ width: 80 }}
            />
          </Space.Compact>

          <Space.Compact>
            <Input
              placeholder="Min GGR"
              value={ggrRange.min}
              onChange={(e) => setGgrRange({ ...ggrRange, min: e.target.value })}
              style={{ width: 80 }}
            />
            <Input
              placeholder="Max"
              value={ggrRange.max}
              onChange={(e) => setGgrRange({ ...ggrRange, max: e.target.value })}
              style={{ width: 80 }}
            />
          </Space.Compact>

          <Space.Compact>
            <Input
              placeholder="Min Risk"
              type="number"
              value={riskScoreRange.min}
              onChange={(e) =>
                setRiskScoreRange({ ...riskScoreRange, min: Number(e.target.value) })
              }
              style={{ width: 80 }}
            />
            <Input
              placeholder="Max"
              type="number"
              value={riskScoreRange.max}
              onChange={(e) =>
                setRiskScoreRange({ ...riskScoreRange, max: Number(e.target.value) })
              }
              style={{ width: 70 }}
            />
          </Space.Compact>
        </Space>
      </Card>

      <Card>
        <DataTable
          data={data?.data || []}
          columns={columns}
          loading={isLoading}
          total={data?.pagination.total || 0}
          page={page}
          pageSize={pageSize}
          rowSelection={rowSelection}
          onPageChange={(p, ps, _filters, sorterResult) => {
            setPage(p);
            setPageSize(ps);
            if (sorterResult) setSorter(sorterResult as SorterResult<UserProfile>);
          }}
          scroll={{ x: 1400 }}
        />
      </Card>
    </div>
  );
}
