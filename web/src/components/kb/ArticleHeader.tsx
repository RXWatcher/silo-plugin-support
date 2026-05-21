import { Badge } from "@/components/ui/badge";
import type { KBArticle } from "@/lib/types";

type Props = { article: KBArticle };

export function ArticleHeader({ article }: Props) {
  return (
    <header className="space-y-2">
      <h1 className="text-3xl font-semibold leading-tight md:text-4xl">{article.title}</h1>
      <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        {article.category && <Badge variant="secondary">{article.category.name}</Badge>}
        {article.tags.map((t) => (
          <Badge key={t.id} variant="outline">{t.label}</Badge>
        ))}
        <span className="ml-2">Updated {humanDate(article.updatedAt)}</span>
      </div>
      {article.summary && <p className="text-muted-foreground">{article.summary}</p>}
    </header>
  );
}

function humanDate(iso: string): string {
  try { return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }); }
  catch { return ""; }
}
