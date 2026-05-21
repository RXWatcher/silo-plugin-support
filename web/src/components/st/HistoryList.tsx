import { Card, CardContent } from "@/components/ui/card";
import type { STResult } from "@/lib/types";

type Props = { history: STResult[] };

export function HistoryList({ history }: Props) {
  if (history.length === 0) {
    return (
      <Card>
        <CardContent className="py-6 text-center text-sm text-muted-foreground">
          No tests yet — run one above to start your history.
        </CardContent>
      </Card>
    );
  }
  return (
    <ul className="divide-y divide-border rounded-md border border-border">
      {history.slice(0, 5).map((r) => (
        <li key={r.id} className="flex items-center gap-3 px-3 py-2 text-sm">
          <span className="font-mono text-xs text-muted-foreground tabular-nums">
            {new Date(r.ranAt).toLocaleString()}
          </span>
          <span className="flex-1 font-medium">{r.endpointLabel}</span>
          <span className="tabular-nums">↓ {r.downloadMbps.toFixed(1)}</span>
          <span className="tabular-nums">↑ {r.uploadMbps.toFixed(1)}</span>
          <span className="tabular-nums">{r.pingMs.toFixed(0)} ms</span>
        </li>
      ))}
    </ul>
  );
}
