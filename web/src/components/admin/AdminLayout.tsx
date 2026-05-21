import { useEffect, useState } from "react";
import { toast } from "sonner";

import { AdminSidebar } from "./AdminSidebar";
import { AdminOverview } from "./AdminOverview";
import { AdminConfig } from "./AdminConfig";
import { readSectionFromURL, writeSectionToURL, type AdminSection } from "@/lib/section";
import { getAdminConfig, updateAdminConfig } from "@/api/admin";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = { modules: ModuleToggles };

export function AdminLayout({ modules: initialModules }: Props) {
  const [section, setSection] = useState<AdminSection>(() => readSectionFromURL());
  const [modules, setModules] = useState<ModuleToggles>(initialModules);

  useEffect(() => {
    const onPop = () => setSection(readSectionFromURL());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  function onSelect(next: AdminSection) {
    writeSectionToURL(next);
    setSection(next);
  }

  async function onSave(patch: Partial<PluginConfig>) {
    try {
      const fresh = await updateAdminConfig(patch);
      setModules(fresh.modules);
      toast.success("Settings saved.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save settings.");
    }
  }

  useEffect(() => {
    let cancelled = false;
    getAdminConfig()
      .then((cfg) => { if (!cancelled) setModules(cfg.modules); })
      .catch(() => { /* keep bootstrap-derived state */ });
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="flex min-h-[100dvh] bg-background text-foreground">
      <AdminSidebar current={section} modules={modules} onSelect={onSelect} />
      <main className="flex-1 px-6 py-8 md:px-10">
        {section === "overview" && <AdminOverview modules={modules} />}
        {section === "config"   && <AdminConfig modules={modules} onSave={onSave} />}
        {section === "kb"        && !modules.kb        && <ComingSoon title="Knowledge Base" />}
        {section === "speedtest" && !modules.speedtest && <ComingSoon title="Speedtest" />}
        {section === "tickets"   && !modules.tickets   && <ComingSoon title="Tickets" />}
        {section === "ai"        && !modules.ai        && <ComingSoon title="AI Assistance" />}
      </main>
    </div>
  );
}

function ComingSoon({ title }: { title: string }) {
  return (
    <section className="space-y-3">
      <h2 className="text-2xl font-semibold">{title}</h2>
      <p className="text-sm text-muted-foreground">This module hasn't shipped yet.</p>
    </section>
  );
}
