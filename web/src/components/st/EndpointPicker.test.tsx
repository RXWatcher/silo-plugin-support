import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { EndpointPicker } from "./EndpointPicker";
import type { STEndpoint } from "@/lib/types";

const endpoints: STEndpoint[] = [
  { id: 1, label: "London",    url: "https://lon/", country: "GB", region: "", sortOrder: 0, active: true, createdAt: "", updatedAt: "" },
  { id: 2, label: "Frankfurt", url: "https://fra/", country: "DE", region: "", sortOrder: 1, active: true, createdAt: "", updatedAt: "" },
];

describe("EndpointPicker", () => {
  afterEach(() => cleanup());

  it("renders 'Auto' plus one option per endpoint", () => {
    render(<EndpointPicker endpoints={endpoints} value="auto" onChange={() => {}} />);
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(3);
  });

  it("calls onChange with 'auto' or the endpoint id when selected", () => {
    const onChange = vi.fn();
    render(<EndpointPicker endpoints={endpoints} value="auto" onChange={onChange} />);
    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "2" } });
    expect(onChange).toHaveBeenLastCalledWith(2);
    fireEvent.change(select, { target: { value: "auto" } });
    expect(onChange).toHaveBeenLastCalledWith("auto");
  });
});
