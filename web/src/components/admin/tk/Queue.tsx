import { Card, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/tk/StatusBadge";
import type { TKTicket } from "@/lib/types";

export function Queue({ rows }: { rows: TKTicket[] }) {
  if (rows.length === 0) return (
    <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">
      No tickets match the current filters.
    </CardContent></Card>
  );
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">Tracking</th>
          <th className="py-2">Subject</th>
          <th className="py-2">Customer</th>
          <th className="py-2">Category</th>
          <th className="py-2">Status</th>
          <th className="py-2">Assignee</th>
          <th className="py-2">Updated</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((t) => (
          <tr key={t.id} className="border-t border-border">
            <td className="py-2 font-mono text-xs">
              <a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="hover:underline">{t.trackingNumber}</a>
            </td>
            <td className="py-2"><a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="hover:underline">{t.subject}</a></td>
            <td className="py-2 text-xs text-muted-foreground">{t.customerEmail}</td>
            <td className="py-2 text-xs">{t.category?.name ?? `#${t.categoryId}`}</td>
            <td className="py-2"><StatusBadge status={t.status} /></td>
            <td className="py-2 font-mono text-xs">{t.assignedAdminId ?? "—"}</td>
            <td className="py-2 text-xs text-muted-foreground">{new Date(t.updatedAt).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
