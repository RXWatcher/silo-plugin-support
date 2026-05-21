import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleStatusCard } from "./ModuleStatusCard";

describe("ModuleStatusCard", () => {
  it("shows 'not shipped' state when shipped=false", () => {
    render(<ModuleStatusCard title="KB" shipped={false} enabled={false} manageHref="./kb" />);
    expect(screen.getAllByText(/not shipped/i)).toHaveLength(2); // paragraph + badge
    expect(screen.queryByRole("link", { name: /manage/i })).toBeNull();
  });

  it("shows 'disabled' state when shipped but not enabled", () => {
    render(<ModuleStatusCard title="KB" shipped enabled={false} manageHref="./kb" />);
    expect(screen.getAllByText(/disabled/i)).toHaveLength(2); // paragraph + badge
    expect(screen.queryByRole("link", { name: /manage/i })).toBeNull();
  });

  it("renders a Manage link when shipped and enabled", () => {
    render(<ModuleStatusCard title="KB" shipped enabled manageHref="./kb" />);
    expect(screen.getByRole("link", { name: /manage/i })).toHaveAttribute("href", "./kb");
  });
});
