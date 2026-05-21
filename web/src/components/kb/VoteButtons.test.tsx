import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { VoteButtons } from "./VoteButtons";

describe("VoteButtons", () => {
  afterEach(() => cleanup());

  it("renders unpressed when no existing vote", () => {
    render(<VoteButtons currentVote={null} onVote={() => {}} />);
    expect(screen.getByRole("button", { name: /^helpful$/i })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: /not helpful/i })).toHaveAttribute("aria-pressed", "false");
  });

  it("marks the matching button pressed", () => {
    render(<VoteButtons currentVote="up" onVote={() => {}} />);
    expect(screen.getByRole("button", { name: /^helpful$/i })).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onVote with the clicked value", () => {
    const onVote = vi.fn();
    render(<VoteButtons currentVote={null} onVote={onVote} />);
    fireEvent.click(screen.getByRole("button", { name: /^helpful$/i }));
    expect(onVote).toHaveBeenCalledWith("up");
    fireEvent.click(screen.getByRole("button", { name: /not helpful/i }));
    expect(onVote).toHaveBeenLastCalledWith("down");
  });
});
