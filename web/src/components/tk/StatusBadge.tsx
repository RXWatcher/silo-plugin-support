import { Badge } from "@/components/ui/badge";
import type { TKTicket } from "@/lib/types";

const VARIANT: Record<TKTicket["status"], "default" | "secondary" | "outline" | "destructive"> = {
  open:              "default",
  in_progress:       "default",
  waiting_customer:  "secondary",
  resolved:          "secondary",
  closed:            "outline",
};

const LABEL: Record<TKTicket["status"], string> = {
  open:              "Open",
  in_progress:       "In progress",
  waiting_customer:  "Waiting on you",
  resolved:          "Resolved",
  closed:            "Closed",
};

export function StatusBadge({ status }: { status: TKTicket["status"] }) {
  return <Badge variant={VARIANT[status]}>{LABEL[status]}</Badge>;
}
