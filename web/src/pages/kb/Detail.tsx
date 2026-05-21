import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { ArticleHeader } from "@/components/kb/ArticleHeader";
import { RelatedArticles } from "@/components/kb/RelatedArticles";
import { VoteButtons } from "@/components/kb/VoteButtons";
import { TrustedHTML } from "@/components/shared/TrustedHTML";
import { getKBArticle, getKBRelated, voteKB } from "@/api/kb";
import type { KBArticle, KBArticleSummary } from "@/lib/types";

export function KBDetail() {
  const slug = decodeURIComponent(window.location.pathname.split("/kb/")[1] ?? "");
  const [article, setArticle] = useState<KBArticle | null>(null);
  const [related, setRelated] = useState<KBArticleSummary[]>([]);
  const [vote, setVote] = useState<"up" | "down" | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setError("");
    getKBArticle(slug)
      .then((a) => {
        if (!cancelled) {
          setArticle(a);
          if (a.myVote === "up" || a.myVote === "down") {
            setVote(a.myVote);
          }
        }
      })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Not found"); });
    getKBRelated(slug)
      .then((r) => { if (!cancelled) setRelated(r); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [slug]);

  async function onVote(v: "up" | "down") {
    setVote(v);
    try {
      await voteKB(slug, v);
      toast.success("Thanks for the feedback.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Vote failed");
      setVote(null);
    }
  }

  if (error) {
    return (
      <main className="mx-auto max-w-3xl px-4 py-16 text-center">
        <h1 className="mb-2 text-2xl font-semibold">Article unavailable</h1>
        <p className="text-muted-foreground">{error}</p>
        <a href="../kb" className="mt-4 inline-flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft className="h-4 w-4" /> Back to Knowledge Base
        </a>
      </main>
    );
  }

  if (!article) {
    return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;
  }

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-6 px-4 py-10 md:px-8">
        <a href="../kb" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to Knowledge Base
        </a>
        <ArticleHeader article={article} />
        <TrustedHTML html={article.bodyHtml} className="prose prose-invert max-w-none text-foreground" />
        <hr className="border-border" />
        <VoteButtons currentVote={vote} onVote={onVote} />
        <RelatedArticles related={related} />
      </div>
    </main>
  );
}
