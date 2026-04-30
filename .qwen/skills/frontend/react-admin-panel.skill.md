## #31 react-admin-panel.skill.md

```markdown
# react-admin-panel.skill.md

## РОЛЬ
Ты — Frontend Developer, создающий Admin Panel для гемблинг-платформы
на React + TypeScript + Ant Design Pro.

## КОНТЕКСТ
- Внутренний инструмент для support, risk, finance, marketing
- RBAC: 7 ролей с разными правами
- Все действия логируются в audit trail
- Не публичное приложение — SEO не нужен, SPA подход

## СТРУКТУРА
admin/
├── src/
│ ├── main.tsx
│ ├── App.tsx
│ ├── routes.tsx # React Router config
│ │
│ ├── api/ # API клиент
│ │ ├── client.ts
│ │ ├── users.ts
│ │ ├── bets.ts
│ │ ├── payments.ts
│ │ ├── bonuses.ts
│ │ ├── fraud.ts
│ │ └── system.ts
│ │
│ ├── components/ # Shared компоненты
│ │ ├── Layout/
│ │ │ ├── AdminLayout.tsx # Sidebar + Header + Content
│ │ │ ├── Sidebar.tsx
│ │ │ └── Header.tsx
│ │ ├── PermissionGuard.tsx # RBAC wrapper
│ │ ├── AuditLog.tsx # встраиваемый лог
│ │ ├── StatusBadge.tsx
│ │ ├── CurrencyDisplay.tsx
│ │ ├── DateRangePicker.tsx
│ │ ├── ExportButton.tsx # CSV/PDF export
│ │ ├── ConfirmAction.tsx # confirm modal
│ │ └── SearchInput.tsx
│ │
│ ├── modules/ # Feature modules
│ │ ├── users/
│ │ │ ├── UserList.tsx
│ │ │ ├── UserDetail.tsx
│ │ │ ├── UserEdit.tsx
│ │ │ └── UserSessions.tsx
│ │ ├── finance/
│ │ │ ├── DepositList.tsx
│ │ │ ├── WithdrawalQueue.tsx
│ │ │ ├── BalanceAdjustment.tsx
│ │ │ └── FinancialReport.tsx
│ │ ├── sports/
│ │ │ ├── EventManagement.tsx
│ │ │ ├── BetManagement.tsx
│ │ │ └── LiabilityMonitor.tsx
│ │ ├── casino/
│ │ │ ├── GameCatalog.tsx
│ │ │ └── RTPMonitor.tsx
│ │ ├── bonuses/
│ │ │ ├── CampaignList.tsx
│ │ │ ├── CampaignEditor.tsx
│ │ │ └── BonusGrant.tsx
│ │ ├── risk/
│ │ │ ├── FraudAlerts.tsx
│ │ │ ├── ReviewQueue.tsx
│ │ │ └── UserRiskProfile.tsx
│ │ ├── content/
│ │ │ ├── PageEditor.tsx
│ │ │ ├── BannerManager.tsx
│ │ │ └── PromotionEditor.tsx
│ │ ├── affiliates/
│ │ │ ├── AffiliateList.tsx
│ │ │ └── CommissionReport.tsx
│ │ └── system/
│ │ ├── FeatureFlags.tsx
│ │ ├── AuditLogViewer.tsx
│ │ └── HealthDashboard.tsx
│ │
│ ├── hooks/
│ │ ├── usePermission.ts
│ │ ├── useAuditAction.ts
│ │ └── useTableQuery.ts
│ │
│ ├── stores/
│ │ └── authStore.ts
│ │
│ └── types/
│ ├── user.ts
│ ├── bet.ts
│ ├── payment.ts
│ └── permissions.ts

text


## RBAC — PERMISSION GUARD

```tsx
// components/PermissionGuard.tsx
import { useAuthStore } from '@/stores/authStore';

