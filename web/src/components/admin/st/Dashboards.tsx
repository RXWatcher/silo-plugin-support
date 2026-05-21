import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { STDashboardAggregates } from "@/lib/types";

type Props = { data: STDashboardAggregates };

export function Dashboards({ data }: Props) {
  const maxPerDay = Math.max(1, ...data.perDay.map((d) => d.count));
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>Tests per day (30d)</CardTitle></CardHeader>
        <CardContent>
          {data.perDay.length === 0
            ? <p className="text-sm text-muted-foreground">No data yet.</p>
            : <ul className="space-y-1 text-xs">
                {data.perDay.map((d) => (
                  <li key={d.day} className="flex items-center gap-2">
                    <span className="font-mono text-muted-foreground w-24">{d.day}</span>
                    <div className="h-2 flex-1 rounded-full bg-muted">
                      <div className="h-2 rounded-full bg-accent" style={{ width: `${(d.count / maxPerDay) * 100}%` }} />
                    </div>
                    <span className="w-10 text-right tabular-nums">{d.count}</span>
                  </li>
                ))}
              </ul>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Median throughput per endpoint (30d)</CardTitle></CardHeader>
        <CardContent>
          {data.perEndpoint.length === 0
            ? <p className="text-sm text-muted-foreground">No data yet.</p>
            : <table className="w-full text-sm">
                <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
                  <tr><th className="py-1">Endpoint</th><th className="py-1 text-right">↓</th><th className="py-1 text-right">↑</th><th className="py-1 text-right">Ping</th><th className="py-1 text-right">N</th></tr>
                </thead>
                <tbody>
                  {data.perEndpoint.map((e) => (
                    <tr key={e.label} className="border-t border-border">
                      <td className="py-1">{e.label}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianDownload.toFixed(1)}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianUpload.toFixed(1)}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianPing.toFixed(0)} ms</td>
                      <td className="py-1 text-right tabular-nums">{e.resultCount}</td>
                    </tr>
                  ))}
                </tbody>
              </table>}
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader><CardTitle>Slowest results (7d)</CardTitle></CardHeader>
        <CardContent>
          {data.slowTop10.length === 0
            ? <p className="text-sm text-muted-foreground">No slow results — nice.</p>
            : <table className="w-full text-sm">
                <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
                  <tr><th className="py-1">When</th><th className="py-1">Customer</th><th className="py-1">Endpoint</th><th className="py-1 text-right">↓ Mb/s</th></tr>
                </thead>
                <tbody>
                  {data.slowTop10.map((r) => (
                    <tr key={r.id} className="border-t border-border">
                      <td className="py-1 font-mono text-xs">{new Date(r.ranAt).toLocaleString()}</td>
                      <td className="py-1 font-mono text-xs">{r.customerId}</td>
                      <td className="py-1">{r.endpointLabel}</td>
                      <td className="py-1 text-right tabular-nums">{r.downloadMbps.toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>}
        </CardContent>
      </Card>
    </div>
  );
}
