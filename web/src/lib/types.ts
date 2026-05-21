export type ModuleToggles = {
  kb: boolean;
  speedtest: boolean;
  tickets: boolean;
  ai: boolean;
};

export type SupportBootstrap = {
  mode:
    | "customer-home"
    | "admin-home"
    | "kb-browse"
    | "kb-detail"
    | "admin-kb-list"
    | "admin-kb-edit"
    | "admin-kb-categories"
    | "admin-kb-tags";
  theme: string;
  modules: ModuleToggles;
  userId: string;
  isAdmin: boolean;
};

export type PluginConfig = {
  modules: ModuleToggles;
};

export type APIError = Error & {
  responseStatus?: number;
  responseCode?: string;
};

export type KBCategory = {
  id: number;
  slug: string;
  name: string;
  sortOrder: number;
  active: boolean;
};

export type KBTag = {
  id: number;
  slug: string;
  label: string;
};

export type KBTagWithCount = KBTag & { useCount: number };

export type KBArticleSummary = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  categoryId: number;
  categoryName: string;
  status: "draft" | "published";
  publishedAt?: string;
  updatedAt: string;
  tags: string[];
};

export type KBArticle = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  bodyHtml: string;
  categoryId: number;
  status: "draft" | "published";
  publishAt?: string | null;
  publishedAt?: string | null;
  lastEditedBy: string;
  createdAt: string;
  updatedAt: string;
  tags: KBTag[];
  category?: KBCategory;
};

export type KBSearchHit = {
  article: KBArticleSummary;
  rank: number;
  snippet: string;
};

export type KBVoteAggregate = {
  helpfulCount: number;
  notHelpfulCount: number;
};

export type KBViewAggregate = {
  totalViews: number;
  uniqueViewers: number;
};

export type KBEngagement = {
  votes: KBVoteAggregate;
  views: KBViewAggregate;
};
