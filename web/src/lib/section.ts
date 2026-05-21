export type AdminSection = "overview" | "config" | "kb" | "speedtest" | "tickets" | "ai";

const KNOWN: ReadonlyArray<AdminSection> = ["overview", "config", "kb", "speedtest", "tickets", "ai"];

export function readSectionFromURL(): AdminSection {
  if (typeof window === "undefined") return "overview";
  const raw = new URLSearchParams(window.location.search).get("section") ?? "";
  return (KNOWN as readonly string[]).includes(raw) ? (raw as AdminSection) : "overview";
}

export function writeSectionToURL(section: AdminSection): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (section === "overview") {
    url.searchParams.delete("section");
  } else {
    url.searchParams.set("section", section);
  }
  window.history.pushState({}, "", url.toString());
}
