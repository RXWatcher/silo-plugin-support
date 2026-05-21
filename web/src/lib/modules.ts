import type { ModuleToggles } from "@/lib/types";

// SHIPPED_MODULES is a binary-time constant: true iff this release of
// the plugin contains the module's code and admin routes. Distinct from
// the runtime ModuleToggles, which carries the admin's enable/disable
// choice for each shipped module. The shell ships with zero modules;
// each module's release flips its own key to true in this file.
//
// Rendering rule:
//   shipped && enabled  -> live link
//   shipped && !enabled -> "disabled" badge, no link
//   !shipped            -> "coming soon" badge, regardless of enabled
export const SHIPPED_MODULES: ModuleToggles = {
  kb: true,
  speedtest: true,
  tickets: false,
  ai: false,
};
