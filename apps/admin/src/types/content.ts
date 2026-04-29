export interface ContentPage {
  id: string;
  slug: string;
  title: string;
  locale: string;
  status: "draft" | "published" | "archived";
  content?: string;
  created_at: string;
  updated_at: string;
}

export type CmsPage = ContentPage;
