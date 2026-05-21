import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { assignTKAdmin, noteTKAdmin, statusTKAdmin } from "@/api/tkAdmin";
import type { TKTicket } from "@/lib/types";

type Props = {
  ticket: TKTicket;
  onChange: () => Promise<void>;
};

export function ActionPanel({ ticket, onChange }: Props) {
  const [note, setNote] = useState("");
  const [assignee, setAssignee] = useState(ticket.assignedAdminId ?? "");

  async function postStatus(to: string) {
    try { await statusTKAdmin(ticket.trackingNumber, to); await onChange(); toast.success(`Status → ${to}`); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Status change failed"); }
  }

  async function postAssign() {
    const trimmed = assignee.trim();
    try {
      await assignTKAdmin(ticket.trackingNumber, trimmed === "" ? null : trimmed);
      await onChange();
      toast.success(trimmed === "" ? "Unassigned." : `Assigned to ${trimmed}`);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Assign failed"); }
  }

  async function postNote() {
    if (!note.trim()) return;
    try { await noteTKAdmin(ticket.trackingNumber, note.trim()); setNote(""); await onChange(); toast.success("Internal note added."); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Note failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Actions</CardTitle></CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Status</p>
          <div className="flex flex-wrap gap-1">
            {(["in_progress","waiting_customer","resolved","closed"] as const).map((s) => (
              <Button key={s} size="sm" variant={ticket.status === s ? "default" : "outline"} onClick={() => postStatus(s)}>{s}</Button>
            ))}
          </div>
        </div>
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Assignee</p>
          <div className="flex gap-1">
            <Input value={assignee} onChange={(e) => setAssignee(e.target.value)} placeholder="admin id (empty = unassigned)" />
            <Button size="sm" onClick={postAssign}>Set</Button>
          </div>
        </div>
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Internal note</p>
          <Textarea rows={3} value={note} onChange={(e) => setNote(e.target.value)} placeholder="Visible to admins only" />
          <Button size="sm" onClick={postNote} disabled={!note.trim()}>Add note</Button>
        </div>
      </CardContent>
    </Card>
  );
}
