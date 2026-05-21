import { api } from "@/lib/api";
import type { KBArticle, KBArticleSummary, KBSearchHit } from "@/lib/types";

export type KBListParams = {
  categoryId?: number;
  tag?: string;
  limit?: number;
  offset?: number;
};

export function listKBArticles(p: KBListParams = {}): Promise<KBArticleSummary[]> {
  const qs = new URLSearchParams();
  if (p.categoryId) qs.set("category", String(p.categoryId));
  if (p.tag) qs.set("tag", p.tag);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/customer/kb/articles" + (qs.toString() ? `?${qs}` : "");
  return api<KBArticleSummary[]>(path);
}

export function getKBArticle(slug: string): Promise<KBArticle> {
  return api<KBArticle>(`/api/customer/kb/articles/${encodeURIComponent(slug)}`);
}

export function getKBRelated(slug: string): Promise<KBArticleSummary[]> {
  return api<KBArticleSummary[]>(`/api/customer/kb/related/${encodeURIComponent(slug)}`);
}

export function searchKB(q: string): Promise<KBSearchHit[]> {
  return api<KBSearchHit[]>(`/api/customer/kb/search?q=${encodeURIComponent(q)}`);
}

export function voteKB(slug: string, vote: "up" | "down"): Promise<{ vote: string }> {
  return api<{ vote: string }>(`/api/customer/kb/articles/${encodeURIComponent(slug)}/vote`, {
    method: "POST",
    body: JSON.stringify({ vote }),
  });
}
