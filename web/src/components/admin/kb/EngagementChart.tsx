import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { KBEngagement } from "@/lib/types";

type Props = { engagement: KBEngagement };

export function EngagementChart({ engagement }: Props) {
  const total = engagement.votes.helpfulCount + engagement.votes.notHelpfulCount;
  const helpfulPct = total === 0 ? 0 : Math.round((engagement.votes.helpfulCount / total) * 100);
  return (
    <Card>
      <CardHeader><CardTitle>Engagement (last 30 days)</CardTitle></CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-2 gap-3 text-sm">
          <Stat label="Total views" value={engagement.views.totalViews} />
          <Stat label="Unique viewers" value={engagement.views.uniqueViewers} />
          <Stat label="Helpful" value={engagement.votes.helpfulCount} />
          <Stat label="Not helpful" value={engagement.votes.notHelpfulCount} />
        </div>
        <div>
          <p className="text-xs text-muted-foreground mb-1">Helpful ratio</p>
          <div className="h-2 w-full rounded-full bg-muted">
            <div className="h-2 rounded-full bg-accent" style={{ width: `${helpfulPct}%` }} />
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{helpfulPct}%</p>
        </div>
      </CardContent>
    </Card>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      <p className="text-xl font-semibold">{value}</p>
    </div>
  );
}
