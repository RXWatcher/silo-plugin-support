import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleTogglesPanel } from "./ModuleTogglesPanel";

describe("ModuleTogglesPanel", () => {
  it("renders one row per module with the current state", () => {
    render(<ModuleTogglesPanel
      modules={{ kb: true, speedtest: false, tickets: false, ai: false }}
      onSave={async () => {}}
    />);
    const switches = screen.getAllByRole("switch");
    expect(switches).toHaveLength(4);
  });

  it("renders module labels and descriptions", () => {
    render(<ModuleTogglesPanel
      modules={{ kb: true, speedtest: false, tickets: false, ai: false }}
      onSave={async () => {}}
    />);
    expect(screen.getAllByText(/operator-authored articles/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/multi-endpoint connection/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/typed support intake/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/suggest kb articles/i).length).toBeGreaterThan(0);
  });
});
