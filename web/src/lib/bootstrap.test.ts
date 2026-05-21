import { describe, expect, it, beforeEach } from "vitest";
import { readBootstrap } from "./bootstrap";

function injectBootstrap(json: string | null) {
  document.body.replaceChildren();
  if (json !== null) {
    const s = document.createElement("script");
    s.id = "support-bootstrap";
    s.type = "application/json";
    s.textContent = json;
    document.body.appendChild(s);
  }
}

describe("readBootstrap", () => {
  beforeEach(() => injectBootstrap(null));

  it("returns customer-home defaults when no bootstrap is injected", () => {
    const bs = readBootstrap();
    expect(bs.mode).toBe("customer-home");
    expect(bs.modules.kb).toBe(false);
    expect(bs.modules.speedtest).toBe(false);
    expect(bs.userId).toBe("");
    expect(bs.isAdmin).toBe(false);
  });

  it("returns customer-home defaults when the placeholder is still present", () => {
    injectBootstrap("%SUPPORT_BOOTSTRAP%");
    const bs = readBootstrap();
    expect(bs.mode).toBe("customer-home");
  });

  it("parses an injected bootstrap and fills missing keys with defaults", () => {
    injectBootstrap(JSON.stringify({ mode: "admin-home", modules: { kb: true }, isAdmin: true }));
    const bs = readBootstrap();
    expect(bs.mode).toBe("admin-home");
    expect(bs.modules.kb).toBe(true);
    expect(bs.modules.speedtest).toBe(false);
    expect(bs.isAdmin).toBe(true);
  });
});
