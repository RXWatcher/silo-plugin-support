import { api, absoluteURL } from "@/lib/api";
import type {
  KBArticle, KBArticleSummary, KBCategory, KBEngagement, KBTagWithCount,
} from "@/lib/types";

export type KBArticleListAdminParams = {
  status?: "draft" | "published";
  categoryId?: number;
  tag?: string;
  q?: string;
  limit?: number;
  offset?: number;
};

export function listKBArticlesAdmin(p: KBArticleListAdminParams = {}): Promise<KBArticleSummary[]> {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.categoryId) qs.set("categoryId", String(p.categoryId));
  if (p.tag) qs.set("tag", p.tag);
  if (p.q) qs.set("q", p.q);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/kb/articles" + (qs.toString() ? `?${qs}` : "");
  return api<KBArticleSummary[]>(path);
}

export function getKBArticleAdmin(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}`);
}

export type KBArticleWrite = {
  slug?: string;
  title: string;
  summary: string;
  bodyHtml: string;
  categoryId: number;
  status: "draft" | "published";
  publishAt?: string | null;
  tagLabels: string[];
};

export function createKBArticle(w: KBArticleWrite): Promise<KBArticle> {
  return api<KBArticle>("/api/admin/kb/articles", { method: "POST", body: JSON.stringify(w) });
}

export function updateKBArticle(id: number, w: KBArticleWrite): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}`, { method: "PUT", body: JSON.stringify(w) });
}

export function deleteKBArticle(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/kb/articles/${id}`, { method: "DELETE" });
}

export function publishKBArticle(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}/publish`, { method: "POST" });
}

export function unpublishKBArticle(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}/unpublish`, { method: "POST" });
}

export function getKBEngagement(id: number): Promise<KBEngagement> {
  return api<KBEngagement>(`/api/admin/kb/articles/${id}/engagement`);
}

export function listKBCategories(): Promise<KBCategory[]> {
  return api<KBCategory[]>("/api/admin/kb/categories");
}

export function createKBCategory(name: string, sortOrder: number, slug?: string): Promise<KBCategory> {
  return api<KBCategory>("/api/admin/kb/categories", {
    method: "POST",
    body: JSON.stringify({ name, sortOrder, slug: slug ?? "", active: true }),
  });
}

export function updateKBCategory(id: number, name: string, sortOrder: number, active: boolean): Promise<KBCategory> {
  return api<KBCategory>(`/api/admin/kb/categories/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name, sortOrder, active }),
  });
}

export function deleteKBCategory(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/kb/categories/${id}`, { method: "DELETE" });
}

export function listKBTags(): Promise<KBTagWithCount[]> {
  return api<KBTagWithCount[]>("/api/admin/kb/tags");
}

export function renameKBTag(id: number, label: string) {
  return api<{ id: number; slug: string; label: string }>(`/api/admin/kb/tags/${id}`, {
    method: "PUT", body: JSON.stringify({ label }),
  });
}

export function deleteKBTag(id: number) {
  return api<{ ok: boolean }>(`/api/admin/kb/tags/${id}`, { method: "DELETE" });
}

export function mergeKBTags(fromId: number, intoId: number) {
  return api<{ ok: boolean }>("/api/admin/kb/tags/merge", {
    method: "POST", body: JSON.stringify({ fromId, intoId }),
  });
}

export async function uploadKBImage(file: File, articleId?: number): Promise<{ id: number; url: string }> {
  const fd = new FormData();
  fd.append("image", file);
  const qs = articleId ? `?articleId=${articleId}` : "";
  const res = await fetch(absoluteURL(`/api/admin/kb/images${qs}`), { method: "POST", body: fd });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? `Upload failed (${res.status})`);
  }
  return res.json();
}
