import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ReplyBox } from "./ReplyBox";

describe("ReplyBox", () => {
  afterEach(() => cleanup());

  it("disables submit when empty", () => {
    render(<ReplyBox onSubmit={async () => {}} disabled={false} />);
    expect(screen.getByRole("button", { name: /send/i })).toBeDisabled();
  });

  it("calls onSubmit with the body when submitted", () => {
    const onSubmit = vi.fn(async () => {});
    render(<ReplyBox onSubmit={onSubmit} disabled={false} />);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Hello there" } });
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(onSubmit).toHaveBeenCalledWith("Hello there");
  });

  it("does not submit when disabled prop is true", () => {
    const onSubmit = vi.fn(async () => {});
    render(<ReplyBox onSubmit={onSubmit} disabled />);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "x" } });
    expect(screen.getByRole("button", { name: /send/i })).toBeDisabled();
  });
});
