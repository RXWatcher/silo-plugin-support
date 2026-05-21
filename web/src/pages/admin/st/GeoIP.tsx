import { useEffect, useState } from "react";
import { GeoIPSourceAdmin } from "@/components/admin/st/GeoIPSourceAdmin";
import { listSTGeoIPSourcesAdmin } from "@/api/stAdmin";
import type { STGeoIPSource } from "@/lib/types";

export function STAdminGeoIP() {
  const [initial, setInitial] = useState<STGeoIPSource[] | null>(null);
  useEffect(() => { listSTGeoIPSourcesAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">GeoIP sources</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <GeoIPSourceAdmin initial={initial} />}
    </section>
  );
}
