import { useEffect, useState } from "react";
import { toast } from "sonner";

import { SearchBar } from "@/components/kb/SearchBar";
import { TagChips } from "@/components/kb/TagChips";
import { TopBar } from "@/components/shared/TopBar";
import { TrustedHTML } from "@/components/shared/TrustedHTML";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { listKBArticles, searchKB } from "@/api/kb";
import type { KBArticleSummary, KBSearchHit, SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function KBBrowse({ bootstrap }: Props) {
  const [tag, setTag] = useState<string>(() =>
    new URLSearchParams(window.location.search).get("tag") ?? ""
  );
  const [query, setQuery] = useState("");
  const [articles, setArticles] = useState<KBArticleSummary[]>([]);
  const [hits, setHits] = useState<KBSearchHit[] | null>(null);
  const [loading, setLoading] = useState(true);

  // Mirror tag selection to ?tag= for bookmarking.
  useEffect(() => {
    const url = new URL(window.location.href);
    if (tag) url.searchParams.set("tag", tag);
    else url.searchParams.delete("tag");
    window.history.pushState({}, "", url.toString());
  }, [tag]);

  // Browse mode (no active query): load filtered list.
  useEffect(() => {
    if (query) return;
    let cancelled = false;
    setLoading(true);
    listKBArticles({ tag: tag || undefined, limit: 100 })
      .then((rows) => { if (!cancelled) setArticles(rows); })
      .catch((err) => toast.error(err instanceof Error ? err.message : "Failed to load articles"))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [tag, query]);

  // Search mode.
  useEffect(() => {
    if (!query) { setHits(null); return; }
    let cancelled = false;
    setLoading(true);
    searchKB(query)
      .then((res) => { if (!cancelled) setHits(res); })
      .catch((err) => toast.error(err instanceof Error ? err.message : "Search failed"))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [query]);

  const allTags = Array.from(new Set(articles.flatMap((a) => a.tags))).sort();
  const groupedByCategory = groupBy(articles, (a) => a.categoryName);

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-5xl space-y-6 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Knowledge Base"
          subtitle="Search articles, FAQs, and how-tos."
        />
        <SearchBar onQuery={setQuery} />
        <TagChips tags={allTags} selected={tag} onSelect={setTag} />
        {hits !== null
          ? <SearchResults hits={hits} loading={loading} tickets={bootstrap.modules.tickets} />
          : <CategoryGroups groups={groupedByCategory} loading={loading} />}
      </div>
    </main>
  );
}

function SearchResults({ hits, loading, tickets }: { hits: KBSearchHit[]; loading: boolean; tickets: boolean }) {
  if (loading) return <p className="text-sm text-muted-foreground">Searching…</p>;
  if (hits.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
        <p className="font-medium text-foreground">No matching articles.</p>
        {tickets && (
          <p className="mt-2">
            Can't find what you need?{" "}
            <a href="../tickets/new" className="text-accent hover:underline">Open a ticket →</a>
          </p>
        )}
      </div>
    );
  }
  return (
    <ul className="grid gap-2">
      {hits.map((h) => (
        <li key={h.article.id}>
          <a href={`./kb/${encodeURIComponent(h.article.slug)}`} className="block">
            <Card className="transition-colors hover:border-accent/40">
              <CardContent className="space-y-1 py-3">
                <p className="font-medium">{h.article.title}</p>
                <TrustedHTML html={h.snippet} className="text-xs text-muted-foreground line-clamp-2" />
              </CardContent>
            </Card>
          </a>
        </li>
      ))}
    </ul>
  );
}

function CategoryGroups({ groups, loading }: { groups: Record<string, KBArticleSummary[]>; loading: boolean }) {
  if (loading) return <p className="text-sm text-muted-foreground">Loading…</p>;
  const names = Object.keys(groups).sort();
  if (names.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
        No articles published yet.
      </div>
    );
  }
  return (
    <div className="space-y-6">
      {names.map((cat) => (
        <section key={cat} className="space-y-2">
          <h2 className="text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">{cat}</h2>
          <ul className="grid gap-2">
            {groups[cat].slice(0, 6).map((a) => (
              <li key={a.id}>
                <a href={`./kb/${encodeURIComponent(a.slug)}`} className="block">
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
          {groups[cat].length > 6 && (
            <Button asChild variant="ghost" size="sm">
              <a href={`./kb?category=${encodeURIComponent(cat)}`}>See all in {cat} →</a>
            </Button>
          )}
        </section>
      ))}
    </div>
  );
}

function groupBy<T, K extends string>(items: T[], key: (t: T) => K): Record<K, T[]> {
  const out = {} as Record<K, T[]>;
  for (const item of items) {
    const k = key(item);
    if (!out[k]) out[k] = [];
    out[k].push(item);
  }
  return out;
}
