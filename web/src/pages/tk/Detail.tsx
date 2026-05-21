import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ReplyBox } from "@/components/tk/ReplyBox";
import { StatusBadge } from "@/components/tk/StatusBadge";
import { Thread } from "@/components/tk/Thread";
import { getTKTicket, reopenTKTicket, replyTKTicket } from "@/api/tk";
import type { TKEntry, TKTicket } from "@/lib/types";

export function TKDetail() {
  const tn = decodeURIComponent(window.location.pathname.split("/tickets/")[1] ?? "");
  const [ticket, setTicket] = useState<TKTicket | null>(null);
  const [entries, setEntries] = useState<TKEntry[]>([]);
  const [err, setErr] = useState("");

  async function refresh() {
    try {
      const r = await getTKTicket(tn);
      setTicket(r.ticket);
      setEntries(r.entries);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Not found");
    }
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
  }, [tn]);

  if (err) return (
    <main className="mx-auto max-w-3xl px-4 py-16 text-center">
      <h1 className="text-2xl font-semibold">Ticket unavailable</h1>
      <p className="text-muted-foreground">{err}</p>
      <a href="../tickets" className="mt-4 inline-flex items-center gap-1 text-sm text-accent">
        <ArrowLeft className="h-4 w-4" /> Back to your tickets
      </a>
    </main>
  );

  if (!ticket) return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;

  const canReply = ticket.status !== "closed";
  const canReopen = ticket.status === "resolved" && ticket.resolvedAt &&
    Date.now() - new Date(ticket.resolvedAt).getTime() < 7 * 24 * 60 * 60 * 1000;

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <a href="../tickets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to your tickets
        </a>
        <header className="space-y-1">
          <p className="font-mono text-xs text-muted-foreground">{ticket.trackingNumber}</p>
          <h1 className="text-2xl font-semibold">{ticket.subject}</h1>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <StatusBadge status={ticket.status} />
            {ticket.category && <span>{ticket.category.name}{ticket.subcategory ? ` · ${ticket.subcategory.name}` : ""}</span>}
          </div>
        </header>
        <Thread entries={entries} />
        {canReply && (
          <ReplyBox onSubmit={async (body) => {
            try {
              await replyTKTicket(tn, body);
              await refresh();
              toast.success("Reply sent.");
            } catch (e) { toast.error(e instanceof Error ? e.message : "Send failed"); }
          }} />
        )}
        {canReopen && (
          <Button variant="secondary" onClick={async () => {
            try { await reopenTKTicket(tn); await refresh(); toast.success("Ticket reopened."); }
            catch (e) { toast.error(e instanceof Error ? e.message : "Reopen failed"); }
          }}>Reopen ticket</Button>
        )}
      </div>
    </main>
  );
}
