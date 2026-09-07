import type { ReactNode } from "react";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// The bindings call into the Go process over the Wails runtime, which does not
// exist in jsdom — so the boundary is mocked and this test asserts the
// CONTRACT: what gets sent, and what happens to a Go error on the way back.
const greet = vi.fn<(name: string) => Promise<string>>();
vi.mock("#bindings", () => ({ DemoService: { Greet: (n: string) => greet(n) } }));

const success = vi.fn();
const failure = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (m: string) => success(m), error: (m: string) => failure(m) },
}));

const { GreetForm } = await import("./greet-form.tsx");

function wrapper({ children }: { children: ReactNode }) {
  // retry: false — a rejected mutation must surface immediately, not after
  // react-query's default back-off.
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("GreetForm", () => {
  beforeEach(() => {
    greet.mockReset();
    success.mockReset();
    failure.mockReset();
  });

  it("sends the typed name to Go and shows what it returns", async () => {
    greet.mockResolvedValue("Hello Ada!");
    render(<GreetForm />, { wrapper });

    await userEvent.type(screen.getByLabelText("Your name"), "Ada");
    await userEvent.click(screen.getByRole("button", { name: "Greet" }));

    await waitFor(() => {
      expect(greet).toHaveBeenCalledWith("Ada");
    });
    expect(success).toHaveBeenCalledWith("Hello Ada!");
    expect(failure).not.toHaveBeenCalled();
  });

  // A Go `(value, error)` return becomes a REJECTED promise. If that is not
  // handled the failure is invisible in the UI and unhandled in the console.
  it("surfaces a Go error instead of rejecting", async () => {
    greet.mockRejectedValue(new Error("name must not be empty"));
    render(<GreetForm />, { wrapper });

    await userEvent.click(screen.getByRole("button", { name: "Greet" }));

    await waitFor(() => {
      expect(failure).toHaveBeenCalledWith("name must not be empty");
    });
    expect(success).not.toHaveBeenCalled();
  });
});
