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
    | "admin-kb-tags"
    | "speedtest"
    | "admin-st-endpoints"
    | "admin-st-geoip"
    | "admin-st-results"
    | "admin-st-dashboards";
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
  myVote?: "up" | "down" | "";
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

export type STEndpoint = {
  id: number;
  label: string;
  url: string;
  country: string;
  region: string;
  sortOrder: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
};

export type STGeoIPSourceKind = "mmdb_auto" | "mmdb_file" | "http_api" | "request_header";

export type STGeoIPSource = {
  id: number;
  label: string;
  kind: STGeoIPSourceKind;
  config: Record<string, unknown>;
  sortOrder: number;
  active: boolean;
  lastStatus: string;
  lastUsedAt?: string | null;
  lastRefreshedAt?: string | null;
  createdAt: string;
  updatedAt: string;
};

export type STResult = {
  id: number;
  customerId: string;
  endpointId?: number | null;
  endpointLabel: string;
  autoStrategy: string;
  downloadMbps: number;
  uploadMbps: number;
  pingMs: number;
  jitterMs: number;
  clientIp?: string;
  userAgent?: string;
  ranAt: string;
};

export type STAutoResolution = {
  strategy: "latency" | "geoip" | "fallback";
  endpoint?: STEndpoint | null;
  candidates?: STEndpoint[];
  geoip: { country?: string; sourceId?: number; sourceLabel?: string };
};

export type STDashboardAggregates = {
  perEndpoint: Array<{
    endpointId?: number | null;
    label: string;
    medianDownload: number;
    medianUpload: number;
    medianPing: number;
    resultCount: number;
  }>;
  perDay: Array<{ day: string; count: number }>;
  slowTop10: STResult[];
  countryHits: Array<{ country: string; count: number }>;
};
