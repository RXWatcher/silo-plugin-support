import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readSectionFromURL, writeSectionToURL, type AdminSection } from "./section";

describe("section URL helpers", () => {
  const pushStateSpy = vi.fn();

  beforeEach(() => {
    vi.spyOn(window.history, "pushState").mockImplementation(pushStateSpy);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    pushStateSpy.mockReset();
  });

  it("defaults to overview when ?section= is missing", () => {
    vi.stubGlobal("location", { search: "" });
    expect(readSectionFromURL()).toBe("overview");
  });

  it("returns the section from the query string when valid", () => {
    vi.stubGlobal("location", { search: "?section=config" });
    expect(readSectionFromURL()).toBe("config");
  });

  it("falls back to overview for unknown sections", () => {
    vi.stubGlobal("location", { search: "?section=bogus" });
    expect(readSectionFromURL()).toBe("overview");
  });

  it("writes the chosen section via pushState", () => {
    vi.stubGlobal("location", { href: "http://localhost/admin" });
    writeSectionToURL("config" as AdminSection);
    expect(pushStateSpy).toHaveBeenCalled();
    const url = pushStateSpy.mock.calls[0][2] as string;
    expect(url).toContain("section=config");
  });
});
