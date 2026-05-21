import type { ModuleToggles, SupportBootstrap } from "@/lib/types";

const DEFAULT_MODULES: ModuleToggles = {
  kb: false,
  speedtest: false,
  tickets: false,
  ai: false,
};

export function readBootstrap(): SupportBootstrap {
  if (typeof document === "undefined") return defaultBootstrap();
  const node = document.getElementById("support-bootstrap");
  if (!node || !node.textContent || node.textContent.includes("%SUPPORT_BOOTSTRAP%")) {
    return defaultBootstrap();
  }
  try {
    const parsed = JSON.parse(node.textContent) as Partial<SupportBootstrap>;
    return {
      mode: parsed.mode ?? "customer-home",
      theme: parsed.theme ?? "default",
      modules: { ...DEFAULT_MODULES, ...(parsed.modules ?? {}) },
      userId: parsed.userId ?? "",
      isAdmin: Boolean(parsed.isAdmin),
    };
  } catch {
    return defaultBootstrap();
  }
}

function defaultBootstrap(): SupportBootstrap {
  return { mode: "customer-home", theme: "default", modules: DEFAULT_MODULES, userId: "", isAdmin: false };
}
