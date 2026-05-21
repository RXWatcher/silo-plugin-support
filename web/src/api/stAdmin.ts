import { api } from "@/lib/api";
import type {
  STDashboardAggregates,
  STEndpoint,
  STGeoIPSource,
  STGeoIPSourceKind,
  STResult,
} from "@/lib/types";

export function listSTEndpointsAdmin(): Promise<STEndpoint[]> {
  return api<STEndpoint[]>("/api/admin/speedtest/endpoints");
}

export type STEndpointWrite = {
  label: string;
  url: string;
  country: string;
  region: string;
  sortOrder: number;
  active: boolean;
};

export function createSTEndpoint(w: STEndpointWrite): Promise<STEndpoint> {
  return api<STEndpoint>("/api/admin/speedtest/endpoints", {
    method: "POST", body: JSON.stringify(w),
  });
}

export function updateSTEndpoint(id: number, w: STEndpointWrite): Promise<STEndpoint> {
  return api<STEndpoint>(`/api/admin/speedtest/endpoints/${id}`, {
    method: "PUT", body: JSON.stringify(w),
  });
}

export function deleteSTEndpoint(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/endpoints/${id}`, { method: "DELETE" });
}

export function pingSTEndpoint(id: number): Promise<{ ok: boolean; status?: number; error?: string; elapsed_ms: number }> {
  return api(`/api/admin/speedtest/endpoints/${id}/ping`, { method: "POST" });
}

export function listSTGeoIPSourcesAdmin(): Promise<STGeoIPSource[]> {
  return api<STGeoIPSource[]>("/api/admin/speedtest/geoip");
}

export type STGeoIPSourceWrite = {
  label: string;
  kind: STGeoIPSourceKind;
  config: Record<string, unknown>;
  sortOrder: number;
  active: boolean;
};

export function createSTGeoIPSource(w: STGeoIPSourceWrite): Promise<STGeoIPSource> {
  return api<STGeoIPSource>("/api/admin/speedtest/geoip", {
    method: "POST", body: JSON.stringify(w),
  });
}

export function updateSTGeoIPSource(id: number, w: STGeoIPSourceWrite): Promise<STGeoIPSource> {
  return api<STGeoIPSource>(`/api/admin/speedtest/geoip/${id}`, {
    method: "PUT", body: JSON.stringify(w),
  });
}

export function deleteSTGeoIPSource(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/geoip/${id}`, { method: "DELETE" });
}

export function refreshSTGeoIPSource(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/geoip/${id}/refresh`, { method: "POST" });
}

export function testSTGeoIPSource(id: number, ip?: string): Promise<{ ip: string; country: string; error: string }> {
  return api(`/api/admin/speedtest/geoip/${id}/test`, {
    method: "POST",
    body: JSON.stringify({ ip: ip ?? "" }),
  });
}

export type STResultsListParams = {
  customerId?: string;
  endpointId?: number;
  autoStrategy?: string;
  slowOnly?: boolean;
  since?: string;
  limit?: number;
  offset?: number;
};

export function listSTResultsAdmin(p: STResultsListParams = {}): Promise<STResult[]> {
  const qs = new URLSearchParams();
  if (p.customerId) qs.set("customerId", p.customerId);
  if (p.endpointId) qs.set("endpointId", String(p.endpointId));
  if (p.autoStrategy) qs.set("autoStrategy", p.autoStrategy);
  if (p.slowOnly) qs.set("slowOnly", "true");
  if (p.since) qs.set("since", p.since);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/speedtest/results" + (qs.toString() ? `?${qs}` : "");
  return api<STResult[]>(path);
}

export function getSTDashboards(): Promise<STDashboardAggregates> {
  return api<STDashboardAggregates>("/api/admin/speedtest/dashboards");
}
