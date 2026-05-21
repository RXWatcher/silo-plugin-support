import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { TagChips } from "./TagChips";

describe("TagChips", () => {
  afterEach(() => cleanup());

  it("renders a chip per tag plus an 'all' chip", () => {
    render(<TagChips tags={["beginner","video","mobile"]} selected="" onSelect={() => {}} />);
    expect(screen.getAllByRole("button")).toHaveLength(4);
  });

  it("calls onSelect with the slug clicked, or '' for All", () => {
    const onSelect = vi.fn();
    render(<TagChips tags={["beginner","video"]} selected="" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: /^beginner$/i }));
    expect(onSelect).toHaveBeenCalledWith("beginner");
    fireEvent.click(screen.getByRole("button", { name: /^all$/i }));
    expect(onSelect).toHaveBeenLastCalledWith("");
  });

  it("marks the selected chip as pressed", () => {
    render(<TagChips tags={["beginner","video"]} selected="video" onSelect={() => {}} />);
    const video = screen.getByRole("button", { name: /^video$/i });
    expect(video).toHaveAttribute("aria-pressed", "true");
  });
});
