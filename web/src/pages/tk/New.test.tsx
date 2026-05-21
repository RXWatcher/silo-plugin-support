import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { TKNew } from "./New";

// Stub the API client.
vi.mock("@/api/tk", () => ({
  getTKCategoriesForm: vi.fn(async () => ({
    categories: [
      { id: 1, slug: "billing", name: "Billing", sortOrder: 0, active: true, createdAt: "", updatedAt: "" },
    ],
    subcategories: { 1: [] },
    fields: { 1: [] },
  })),
  createTKTicket: vi.fn(async (req: { subject: string }) => ({
    id: 42, trackingNumber: "SUP-42", customerId: "u",
    customerEmail: "a@b", categoryId: 1, subject: req.subject,
    status: "open", createdAt: "", updatedAt: "",
  })),
}));

// Stub sonner so toasts don't choke.
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

describe("TKNew", () => {
  beforeEach(() => {
    // jsdom doesn't implement clipboard; stub it so the copy button renders without errors.
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn(async () => {}) },
    });
  });

  afterEach(() => cleanup());

  it("walks category → form → submit → confirmation with tracking number", async () => {
    render(<TKNew />);

    // Step 1: category picker appears after categories load
    await waitFor(() => expect(screen.getByText("Billing")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Billing"));

    // Step 2: form (no subcategories for this category, so we jump straight to form)
    const subject = await screen.findByLabelText(/subject/i);
    fireEvent.change(subject, { target: { value: "Need help" } });
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "u@example.com" } });
    fireEvent.change(screen.getByLabelText(/describe/i), { target: { value: "There's a problem." } });

    fireEvent.click(screen.getByRole("button", { name: /submit ticket/i }));

    // Step 3: confirmation shows the tracking number
    await waitFor(() => expect(screen.getByText(/SUP-42/)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /copy tracking number/i })).toBeInTheDocument();
  });
});
