import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { ActionPanel } from "@/components/admin/tk/ActionPanel";
import { ReplyBox } from "@/components/tk/ReplyBox";
import { StatusBadge } from "@/components/tk/StatusBadge";
import { Thread } from "@/components/tk/Thread";
import { Card, CardContent } from "@/components/ui/card";
import { getTKAdminTicket, replyTKAdmin } from "@/api/tkAdmin";
import type { TKEntry, TKTicket } from "@/lib/types";

export function TKAdminDetail() {
  const tn = decodeURIComponent(window.location.pathname.split("/admin/tickets/")[1] ?? "");
  const [ticket, setTicket] = useState<TKTicket | null>(null);
  const [entries, setEntries] = useState<TKEntry[]>([]);
  const [err, setErr] = useState("");

  async function refresh() {
    try {
      const r = await getTKAdminTicket(tn);
      setTicket(r.ticket); setEntries(r.entries);
    } catch (e) { setErr(e instanceof Error ? e.message : "Not found"); }
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
  }, [tn]);

  if (err) return (
    <main className="mx-auto max-w-5xl px-4 py-16 text-center">
      <h1 className="text-2xl font-semibold">Ticket unavailable</h1>
      <p className="text-muted-foreground">{err}</p>
      <a href="../tickets" className="mt-4 inline-flex items-center gap-1 text-sm text-accent">
        <ArrowLeft className="h-4 w-4" /> Back to queue
      </a>
    </main>
  );
  if (!ticket) return <p className="px-4 py-8 text-sm text-muted-foreground">Loading…</p>;

  return (
    <section className="grid gap-6 md:grid-cols-3">
      <div className="space-y-5 md:col-span-2">
        <a href="../tickets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to queue
        </a>
        <header className="space-y-1">
          <p className="font-mono text-xs text-muted-foreground">{ticket.trackingNumber}</p>
          <h2 className="text-2xl font-semibold">{ticket.subject}</h2>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <StatusBadge status={ticket.status} />
            <span>{ticket.customerEmail}</span>
            {ticket.category && <span>· {ticket.category.name}{ticket.subcategory ? ` · ${ticket.subcategory.name}` : ""}</span>}
          </div>
        </header>
        {ticket.fieldValues && ticket.fieldValues.length > 0 && (
          <Card>
            <CardContent className="py-3">
              <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground mb-2">Form fields</p>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {ticket.fieldValues.map((fv) => (
                  <div key={fv.fieldId}>
                    <dt className="text-xs text-muted-foreground">{fv.fieldLabel}</dt>
                    <dd>{fv.value}</dd>
                  </div>
                ))}
              </dl>
            </CardContent>
          </Card>
        )}
        <Thread entries={entries} isAdmin />
        <ReplyBox onSubmit={async (body) => {
          try { await replyTKAdmin(tn, body); await refresh(); toast.success("Reply sent."); }
          catch (e) { toast.error(e instanceof Error ? e.message : "Send failed"); }
        }} disabled={ticket.status === "closed"} />
      </div>
      <ActionPanel ticket={ticket} onChange={refresh} />
    </section>
  );
}
