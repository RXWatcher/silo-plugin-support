import { useEffect, useState } from "react";
import { toast } from "sonner";

import { ArticleEditor } from "@/components/admin/kb/ArticleEditor";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  createKBArticle, getKBArticleAdmin, listKBCategories, listKBTags,
  publishKBArticle, unpublishKBArticle, updateKBArticle, type KBArticleWrite,
} from "@/api/kbAdmin";
import type { KBArticle, KBCategory, KBTagWithCount } from "@/lib/types";

export function KBAdminEdit() {
  const idMatch = window.location.pathname.match(/\/admin\/kb\/(?:new|(\d+))/);
  const id = idMatch?.[1] ? Number(idMatch[1]) : null;

  const [categories, setCategories] = useState<KBCategory[]>([]);
  const [allTags, setAllTags] = useState<KBTagWithCount[]>([]);
  const [article, setArticle] = useState<KBArticle | null>(null);
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [bodyHtml, setBodyHtml] = useState("");
  const [categoryId, setCategoryId] = useState<number>(0);
  const [tagLabels, setTagLabels] = useState<string[]>([]);
  const [publishAt, setPublishAt] = useState<string>("");

  useEffect(() => {
    listKBCategories().then(setCategories).catch(() => {});
    listKBTags().then(setAllTags).catch(() => {});
    if (id !== null) {
      getKBArticleAdmin(id).then((a) => {
        setArticle(a);
        setTitle(a.title);
        setSummary(a.summary);
        setBodyHtml(a.bodyHtml);
        setCategoryId(a.categoryId);
        setTagLabels(a.tags.map((t) => t.label));
        setPublishAt(a.publishAt ?? "");
      }).catch(() => toast.error("Could not load article"));
    }
  }, [id]);

  function buildWrite(status: "draft" | "published"): KBArticleWrite {
    return { title, summary, bodyHtml, categoryId, status, publishAt: publishAt || null, tagLabels };
  }

  async function save(status: "draft" | "published") {
    try {
      const saved = id === null
        ? await createKBArticle(buildWrite(status))
        : await updateKBArticle(id, buildWrite(status));
      setArticle(saved);
      toast.success("Saved.");
      if (id === null) window.location.assign(`./kb/${saved.id}`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function togglePublish() {
    if (!article) return;
    try {
      const updated = article.status === "draft"
        ? await publishKBArticle(article.id)
        : await unpublishKBArticle(article.id);
      setArticle(updated);
      toast.success(updated.status === "published" ? "Published." : "Reverted to draft.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Action failed"); }
  }

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">{id ? "Edit article" : "New article"}</h2>

      <Card>
        <CardHeader><CardTitle>Basics</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="title">Title</Label>
            <Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label htmlFor="category">Category</Label>
            <select id="category" value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}
                    className="w-full rounded border border-border bg-background px-2 py-1 text-sm">
              <option value={0}>— pick a category —</option>
              {categories.filter((c) => c.active).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="tags">Tags (comma-separated)</Label>
            <Input id="tags" value={tagLabels.join(", ")}
                   onChange={(e) => setTagLabels(e.target.value.split(",").map((s) => s.trim()).filter(Boolean))}
                   placeholder="beginner, video, mobile" />
            <p className="text-xs text-muted-foreground">
              Existing tags: {allTags.length === 0 ? "none yet" : allTags.map((t) => t.label).join(", ")}
            </p>
          </div>
          <div className="space-y-1">
            <Label htmlFor="summary">Summary</Label>
            <Textarea id="summary" rows={2} value={summary} onChange={(e) => setSummary(e.target.value)}
                      placeholder="One-line description shown in search results." />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Body</CardTitle></CardHeader>
        <CardContent>
          <ArticleEditor initialHTML={article?.bodyHtml ?? ""} articleId={article?.id} onChange={setBodyHtml} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Publish</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {article && (
            <p className="text-sm text-muted-foreground">
              Status: <strong>{article.status}</strong>{article.publishedAt ? ` · last published ${new Date(article.publishedAt).toLocaleString()}` : ""}
            </p>
          )}
          <div className="space-y-1">
            <Label htmlFor="schedule">Schedule (optional RFC3339)</Label>
            <Input id="schedule" placeholder="2026-06-01T09:00:00Z" value={publishAt}
                   onChange={(e) => setPublishAt(e.target.value)} />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => save("draft")}>Save draft</Button>
            <Button onClick={() => save("published")} variant="default">Save &amp; publish</Button>
            {article && (
              <Button variant="secondary" onClick={togglePublish}>
                {article.status === "draft" ? "Publish now" : "Revert to draft"}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
