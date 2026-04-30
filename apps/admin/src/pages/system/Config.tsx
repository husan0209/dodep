import { Card, Typography, Table, Switch, message } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { systemService } from "@/services/system.service";
import { getErrorMessage } from "@/utils/errors";

const { Title } = Typography;

export default function Config() {
  const queryClient = useQueryClient();

  const { data: flags, isLoading } = useQuery({
    queryKey: ["feature-flags"],
    queryFn: systemService.getFeatureFlags,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      systemService.toggleFeatureFlag(key, enabled),
    onSuccess: () => {
      message.success("Feature flag updated");
      queryClient.invalidateQueries({ queryKey: ["feature-flags"] });
    },
    onError: (error: unknown) => message.error(getErrorMessage(error)),
  });

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>
        Feature Flags
      </Title>
      <Card>
        <Table
          dataSource={flags || []}
          loading={isLoading}
          rowKey="key"
          columns={[
            { title: "Key", dataIndex: "key" },
            { title: "Description", dataIndex: "description", ellipsis: true },
            {
              title: "Enabled",
              dataIndex: "enabled",
              width: 100,
              render: (enabled: boolean, record: Record<string, unknown>) => (
                <Switch
                  checked={enabled}
                  onChange={(checked) =>
                    toggleMutation.mutate({
                      key: record.key as string,
                      enabled: checked,
                    })
                  }
                  loading={toggleMutation.isPending}
                />
              ),
            },
          ]}
          pagination={false}
        />
      </Card>
    </div>
  );
}
