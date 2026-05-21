import { api, absoluteURL } from "@/lib/api";
import type {
  TKCategoriesResponse, TKDetailResponse, TKEntry, TKTicket,
} from "@/lib/types";

export type TKListParams = {
  statusGroup?: "active" | "closed";
  limit?: number;
  offset?: number;
};

export function listTKTickets(p: TKListParams = {}): Promise<TKTicket[]> {
  const qs = new URLSearchParams();
  if (p.statusGroup) qs.set("statusGroup", p.statusGroup);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/customer/tickets" + (qs.toString() ? `?${qs}` : "");
  return api<TKTicket[]>(path);
}

export function getTKTicket(tn: string): Promise<TKDetailResponse> {
  return api<TKDetailResponse>(`/api/customer/tickets/${encodeURIComponent(tn)}`);
}

export function getTKCategoriesForm(): Promise<TKCategoriesResponse> {
  return api<TKCategoriesResponse>("/api/customer/categories");
}

export type TKCreateRequest = {
  categoryId: number;
  subcategoryId?: number;
  subject: string;
  body: string;
  fieldValues?: Record<string, string>;
  customerEmail: string;
};

export function createTKTicket(req: TKCreateRequest): Promise<TKTicket> {
  return api<TKTicket>("/api/customer/tickets", {
    method: "POST", body: JSON.stringify(req),
  });
}

export function replyTKTicket(tn: string, body: string): Promise<{ entry: TKEntry; ticket: TKTicket }> {
  return api(`/api/customer/tickets/${encodeURIComponent(tn)}/reply`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function reopenTKTicket(tn: string): Promise<TKTicket> {
  return api<TKTicket>(`/api/customer/tickets/${encodeURIComponent(tn)}/reopen`, { method: "POST" });
}

export async function uploadTKAttachment(entryID: number, file: File): Promise<{ id: number; url: string }> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(absoluteURL(`/api/tickets/entries/${entryID}/attachments`), {
    method: "POST", body: fd,
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? `Upload failed (${res.status})`);
  }
  const meta = await res.json();
  return { id: meta.id, url: `/api/attachments/${meta.id}` };
}
