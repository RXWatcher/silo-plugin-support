import { Card, CardContent } from "@/components/ui/card";
import { StatusBadge } from "./StatusBadge";
import type { TKTicket } from "@/lib/types";

export function TicketCard({ t }: { t: TKTicket }) {
  return (
    <a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="block">
      <Card className="transition-colors hover:border-accent/40">
        <CardContent className="space-y-1 py-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="font-mono">{t.trackingNumber}</span>
            <StatusBadge status={t.status} />
            <span className="ml-auto">{new Date(t.updatedAt).toLocaleString()}</span>
          </div>
          <p className="font-medium">{t.subject}</p>
        </CardContent>
      </Card>
    </a>
  );
}
