import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleCard } from "./ModuleCard";

describe("ModuleCard", () => {
  it("renders an anchor when enabled", () => {
    render(<ModuleCard title="Knowledge Base" href="./kb" enabled description="Browse articles" />);
    expect(screen.getByRole("link", { name: /knowledge base/i })).toHaveAttribute("href", "./kb");
    expect(screen.queryByText(/coming soon/i)).not.toBeInTheDocument();
  });

  it("renders a non-clickable placeholder when disabled", () => {
    const { container } = render(<ModuleCard title="Speedtest" href="./speedtest" enabled={false} description="Test your connection" />);
    expect(container.querySelector("a")).toBeNull();
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });
});
