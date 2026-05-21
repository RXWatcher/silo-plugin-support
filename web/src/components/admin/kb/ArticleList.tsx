import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { KBArticleSummary } from "@/lib/types";

type Props = { rows: KBArticleSummary[] };

export function ArticleList({ rows }: Props) {
  if (rows.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          No articles match the current filters.
        </CardContent>
      </Card>
    );
  }
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">Title</th>
          <th className="py-2">Category</th>
          <th className="py-2">Tags</th>
          <th className="py-2">Status</th>
          <th className="py-2">Updated</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id} className="border-t border-border">
            <td className="py-2"><a href={`./kb/${r.id}`} className="font-medium hover:underline">{r.title}</a></td>
            <td className="py-2">{r.categoryName}</td>
            <td className="py-2">
              <div className="flex flex-wrap gap-1">
                {r.tags.map((t) => <Badge key={t} variant="outline">{t}</Badge>)}
              </div>
            </td>
            <td className="py-2"><Badge variant={r.status === "published" ? "default" : "secondary"}>{r.status}</Badge></td>
            <td className="py-2 text-muted-foreground">{humanDate(r.updatedAt)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function humanDate(iso: string): string {
  try { return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }); }
  catch { return ""; }
}
