import { ArrowRight } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

type Props = {
  title: string;
  shipped: boolean;
  enabled: boolean;
  manageHref: string;
};

export function ModuleStatusCard({ title, shipped, enabled, manageHref }: Props) {
  const stateLabel = !shipped ? "not shipped" : enabled ? "enabled" : "disabled";
  const stateVariant: "outline" | "default" | "secondary" =
    !shipped ? "outline" : enabled ? "default" : "secondary";

  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-4 py-4">
        <div>
          <p className="font-semibold">{title}</p>
          <p className="text-xs text-muted-foreground capitalize">{stateLabel}</p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={stateVariant}>{stateLabel}</Badge>
          {shipped && enabled && (
            <a
              href={manageHref}
              className="inline-flex items-center gap-1 text-sm font-medium text-accent hover:underline"
            >
              Manage <ArrowRight className="h-3.5 w-3.5" />
            </a>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
