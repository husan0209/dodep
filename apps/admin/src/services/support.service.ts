import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";
import type {
  SupportTicket,
  TicketFilters,
  TicketMessage,
  CreateTicketPayload,
  SendMessagePayload,
  UpdateTicketStatusPayload,
  AssignTicketPayload,
  LinkEntityPayload,
  SlaConfig,
  TicketStats,
  SupportTeamDashboard,
} from "@/types/support";

export const supportService = {
  // Tickets
  async getTickets(filters: TicketFilters): Promise<PaginatedResponse<SupportTicket>> {
    const response = await apiClient.get<PaginatedResponse<SupportTicket>>(
      "/admin/tickets",
      { params: filters },
    );
    return response.data;
  },

  async getTicket(id: string): Promise<SupportTicket> {
    const response = await apiClient.get<ApiResponse<SupportTicket>>(
      `/admin/tickets/${id}`,
    );
    return response.data.data;
  },

  async createTicket(payload: CreateTicketPayload): Promise<SupportTicket> {
    const response = await apiClient.post<ApiResponse<SupportTicket>>(
      "/admin/tickets",
      payload,
    );
    return response.data.data;
  },

  async updateTicketStatus(
    id: string,
    payload: UpdateTicketStatusPayload,
  ): Promise<void> {
    await apiClient.put(`/admin/tickets/${id}`, payload);
  },

  async assignTicket(id: string, payload: AssignTicketPayload): Promise<void> {
    await apiClient.put(`/admin/tickets/${id}`, payload);
  },

  async changePriority(
    id: string,
    priority: SupportTicket["priority"],
  ): Promise<void> {
    await apiClient.put(`/admin/tickets/${id}`, { priority });
  },

  async linkEntity(id: string, payload: LinkEntityPayload): Promise<void> {
    await apiClient.post(`/admin/tickets/${id}/links`, payload);
  },

  // Messages
  async getMessages(ticketId: string): Promise<TicketMessage[]> {
    const response = await apiClient.get<ApiResponse<TicketMessage[]>>(
      `/admin/tickets/${ticketId}/messages`,
    );
    return response.data.data;
  },

  async sendMessage(
    ticketId: string,
    payload: SendMessagePayload,
  ): Promise<TicketMessage> {
    const response = await apiClient.post<ApiResponse<TicketMessage>>(
      `/admin/tickets/${ticketId}/messages`,
      payload,
    );
    return response.data.data;
  },

  // SLA Config
  async getSlaConfig(): Promise<SlaConfig[]> {
    const response = await apiClient.get<ApiResponse<SlaConfig[]>>(
      "/admin/tickets/sla-config",
    );
    return response.data.data;
  },

  async updateSlaConfig(
    category: string,
    payload: Partial<SlaConfig>,
  ): Promise<SlaConfig> {
    const response = await apiClient.put<ApiResponse<SlaConfig>>(
      `/admin/tickets/sla-config/${category}`,
      payload,
    );
    return response.data.data;
  },

  // Stats / Dashboard
  async getStats(): Promise<TicketStats> {
    const response = await apiClient.get<ApiResponse<TicketStats>>(
      "/admin/tickets/stats",
    );
    return response.data.data;
  },

  async getTeamDashboard(): Promise<SupportTeamDashboard> {
    const response = await apiClient.get<ApiResponse<SupportTeamDashboard>>(
      "/admin/tickets/team-dashboard",
    );
    return response.data.data;
  },
};
