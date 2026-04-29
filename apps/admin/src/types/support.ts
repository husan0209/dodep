export type TicketCategory = "payment" | "bonus" | "technical" | "account" | "kyc" | "general";
export type TicketPriority = "low" | "normal" | "high" | "urgent";
export type TicketStatus = "open" | "pending_player" | "pending_internal" | "resolved" | "closed";
export type TicketCreatedVia = "chat" | "email" | "manual";

export interface SupportTicket {
  id: string;
  player_id: string;
  player_email: string;
  player_username: string;
  player_group: string;
  subject: string;
  category: TicketCategory;
  priority: TicketPriority;
  status: TicketStatus;
  assigned_to: string | null;
  assigned_to_name: string | null;
  created_via: TicketCreatedVia;
  source_chat_id: string | null;
  sla_first_response_at: string | null;
  first_response_at: string | null;
  sla_resolve_at: string | null;
  resolved_at: string | null;
  closed_at: string | null;
  created_at: string;
  updated_at: string;
  last_message_preview: string | null;
  last_message_at: string | null;
  message_count: number;
  is_sla_breach: boolean;
}

export interface TicketFilters {
  status?: TicketStatus;
  category?: TicketCategory;
  priority?: TicketPriority;
  assigned_to?: string;
  search?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export interface TicketMessage {
  id: string;
  ticket_id: string;
  author_type: "player" | "admin";
  author_id: string;
  author_name: string;
  is_internal: boolean;
  body: string;
  attachments: TicketAttachment[];
  created_at: string;
}

export interface TicketAttachment {
  url: string;
  name: string;
  size: number;
}

export interface TicketLink {
  ticket_id: string;
  entity_type: "withdrawal" | "deposit" | "bonus" | "bet" | "chargeback";
  entity_id: string;
  entity_summary: string;
  created_at: string;
}

export interface CreateTicketPayload {
  player_id: string;
  subject: string;
  category: TicketCategory;
  priority?: TicketPriority;
  body: string;
  created_via?: TicketCreatedVia;
}

export interface SendMessagePayload {
  body: string;
  is_internal?: boolean;
  attachments?: TicketAttachment[];
}

export interface UpdateTicketStatusPayload {
  status: TicketStatus;
  reason?: string;
}

export interface AssignTicketPayload {
  assigned_to: string;
}

export interface LinkEntityPayload {
  entity_type: TicketLink["entity_type"];
  entity_id: string;
}

export interface SlaConfig {
  id: string;
  category: TicketCategory;
  first_response_minutes: number;
  resolution_minutes: number;
  active: boolean;
}

export interface TicketStats {
  total_open: number;
  total_pending_player: number;
  total_pending_internal: number;
  total_resolved_today: number;
  sla_breach_count: number;
  avg_resolution_minutes: number;
  by_category: Record<TicketCategory, number>;
  by_priority: Record<TicketPriority, number>;
}

export interface AgentWorkload {
  agent_id: string;
  agent_name: string;
  open_tickets: number;
  resolved_today: number;
  avg_resolution_minutes: number;
}

export interface SupportTeamDashboard {
  stats: TicketStats;
  agent_workloads: AgentWorkload[];
  sla_breaches: SupportTicket[];
}
