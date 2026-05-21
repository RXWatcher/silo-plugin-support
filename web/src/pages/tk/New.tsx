import { useEffect, useState } from "react";
import { toast } from "sonner";

import { TopBar } from "@/components/shared/TopBar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { createTKTicket, getTKCategoriesForm } from "@/api/tk";
import type { TKCategoriesResponse, TKCategory, TKCategoryField, TKSubcategory } from "@/lib/types";

type Step = "category" | "subcategory" | "form" | "done";

export function TKNew() {
  const [data, setData] = useState<TKCategoriesResponse | null>(null);
  const [step, setStep] = useState<Step>("category");
  const [category, setCategory] = useState<TKCategory | null>(null);
  const [subcategory, setSubcategory] = useState<TKSubcategory | null>(null);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [customerEmail, setCustomerEmail] = useState("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [createdTN, setCreatedTN] = useState<string | null>(null);

  useEffect(() => {
    getTKCategoriesForm().then(setData).catch(() => toast.error("Could not load categories"));
  }, []);

  function fieldsFor(c: TKCategory | null): TKCategoryField[] {
    if (!data || !c) return [];
    return data.fields[c.id] ?? [];
  }
  function subcategoriesFor(c: TKCategory | null): TKSubcategory[] {
    if (!data || !c) return [];
    return data.subcategories[c.id] ?? [];
  }

  async function submit() {
    if (!category) return;
    setSubmitting(true);
    try {
      const t = await createTKTicket({
        categoryId: category.id,
        subcategoryId: subcategory?.id,
        subject: subject.trim(),
        body: body.trim(),
        customerEmail: customerEmail.trim(),
        fieldValues,
      });
      setCreatedTN(t.trackingNumber);
      setStep("done");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Submit failed");
    } finally {
      setSubmitting(false);
    }
  }

  if (!data) return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar eyebrow="Support" title="Open a new ticket" subtitle="Tell us what's going on." />

        {step === "category" && (
          <div className="grid gap-2 sm:grid-cols-2">
            {data.categories.filter((c) => c.active).map((c) => (
              <button key={c.id} type="button"
                      className="rounded-md border border-border bg-card p-4 text-left transition-colors hover:border-accent/60"
                      onClick={() => { setCategory(c); setStep(subcategoriesFor(c).length > 0 ? "subcategory" : "form"); }}>
                <p className="font-medium">{c.name}</p>
              </button>
            ))}
          </div>
        )}

        {step === "subcategory" && category && (
          <>
            <p className="text-sm text-muted-foreground">Category: <strong>{category.name}</strong></p>
            <div className="grid gap-2 sm:grid-cols-2">
              {subcategoriesFor(category).filter((s) => s.active).map((s) => (
                <button key={s.id} type="button"
                        className="rounded-md border border-border bg-card p-4 text-left transition-colors hover:border-accent/60"
                        onClick={() => { setSubcategory(s); setStep("form"); }}>
                  <p className="font-medium">{s.name}</p>
                </button>
              ))}
            </div>
            <Button variant="ghost" onClick={() => { setCategory(null); setStep("category"); }}>← Back</Button>
          </>
        )}

        {step === "form" && category && (
          <Card>
            <CardHeader><CardTitle>{category.name}{subcategory ? ` · ${subcategory.name}` : ""}</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="subject">Subject</Label>
                <Input id="subject" value={subject} onChange={(e) => setSubject(e.target.value)} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" value={customerEmail} onChange={(e) => setCustomerEmail(e.target.value)} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="body">Describe the problem</Label>
                <Textarea id="body" rows={5} value={body} onChange={(e) => setBody(e.target.value)} />
              </div>
              {fieldsFor(category).map((f) => (
                <div key={f.id} className="space-y-1">
                  <Label htmlFor={`f-${f.id}`}>{f.label}{f.required ? " *" : ""}</Label>
                  {f.kind === "textarea" ? (
                    <Textarea id={`f-${f.id}`} rows={3} value={fieldValues[f.key] ?? ""}
                              onChange={(e) => setFieldValues({ ...fieldValues, [f.key]: e.target.value })} />
                  ) : (
                    <Input id={`f-${f.id}`} type={f.kind === "number" ? "number" : f.kind === "url" ? "url" : "text"}
                           value={fieldValues[f.key] ?? ""}
                           onChange={(e) => setFieldValues({ ...fieldValues, [f.key]: e.target.value })} />
                  )}
                </div>
              ))}
              <div className="flex justify-between">
                <Button variant="ghost" onClick={() => setStep(subcategoriesFor(category).length > 0 ? "subcategory" : "category")}>← Back</Button>
                <Button onClick={submit} disabled={submitting || !subject.trim() || !body.trim() || !customerEmail.trim()}>
                  {submitting ? "Submitting…" : "Submit ticket"}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {step === "done" && createdTN && (
          <Card>
            <CardHeader><CardTitle>Ticket opened</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <p>Your tracking number is <strong className="font-mono">{createdTN}</strong>.</p>
              <div className="flex gap-2">
                <Button onClick={() => navigator.clipboard.writeText(createdTN)}>Copy tracking number</Button>
                <Button asChild variant="outline"><a href={`./tickets/${createdTN}`}>View ticket →</a></Button>
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </main>
  );
}
