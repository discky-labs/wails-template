import { createFileRoute } from "@tanstack/react-router";

import { DivideForm } from "#/components/divide-form.tsx";
import { GreetForm } from "#/components/greet-form.tsx";
import { env } from "#/env.ts";
import { useGoClock } from "#/hooks/use-go-clock.ts";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const time = useGoClock();

  return (
    <main className="mx-auto flex w-full max-w-xl flex-1 flex-col justify-center gap-8 p-8">
      <header className="flex items-center gap-3">
        <img src="/wails.png" alt="" className="size-10" />
        <img src="/react.svg" alt="" className="size-8" />
      </header>

      <div>
        <h1 className="text-3xl font-semibold tracking-tight">{env.VITE_APP_TITLE}</h1>
        <p className="mt-2 text-neutral-400">
          A Go backend and a React frontend in one native window.
        </p>
      </div>

      <GreetForm />
      <DivideForm />

      <footer className="text-sm text-neutral-500">
        Event from Go: <span className="font-mono">{time}</span>
      </footer>
    </main>
  );
}
