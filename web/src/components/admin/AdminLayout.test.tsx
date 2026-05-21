import { describe, expect, it, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { AdminLayout } from "./AdminLayout";

describe("AdminLayout", () => {
  afterEach(() => cleanup());

  it("renders the Overview section by default", () => {
    history.pushState({}, "", "/admin");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { level: 2, name: /overview/i })).toBeInTheDocument();
  });

  it("renders the Configuration section when ?section=config is present", () => {
    history.pushState({}, "", "/admin?section=config");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { level: 2, name: /configuration/i })).toBeInTheDocument();
  });

  it("falls back to Overview on an unknown section value", () => {
    history.pushState({}, "", "/admin?section=bogus");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { level: 2, name: /overview/i })).toBeInTheDocument();
  });
});
