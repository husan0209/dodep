import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  Typography,
  Tag,
  Space,
  Button,
  Input,
  List,
  Avatar,
  Select,
  message,
  Divider,
  Descriptions,
  Badge,
  Modal,
} from "antd";
import {
  SendOutlined,
  LockOutlined,
  ArrowLeftOutlined,
  CheckCircleOutlined,
  UserAddOutlined,
} from "@ant-design/icons";
import { supportService } from "@/services/support.service";
import { useAuthStore } from "@/stores/authStore";
import type { TicketMessage, SupportTicket } from "@/types/support";

const { Title, Text } = Typography;
const { TextArea } = Input;

const STATUS_COLORS: Record<string, string> = {
  open: "blue",
  pending_player: "orange",
  pending_internal: "cyan",
  resolved: "green",
  closed: "default",
};

const PRIORITY_COLORS: Record<string, string> = {
  low: "blue",
  normal: "green",
  high: "orange",
  urgent: "red",
};

export default function TicketDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [replyBody, setReplyBody] = useState("");
  const [isInternal, setIsInternal] = useState(false);

  const { data: ticket, isLoading } = useQuery({
    queryKey: ["ticket", id],
    queryFn: () => supportService.getTicket(id!),
    enabled: !!id,
  });

  const { data: messages } = useQuery({
    queryKey: ["ticket-messages", id],
    queryFn: () => supportService.getMessages(id!),
    enabled: !!id,
  });

  const sendMessage = useMutation({
    mutationFn: (body: string) =>
      supportService.sendMessage(id!, { body, is_internal: isInternal }),
    onSuccess: () => {
      setReplyBody("");
      queryClient.invalidateQueries({ queryKey: ["ticket-messages", id] });
      queryClient.invalidateQueries({ queryKey: ["ticket", id] });
      message.success("Message sent");
    },
  });

  const updateStatus = useMutation({
    mutationFn: (status: SupportTicket["status"]) =>
      supportService.updateTicketStatus(id!, { status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ticket", id] });
      message.success("Status updated");
    },
  });

  const changePriority = useMutation({
    mutationFn: (priority: SupportTicket["priority"]) =>
      supportService.changePriority(id!, priority),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ticket", id] });
      message.success("Priority updated");
    },
  });

  const assignMutation = useMutation({
    mutationFn: (adminId: string) => supportService.assignTicket(id!, { assigned_to: adminId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ticket", id] });
      message.success("Ticket assigned");
    },
    onError: () => message.error("Failed to assign ticket"),
  });

  const adminId = useAuthStore((s) => s.adminId);

  if (isLoading || !ticket) return <div>Loading...</div>;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/support/tickets")}>
          Back to List
        </Button>
      </Space>

      <Title level={3}>{ticket.subject}</Title>

      <Descriptions column={2} bordered size="small">
        <Descriptions.Item label="Ticket ID">{ticket.id}</Descriptions.Item>
        <Descriptions.Item label="Player">
          {ticket.player_email} (@{ticket.player_username})
        </Descriptions.Item>
        <Descriptions.Item label="Category">{ticket.category.toUpperCase()}</Descriptions.Item>
        <Descriptions.Item label="Created Via">{ticket.created_via}</Descriptions.Item>
        <Descriptions.Item label="Status">
          <Select
            value={ticket.status}
            onChange={(v) => updateStatus.mutate(v)}
            style={{ width: 160 }}
            options={[
              { value: "open", label: "Open" },
              { value: "pending_player", label: "Pending Player" },
              { value: "pending_internal", label: "Pending Internal" },
              { value: "resolved", label: "Resolved" },
              { value: "closed", label: "Closed" },
            ]}
          />
        </Descriptions.Item>
        <Descriptions.Item label="Priority">
          <Select
            value={ticket.priority}
            onChange={(v) => changePriority.mutate(v)}
            style={{ width: 120 }}
            options={[
              { value: "low", label: "Low" },
              { value: "normal", label: "Normal" },
              { value: "high", label: "High" },
              { value: "urgent", label: "Urgent" },
            ]}
          />
        </Descriptions.Item>
        <Descriptions.Item label="Assigned">
          {ticket.assigned_to_name || (
            <Button
              size="small"
              icon={<UserAddOutlined />}
              loading={assignMutation.isPending}
              onClick={() => {
                if (!adminId) return;
                Modal.confirm({
                  title: "Assign to yourself?",
                  onOk: () => assignMutation.mutate(adminId),
                });
              }}
            >
              Assign
            </Button>
          )}
        </Descriptions.Item>
        <Descriptions.Item label="SLA">
          {ticket.is_sla_breach ? <Badge color="red" text="BREACH" /> : <Badge color="green" text="OK" />}
        </Descriptions.Item>
      </Descriptions>

      <Divider />

      <Card title={`Message Thread (${messages?.length || 0})`}>
        <List
          dataSource={messages || []}
          renderItem={(msg: TicketMessage) => (
            <List.Item>
              <List.Item.Meta
                avatar={
                  <Avatar
                    style={{ backgroundColor: msg.author_type === "admin" ? "#1677ff" : "#52c41a" }}
                    icon={msg.author_type === "admin" ? <UserAddOutlined /> : undefined}
                  >
                    {msg.author_name?.[0]?.toUpperCase()}
                  </Avatar>
                }
                title={
                  <Space>
                    <Text strong>{msg.author_name}</Text>
                    <Tag>{msg.author_type.toUpperCase()}</Tag>
                    {msg.is_internal && (
                      <Tag icon={<LockOutlined />} color="orange">
                        INTERNAL
                      </Tag>
                    )}
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {msg.created_at}
                    </Text>
                  </Space>
                }
                description={msg.body}
              />
            </List.Item>
          )}
        />
      </Card>

      <Card style={{ marginTop: 16 }}>
        <Space direction="vertical" style={{ width: "100%" }}>
          <TextArea
            rows={4}
            placeholder="Type your reply..."
            value={replyBody}
            onChange={(e) => setReplyBody(e.target.value)}
          />
          <Space>
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={() => sendMessage.mutate(replyBody)}
              loading={sendMessage.isPending}
              disabled={!replyBody.trim()}
            >
              Send Reply
            </Button>
            <Button
              icon={<LockOutlined />}
              type={isInternal ? "primary" : "default"}
              onClick={() => setIsInternal(!isInternal)}
            >
              {isInternal ? "Internal Note" : "Public Reply"}
            </Button>
          </Space>
        </Space>
      </Card>
    </div>
  );
}
