import { api } from "@/lib/api";
import type {
  TKCategory, TKCategoryField, TKDetailResponse, TKEntry, TKSubcategory, TKTicket,
} from "@/lib/types";

export type TKQueueParams = {
  status?: string;
  statusGroup?: "active" | "closed";
  categoryId?: number;
  assignee?: string;
  q?: string;
  limit?: number;
  offset?: number;
};

export function listTKAdminQueue(p: TKQueueParams = {}): Promise<TKTicket[]> {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.statusGroup) qs.set("statusGroup", p.statusGroup);
  if (p.categoryId) qs.set("categoryId", String(p.categoryId));
  if (p.assignee) qs.set("assignee", p.assignee);
  if (p.q) qs.set("q", p.q);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/tickets" + (qs.toString() ? `?${qs}` : "");
  return api<TKTicket[]>(path);
}

export function getTKAdminTicket(tn: string): Promise<TKDetailResponse> {
  return api<TKDetailResponse>(`/api/admin/tickets/${encodeURIComponent(tn)}`);
}

export function replyTKAdmin(tn: string, body: string): Promise<{ entry: TKEntry; ticket: TKTicket }> {
  return api(`/api/admin/tickets/${encodeURIComponent(tn)}/reply`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function noteTKAdmin(tn: string, body: string): Promise<TKEntry> {
  return api<TKEntry>(`/api/admin/tickets/${encodeURIComponent(tn)}/note`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function statusTKAdmin(tn: string, to: string): Promise<TKTicket> {
  return api<TKTicket>(`/api/admin/tickets/${encodeURIComponent(tn)}/status`, {
    method: "POST", body: JSON.stringify({ to }),
  });
}

export function assignTKAdmin(tn: string, adminID: string | null): Promise<TKTicket> {
  return api<TKTicket>(`/api/admin/tickets/${encodeURIComponent(tn)}/assign`, {
    method: "POST", body: JSON.stringify({ adminId: adminID }),
  });
}

export function runTKCronAdmin(): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>("/api/admin/tickets/cron/run", { method: "POST" });
}

// Categories
export type TKCategoryWrite = { slug: string; name: string; sortOrder: number; active: boolean };
export function listTKCategoriesAdmin(): Promise<TKCategory[]> { return api("/api/admin/categories"); }
export function createTKCategoryAdmin(w: TKCategoryWrite) { return api<TKCategory>("/api/admin/categories", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKCategoryAdmin(id: number, w: TKCategoryWrite) { return api<TKCategory>(`/api/admin/categories/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKCategoryAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/categories/${id}`, { method: "DELETE" }); }

// Subcategories
export type TKSubcategoryWrite = { categoryId: number; slug: string; name: string; sortOrder: number; active: boolean };
export function listTKSubcategoriesAdmin(categoryID: number): Promise<TKSubcategory[]> {
  return api(`/api/admin/subcategories?categoryId=${categoryID}`);
}
export function createTKSubcategoryAdmin(w: TKSubcategoryWrite) { return api<TKSubcategory>("/api/admin/subcategories", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKSubcategoryAdmin(id: number, w: TKSubcategoryWrite) { return api<TKSubcategory>(`/api/admin/subcategories/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKSubcategoryAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/subcategories/${id}`, { method: "DELETE" }); }

// Category fields
export type TKFieldWrite = { categoryId: number; key: string; label: string; kind: TKCategoryField["kind"]; required: boolean; sortOrder: number };
export function listTKFieldsAdmin(categoryID: number): Promise<TKCategoryField[]> {
  return api(`/api/admin/category-fields?categoryId=${categoryID}`);
}
export function createTKFieldAdmin(w: TKFieldWrite) { return api<TKCategoryField>("/api/admin/category-fields", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKFieldAdmin(id: number, w: TKFieldWrite) { return api<TKCategoryField>(`/api/admin/category-fields/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKFieldAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/category-fields/${id}`, { method: "DELETE" }); }
