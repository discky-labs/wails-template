import { useState } from "react";

import { attempt } from "@local/ts-shared";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";

import { DemoService } from "#bindings";

/**
 * Calls a bound Go method that can fail, so the error path is visible:
 * dividing by zero returns an error from Go, which arrives as a rejection.
 */
export function DivideForm() {
  const [a, setA] = useState("10");
  const [b, setB] = useState("0");

  const divide = useMutation({
    mutationFn: () => attempt(() => DemoService.Divide(Number(a), Number(b))),
    onSuccess: (result) => {
      if (result.ok) {
        toast.success(`${a} / ${b} = ${result.value}`);
      } else {
        toast.error(result.error.message);
      }
    },
  });

  return (
    <form
      className="flex items-center gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        divide.mutate();
      }}
    >
      <input
        aria-label="Dividend"
        className="w-20 rounded-md border border-border bg-surface px-3 py-2 outline-none focus:border-brand"
        inputMode="numeric"
        value={a}
        onChange={(event) => setA(event.target.value)}
      />
      <span className="text-muted-foreground">/</span>
      <input
        aria-label="Divisor"
        className="w-20 rounded-md border border-border bg-surface px-3 py-2 outline-none focus:border-brand"
        inputMode="numeric"
        value={b}
        onChange={(event) => setB(event.target.value)}
      />
      <button
        className="rounded-md border border-border px-4 py-2 font-medium disabled:opacity-50"
        disabled={divide.isPending}
        type="submit"
      >
        Divide
      </button>
    </form>
  );
}
