import { useEffect, useState } from "react";
import { ArticleList } from "@/components/admin/kb/ArticleList";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { listKBArticlesAdmin } from "@/api/kbAdmin";
import type { KBArticleSummary } from "@/lib/types";

export function KBAdminList() {
  const [rows, setRows] = useState<KBArticleSummary[]>([]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState<"" | "draft" | "published">("");
  useEffect(() => {
    let cancelled = false;
    listKBArticlesAdmin({ q: q || undefined, status: status || undefined, limit: 200 })
      .then((r) => { if (!cancelled) setRows(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [q, status]);
  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold">Knowledge Base</h2>
        <Button asChild><a href="./kb/new">New article</a></Button>
      </div>
      <div className="flex flex-wrap gap-2">
        <Input placeholder="Search title…" value={q} onChange={(e) => setQ(e.target.value)} className="max-w-sm" />
        <select value={status} onChange={(e) => setStatus(e.target.value as "" | "draft" | "published")}
                className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="published">Published</option>
        </select>
      </div>
      <ArticleList rows={rows} />
    </section>
  );
}
