import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { TopBar } from "@/components/shared/TopBar";
import { TicketCard } from "@/components/tk/TicketCard";
import { listTKTickets } from "@/api/tk";
import type { TKTicket } from "@/lib/types";

export function TKList() {
  const [rows, setRows] = useState<TKTicket[]>([]);
  const [tab, setTab] = useState<"active" | "closed" | "all">("active");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listTKTickets({ statusGroup: tab === "all" ? undefined : tab, limit: 100 })
      .then((r) => { if (!cancelled) setRows(r); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [tab]);

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar eyebrow="Support" title="Your tickets" subtitle="Open, in-progress, and resolved tickets." />
        <div className="flex items-center gap-2">
          <Button variant={tab === "active" ? "default" : "outline"} size="sm" onClick={() => setTab("active")}>Active</Button>
          <Button variant={tab === "closed" ? "default" : "outline"} size="sm" onClick={() => setTab("closed")}>Closed</Button>
          <Button variant={tab === "all" ? "default" : "outline"} size="sm" onClick={() => setTab("all")}>All</Button>
          <Button asChild className="ml-auto"><a href="./tickets/new">Open new ticket</a></Button>
        </div>
        {loading ? <p className="text-sm text-muted-foreground">Loading…</p> :
          rows.length === 0 ? (
            <div className="rounded-md border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
              <p className="font-medium text-foreground">No tickets yet.</p>
              <p className="mt-2">When you open one, it'll show up here.</p>
              <Button asChild className="mt-4"><a href="./tickets/new">Open your first ticket</a></Button>
            </div>
          ) :
          <ul className="grid gap-2">{rows.map((t) => <li key={t.id}><TicketCard t={t} /></li>)}</ul>}
      </div>
    </main>
  );
}
