import apiClient from "./api";
import type { ApiResponse, PaginatedResponse } from "@/types/api";

export interface ContentPage {
  id: string;
  slug: string;
  title: string;
  body: string;
  status: "draft" | "published";
  locale: string;
  updated_at: string;
}

export interface Promotion {
  id: string;
  title: string;
  description: string;
  image_url: string;
  start_date: string;
  end_date: string;
  status: "active" | "scheduled" | "expired" | "draft";
  target_countries: string[];
}

export const contentService = {
  async getPages(params?: {
    status?: string;
    search?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<ContentPage>> {
    const response = await apiClient.get<PaginatedResponse<ContentPage>>(
      "/admin/content/pages",
      { params },
    );
    return response.data;
  },

  async getPage(pageId: string): Promise<ContentPage> {
    const response = await apiClient.get<ApiResponse<ContentPage>>(
      `/admin/content/pages/${pageId}`,
    );
    return response.data.data;
  },

  async createPage(data: Partial<ContentPage>): Promise<ContentPage> {
    const response = await apiClient.post<ApiResponse<ContentPage>>(
      "/admin/content/pages",
      data,
    );
    return response.data.data;
  },

  async updatePage(
    pageId: string,
    data: Partial<ContentPage>,
  ): Promise<ContentPage> {
    const response = await apiClient.put<ApiResponse<ContentPage>>(
      `/admin/content/pages/${pageId}`,
      data,
    );
    return response.data.data;
  },

  async getPromotions(params?: {
    status?: string;
    page?: number;
    page_size?: number;
  }): Promise<PaginatedResponse<Promotion>> {
    const response = await apiClient.get<PaginatedResponse<Promotion>>(
      "/admin/content/promotions",
      { params },
    );
    return response.data;
  },

  async createPromotion(data: Partial<Promotion>): Promise<Promotion> {
    const response = await apiClient.post<ApiResponse<Promotion>>(
      "/admin/content/promotions",
      data,
    );
    return response.data.data;
  },

  async updatePromotion(
    promotionId: string,
    data: Partial<Promotion>,
  ): Promise<Promotion> {
    const response = await apiClient.put<ApiResponse<Promotion>>(
      `/admin/content/promotions/${promotionId}`,
      data,
    );
    return response.data.data;
  },

  async getTemplates(params?: { search?: string; channel?: string }): Promise<unknown[]> {
    const response = await apiClient.get<ApiResponse<unknown[]>>('/admin/content/templates', { params });
    return response.data.data;
  },

  async createTemplate(data: unknown): Promise<unknown> {
    const response = await apiClient.post<ApiResponse<unknown>>('/admin/content/templates', data);
    return response.data.data;
  },

  async updateTemplate(templateId: string, data: unknown): Promise<unknown> {
    const response = await apiClient.put<ApiResponse<unknown>>(`/admin/content/templates/${templateId}`, data);
    return response.data.data;
  },

  async getCampaigns(params?: { status?: string; search?: string }): Promise<Promotion[]> {
    const response = await apiClient.get<ApiResponse<Promotion[]>>("/admin/content/campaigns", { params });
    return response.data.data;
  },

  async createCampaign(data: Partial<Promotion>): Promise<Promotion> {
    const response = await apiClient.post<ApiResponse<Promotion>>("/admin/content/campaigns", data);
    return response.data.data;
  },

  async updateCampaign(campaignId: string, data: Partial<Promotion>): Promise<Promotion> {
    const response = await apiClient.put<ApiResponse<Promotion>>(`/admin/content/campaigns/${campaignId}`, data);
    return response.data.data;
  },

  async launchCampaign(campaignId: string): Promise<void> {
    await apiClient.post(`/admin/content/campaigns/${campaignId}/launch`);
  },
};
