import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = {
  modules: ModuleToggles;
  onSave: (patch: Partial<PluginConfig>) => Promise<void>;
};

const ROWS: Array<{ key: keyof ModuleToggles; label: string; description: string }> = [
  { key: "kb",        label: "Knowledge Base", description: "Operator-authored articles and FAQs." },
  { key: "speedtest", label: "Speedtest",      description: "Multi-endpoint connection diagnostic." },
  { key: "tickets",   label: "Tickets",        description: "Typed support intake (bad media / billing / config)." },
  { key: "ai",        label: "AI Assistance",  description: "Suggest KB articles + auto-categorise tickets." },
];

export function ModuleTogglesPanel({ modules, onSave }: Props) {
  function toggle(key: keyof ModuleToggles, value: boolean) {
    void onSave({ modules: { ...modules, [key]: value } });
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Modules</CardTitle>
        <p className="text-sm text-muted-foreground">
          Enable a module to surface it in the customer portal and admin nav.
        </p>
      </CardHeader>
      <CardContent className="divide-y divide-border">
        {ROWS.map(({ key, label, description }) => (
          <div key={key} className="flex items-start justify-between gap-3 py-3">
            <div>
              <p className="text-sm font-medium">{label}</p>
              <p className="text-xs text-muted-foreground">{description}</p>
            </div>
            <Switch
              checked={modules[key]}
              onCheckedChange={(v) => toggle(key, Boolean(v))}
              aria-label={`Toggle ${label}`}
            />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
