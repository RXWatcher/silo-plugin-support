import { SHIPPED_MODULES } from "@/lib/modules";
import type { ModuleToggles } from "@/lib/types";
import type { AdminSection } from "@/lib/section";

type Entry = {
  id: AdminSection;
  label: string;
  moduleKey?: keyof ModuleToggles;
};

const ENTRIES: ReadonlyArray<Entry> = [
  { id: "overview", label: "Overview" },
  { id: "config",   label: "Configuration" },
  { id: "kb",        label: "Knowledge Base", moduleKey: "kb" },
  { id: "speedtest", label: "Speedtest",      moduleKey: "speedtest" },
  { id: "tickets",   label: "Tickets",        moduleKey: "tickets" },
  { id: "ai",        label: "AI Assistance",   moduleKey: "ai" },
];

type Props = {
  current: AdminSection;
  modules: ModuleToggles;
  onSelect: (section: AdminSection) => void;
};

export function AdminSidebar({ current, modules, onSelect }: Props) {
  return (
    <nav className="w-56 shrink-0 border-r border-border bg-card py-4 text-sm">
      <p className="px-4 pb-3 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        Support admin
      </p>
      <ul className="space-y-0.5">
        {ENTRIES.map((entry) => {
          const isModule = entry.moduleKey !== undefined;
          // "shipped" = the binary contains this module's admin route.
          // "enabled" = the admin has the toggle on. Only shipped+enabled
          // becomes a live anchor; otherwise the entry stays in-page so
          // the user lands on a placeholder section rather than a 404.
          const shipped = isModule ? SHIPPED_MODULES[entry.moduleKey!] : true;
          const enabled = isModule ? modules[entry.moduleKey!] : true;

          if (isModule && shipped && enabled) {
            return (
              <li key={entry.id}>
                <a href={`./${entry.id}`} className="block px-4 py-2 hover:bg-accent/10">
                  {entry.label}
                </a>
              </li>
            );
          }
          if (isModule) {
            const note = shipped ? "(disabled)" : "(coming soon)";
            return (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={() => onSelect(entry.id)}
                  className={`w-full px-4 py-2 text-left ${current === entry.id ? "bg-accent/10 font-medium" : "text-muted-foreground hover:bg-accent/5"}`}
                >
                  {entry.label}
                  <span className="ml-2 text-xs text-muted-foreground">{note}</span>
                </button>
              </li>
            );
          }
          return (
            <li key={entry.id}>
              <button
                type="button"
                onClick={() => onSelect(entry.id)}
                className={`w-full px-4 py-2 text-left ${current === entry.id ? "bg-accent/10 font-medium" : "hover:bg-accent/5"}`}
              >
                {entry.label}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
