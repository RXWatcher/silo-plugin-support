import { useEffect, useState } from "react";
import { CategoryAdmin } from "@/components/admin/kb/CategoryAdmin";
import { listKBCategories } from "@/api/kbAdmin";
import type { KBCategory } from "@/lib/types";

export function KBAdminCategories() {
  const [initial, setInitial] = useState<KBCategory[] | null>(null);
  useEffect(() => { listKBCategories().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">KB Categories</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <CategoryAdmin initial={initial} />}
    </section>
  );
}
