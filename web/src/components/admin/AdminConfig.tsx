import { ModuleTogglesPanel } from "./ModuleTogglesPanel";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = {
  modules: ModuleToggles;
  onSave: (patch: Partial<PluginConfig>) => Promise<void>;
};

export function AdminConfig({ modules, onSave }: Props) {
  return (
    <section className="space-y-6">
      <h2 className="text-2xl font-semibold">Configuration</h2>
      <ModuleTogglesPanel modules={modules} onSave={onSave} />
    </section>
  );
}
