import { Typography } from "antd";
import { formatMoney } from "@/utils/format";

const { Text } = Typography;

interface MoneyDisplayProps {
  amount: string | number;
  currency?: string;
  type?: "positive" | "negative" | "neutral";
  bold?: boolean;
}

export default function MoneyDisplay({
  amount,
  currency = "USD",
  type = "neutral",
  bold = false,
}: MoneyDisplayProps) {
  const color =
    type === "positive"
      ? "#52c41a"
      : type === "negative"
        ? "#ff4d4f"
        : undefined;

  return (
    <Text strong={bold} style={{ color, fontFamily: "monospace" }}>
      {formatMoney(amount, currency)}
    </Text>
  );
}
