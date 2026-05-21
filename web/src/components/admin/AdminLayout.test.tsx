import { describe, expect, it, afterEach } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

  it("switches the rendered section and the ?section= URL when a sidebar entry is clicked", () => {
    history.pushState({}, "", "/admin");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    // Starts on Overview.
    expect(screen.getByRole("heading", { level: 2, name: /overview/i })).toBeInTheDocument();
    expect(window.location.search).toBe("");

    fireEvent.click(screen.getByRole("button", { name: /^configuration$/i }));

    expect(screen.getByRole("heading", { level: 2, name: /configuration/i })).toBeInTheDocument();
    expect(new URLSearchParams(window.location.search).get("section")).toBe("config");

    // Clicking Overview clears the param again (overview is the default).
    fireEvent.click(screen.getByRole("button", { name: /^overview$/i }));
    expect(screen.getByRole("heading", { level: 2, name: /overview/i })).toBeInTheDocument();
    expect(new URLSearchParams(window.location.search).get("section")).toBeNull();
  });
});
