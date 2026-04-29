import { Tag } from "antd";
import type {
  USER_STATUSES,
  KYC_LEVELS,
  BET_STATUSES,
  TRANSACTION_STATUSES,
  WITHDRAWAL_STATUSES,
  ALERT_SEVERITIES,
  ALERT_STATUSES,
  BONUS_TYPES,
} from "@/utils/constants";

export type StatusConfig = { label: string; color: string };

interface StatusTagProps {
  status: string;
  config?: Record<string, StatusConfig>;
}

export default function StatusTag({ status, config }: StatusTagProps) {
  const statusConfig = config?.[status] as StatusConfig | undefined;
  if (!statusConfig) {
    return <Tag>{status}</Tag>;
  }
  return <Tag color={statusConfig.color}>{statusConfig.label}</Tag>;
}
