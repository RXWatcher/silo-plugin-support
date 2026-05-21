import { Card, CardContent } from "@/components/ui/card";
import type { TKEntry } from "@/lib/types";

type Props = {
  entries: TKEntry[];
  isAdmin?: boolean;
};

export function Thread({ entries, isAdmin }: Props) {
  return (
    <ol className="space-y-3">
      {entries.map((e) => (
        <li key={e.id}>
          {e.kind === "system" || e.kind === "status_change" ? (
            <p className="text-xs italic text-muted-foreground text-center">
              {e.body} · {new Date(e.createdAt).toLocaleString()}
            </p>
          ) : (
            <Card className={e.kind === "internal_note" ? "border-amber-500/60 bg-amber-500/10" : ""}>
              <CardContent className="space-y-1 py-3">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-medium">
                    {e.authorRole === "admin" ? "Support team" :
                     e.authorRole === "system" ? "System" : "You"}
                  </span>
                  {e.kind === "internal_note" && isAdmin && (
                    <span className="rounded bg-amber-600 px-1.5 py-0.5 text-xs font-semibold text-white">
                      INTERNAL · admin-only
                    </span>
                  )}
                  <span className="ml-auto">{new Date(e.createdAt).toLocaleString()}</span>
                </div>
                <p className="whitespace-pre-wrap text-sm">{e.body}</p>
                {e.attachments && e.attachments.length > 0 && (
                  <ul className="mt-2 space-y-1 text-xs">
                    {e.attachments.map((a) => (
                      <li key={a.id}>
                        <a className="text-accent hover:underline" href={`/api/attachments/${a.id}`}>
                          📎 {a.filename} ({Math.round(a.bytes / 1024)} KB)
                        </a>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          )}
        </li>
      ))}
    </ol>
  );
}
