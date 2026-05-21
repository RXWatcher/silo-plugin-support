export type ModuleToggles = {
  kb: boolean;
  speedtest: boolean;
  tickets: boolean;
  ai: boolean;
};

export type SupportBootstrap = {
  mode: "customer-home" | "admin-home";
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
