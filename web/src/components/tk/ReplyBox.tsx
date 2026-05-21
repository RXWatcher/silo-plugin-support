import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

type Props = {
  onSubmit: (body: string) => Promise<void>;
  disabled?: boolean;
  placeholder?: string;
};

export function ReplyBox({ onSubmit, disabled, placeholder = "Write a reply…" }: Props) {
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (!body.trim() || disabled || busy) return;
    setBusy(true);
    try {
      await onSubmit(body.trim());
      setBody("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <Textarea
        rows={4}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder={placeholder}
        disabled={disabled || busy}
      />
      <div className="flex justify-end">
        <Button onClick={submit} disabled={disabled || busy || !body.trim()}>
          {busy ? "Sending…" : "Send"}
        </Button>
      </div>
    </div>
  );
}
