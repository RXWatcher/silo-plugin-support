import { useEffect, useState } from "react";
import { TagAdmin } from "@/components/admin/kb/TagAdmin";
import { listKBTags } from "@/api/kbAdmin";
import type { KBTagWithCount } from "@/lib/types";

export function KBAdminTags() {
  const [initial, setInitial] = useState<KBTagWithCount[] | null>(null);
  useEffect(() => { listKBTags().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">KB Tags</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <TagAdmin initial={initial} />}
    </section>
  );
}
