import { ModuleStatusCard } from "./ModuleStatusCard";
import type { ModuleToggles } from "@/lib/types";

const ENTRIES: Array<{ title: string; key: keyof ModuleToggles; href: string }> = [
  { title: "Knowledge Base", key: "kb",        href: "./kb" },
  { title: "Speedtest",      key: "speedtest", href: "./speedtest" },
  { title: "Tickets",        key: "tickets",   href: "./tickets" },
  { title: "AI Assistance",  key: "ai",        href: "./ai" },
];

export function AdminOverview({ modules }: { modules: ModuleToggles }) {
  return (
    <section className="space-y-6">
      <h2 className="text-2xl font-semibold">Overview</h2>
      <div className="rounded-md border border-border bg-card p-4 text-sm">
        <p className="font-medium">System</p>
        <ul className="mt-1 space-y-1 text-muted-foreground">
          <li>● Plugin version 0.1.0</li>
        </ul>
      </div>
      <div>
        <p className="mb-2 text-sm font-medium">Modules</p>
        <div className="grid gap-2">
          {ENTRIES.map((e) => (
            <ModuleStatusCard
              key={e.key}
              title={e.title}
              shipped={false}
              enabled={modules[e.key]}
              manageHref={e.href}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
