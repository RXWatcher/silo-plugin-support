import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { deleteKBTag, mergeKBTags, renameKBTag } from "@/api/kbAdmin";
import type { KBTagWithCount } from "@/lib/types";

type Props = { initial: KBTagWithCount[] };

export function TagAdmin({ initial }: Props) {
  const [tags, setTags] = useState<KBTagWithCount[]>(initial);
  const [mergeFrom, setMergeFrom] = useState<number | "">("");
  const [mergeInto, setMergeInto] = useState<number | "">("");

  async function rename(id: number, label: string) {
    try {
      await renameKBTag(id, label);
      setTags((ts) => ts.map((t) => t.id === id ? { ...t, label } : t));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Rename failed"); }
  }

  async function remove(t: KBTagWithCount) {
    if (!confirm(`Delete tag "${t.label}"?`)) return;
    try {
      await deleteKBTag(t.id);
      setTags((ts) => ts.filter((x) => x.id !== t.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function merge() {
    if (typeof mergeFrom !== "number" || typeof mergeInto !== "number" || mergeFrom === mergeInto) return;
    try {
      await mergeKBTags(mergeFrom, mergeInto);
      toast.success("Tags merged.");
      setTags((ts) => {
        const fromTag = ts.find((t) => t.id === mergeFrom);
        const fromCount = fromTag?.useCount ?? 0;
        return ts
          .filter((t) => t.id !== mergeFrom)
          .map((t) => t.id === mergeInto ? { ...t, useCount: t.useCount + fromCount } : t);
      });
      setMergeFrom(""); setMergeInto("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Merge failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Tags</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <ul className="divide-y divide-border">
          {tags.map((t) => (
            <li key={t.id} className="flex items-center gap-2 py-2">
              <Input defaultValue={t.label} onBlur={(e) => e.target.value !== t.label && rename(t.id, e.target.value)} className="flex-1" />
              <Badge variant="secondary">{t.useCount} use{t.useCount === 1 ? "" : "s"}</Badge>
              <Button variant="destructive" size="sm" disabled={t.useCount > 0} onClick={() => remove(t)}>Delete</Button>
            </li>
          ))}
        </ul>
        <div className="rounded-md border border-border bg-card p-3 space-y-2">
          <p className="text-sm font-medium">Merge tags</p>
          <p className="text-xs text-muted-foreground">Move every article on <em>from</em> onto <em>into</em>, then delete <em>from</em>.</p>
          <div className="flex flex-wrap gap-2">
            <select className="rounded border border-border bg-background px-2 py-1 text-sm" value={mergeFrom} onChange={(e) => setMergeFrom(Number(e.target.value) || "")}>
              <option value="">from</option>
              {tags.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
            <select className="rounded border border-border bg-background px-2 py-1 text-sm" value={mergeInto} onChange={(e) => setMergeInto(Number(e.target.value) || "")}>
              <option value="">into</option>
              {tags.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
            <Button size="sm" onClick={merge} disabled={!mergeFrom || !mergeInto || mergeFrom === mergeInto}>Merge</Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
