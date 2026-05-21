import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createSTEndpoint, deleteSTEndpoint, pingSTEndpoint, updateSTEndpoint,
} from "@/api/stAdmin";
import type { STEndpoint } from "@/lib/types";

type Props = { initial: STEndpoint[] };

export function EndpointAdmin({ initial }: Props) {
  const [rows, setRows] = useState<STEndpoint[]>(initial);
  const [draft, setDraft] = useState({ label: "", url: "", country: "" });

  async function add() {
    if (!draft.label.trim() || !draft.url.trim()) return;
    try {
      const saved = await createSTEndpoint({
        label: draft.label.trim(), url: draft.url.trim(),
        country: draft.country.trim().toUpperCase(), region: "",
        sortOrder: rows.length, active: true,
      });
      setRows((r) => [...r, saved]);
      setDraft({ label: "", url: "", country: "" });
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(ep: STEndpoint) {
    try {
      const saved = await updateSTEndpoint(ep.id, {
        label: ep.label, url: ep.url, country: ep.country, region: ep.region,
        sortOrder: ep.sortOrder, active: ep.active,
      });
      setRows((rs) => rs.map((x) => x.id === saved.id ? saved : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(ep: STEndpoint) {
    if (!confirm(`Delete endpoint "${ep.label}"?`)) return;
    try {
      await deleteSTEndpoint(ep.id);
      setRows((rs) => rs.filter((x) => x.id !== ep.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function ping(ep: STEndpoint) {
    try {
      const res = await pingSTEndpoint(ep.id);
      if (res.ok) toast.success(`OK (${res.elapsed_ms}ms)`);
      else toast.error(`${res.error ?? `HTTP ${res.status}`} (${res.elapsed_ms}ms)`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Ping failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Endpoints</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-4">
          <Input placeholder="Label" value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} />
          <Input placeholder="https://librespeed.example.com" value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} />
          <Input placeholder="GB" maxLength={2} value={draft.country} onChange={(e) => setDraft({ ...draft, country: e.target.value })} />
          <Button onClick={add}>Add</Button>
        </div>
        <ul className="divide-y divide-border">
          {rows.map((ep) => (
            <li key={ep.id} className="grid grid-cols-12 items-center gap-2 py-2 text-sm">
              <Input className="col-span-3" value={ep.label}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, label: e.target.value } : x))}
                     onBlur={() => save(ep)} />
              <Input className="col-span-5 font-mono text-xs" value={ep.url}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, url: e.target.value } : x))}
                     onBlur={() => save(ep)} />
              <Input className="col-span-1" maxLength={2} value={ep.country}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, country: e.target.value.toUpperCase() } : x))}
                     onBlur={() => save(ep)} />
              <Badge variant={ep.active ? "default" : "secondary"} className="col-span-1 justify-center">
                {ep.active ? "On" : "Off"}
              </Badge>
              <Switch className="col-span-1"
                      checked={ep.active}
                      onCheckedChange={(v) => {
                        setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, active: v } : x));
                        save({ ...ep, active: v });
                      }} />
              <div className="col-span-1 flex gap-1">
                <Button size="sm" variant="ghost" onClick={() => ping(ep)}>Ping</Button>
                <Button size="sm" variant="destructive" onClick={() => remove(ep)}>×</Button>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
