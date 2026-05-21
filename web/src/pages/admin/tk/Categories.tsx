import { useEffect, useState } from "react";
import { CategoryAdmin } from "@/components/admin/tk/CategoryAdmin";
import { listTKCategoriesAdmin } from "@/api/tkAdmin";
import type { TKCategory } from "@/lib/types";

export function TKAdminCategories() {
  const [initial, setInitial] = useState<TKCategory[] | null>(null);
  useEffect(() => { listTKCategoriesAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Ticket categories</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <CategoryAdmin initial={initial} />}
    </section>
  );
}
