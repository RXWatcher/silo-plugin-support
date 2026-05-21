import { useEffect, useState } from "react";
import { EndpointAdmin } from "@/components/admin/st/EndpointAdmin";
import { listSTEndpointsAdmin } from "@/api/stAdmin";
import type { STEndpoint } from "@/lib/types";

export function STAdminEndpoints() {
  const [initial, setInitial] = useState<STEndpoint[] | null>(null);
  useEffect(() => { listSTEndpointsAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Speedtest endpoints</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <EndpointAdmin initial={initial} />}
    </section>
  );
}
