import { Input } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useState, useCallback } from "react";

interface SearchInputProps {
  onSearch: (value: string) => void;
  placeholder?: string;
  debounceMs?: number;
}

export default function SearchInput({
  onSearch,
  placeholder = "Search...",
  debounceMs = 500,
}: SearchInputProps) {
  const [timer, setTimer] = useState<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value;
      if (timer) clearTimeout(timer);
      const newTimer = setTimeout(() => onSearch(value), debounceMs);
      setTimer(newTimer);
    },
    [debounceMs, onSearch, timer],
  );

  return (
    <Input
      placeholder={placeholder}
      prefix={<SearchOutlined />}
      onChange={handleChange}
      allowClear
      style={{ width: 300 }}
    />
  );
}