interface PermissionGuardProps {
  permission: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function PermissionGuard({ 
  permission, children, fallback = null 
}: PermissionGuardProps) {
  const permissions = useAuthStore((s) => s.permissions);
  
  if (!permissions.includes(permission) && !permissions.includes('all.*')) {
    return <>{fallback}</>;
  }
  
  return <>{children}</>;
}

// Использование:
<PermissionGuard permission="withdrawal.approve_large">
  <Button onClick={approveWithdrawal}>Approve $10,000</Button>
</PermissionGuard>

// hooks/usePermission.ts
export function usePermission(permission: string): boolean {
  const permissions = useAuthStore((s) => s.permissions);
  return permissions.includes(permission) || permissions.includes('all.*');
}

// Использование:
const canApprove = usePermission('withdrawal.approve_large');
ТИПОВАЯ СТРАНИЦА — СПИСОК С ФИЛЬТРАМИ
React

// modules/users/UserList.tsx
import { Table, Input, Select, DatePicker, Space, Tag, Button } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { usersApi } from '@/api/users';
import { useTableQuery } from '@/hooks/useTableQuery';
import { StatusBadge } from '@/components/StatusBadge';
import { CurrencyDisplay } from '@/components/CurrencyDisplay';
import { ExportButton } from '@/components/ExportButton';

export function UserList() {
  const {
    params, setSearch, setFilter, setPagination, setSorter
  } = useTableQuery({
    defaultSort: 'created_at',
    defaultOrder: 'desc',
  });

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'users', params],
    queryFn: () => usersApi.list(params),
  });

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      sorter: true,
      width: 80,
    },
    {
      title: 'Email',
      dataIndex: 'email',
      render: (email: string, record: User) => (
        <Link to={`/users/${record.id}`}>{email}</Link>
      ),
    },
    {
      title: 'Country',
      dataIndex: 'country_code',
      filters: countryOptions,
      width: 100,
    },
    {
      title: 'KYC',
      dataIndex: 'kyc_level',
      render: (level: number) => (
        <Tag color={level >= 2 ? 'green' : 'orange'}>Level {level}</Tag>
      ),
      width: 100,
    },
    {
      title: 'Balance',
      dataIndex: 'balance',
      render: (bal: number, record: User) => (
        <CurrencyDisplay amount={bal} currency={record.currency} />
      ),
      sorter: true,
      align: 'right' as const,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      render: (status: string) => <StatusBadge status={status} />,
      filters: [
        { text: 'Active', value: 'active' },
        { text: 'Blocked', value: 'blocked' },
        { text: 'Pending', value: 'pending' },
      ],
    },
    {
      title: 'Registered',
      dataIndex: 'created_at',
      render: (date: string) => formatDate(date),
      sorter: true,
    },
    {
      title: 'Actions',
      render: (_: unknown, record: User) => (
        <Space>
          <Button size="small" onClick={() => navigate(`/users/${record.id}`)}>
            View
          </Button>
          <PermissionGuard permission="user.block">
            <Button size="small" danger onClick={() => blockUser(record.id)}>
              Block
            </Button>
          </PermissionGuard>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-4 flex justify-between">
        <Space>
          <Input.Search
            placeholder="Search by email, phone, ID..."
            onSearch={setSearch}
            style={{ width: 300 }}
          />
          <Select
            placeholder="KYC Level"
            allowClear
            onChange={(v) => setFilter('kyc_level', v)}
            options={[
              { label: 'Level 0', value: 0 },
              { label: 'Level 1', value: 1 },
              { label: 'Level 2', value: 2 },
              { label: 'Level 3', value: 3 },
            ]}
          />
        </Space>
        <ExportButton queryKey={['admin', 'users', params]} filename="users" />
      </div>

      <Table
        columns={columns}
        dataSource={data?.items}
        loading={isLoading}
        rowKey="id"
        pagination={{
          total: data?.total,
          current: params.page,
          pageSize: params.pageSize,
          showSizeChanger: true,
          showTotal: (total) => `Total: ${total}`,
        }}
        onChange={(pagination, filters, sorter) => {
          setPagination(pagination);
          setSorter(sorter);
        }}
      />
    </div>
  );
}
AUDIT ACTION HOOK
React

// hooks/useAuditAction.ts
import { useMutation } from '@tanstack/react-query';
import { Modal, message } from 'antd';

interface AuditActionOptions {
  action: string;
  entityType: string;
  entityId: string | number;
  description: string;
  requireConfirm?: boolean;
  confirmMessage?: string;
}

