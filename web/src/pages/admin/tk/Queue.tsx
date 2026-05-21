import { useEffect, useState } from "react";
import { Queue } from "@/components/admin/tk/Queue";
import { Input } from "@/components/ui/input";
import { listTKAdminQueue, listTKCategoriesAdmin } from "@/api/tkAdmin";
import type { TKCategory, TKTicket } from "@/lib/types";

export function TKAdminQueue() {
  const [rows, setRows] = useState<TKTicket[]>([]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [categoryId, setCategoryId] = useState(0);
  const [assignee, setAssignee] = useState("");
  const [categories, setCategories] = useState<TKCategory[]>([]);

  useEffect(() => {
    listTKCategoriesAdmin().then(setCategories).catch(() => {});
  }, []);

  async function refresh() {
    try {
      const r = await listTKAdminQueue({
        q: q || undefined, status: status || undefined,
        categoryId: categoryId > 0 ? categoryId : undefined,
        assignee: assignee || undefined, limit: 200,
      });
      setRows(r);
    } catch {}
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, status, categoryId, assignee]);

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Tickets</h2>
      <div className="flex flex-wrap items-center gap-2">
        <Input className="max-w-sm" placeholder="Tracking# or subject…" value={q} onChange={(e) => setQ(e.target.value)} />
        <select value={status} onChange={(e) => setStatus(e.target.value)} className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All statuses</option>
          <option value="open">Open</option>
          <option value="in_progress">In progress</option>
          <option value="waiting_customer">Waiting on customer</option>
          <option value="resolved">Resolved</option>
          <option value="closed">Closed</option>
        </select>
        <select value={String(categoryId)} onChange={(e) => setCategoryId(Number(e.target.value))} className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="0">All categories</option>
          {categories.map((c) => <option key={c.id} value={String(c.id)}>{c.name}</option>)}
        </select>
        <select value={assignee} onChange={(e) => setAssignee(e.target.value)} className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All assignees</option>
          <option value="__mine__">Mine</option>
          <option value="__unassigned__">Unassigned</option>
        </select>
      </div>
      <Queue rows={rows} />
    </section>
  );
}
