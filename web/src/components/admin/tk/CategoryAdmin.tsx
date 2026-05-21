import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createTKCategoryAdmin, createTKFieldAdmin, createTKSubcategoryAdmin,
  deleteTKCategoryAdmin, deleteTKFieldAdmin, deleteTKSubcategoryAdmin,
  listTKFieldsAdmin, listTKSubcategoriesAdmin, updateTKCategoryAdmin,
  updateTKFieldAdmin, updateTKSubcategoryAdmin,
} from "@/api/tkAdmin";
import type { TKCategory, TKCategoryField, TKSubcategory } from "@/lib/types";

type Props = { initial: TKCategory[] };

export function CategoryAdmin({ initial }: Props) {
  const [cats, setCats] = useState<TKCategory[]>(initial);
  const [selected, setSelected] = useState<TKCategory | null>(initial[0] ?? null);
  const [subs, setSubs] = useState<TKSubcategory[]>([]);
  const [fields, setFields] = useState<TKCategoryField[]>([]);

  useEffect(() => {
    if (!selected) { setSubs([]); setFields([]); return; }
    listTKSubcategoriesAdmin(selected.id).then(setSubs).catch(() => {});
    listTKFieldsAdmin(selected.id).then(setFields).catch(() => {});
  }, [selected]);

  async function addCategory() {
    const name = window.prompt("Category name?")?.trim();
    if (!name) return;
    const slug = window.prompt("Slug?", name.toLowerCase().replace(/\s+/g, "-"))?.trim() ?? "";
    if (!slug) return;
    try {
      const c = await createTKCategoryAdmin({ slug, name, sortOrder: cats.length, active: true });
      setCats((rs) => [...rs, c]); setSelected(c);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function saveCategory(c: TKCategory) {
    try {
      const u = await updateTKCategoryAdmin(c.id, { slug: c.slug, name: c.name, sortOrder: c.sortOrder, active: c.active });
      setCats((rs) => rs.map((x) => x.id === u.id ? u : x));
      if (selected?.id === u.id) setSelected(u);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Save failed"); }
  }

  async function removeCategory(c: TKCategory) {
    if (!confirm(`Delete category "${c.name}"?`)) return;
    try {
      await deleteTKCategoryAdmin(c.id);
      setCats((rs) => rs.filter((x) => x.id !== c.id));
      if (selected?.id === c.id) setSelected(null);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  async function addSubcategory() {
    if (!selected) return;
    const name = window.prompt("Subcategory name?")?.trim(); if (!name) return;
    const slug = window.prompt("Slug?", name.toLowerCase().replace(/\s+/g, "-"))?.trim(); if (!slug) return;
    try {
      const s = await createTKSubcategoryAdmin({ categoryId: selected.id, slug, name, sortOrder: subs.length, active: true });
      setSubs((rs) => [...rs, s]);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function removeSubcategory(s: TKSubcategory) {
    if (!confirm(`Delete subcategory "${s.name}"?`)) return;
    try { await deleteTKSubcategoryAdmin(s.id); setSubs((rs) => rs.filter((x) => x.id !== s.id)); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  async function addField() {
    if (!selected) return;
    const key = window.prompt("Field key (e.g. order_id)?")?.trim(); if (!key) return;
    const label = window.prompt("Label?", key)?.trim() ?? key;
    const kind = (window.prompt("Kind: text / textarea / number / url", "text") ?? "text").trim();
    if (!["text","textarea","number","url"].includes(kind)) { toast.error("Invalid kind"); return; }
    try {
      const f = await createTKFieldAdmin({
        categoryId: selected.id, key, label,
        kind: kind as TKCategoryField["kind"],
        required: false, sortOrder: fields.length,
      });
      setFields((rs) => [...rs, f]);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function removeField(f: TKCategoryField) {
    if (!confirm(`Delete field "${f.label}"?`)) return;
    try { await deleteTKFieldAdmin(f.id); setFields((rs) => rs.filter((x) => x.id !== f.id)); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  return (
    <div className="grid gap-4 md:grid-cols-3">
      <Card>
        <CardHeader><CardTitle>Categories</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <ul className="space-y-1">
            {cats.map((c) => (
              <li key={c.id} className={`flex items-center gap-2 rounded px-2 py-1 text-sm ${selected?.id === c.id ? "bg-accent/20" : ""}`}>
                <button onClick={() => setSelected(c)} className="flex-1 text-left">{c.name}</button>
                <Switch checked={c.active} onCheckedChange={(v) => saveCategory({ ...c, active: v })} />
                <Button size="sm" variant="destructive" onClick={() => removeCategory(c)}>×</Button>
              </li>
            ))}
          </ul>
          <Button size="sm" onClick={addCategory}>+ Add category</Button>
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader><CardTitle>{selected ? `${selected.name} — details` : "Pick a category"}</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          {selected && (
            <>
              <div className="space-y-1">
                <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Rename</p>
                <div className="flex gap-2">
                  <Input value={selected.name} onChange={(e) => setSelected({ ...selected, name: e.target.value })}
                         onBlur={() => saveCategory(selected)} />
                </div>
              </div>

              <div>
                <p className="mb-1 text-xs uppercase tracking-[0.08em] text-muted-foreground">Subcategories</p>
                <ul className="space-y-1 text-sm">
                  {subs.map((s) => (
                    <li key={s.id} className="flex items-center gap-2">
                      <Input value={s.name} onChange={(e) => setSubs((rs) => rs.map((x) => x.id === s.id ? { ...x, name: e.target.value } : x))}
                             onBlur={() => updateTKSubcategoryAdmin(s.id, { categoryId: s.categoryId, slug: s.slug, name: s.name, sortOrder: s.sortOrder, active: s.active })}
                             className="flex-1" />
                      <Button size="sm" variant="destructive" onClick={() => removeSubcategory(s)}>×</Button>
                    </li>
                  ))}
                </ul>
                <Button size="sm" className="mt-2" onClick={addSubcategory}>+ Add subcategory</Button>
              </div>

              <div>
                <p className="mb-1 text-xs uppercase tracking-[0.08em] text-muted-foreground">Form fields</p>
                <ul className="space-y-1 text-sm">
                  {fields.map((f) => (
                    <li key={f.id} className="grid grid-cols-12 items-center gap-2">
                      <span className="col-span-2 font-mono text-xs">{f.key}</span>
                      <Input className="col-span-4" value={f.label}
                             onChange={(e) => setFields((rs) => rs.map((x) => x.id === f.id ? { ...x, label: e.target.value } : x))}
                             onBlur={() => updateTKFieldAdmin(f.id, { categoryId: f.categoryId, key: f.key, label: f.label, kind: f.kind, required: f.required, sortOrder: f.sortOrder })} />
                      <select className="col-span-2 rounded border border-border bg-background px-2 py-1 text-sm"
                              value={f.kind}
                              onChange={(e) => {
                                const next = { ...f, kind: e.target.value as TKCategoryField["kind"] };
                                setFields((rs) => rs.map((x) => x.id === f.id ? next : x));
                                updateTKFieldAdmin(f.id, { categoryId: next.categoryId, key: next.key, label: next.label, kind: next.kind, required: next.required, sortOrder: next.sortOrder });
                              }}>
                        <option value="text">text</option>
                        <option value="textarea">textarea</option>
                        <option value="number">number</option>
                        <option value="url">url</option>
                      </select>
                      <label className="col-span-2 text-xs"><input type="checkbox" checked={f.required}
                        onChange={(e) => {
                          const next = { ...f, required: e.target.checked };
                          setFields((rs) => rs.map((x) => x.id === f.id ? next : x));
                          updateTKFieldAdmin(f.id, { categoryId: next.categoryId, key: next.key, label: next.label, kind: next.kind, required: next.required, sortOrder: next.sortOrder });
                        }} /> required</label>
                      <Button size="sm" variant="destructive" className="col-span-2" onClick={() => removeField(f)}>×</Button>
                    </li>
                  ))}
                </ul>
                <Button size="sm" className="mt-2" onClick={addField}>+ Add field</Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
