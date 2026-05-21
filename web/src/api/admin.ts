import { api } from "@/lib/api";
import type { PluginConfig } from "@/lib/types";

export function getAdminConfig(): Promise<PluginConfig> {
  return api<PluginConfig>("/api/admin/config");
}

export function updateAdminConfig(patch: Partial<PluginConfig>): Promise<PluginConfig> {
  return api<PluginConfig>("/api/admin/config", {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}
