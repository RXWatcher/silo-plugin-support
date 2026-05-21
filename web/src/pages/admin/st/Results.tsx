import { useEffect, useState } from "react";
import { ResultsTable } from "@/components/admin/st/ResultsTable";
import { Input } from "@/components/ui/input";
import { listSTResultsAdmin } from "@/api/stAdmin";
import type { STResult } from "@/lib/types";

export function STAdminResults() {
  const [rows, setRows] = useState<STResult[]>([]);
  const [customerId, setCustomerId] = useState("");
  const [slowOnly, setSlowOnly] = useState(false);

  useEffect(() => {
    let cancelled = false;
    listSTResultsAdmin({ customerId: customerId || undefined, slowOnly, limit: 200 })
      .then((r) => { if (!cancelled) setRows(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [customerId, slowOnly]);

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Speedtest results</h2>
      <div className="flex flex-wrap gap-2 items-center">
        <Input placeholder="Customer id filter…" value={customerId}
               onChange={(e) => setCustomerId(e.target.value)} className="max-w-sm" />
        <label className="text-sm flex items-center gap-1">
          <input type="checkbox" checked={slowOnly} onChange={(e) => setSlowOnly(e.target.checked)} />
          Slow only
        </label>
      </div>
      <ResultsTable rows={rows} />
    </section>
  );
}
