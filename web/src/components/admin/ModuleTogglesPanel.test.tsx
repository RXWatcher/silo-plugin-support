import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ModuleTogglesPanel } from "./ModuleTogglesPanel";

describe("ModuleTogglesPanel", () => {
  afterEach(() => cleanup());

  it("renders one row per module with the current state", () => {
    render(<ModuleTogglesPanel
      modules={{ kb: true, speedtest: false, tickets: false, ai: false }}
      onSave={async () => {}}
    />);
    const switches = screen.getAllByRole("switch");
    expect(switches).toHaveLength(4);
  });

  it("calls onSave with a patch when a switch is toggled", () => {
    const onSave = vi.fn(async () => {});
    render(<ModuleTogglesPanel
      modules={{ kb: false, speedtest: false, tickets: false, ai: false }}
      onSave={onSave}
    />);
    // The aria-label on each Switch is `Toggle <label>`. KB is first.
    fireEvent.click(screen.getByRole("switch", { name: /toggle knowledge base/i }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith({
      modules: { kb: true, speedtest: false, tickets: false, ai: false },
    });
  });
});
