import { api } from "@/lib/api";
import type { STAutoResolution, STEndpoint, STResult } from "@/lib/types";

export function listSTEndpoints(): Promise<STEndpoint[]> {
  return api<STEndpoint[]>("/api/customer/speedtest/endpoints");
}

export function getSTAuto(): Promise<STAutoResolution> {
  return api<STAutoResolution>("/api/customer/speedtest/auto");
}

export type STSaveResultPayload = {
  endpointId?: number;
  endpointLabel: string;
  autoStrategy?: string;
  downloadMbps: number;
  uploadMbps: number;
  pingMs: number;
  jitterMs: number;
};

export function saveSTResult(p: STSaveResultPayload): Promise<STResult> {
  return api<STResult>("/api/customer/speedtest/results", {
    method: "POST",
    body: JSON.stringify(p),
  });
}

export function getSTHistory(): Promise<STResult[]> {
  return api<STResult[]>("/api/customer/speedtest/results");
}
