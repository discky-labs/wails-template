import { useState } from "react";

import { attempt } from "@local/ts-shared";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";

import { DemoService } from "#bindings";

/** Calls a bound Go method that succeeds. */
export function GreetForm() {
  const [name, setName] = useState("");

  const greet = useMutation({
    // A bound Go method returns a promise that REJECTS with the Go error's
    // message, so `attempt` turns that into a value to branch on rather than
    // an unhandled rejection.
    mutationFn: (who: string) => attempt(() => DemoService.Greet(who)),
    onSuccess: (result) => {
      if (result.ok) {
        toast.success(result.value);
      } else {
        toast.error(result.error.message);
      }
    },
  });

  return (
    <form
      className="flex gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        greet.mutate(name);
      }}
    >
      <input
        aria-label="Your name"
        className="flex-1 rounded-md border border-border bg-surface px-3 py-2 outline-none focus:border-brand"
        placeholder="Your name"
        value={name}
        onChange={(event) => setName(event.target.value)}
      />
      <button
        className="rounded-md bg-brand text-brand-foreground px-4 py-2 font-medium disabled:opacity-50"
        disabled={greet.isPending}
        type="submit"
      >
        Greet
      </button>
    </form>
  );
}
