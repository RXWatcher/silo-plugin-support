import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { createKBCategory, deleteKBCategory, updateKBCategory } from "@/api/kbAdmin";
import type { KBCategory } from "@/lib/types";

type Props = { initial: KBCategory[] };

export function CategoryAdmin({ initial }: Props) {
  const [rows, setRows] = useState<KBCategory[]>(initial);
  const [newName, setNewName] = useState("");

  async function add() {
    if (!newName.trim()) return;
    try {
      const c = await createKBCategory(newName.trim(), rows.length);
      setRows((r) => [...r, c]);
      setNewName("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(c: KBCategory) {
    try {
      const updated = await updateKBCategory(c.id, c.name, c.sortOrder, c.active);
      setRows((rs) => rs.map((x) => x.id === updated.id ? updated : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(c: KBCategory) {
    if (!confirm(`Delete category "${c.name}"?`)) return;
    try {
      await deleteKBCategory(c.id);
      setRows((rs) => rs.filter((x) => x.id !== c.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Categories</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex gap-2">
          <Input placeholder="New category name" value={newName} onChange={(e) => setNewName(e.target.value)} />
          <Button onClick={add}>Add</Button>
        </div>
        <ul className="divide-y divide-border">
          {rows.map((c) => (
            <li key={c.id} className="flex items-center gap-2 py-2">
              <Input
                value={c.name}
                onChange={(e) => setRows((rs) => rs.map((x) => x.id === c.id ? { ...x, name: e.target.value } : x))}
                onBlur={() => save(c)}
                className="flex-1"
              />
              <Switch
                checked={c.active}
                onCheckedChange={(v) => {
                  setRows((rs) => rs.map((x) => x.id === c.id ? { ...x, active: v } : x));
                  save({ ...c, active: v });
                }}
              />
              <Button variant="destructive" size="sm" onClick={() => remove(c)}>Delete</Button>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
