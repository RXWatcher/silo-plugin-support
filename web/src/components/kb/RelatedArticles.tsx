import { Card, CardContent } from "@/components/ui/card";
import type { KBArticleSummary } from "@/lib/types";

type Props = { related: KBArticleSummary[] };

export function RelatedArticles({ related }: Props) {
  if (related.length === 0) return null;
  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        Related articles
      </h2>
      <ul className="grid gap-2">
        {related.map((a) => (
          <li key={a.id}>
            <a href={`./${encodeURIComponent(a.slug)}`} className="block">
              <Card className="transition-colors hover:border-accent/40">
                <CardContent className="space-y-1 py-3">
                  <p className="font-medium">{a.title}</p>
                  {a.summary && <p className="text-xs text-muted-foreground line-clamp-2">{a.summary}</p>}
                </CardContent>
              </Card>
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
