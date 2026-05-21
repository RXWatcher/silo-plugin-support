import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SearchBar } from "./SearchBar";

describe("SearchBar", () => {
  afterEach(() => cleanup());

  it("debounces calls to onQuery", async () => {
    vi.useFakeTimers();
    const onQuery = vi.fn();
    render(<SearchBar onQuery={onQuery} debounceMs={250} />);
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "buf" } });
    fireEvent.change(input, { target: { value: "buffe" } });
    fireEvent.change(input, { target: { value: "buffering" } });
    expect(onQuery).not.toHaveBeenCalled();
    vi.advanceTimersByTime(260);
    expect(onQuery).toHaveBeenCalledTimes(1);
    expect(onQuery).toHaveBeenCalledWith("buffering");
    vi.useRealTimers();
  });
});
