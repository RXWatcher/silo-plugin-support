import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createSTGeoIPSource, deleteSTGeoIPSource, refreshSTGeoIPSource,
  testSTGeoIPSource, updateSTGeoIPSource,
} from "@/api/stAdmin";
import type { STGeoIPSource, STGeoIPSourceKind } from "@/lib/types";

type Props = { initial: STGeoIPSource[] };

const DEFAULT_CONFIG: Record<STGeoIPSourceKind, Record<string, unknown>> = {
  mmdb_auto:      { url_pattern: "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz", refresh_days: 25 },
  mmdb_file:      { path: "/var/lib/maxmind/GeoLite2-Country.mmdb" },
  http_api:       { url_pattern: "https://ipapi.co/{ip}/country/", format: "text" },
  request_header: { header: "CF-IPCountry" },
};

export function GeoIPSourceAdmin({ initial }: Props) {
  const [rows, setRows] = useState<STGeoIPSource[]>(initial);
  const [newKind, setNewKind] = useState<STGeoIPSourceKind>("http_api");
  const [newLabel, setNewLabel] = useState("");

  async function add() {
    if (!newLabel.trim()) return;
    try {
      const saved = await createSTGeoIPSource({
        label: newLabel.trim(), kind: newKind,
        config: DEFAULT_CONFIG[newKind],
        sortOrder: rows.length, active: true,
      });
      setRows((r) => [...r, saved]);
      setNewLabel("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(src: STGeoIPSource) {
    try {
      const saved = await updateSTGeoIPSource(src.id, {
        label: src.label, kind: src.kind, config: src.config,
        sortOrder: src.sortOrder, active: src.active,
      });
      setRows((rs) => rs.map((x) => x.id === saved.id ? saved : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(src: STGeoIPSource) {
    if (!confirm(`Delete source "${src.label}"?`)) return;
    try {
      await deleteSTGeoIPSource(src.id);
      setRows((rs) => rs.filter((x) => x.id !== src.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function refresh(src: STGeoIPSource) {
    try { await refreshSTGeoIPSource(src.id); toast.success("Refresh queued."); }
    catch (err) { toast.error(err instanceof Error ? err.message : "Refresh failed"); }
  }

  async function test(src: STGeoIPSource) {
    const ip = window.prompt("Test IP (leave empty for your own IP)", "");
    try {
      const res = await testSTGeoIPSource(src.id, ip ?? undefined);
      if (res.error) toast.error(`${res.error}`);
      else toast.success(`${res.ip} → ${res.country || "(no country)"}`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Test failed"); }
  }

  async function reorder(srcID: number, dir: -1 | 1) {
    const idx = rows.findIndex((r) => r.id === srcID);
    if (idx < 0) return;
    const other = idx + dir;
    if (other < 0 || other >= rows.length) return;
    const a = rows[idx], b = rows[other];
    const next = [...rows];
    next[idx] = { ...b, sortOrder: a.sortOrder };
    next[other] = { ...a, sortOrder: b.sortOrder };
    setRows(next);
    await Promise.all([save(next[idx]), save(next[other])]);
  }

  return (
    <Card>
      <CardHeader><CardTitle>GeoIP sources</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-xs text-muted-foreground">
          Sources are tried in order — first non-empty country wins. Use ↑/↓ to reorder.
        </p>
        <ul className="divide-y divide-border">
          {rows.map((src, i) => (
            <li key={src.id} className="grid grid-cols-12 items-center gap-2 py-2 text-sm">
              <div className="col-span-1 flex flex-col">
                <Button size="sm" variant="ghost" disabled={i === 0} onClick={() => reorder(src.id, -1)}>↑</Button>
                <Button size="sm" variant="ghost" disabled={i === rows.length - 1} onClick={() => reorder(src.id, 1)}>↓</Button>
              </div>
              <Badge variant="outline" className="col-span-2 justify-center text-xs">{src.kind}</Badge>
              <Input className="col-span-3" value={src.label}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, label: e.target.value } : x))}
                     onBlur={() => save(src)} />
              <Input className="col-span-3 font-mono text-xs"
                     value={JSON.stringify(src.config)}
                     onChange={(e) => {
                       try {
                         const parsed = JSON.parse(e.target.value);
                         setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, config: parsed } : x));
                       } catch { /* keep typing — apply on blur */ }
                     }}
                     onBlur={() => save(src)} />
              <Switch className="col-span-1"
                      checked={src.active}
                      onCheckedChange={(v) => {
                        setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, active: v } : x));
                        save({ ...src, active: v });
                      }} />
              <div className="col-span-2 flex gap-1">
                <Button size="sm" variant="ghost" onClick={() => test(src)}>Test</Button>
                {src.kind === "mmdb_auto" && <Button size="sm" variant="ghost" onClick={() => refresh(src)}>↻</Button>}
                <Button size="sm" variant="destructive" onClick={() => remove(src)}>×</Button>
              </div>
              <div className="col-span-12 text-xs text-muted-foreground">
                {src.lastStatus || "—"}
                {src.lastUsedAt ? ` · used ${new Date(src.lastUsedAt).toLocaleString()}` : ""}
                {src.lastRefreshedAt ? ` · refreshed ${new Date(src.lastRefreshedAt).toLocaleString()}` : ""}
              </div>
            </li>
          ))}
        </ul>
        <div className="rounded-md border border-border bg-card p-3 space-y-2">
          <p className="text-sm font-medium">Add a source</p>
          <div className="flex flex-wrap gap-2">
            <Input placeholder="Label" value={newLabel} onChange={(e) => setNewLabel(e.target.value)} className="max-w-sm" />
            <select className="rounded border border-border bg-background px-2 py-1 text-sm"
                    value={newKind} onChange={(e) => setNewKind(e.target.value as STGeoIPSourceKind)}>
              <option value="mmdb_auto">mmdb_auto</option>
              <option value="mmdb_file">mmdb_file</option>
              <option value="http_api">http_api</option>
              <option value="request_header">request_header</option>
            </select>
            <Button onClick={add}>Add</Button>
          </div>
          <p className="text-xs text-muted-foreground">Default config is filled in based on kind; edit inline after creating.</p>
        </div>
      </CardContent>
    </Card>
  );
}
