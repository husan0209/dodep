import { Table, type TableProps } from "antd";
import type { ColumnsType, TablePaginationConfig } from "antd/es/table";
import type { FilterValue, SorterResult } from "antd/es/table/interface";

interface DataTableProps<T> extends Omit<TableProps<T>, "onChange"> {
  data: T[];
  columns: ColumnsType<T>;
  loading?: boolean;
  total?: number;
  page?: number;
  pageSize?: number;
  onPageChange?: (
    page: number,
    pageSize: number,
    filters?: Record<string, FilterValue | null>,
    sorter?: SorterResult<T>,
  ) => void;
}

export default function DataTable<T extends object>({
  data,
  columns,
  loading = false,
  total = 0,
  page = 1,
  pageSize = 20,
  onPageChange,
  ...rest
}: DataTableProps<T>) {
  const pagination: TablePaginationConfig = {
    current: page,
    pageSize,
    total,
    showSizeChanger: true,
    showTotal: (total) => `Total ${total} items`,
    pageSizeOptions: ["10", "20", "50", "100"],
  };

  const handleChange: TableProps<T>["onChange"] = (
    pagination,
    filters,
    sorter,
  ) => {
    if (onPageChange) {
      onPageChange(
        pagination.current || 1,
        pagination.pageSize || 20,
        filters as Record<string, FilterValue | null>,
        sorter as SorterResult<T>,
      );
    }
  };

  return (
    <Table
      dataSource={data}
      columns={columns}
      loading={loading}
      pagination={pagination}
      onChange={handleChange}
      rowKey={(record: T) => (record as { id: string }).id}
      scroll={{ x: "max-content" }}
      {...rest}
    />
  );
}
