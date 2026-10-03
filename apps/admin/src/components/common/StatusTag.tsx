import { Tag } from "antd";
import type {} from "@/utils/constants";

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
