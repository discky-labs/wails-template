import { createRootRoute, Outlet } from "@tanstack/react-router";
import { Toaster } from "sonner";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="flex min-h-full flex-col bg-neutral-950 text-neutral-100">
      <Outlet />
      <Toaster theme="dark" position="bottom-center" />
    </div>
  );
}
