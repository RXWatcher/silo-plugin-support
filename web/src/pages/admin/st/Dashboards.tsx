import { useEffect, useState } from "react";
import { Dashboards } from "@/components/admin/st/Dashboards";
import { getSTDashboards } from "@/api/stAdmin";
import type { STDashboardAggregates } from "@/lib/types";

export function STAdminDashboards() {
  const [data, setData] = useState<STDashboardAggregates | null>(null);
  useEffect(() => { getSTDashboards().then(setData).catch(() => setData(null)); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Dashboards</h2>
      {data === null
        ? <p className="text-sm text-muted-foreground">Loading…</p>
        : <Dashboards data={data} />}
    </section>
  );
}