export function useAuditAction<TData, TVariables>(
  mutationFn: (variables: TVariables) => Promise<TData>,
  options: AuditActionOptions,
) {
  const mutation = useMutation({
    mutationFn: async (variables: TVariables) => {
      const result = await mutationFn(variables);
      // Audit log отправляется автоматически backend-ом
      // Здесь только UI feedback
      return result;
    },
    onSuccess: () => {
      message.success(`${options.description} — выполнено`);
    },
    onError: (error: any) => {
      message.error(error?.message ?? 'Ошибка');
    },
  });

  const execute = (variables: TVariables) => {
    if (options.requireConfirm) {
      Modal.confirm({
        title: 'Подтверждение',
        content: options.confirmMessage ?? `Вы уверены: ${options.description}?`,
        okText: 'Подтвердить',
        cancelText: 'Отмена',
        okButtonProps: { danger: true },
        onOk: () => mutation.mutateAsync(variables),
      });
    } else {
      mutation.mutate(variables);
    }
  };

  return { execute, ...mutation };
}

// Использование:
const blockUser = useAuditAction(
  (userId: number) => usersApi.block(userId),
  {
    action: 'user.block',
    entityType: 'user',
    entityId: userId,
    description: 'Блокировка пользователя',
    requireConfirm: true,
    confirmMessage: 'Заблокировать пользователя? Все сессии будут завершены.',
  },
);

// В JSX:
<Button danger onClick={() => blockUser.execute(user.id)}>
  Block User
</Button>
WITHDRAWAL REVIEW QUEUE
React

// modules/finance/WithdrawalQueue.tsx — пример сложного модуля
export function WithdrawalQueue() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin', 'withdrawals', 'pending'],
    queryFn: () => paymentsApi.getPendingWithdrawals(),
    refetchInterval: 10_000, // обновлять каждые 10 сек
  });

  const approve = useAuditAction(
    (id: number) => paymentsApi.approveWithdrawal(id),
    {
      action: 'withdrawal.approve',
      entityType: 'withdrawal',
      entityId: 'dynamic',
      description: 'Одобрение вывода',
      requireConfirm: true,
    },
  );

  const reject = useAuditAction(
    ({ id, reason }: { id: number; reason: string }) =>
      paymentsApi.rejectWithdrawal(id, reason),
    {
      action: 'withdrawal.reject',
      entityType: 'withdrawal',
      entityId: 'dynamic',
      description: 'Отклонение вывода',
      requireConfirm: true,
    },
  );

  // ... Table с колонками и действиями
}
АНТИПАТТЕРНЫ
React

// ❌ ПЛОХО: нет RBAC проверки
<Button onClick={deleteUser}>Delete</Button>

// ✅ ПРАВИЛЬНО: всегда оборачивать в PermissionGuard
<PermissionGuard permission="user.delete">
  <Button onClick={deleteUser}>Delete</Button>
</PermissionGuard>

// ❌ ПЛОХО: действие без подтверждения
const handleBlock = () => api.blockUser(id);

// ✅ ПРАВИЛЬНО: критические действия через useAuditAction с confirm

// ❌ ПЛОХО: кастомные UI-компоненты вместо Ant Design
<div className="custom-table">...</div>

// ✅ ПРАВИЛЬНО: используй Ant Design Table, Form, Modal и т.д.
// Кастом только когда Ant Design не покрывает

// ❌ ПЛОХО: хардкод ролей
if (user.role === 'admin') { ... }

// ✅ ПРАВИЛЬНО: проверка по permissions
if (hasPermission('user.delete')) { ... }
ПРАВИЛА
text

1. Все деструктивные действия — confirm modal + audit log
2. Все списки — с pagination, search, filters, export
3. Все формы — валидация (Ant Form + yup/zod)
4. Все данные — через TanStack Query (кэш + refetch)
5. Все компоненты — TypeScript strict
6. Все таблицы — сортировка по дате desc по умолчанию
7. Финансовые суммы — всегда с валютой, 2 знака
8. Даты — в UTC на сервере, в local timezone для отображения