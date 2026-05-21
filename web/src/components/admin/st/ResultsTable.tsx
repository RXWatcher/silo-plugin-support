import { Card, CardContent } from "@/components/ui/card";
import type { STResult } from "@/lib/types";

type Props = { rows: STResult[] };

export function ResultsTable({ rows }: Props) {
  if (rows.length === 0) {
    return (
      <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">
        No results match the current filters.
      </CardContent></Card>
    );
  }
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">When</th>
          <th className="py-2">Customer</th>
          <th className="py-2">Endpoint</th>
          <th className="py-2">Strategy</th>
          <th className="py-2 text-right">↓ Mb/s</th>
          <th className="py-2 text-right">↑ Mb/s</th>
          <th className="py-2 text-right">Ping</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id} className="border-t border-border">
            <td className="py-2 font-mono text-xs">{new Date(r.ranAt).toLocaleString()}</td>
            <td className="py-2 font-mono text-xs">{r.customerId}</td>
            <td className="py-2">{r.endpointLabel}</td>
            <td className="py-2 text-xs text-muted-foreground">{r.autoStrategy || "—"}</td>
            <td className="py-2 text-right tabular-nums">{r.downloadMbps.toFixed(1)}</td>
            <td className="py-2 text-right tabular-nums">{r.uploadMbps.toFixed(1)}</td>
            <td className="py-2 text-right tabular-nums">{r.pingMs.toFixed(0)} ms</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
