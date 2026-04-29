import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Table,
  Button,
  Slider,
  InputNumber,
  message,
  Space,
  Tag,
  Modal,
  Form,
} from "antd";
import { ReloadOutlined, SaveOutlined, EditOutlined } from "@ant-design/icons";
import { casinoService } from "@/services/casino.service";

const { Title } = Typography;

interface RtpGame {
  id: string;
  game_name: string;
  provider: string;
  current_rtp: number;
  target_rtp: number;
  min_bet: number;
  max_bet: number;
  status: "active" | "disabled";
  updated_at: string;
}

export default function RtpConfig() {
  const queryClient = useQueryClient();
  const [editModal, setEditModal] = useState<{ open: boolean; game: RtpGame | null }>({
    open: false,
    game: null,
  });
  const [form] = Form.useForm();

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["rtp-config"],
    queryFn: () => casinoService.getRtpConfig() as Promise<RtpGame[]>,
  });

  const saveMutation = useMutation({
    mutationFn: (payload: { gameId: string; targetRtp: number }) =>
      casinoService.updateRtp(payload.gameId, payload.targetRtp),
    onSuccess: () => {
      message.success("RTP updated");
      setEditModal({ open: false, game: null });
      queryClient.invalidateQueries({ queryKey: ["rtp-config"] });
    },
    onError: () => message.error("Failed to update RTP"),
  });

  const columns = [
    { title: "Game", dataIndex: "game_name" },
    { title: "Provider", dataIndex: "provider", render: (v: string) => <Tag>{v}</Tag> },
    {
      title: "Current RTP",
      dataIndex: "current_rtp",
      render: (v: number) => `${(v * 100).toFixed(2)}%`,
    },
    {
      title: "Target RTP",
      dataIndex: "target_rtp",
      render: (v: number) => `${(v * 100).toFixed(2)}%`,
    },
    {
      title: "Min / Max Bet",
      render: (_: unknown, r: RtpGame) => `${r.min_bet} / ${r.max_bet}`,
    },
    {
      title: "Status",
      dataIndex: "status",
      render: (v: string) => (
        <Tag color={v === "active" ? "green" : "default"}>{v.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Actions",
      render: (_: unknown, r: RtpGame) => (
        <Button
          icon={<EditOutlined />}
          size="small"
          onClick={() => {
            setEditModal({ open: true, game: r });
            form.setFieldsValue({ targetRtp: r.target_rtp * 100 });
          }}
        >
          Edit
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Title level={3}>RTP Configuration</Title>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
          Refresh
        </Button>
      </Space>
      <Card>
        <Table
          rowKey="id"
          dataSource={data || []}
          columns={columns}
          loading={isLoading}
          pagination={{ pageSize: 20 }}
        />
      </Card>

      <Modal
        title={editModal.game?.game_name || "Edit RTP"}
        open={editModal.open}
        onCancel={() => setEditModal({ open: false, game: null })}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => {
            if (editModal.game) {
              saveMutation.mutate({
                gameId: editModal.game.id,
                targetRtp: values.targetRtp / 100,
              });
            }
          }}
        >
          <Form.Item name="targetRtp" label="Target RTP (%)" rules={[{ required: true }]}>
            <Slider min={85} max={99} step={0.1} />
          </Form.Item>
          <Form.Item name="targetRtp" noStyle>
            <InputNumber min={85} max={99} step={0.1} style={{ width: 120 }} suffix="%" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
