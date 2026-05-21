import { ArrowRight } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

type Props = {
  title: string;
  href: string;
  enabled: boolean;
  description: string;
};

export function ModuleCard({ title, href, enabled, description }: Props) {
  if (enabled) {
    return (
      <a href={href} className="block rounded-md focus:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <Card className="transition-colors hover:border-accent/40">
          <CardContent className="space-y-2 py-5">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold">{title}</h3>
              <ArrowRight className="h-4 w-4 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">{description}</p>
          </CardContent>
        </Card>
      </a>
    );
  }
  return (
    <Card>
      <CardContent className="space-y-2 py-5">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-muted-foreground">{title}</h3>
          <Badge variant="outline">Coming soon</Badge>
        </div>
        <p className="text-sm text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  );
}
